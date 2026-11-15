package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"zoo-go/internal/domain"
)

const (
	newUserWindow       = 7 * 24 * time.Hour
	newUserDailyPosts   = 3
	dailyPostLimit      = 10
	dailyUploadLimit    = 30
	ugcDayOffsetSeconds = 8 * 60 * 60
)

var chinaTimeZone = time.FixedZone("Asia/Shanghai", ugcDayOffsetSeconds)

func ugcDayBounds(now time.Time) (time.Time, time.Time) {
	local := now.In(chinaTimeZone)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, chinaTimeZone)
	return start.UTC(), start.AddDate(0, 0, 1).UTC()
}

func (s *Store) enforceCommunityPostLimit(authorID string, now time.Time) error {
	var accountStatus, createdAt string
	err := s.db.QueryRow(`SELECT account_status, created_at FROM users WHERE id = ?`, authorID).Scan(&accountStatus, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("用户不存在")
	}
	if err != nil {
		return err
	}
	if accountStatus == domain.AccountDisabled {
		return forbidden("账号已被停用，无法发布动态")
	}

	limit := dailyPostLimit
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil || now.Sub(created) < newUserWindow {
		limit = newUserDailyPosts
	}
	start, end := ugcDayBounds(now)
	var count int
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM community_posts
		WHERE author_id = ? AND created_at >= ? AND created_at < ?
	`, authorID, timestamp(start), timestamp(end)).Scan(&count); err != nil {
		return err
	}
	if count >= limit {
		if limit == newUserDailyPosts {
			return rateLimited(fmt.Sprintf("新用户每天最多发布 %d 条动态，请明天再试", limit))
		}
		return rateLimited(fmt.Sprintf("每天最多发布 %d 条动态，请明天再试", limit))
	}
	return nil
}

func (s *Store) enforceUploadLimit(ownerID string, now time.Time) error {
	var accountStatus string
	err := s.db.QueryRow(`SELECT account_status FROM users WHERE id = ?`, ownerID).Scan(&accountStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("用户不存在")
	}
	if err != nil {
		return err
	}
	if accountStatus == domain.AccountDisabled {
		return forbidden("账号已被停用，无法上传图片")
	}
	start, end := ugcDayBounds(now)
	var count int
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM uploads
		WHERE owner_id = ? AND created_at >= ? AND created_at < ?
	`, ownerID, timestamp(start), timestamp(end)).Scan(&count); err != nil {
		return err
	}
	if count >= dailyUploadLimit {
		return rateLimited(fmt.Sprintf("每天最多上传 %d 张图片，请明天再试", dailyUploadLimit))
	}
	return nil
}
