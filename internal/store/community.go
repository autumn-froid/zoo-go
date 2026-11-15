package store

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"zoo-go/internal/domain"
	"zoo-go/internal/moderation"
)

func (s *Store) communitySummary(viewerID string, limit int) (domain.CommunitySummary, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM community_posts WHERE moderation_status = ?`, domain.PostApproved).Scan(&total); err != nil {
		return domain.CommunitySummary{}, err
	}
	feed, err := s.communityPosts(viewerID, "", limit, false)
	if err != nil {
		return domain.CommunitySummary{}, err
	}
	return domain.CommunitySummary{Total: total, Latest: feed.Items}, nil
}

func (s *Store) CommunityPosts(viewerID, cursor string, limit int) (domain.CommunityFeed, error) {
	return s.communityPosts(viewerID, cursor, limit, true)
}

func (s *Store) communityPosts(viewerID, cursor string, limit int, includeOwn bool) (domain.CommunityFeed, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	args := make([]any, 0, 6)
	where := "WHERE (p.moderation_status = ?"
	args = append(args, domain.PostApproved)
	if includeOwn {
		where += " OR p.author_id = ?"
		args = append(args, viewerID)
	}
	where += ")"
	if cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return domain.CommunityFeed{}, validation("分页游标无效")
		}
		parts := strings.SplitN(string(decoded), "|", 2)
		if len(parts) != 2 {
			return domain.CommunityFeed{}, validation("分页游标无效")
		}
		where += " AND (p.created_at < ? OR (p.created_at = ? AND p.id < ?))"
		args = append(args, parts[0], parts[0], parts[1])
	}
	args = append(args, limit+1)
	rows, err := s.db.Query(`
		SELECT p.id, p.author_id, u.name, u.avatar, u.owner_verified, u.student_verified,
		       p.content, p.images_json, p.moderation_status, p.moderation_reason,
		       p.moderation_risk, p.review_note, p.reviewed_at, p.created_at
		FROM community_posts p JOIN users u ON u.id = p.author_id
		`+where+`
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return domain.CommunityFeed{}, err
	}
	defer rows.Close()
	items := []domain.CommunityPost{}
	for rows.Next() {
		var post domain.CommunityPost
		var imagesJSON, reviewedAt, createdAt string
		var ownerVerified, studentVerified int
		if err := rows.Scan(&post.ID, &post.Author.ID, &post.Author.Name, &post.Author.Avatar,
			&ownerVerified, &studentVerified, &post.Content, &imagesJSON,
			&post.ModerationStatus, &post.ModerationReason, &post.RiskLevel,
			&post.ReviewNote, &reviewedAt, &createdAt); err != nil {
			return domain.CommunityFeed{}, err
		}
		post.Author.OwnerVerified = ownerVerified == 1
		post.Author.StudentVerified = studentVerified == 1
		post.Images = decodeStrings(imagesJSON)
		post.ReviewedAt = nullableTimestamp(sql.NullString{String: reviewedAt, Valid: reviewedAt != ""})
		post.CreatedAt = parseTimestamp(createdAt)
		post.Mine = post.Author.ID == viewerID
		if post.ModerationStatus == domain.PostApproved || !post.Mine {
			post.ModerationReason = ""
			post.RiskLevel = ""
			post.ReviewNote = ""
			post.ReviewedAt = nil
		}
		items = append(items, post)
	}
	if err := rows.Err(); err != nil {
		return domain.CommunityFeed{}, err
	}
	feed := domain.CommunityFeed{Items: items}
	if len(items) > limit {
		feed.HasMore = true
		feed.Items = items[:limit]
		last := feed.Items[len(feed.Items)-1]
		feed.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(timestamp(last.CreatedAt) + "|" + last.ID))
	}
	return feed, nil
}

func (s *Store) CreateCommunityPost(authorID string, req domain.CreatePostRequest, now time.Time) (domain.CommunityPost, error) {
	user, err := s.userByID(authorID)
	if err != nil {
		return domain.CommunityPost{}, err
	}
	if err := s.enforceCommunityPostLimit(authorID, now); err != nil {
		return domain.CommunityPost{}, err
	}
	content := strings.TrimSpace(req.Content)
	images := nonEmptyStrings(req.Images)
	if content == "" && len(images) == 0 {
		return domain.CommunityPost{}, validation("动态内容和图片至少填写一项")
	}
	if len([]rune(content)) > 500 {
		return domain.CommunityPost{}, validation("动态内容不能超过 500 字")
	}
	if len(images) > 9 {
		return domain.CommunityPost{}, validation("每条动态最多 9 张图片")
	}
	seen := map[string]bool{}
	uniqueImages := images[:0]
	for _, image := range images {
		canonical, ok := canonicalUploadPath(image)
		if !ok {
			return domain.CommunityPost{}, validation("图片必须先上传到平台")
		}
		filename := strings.TrimPrefix(canonical, "/uploads/")
		ownerID, err := s.UploadOwner(filename)
		if err != nil {
			return domain.CommunityPost{}, err
		}
		if ownerID != authorID {
			return domain.CommunityPost{}, validation("图片必须由当前用户上传")
		}
		if !seen[canonical] {
			seen[canonical] = true
			uniqueImages = append(uniqueImages, canonical)
		}
	}
	images = uniqueImages
	review := s.moderation.Review(content, len(images))
	if review.Action == moderation.ActionReject {
		return domain.CommunityPost{}, validation("内容包含平台禁止的信息，请修改后重试")
	}
	status := domain.PostPending
	reason := strings.Join(review.Reasons, "；")
	if review.Action == moderation.ActionApprove {
		status = domain.PostApproved
		reason = ""
	}
	post := domain.CommunityPost{
		ID: newID("post"),
		Author: domain.CommunityAuthor{
			ID: user.ID, Name: user.Name, Avatar: user.Avatar,
			OwnerVerified: user.OwnerVerified, StudentVerified: user.StudentVerified,
		},
		Content: content, Images: images, ModerationStatus: status,
		ModerationReason: reason, RiskLevel: string(review.Level), CreatedAt: now, Mine: true,
	}
	if _, err := s.db.Exec(`
		INSERT INTO community_posts(
			id, author_id, content, images_json, moderation_status,
			moderation_reason, moderation_risk, created_at
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
	`, post.ID, authorID, post.Content, marshalJSON(post.Images), post.ModerationStatus,
		post.ModerationReason, post.RiskLevel, timestamp(now)); err != nil {
		return domain.CommunityPost{}, err
	}
	return post, nil
}

func (s *Store) AdminCommunityPosts(status string, limit int) ([]domain.AdminCommunityPost, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	if status == "ALL" {
		status = ""
	}
	if status != "" && status != domain.PostPending && status != domain.PostApproved && status != domain.PostRejected {
		return nil, validation("动态审核状态筛选无效")
	}
	order := "p.created_at DESC, p.id DESC"
	if status == domain.PostPending {
		order = `CASE p.moderation_risk
			WHEN 'HIGH' THEN 0 WHEN 'MEDIUM' THEN 1 WHEN 'LOW' THEN 2 ELSE 3 END ASC,
			p.created_at ASC, p.id ASC`
	}
	rows, err := s.db.Query(`
		SELECT p.id, p.author_id, u.name, u.avatar, p.content, p.images_json,
		       p.moderation_status, p.moderation_risk, p.moderation_reason,
		       p.review_note, p.created_at, p.reviewed_at
		FROM community_posts p JOIN users u ON u.id = p.author_id
		WHERE (? = '' OR p.moderation_status = ?)
		ORDER BY `+order+`
		LIMIT ?
	`, status, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.AdminCommunityPost{}
	for rows.Next() {
		var item domain.AdminCommunityPost
		var imagesJSON, createdAt, reviewedAt string
		if err := rows.Scan(&item.ID, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar,
			&item.Content, &imagesJSON, &item.ModerationStatus, &item.RiskLevel,
			&item.ModerationReason, &item.ReviewNote, &createdAt, &reviewedAt); err != nil {
			return nil, err
		}
		item.Images = decodeStrings(imagesJSON)
		item.CreatedAt = parseTimestamp(createdAt)
		item.ReviewedAt = nullableTimestamp(sql.NullString{String: reviewedAt, Valid: reviewedAt != ""})
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ApproveCommunityPost(id string, now time.Time) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return validation("动态 ID 无效")
	}
	result, err := s.db.Exec(`
		UPDATE community_posts
		SET moderation_status = ?, review_note = '', reviewed_by = 'admin', reviewed_at = ?
		WHERE id = ?
	`, domain.PostApproved, timestamp(now), id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return notFound("动态不存在")
	}
	return nil
}

func (s *Store) RejectCommunityPost(id, reason string, now time.Time) error {
	id = strings.TrimSpace(id)
	reason = strings.TrimSpace(reason)
	if id == "" {
		return validation("动态 ID 无效")
	}
	if reason == "" {
		return validation("请填写驳回原因")
	}
	if len([]rune(reason)) > 200 {
		return validation("驳回原因不能超过 200 字")
	}
	result, err := s.db.Exec(`
		UPDATE community_posts
		SET moderation_status = ?, review_note = ?, reviewed_by = 'admin', reviewed_at = ?
		WHERE id = ?
	`, domain.PostRejected, reason, timestamp(now), id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return notFound("动态不存在")
	}
	return nil
}

func (s *Store) RecordUpload(ownerID, filename, originalName, mimeType string, size int64, now time.Time) error {
	if err := s.enforceUploadLimit(ownerID, now); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO uploads(id, owner_id, filename, original_name, mime_type, size_bytes, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`, newID("upload"), ownerID, filename, originalName, mimeType, size, timestamp(now))
	return err
}

func (s *Store) UploadOwner(filename string) (string, error) {
	var ownerID string
	err := s.db.QueryRow(`SELECT owner_id FROM uploads WHERE filename = ?`, filename).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", notFound("上传文件不存在")
	}
	return ownerID, err
}
