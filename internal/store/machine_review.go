package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"zoo-go/internal/domain"
	"zoo-go/internal/moderation"
)

type MachineVerdict string

const (
	MachinePass    MachineVerdict = "PASS"
	MachineReview  MachineVerdict = "REVIEW"
	MachineRisky   MachineVerdict = "RISKY"
	MachineUnknown MachineVerdict = "UNKNOWN"
)

const (
	mediaReviewPending = "PENDING"
	mediaReviewPass    = "PASS"
	mediaReviewReview  = "REVIEW"
	mediaReviewRisky   = "RISKY"
	mediaReviewError   = "ERROR"
	mediaReviewExpired = "EXPIRED"
)

var ErrMediaReviewNotFound = errors.New("media review trace not found")

type MachineImageSubmission struct {
	TraceID string
	Path    string
}

func (s *Store) ApplyMachineTextReview(postID string, verdict MachineVerdict, reasons []string, now time.Time) (domain.CommunityPost, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.CommunityPost{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var authorID, content, imagesJSON, currentStatus, currentRisk, currentReason string
	err = tx.QueryRow(`
		SELECT author_id, content, images_json, moderation_status, moderation_risk, moderation_reason
		FROM community_posts WHERE id = ?
	`, postID).Scan(&authorID, &content, &imagesJSON, &currentStatus, &currentRisk, &currentReason)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CommunityPost{}, false, notFound("动态不存在")
	}
	if err != nil {
		return domain.CommunityPost{}, false, err
	}
	if currentStatus != domain.PostPending {
		post, err := s.communityPostByID(postID, authorID)
		return post, false, err
	}

	images := decodeStrings(imagesJSON)
	local := s.moderation.ReviewLocal(content, 0)
	status := currentStatus
	risk := currentRisk
	reason := currentReason
	needsImages := false
	switch verdict {
	case MachineRisky:
		status = domain.PostRejected
		risk = string(moderation.LevelHigh)
		reason = machineReason(reasons, "文字机器审核未通过")
	case MachinePass:
		if local.Action == moderation.ActionReject {
			status = domain.PostRejected
			risk = string(moderation.LevelHigh)
			reason = strings.Join(local.Reasons, "；")
		} else if local.Action == moderation.ActionReview {
			status = domain.PostPending
			risk = higherRisk(risk, string(local.Level))
			reason = strings.Join(local.Reasons, "；")
		} else if len(images) > 0 {
			status = domain.PostPending
			risk = higherRisk(risk, string(moderation.LevelMedium))
			reason = "图片机器审核提交中"
			needsImages = true
		} else if s.moderation.AutoApproveText {
			status = domain.PostApproved
			risk = string(moderation.LevelLow)
			reason = ""
		} else {
			status = domain.PostPending
			risk = higherRisk(risk, string(moderation.LevelMedium))
			reason = "文字机审通过，等待人工复核"
		}
	default:
		status = domain.PostPending
		risk = higherRisk(risk, string(moderation.LevelMedium))
		reason = machineReason(reasons, "机器审核结果不确定，转人工复核")
	}
	if _, err := tx.Exec(`
		UPDATE community_posts
		SET moderation_status = ?, moderation_reason = ?, moderation_risk = ?
		WHERE id = ?
	`, status, reason, risk, postID); err != nil {
		return domain.CommunityPost{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CommunityPost{}, false, err
	}
	post, err := s.communityPostByID(postID, authorID)
	return post, needsImages, err
}

func (s *Store) MarkMachineReviewing(postID string, submissions []MachineImageSubmission, now, expiresAt time.Time) (domain.CommunityPost, error) {
	if len(submissions) == 0 {
		return s.communityPostByID(postID, "")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.CommunityPost{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var authorID, imagesJSON, status string
	err = tx.QueryRow(`
		SELECT author_id, images_json, moderation_status FROM community_posts WHERE id = ?
	`, postID).Scan(&authorID, &imagesJSON, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CommunityPost{}, notFound("动态不存在")
	}
	if err != nil {
		return domain.CommunityPost{}, err
	}
	if status != domain.PostPending && status != domain.PostMachineReviewing {
		post, err := s.communityPostByID(postID, authorID)
		return post, err
	}
	images := map[string]bool{}
	for _, image := range decodeStrings(imagesJSON) {
		images[image] = true
	}
	for _, submission := range submissions {
		submission.TraceID = strings.TrimSpace(submission.TraceID)
		submission.Path = strings.TrimSpace(submission.Path)
		if submission.TraceID == "" || !images[submission.Path] {
			return domain.CommunityPost{}, validation("图片机器审核记录无效")
		}
		if _, err := tx.Exec(`
			INSERT INTO community_media_reviews(
				id, post_id, trace_id, image_path, status, created_at, updated_at, expires_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		`, newID("media_review"), postID, submission.TraceID, submission.Path, mediaReviewPending,
			timestamp(now), timestamp(now), timestamp(expiresAt)); err != nil {
			return domain.CommunityPost{}, err
		}
	}
	if _, err := tx.Exec(`
		UPDATE community_posts
		SET moderation_status = ?, moderation_reason = ?, moderation_risk = ?
		WHERE id = ?
	`, domain.PostMachineReviewing, "图片机器审核中", string(moderation.LevelMedium), postID); err != nil {
		return domain.CommunityPost{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CommunityPost{}, err
	}
	return s.communityPostByID(postID, authorID)
}

func (s *Store) HandleMachineMediaResult(traceID string, verdict MachineVerdict, label, errcode int, errmsg string, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var reviewID, postID, reviewStatus, expiresAt string
	err = tx.QueryRow(`
		SELECT id, post_id, status, expires_at FROM community_media_reviews WHERE trace_id = ?
	`, traceID).Scan(&reviewID, &postID, &reviewStatus, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMediaReviewNotFound
	}
	if err != nil {
		return err
	}
	if reviewStatus == mediaReviewPending && !now.Before(parseTimestamp(expiresAt)) {
		if _, err := tx.Exec(`
			UPDATE community_media_reviews
			SET status = ?, updated_at = ?
			WHERE id = ? AND status = ?
		`, mediaReviewExpired, timestamp(now), reviewID, mediaReviewPending); err != nil {
			return err
		}
		if err := reconcileMachinePost(tx, postID, now); err != nil {
			return err
		}
		return tx.Commit()
	}
	if reviewStatus != mediaReviewPending {
		if err := reconcileMachinePost(tx, postID, now); err != nil {
			return err
		}
		return tx.Commit()
	}
	status := mediaReviewReview
	switch verdict {
	case MachinePass:
		status = mediaReviewPass
	case MachineRisky:
		status = mediaReviewRisky
	case MachineReview:
		status = mediaReviewReview
	default:
		status = mediaReviewError
	}
	if _, err := tx.Exec(`
		UPDATE community_media_reviews
		SET status = ?, suggest = ?, label = ?, errcode = ?, errmsg = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, status, string(verdict), label, errcode, strings.TrimSpace(errmsg), timestamp(now), reviewID, mediaReviewPending); err != nil {
		return err
	}
	if err := reconcileMachinePost(tx, postID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ExpireMachineReviews(now time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.Query(`
		SELECT DISTINCT post_id FROM community_media_reviews
		WHERE status = ? AND expires_at <= ?
	`, mediaReviewPending, timestamp(now))
	if err != nil {
		return 0, err
	}
	postIDs := []string{}
	for rows.Next() {
		var postID string
		if err := rows.Scan(&postID); err != nil {
			rows.Close()
			return 0, err
		}
		postIDs = append(postIDs, postID)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	result, err := tx.Exec(`
		UPDATE community_media_reviews
		SET status = ?, updated_at = ?
		WHERE status = ? AND expires_at <= ?
	`, mediaReviewExpired, timestamp(now), mediaReviewPending, timestamp(now))
	if err != nil {
		return 0, err
	}
	for _, postID := range postIDs {
		if err := reconcileMachinePost(tx, postID, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func reconcileMachinePost(tx *sql.Tx, postID string, now time.Time) error {
	var currentStatus string
	if err := tx.QueryRow(`SELECT moderation_status FROM community_posts WHERE id = ?`, postID).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if currentStatus != domain.PostMachineReviewing {
		return nil
	}
	rows, err := tx.Query(`
		SELECT status, COUNT(*) FROM community_media_reviews WHERE post_id = ? GROUP BY status
	`, postID)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return err
		}
		counts[status] = count
	}
	if err := rows.Err(); err != nil {
		return err
	}
	status := domain.PostMachineReviewing
	risk := string(moderation.LevelMedium)
	reason := "图片机器审核中"
	reviewedBy := ""
	reviewedAt := ""
	switch {
	case counts[mediaReviewRisky] > 0:
		status = domain.PostRejected
		risk = string(moderation.LevelHigh)
		reason = "图片机器审核未通过"
		reviewedBy = "machine"
		reviewedAt = timestamp(now)
	case counts[mediaReviewPending] > 0:
		status = domain.PostMachineReviewing
	case counts[mediaReviewReview] > 0 || counts[mediaReviewError] > 0 || counts[mediaReviewExpired] > 0:
		status = domain.PostPending
		reason = "图片机器审核结果不确定，转人工复核"
	case counts[mediaReviewPass] > 0:
		status = domain.PostApproved
		risk = string(moderation.LevelLow)
		reason = ""
		reviewedBy = "machine"
		reviewedAt = timestamp(now)
	}
	_, err = tx.Exec(`
		UPDATE community_posts
		SET moderation_status = ?, moderation_reason = ?, moderation_risk = ?,
		    reviewed_by = ?, reviewed_at = ?
		WHERE id = ? AND moderation_status IN (?, ?)
	`, status, reason, risk, reviewedBy, reviewedAt, postID, domain.PostPending, domain.PostMachineReviewing)
	return err
}

func (s *Store) communityPostByID(postID, viewerID string) (domain.CommunityPost, error) {
	row := s.db.QueryRow(`
		SELECT p.id, p.author_id, u.name, u.avatar, u.owner_verified, u.student_verified,
		       p.content, p.images_json, p.moderation_status, p.moderation_reason,
		       p.moderation_risk, p.review_note, p.reviewed_at, p.created_at
		FROM community_posts p JOIN users u ON u.id = p.author_id
		WHERE p.id = ?
	`, postID)
	var post domain.CommunityPost
	var imagesJSON, reviewedAt, createdAt string
	var ownerVerified, studentVerified int
	if err := row.Scan(&post.ID, &post.Author.ID, &post.Author.Name, &post.Author.Avatar,
		&ownerVerified, &studentVerified, &post.Content, &imagesJSON,
		&post.ModerationStatus, &post.ModerationReason, &post.RiskLevel,
		&post.ReviewNote, &reviewedAt, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.CommunityPost{}, notFound("动态不存在")
		}
		return domain.CommunityPost{}, err
	}
	post.Author.OwnerVerified = ownerVerified == 1
	post.Author.StudentVerified = studentVerified == 1
	post.Images = decodeStrings(imagesJSON)
	post.ReviewedAt = nullableTimestamp(sql.NullString{String: reviewedAt, Valid: reviewedAt != ""})
	post.CreatedAt = parseTimestamp(createdAt)
	post.Mine = viewerID == "" || post.Author.ID == viewerID
	if post.ModerationStatus == domain.PostApproved || !post.Mine {
		post.ModerationReason = ""
		post.RiskLevel = ""
		post.ReviewNote = ""
		post.ReviewedAt = nil
	}
	return post, nil
}

func machineReason(reasons []string, fallback string) string {
	for _, reason := range reasons {
		if reason = strings.TrimSpace(reason); reason != "" {
			return reason
		}
	}
	return fallback
}

func higherRisk(current, candidate string) string {
	rank := func(value string) int {
		switch value {
		case string(moderation.LevelHigh):
			return 3
		case string(moderation.LevelMedium):
			return 2
		case string(moderation.LevelLow):
			return 1
		default:
			return 0
		}
	}
	if rank(candidate) > rank(current) {
		return candidate
	}
	return current
}
