package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"zoo-go/internal/agreement"
	"zoo-go/internal/domain"
)

func (s *Store) seedAgreementVersions() error {
	for _, role := range []string{agreement.RoleOwner, agreement.RoleSitter} {
		definition, ok := agreement.Current(role)
		if !ok {
			return fmt.Errorf("unknown agreement role %s", role)
		}
		var existingHash string
		err := s.db.QueryRow(`
			SELECT content_hash FROM agreement_versions WHERE version = ? AND role = ?
		`, definition.Version, role).Scan(&existingHash)
		if err == nil {
			if existingHash != definition.ContentHash {
				return fmt.Errorf("agreement %s %s content changed without a version bump", definition.Version, role)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := s.db.Exec(`
			INSERT INTO agreement_versions(
				version, role, title, content, content_hash, required_items_json, published_at
			) VALUES(?, ?, ?, ?, ?, ?, ?)
		`, definition.Version, role, definition.Title, definition.Content, definition.ContentHash,
			marshalJSON(definition.RequiredItems), timestamp(time.Now())); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) recordAgreementAcceptanceTx(
	tx *sql.Tx,
	taskID, bidID, userID, role, version string,
	accepted []string,
	clientIP, userAgent string,
	now time.Time,
) error {
	definition, ok := agreement.Current(role)
	if !ok {
		return validation("协议角色无效")
	}
	if version != definition.Version {
		return conflict("服务协议已更新，请刷新页面后重新确认")
	}
	if err := agreement.Validate(role, version, accepted); err != nil {
		return validation("请逐项阅读并同意当前版本的服务协议")
	}
	items := agreement.NormalizedItems(role, accepted)
	_, err := tx.Exec(`
		INSERT INTO agreement_acceptances(
			id, agreement_version, role, content_hash, user_id, task_id, bid_id, order_id,
			required_items_json, client_ip, user_agent, accepted_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?)
		ON CONFLICT(task_id, role, user_id) DO UPDATE SET
			agreement_version = excluded.agreement_version,
			content_hash = excluded.content_hash,
			bid_id = excluded.bid_id,
			required_items_json = excluded.required_items_json,
			client_ip = excluded.client_ip,
			user_agent = excluded.user_agent,
			accepted_at = excluded.accepted_at
	`, newID("agree"), definition.Version, role, definition.ContentHash, userID, taskID, bidID,
		marshalJSON(items), truncateText(clientIP, 64), truncateText(userAgent, 300), timestamp(now))
	return err
}

func (s *Store) agreementAcceptedTx(tx *sql.Tx, taskID, userID, role string) (bool, error) {
	definition, ok := agreement.Current(role)
	if !ok {
		return false, validation("协议角色无效")
	}
	var count int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM agreement_acceptances
		WHERE task_id = ? AND user_id = ? AND role = ? AND agreement_version = ? AND content_hash = ?
	`, taskID, userID, role, definition.Version, definition.ContentHash).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) linkAgreementAcceptancesToOrderTx(tx *sql.Tx, taskID, orderID, ownerID, sitterID string) error {
	_, err := tx.Exec(`
		UPDATE agreement_acceptances SET order_id = ?
		WHERE task_id = ? AND (
			(role = ? AND user_id = ?) OR (role = ? AND user_id = ?)
		)
	`, orderID, taskID, agreement.RoleOwner, ownerID, agreement.RoleSitter, sitterID)
	return err
}

func (s *Store) createOrderAgreementSnapshotTx(tx *sql.Tx, taskID, orderID, ownerID, sitterID string, now time.Time) error {
	var owner, sitter domain.AgreementAcceptance
	if err := scanAgreementAcceptance(tx.QueryRow(`
		SELECT id, user_id, role, content_hash, required_items_json, accepted_at, client_ip, user_agent
		FROM agreement_acceptances
		WHERE task_id = ? AND role = ? AND user_id = ?
		ORDER BY accepted_at DESC LIMIT 1
	`, taskID, agreement.RoleOwner, ownerID), &owner); err != nil {
		return err
	}
	if err := scanAgreementAcceptance(tx.QueryRow(`
		SELECT id, user_id, role, content_hash, required_items_json, accepted_at, client_ip, user_agent
		FROM agreement_acceptances
		WHERE task_id = ? AND role = ? AND user_id = ?
		ORDER BY accepted_at DESC LIMIT 1
	`, taskID, agreement.RoleSitter, sitterID), &sitter); err != nil {
		return err
	}
	var version, title, content, contentHash string
	if err := tx.QueryRow(`
		SELECT version, title, content, content_hash FROM agreement_versions WHERE version = ? AND role = ?
	`, currentAgreementVersion(), agreement.RoleOwner).Scan(&version, &title, &content, &contentHash); err != nil {
		return err
	}
	ownerJSON, err := json.Marshal(owner)
	if err != nil {
		return err
	}
	sitterJSON, err := json.Marshal(sitter)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO order_agreement_snapshots(
			order_id, agreement_version, title, content, content_hash, owner_json, sitter_json, created_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
	`, orderID, version, title, content, contentHash, string(ownerJSON), string(sitterJSON), timestamp(now))
	return err
}

func scanAgreementAcceptance(scanner interface{ Scan(...any) error }, acceptance *domain.AgreementAcceptance) error {
	var itemsJSON, acceptedAt string
	if err := scanner.Scan(&acceptance.ID, &acceptance.UserID, &acceptance.Role, &acceptance.ContentHash,
		&itemsJSON, &acceptedAt, &acceptance.ClientIP, &acceptance.UserAgent); err != nil {
		return err
	}
	acceptance.RequiredItems = decodeStrings(itemsJSON)
	acceptance.AcceptedAt = parseTimestamp(acceptedAt)
	return nil
}

func (s *Store) OrderAgreement(orderID, userID string) (domain.OrderAgreementSnapshot, error) {
	var ownerID, studentID string
	err := s.db.QueryRow(`SELECT owner_id, student_id FROM orders WHERE id = ?`, orderID).Scan(&ownerID, &studentID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OrderAgreementSnapshot{}, notFound("订单不存在")
	}
	if err != nil {
		return domain.OrderAgreementSnapshot{}, err
	}
	if userID != ownerID && userID != studentID {
		return domain.OrderAgreementSnapshot{}, forbidden("无权查看该订单协议")
	}
	var snapshot domain.OrderAgreementSnapshot
	var ownerJSON, sitterJSON string
	err = s.db.QueryRow(`
		SELECT agreement_version, title, content, content_hash, owner_json, sitter_json
		FROM order_agreement_snapshots WHERE order_id = ?
	`, orderID).Scan(&snapshot.Version, &snapshot.Title, &snapshot.Content, &snapshot.ContentHash, &ownerJSON, &sitterJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OrderAgreementSnapshot{}, conflict("订单缺少协议签署快照")
	}
	if err != nil {
		return domain.OrderAgreementSnapshot{}, err
	}
	if err := json.Unmarshal([]byte(ownerJSON), &snapshot.Owner); err != nil {
		return domain.OrderAgreementSnapshot{}, err
	}
	if err := json.Unmarshal([]byte(sitterJSON), &snapshot.Sitter); err != nil {
		return domain.OrderAgreementSnapshot{}, err
	}
	return snapshot, nil
}

func currentAgreementVersion() string {
	definition, _ := agreement.Current(agreement.RoleOwner)
	return definition.Version
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
