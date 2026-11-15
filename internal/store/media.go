package store

import (
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
