package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"zoo-go/internal/agreement"
	"zoo-go/internal/domain"
	"zoo-go/internal/security"
)

func (s *Store) SubmitIdentityVerification(userID string, req domain.SubmitIdentityVerificationRequest, now time.Time) (domain.IdentityVerification, error) {
	if _, err := s.userByID(userID); err != nil {
		return domain.IdentityVerification{}, err
	}
	req.RealName = strings.TrimSpace(req.RealName)
	if len([]rune(req.RealName)) < 2 || len([]rune(req.RealName)) > 40 {
		return domain.IdentityVerification{}, validation("请填写真实姓名")
	}
	idNumber, err := security.NormalizeIDNumber(req.IDNumber)
	if err != nil {
		return domain.IdentityVerification{}, validation("请填写有效的 18 位身份证件号码")
	}
	if !req.Consent || strings.TrimSpace(req.ConsentVersion) != agreement.IdentityConsentVersion {
		return domain.IdentityVerification{}, validation("请阅读并同意当前版本的实名核验授权说明")
	}
	var status string
	err = s.db.QueryRow(`
		SELECT status FROM identity_verifications WHERE user_id = ?
		ORDER BY submitted_at DESC, id DESC LIMIT 1
	`, userID).Scan(&status)
	if err == nil {
		switch status {
		case domain.IdentityPending:
			return domain.IdentityVerification{}, conflict("实名资料正在审核，请勿重复提交")
		case domain.IdentityVerified:
			return domain.IdentityVerification{}, conflict("该账号已完成实名认证")
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return domain.IdentityVerification{}, err
	}
	encryptedName, err := s.encryptPII(piiScopeIdentityName, req.RealName)
	if err != nil {
		return domain.IdentityVerification{}, err
	}
	encryptedNumber, err := s.encryptPII(piiScopeIdentityID, idNumber)
	if err != nil {
		return domain.IdentityVerification{}, err
	}
	nowText := timestamp(now)
	verificationID := newID("identity")
	_, err = s.db.Exec(`
		INSERT INTO identity_verifications(
			id, user_id, real_name_enc, id_number_enc, id_number_hash, id_number_mask,
			consent_version, consent_hash, status, provider, client_ip, user_agent,
			submitted_at, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', 'MANUAL_REVIEW', ?, ?, ?, ?, ?)
	`, verificationID, userID, encryptedName, encryptedNumber, s.lookupPII(piiScopeIdentityID, idNumber),
		security.MaskIDNumber(idNumber), agreement.IdentityConsentVersion, agreement.IdentityConsentHash(),
		truncateText(req.ClientIP, 64), truncateText(req.UserAgent, 300), nowText, nowText, nowText)
	if err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "id_number_hash") {
			return domain.IdentityVerification{}, conflict("该身份证件号码已被其他账号使用")
		}
		if strings.Contains(message, "pending") || strings.Contains(message, "unique") {
			return domain.IdentityVerification{}, conflict("实名资料正在审核，请勿重复提交")
		}
		return domain.IdentityVerification{}, err
	}
	return s.identityVerificationByID(verificationID, false)
}

func (s *Store) IdentityVerificationForUser(userID string) (domain.IdentityVerification, error) {
	var verificationID string
	err := s.db.QueryRow(`
		SELECT id FROM identity_verifications WHERE user_id = ?
		ORDER BY submitted_at DESC, id DESC LIMIT 1
	`, userID).Scan(&verificationID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.IdentityVerification{UserID: userID, Status: domain.IdentityUnverified}, nil
	}
	if err != nil {
		return domain.IdentityVerification{}, err
	}
	return s.identityVerificationByID(verificationID, false)
}

func (s *Store) IdentityVerificationApplications(status string) ([]domain.IdentityVerification, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status == "" {
		status = domain.IdentityPending
	}
	if status != "ALL" && status != domain.IdentityPending && status != domain.IdentityVerified && status != domain.IdentityRejected {
		return nil, validation("实名认证状态无效")
	}
	query := `
		SELECT v.id FROM identity_verifications v
		JOIN users u ON u.id = v.user_id
	`
	args := []any{}
	if status != "ALL" {
		query += ` WHERE v.status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY v.submitted_at ASC, v.id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	verifications := make([]domain.IdentityVerification, 0, len(ids))
	for _, id := range ids {
		verification, err := s.identityVerificationByID(id, true)
		if err != nil {
			return nil, err
		}
		verifications = append(verifications, verification)
	}
	return verifications, nil
}

func (s *Store) ApproveIdentityVerification(verificationID, reviewer string, now time.Time) error {
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" {
		reviewer = "admin"
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var userID, status string
	err = tx.QueryRow(`
		SELECT user_id, status FROM identity_verifications WHERE id = ?
	`, verificationID).Scan(&userID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("实名认证申请不存在")
	}
	if err != nil {
		return err
	}
	if status != domain.IdentityPending {
		return conflict("该实名认证申请已处理")
	}
	nowText := timestamp(now)
	result, err := tx.Exec(`
		UPDATE identity_verifications SET status = 'VERIFIED', review_note = '', reviewed_by = ?,
		       reviewed_at = ?, verified_at = ?, updated_at = ?
		WHERE id = ? AND status = 'PENDING'
	`, reviewer, nowText, nowText, nowText, verificationID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return conflict("该实名认证申请已处理")
	}
	if _, err := tx.Exec(`
		UPDATE users SET owner_verified = 1, updated_at = ? WHERE id = ?
	`, nowText, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RejectIdentityVerification(verificationID, reviewer, reason string, now time.Time) error {
	reason = strings.TrimSpace(reason)
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" {
		reviewer = "admin"
	}
	if reason == "" || len([]rune(reason)) > 300 {
		return validation("请填写 300 字以内的驳回原因")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	err = tx.QueryRow(`SELECT status FROM identity_verifications WHERE id = ?`, verificationID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("实名认证申请不存在")
	}
	if err != nil {
		return err
	}
	if status != domain.IdentityPending {
		return conflict("该实名认证申请已处理")
	}
	nowText := timestamp(now)
	result, err := tx.Exec(`
		UPDATE identity_verifications SET status = 'REJECTED', review_note = ?, reviewed_by = ?,
		       reviewed_at = ?, updated_at = ?
		WHERE id = ? AND status = 'PENDING'
	`, reason, reviewer, nowText, nowText, verificationID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return conflict("该实名认证申请已处理")
	}
	return tx.Commit()
}

func (s *Store) identityVerificationByID(verificationID string, revealFull bool) (domain.IdentityVerification, error) {
	var verification domain.IdentityVerification
	var realName, idNumber, submittedAt, reviewedAt, verifiedAt string
	err := s.db.QueryRow(`
		SELECT v.id, v.user_id, u.name, v.real_name_enc, v.id_number_enc, v.id_number_mask,
		       v.provider, v.status, v.review_note, v.submitted_at, v.reviewed_at, v.verified_at
		FROM identity_verifications v JOIN users u ON u.id = v.user_id
		WHERE v.id = ?
	`, verificationID).Scan(
		&verification.ID, &verification.UserID, &verification.UserName, &realName, &idNumber,
		&verification.IDNumberMasked, &verification.Provider, &verification.Status,
		&verification.ReviewNote, &submittedAt, &reviewedAt, &verifiedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.IdentityVerification{}, notFound("实名认证申请不存在")
	}
	if err != nil {
		return domain.IdentityVerification{}, err
	}
	if verification.RealName, err = s.decryptPII(piiScopeIdentityName, realName); err != nil {
		return domain.IdentityVerification{}, err
	}
	decryptedNumber, err := s.decryptPII(piiScopeIdentityID, idNumber)
	if err != nil {
		return domain.IdentityVerification{}, err
	}
	if revealFull {
		verification.IDNumber = decryptedNumber
	}
	verification.SubmittedAt = parseTimestamp(submittedAt)
	verification.ReviewedAt = nullableTimestamp(sql.NullString{String: reviewedAt, Valid: reviewedAt != ""})
	verification.VerifiedAt = nullableTimestamp(sql.NullString{String: verifiedAt, Valid: verifiedAt != ""})
	return verification, nil
}
