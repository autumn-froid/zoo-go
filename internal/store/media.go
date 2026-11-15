package store

import (
	"database/sql"
	"errors"
	"net/url"
	"path"
	"strings"
)

func canonicalUploadPath(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Fragment != "" {
		return "", false
	}
	if !strings.HasPrefix(parsed.Path, "/uploads/") {
		return "", false
	}
	filename := strings.TrimPrefix(parsed.Path, "/uploads/")
	if filename == "" || filename == "." || filename == ".." ||
		filename != path.Base(filename) || strings.ContainsAny(filename, `/\`+"\x00") {
		return "", false
	}
	return "/uploads/" + filename, true
}

func normalizeMediaReference(value string) string {
	value = strings.TrimSpace(value)
	if canonical, ok := canonicalUploadPath(value); ok {
		return canonical
	}
	return value
}

func (s *Store) ensureUploadOwner(value, ownerID string) (string, error) {
	return s.ensureUploadOwnerWith(s.db, value, ownerID)
}

func (s *Store) ensureUploadOwnerWith(queryer rowQuerier, value, ownerID string) (string, error) {
	canonical, ok := canonicalUploadPath(value)
	if !ok {
		return "", validation("图片必须先上传到平台")
	}
	filename := strings.TrimPrefix(canonical, "/uploads/")
	var uploadOwnerID string
	if err := queryer.QueryRow(`SELECT owner_id FROM uploads WHERE filename = ?`, filename).Scan(&uploadOwnerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", notFound("上传文件不存在")
		}
		return "", err
	}
	if uploadOwnerID != ownerID {
		return "", validation("图片必须由当前用户上传")
	}
	return canonical, nil
}
