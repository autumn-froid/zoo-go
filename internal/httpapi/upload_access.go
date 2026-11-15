package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultUploadURLTTL = 2 * time.Hour

func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/uploads/" || strings.HasSuffix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}
	filename := filepath.Base(strings.TrimPrefix(r.URL.Path, "/uploads/"))
	if filename == "." || filename == ".." || filename != filepath.Base(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	if !s.canReadUpload(r, filename) {
		writeJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{
			Code: "UNAUTHORIZED", Message: "请登录后查看图片，或使用有效的访问链接",
		}})
		return
	}
	path := s.uploadPath(filename)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

func (s *Server) canReadUpload(r *http.Request, filename string) bool {
	if s.validUploadSignature(filename, r.URL.Query().Get("exp"), r.URL.Query().Get("sig")) {
		return true
	}
	if s.config.AdminToken != "" && r.Header.Get("X-Admin-Token") == s.config.AdminToken {
		return true
	}
	if token := bearerToken(r); token != "" {
		if _, err := s.store.Authenticate(token, s.now()); err == nil {
			return true
		}
	}
	return false
}

func (s *Server) signedUploadURL(path string, ttl time.Duration) string {
	path = canonicalUploadPath(path)
	if path == "" {
		return path
	}
	if ttl <= 0 {
		ttl = defaultUploadURLTTL
	}
	filename := strings.TrimPrefix(path, "/uploads/")
	exp := s.now().Add(ttl).Unix()
	sig := s.uploadSignature(filename, exp)
	return path + "?exp=" + strconv.FormatInt(exp, 10) + "&sig=" + sig
}

func (s *Server) validUploadSignature(filename, expRaw, sig string) bool {
	if filename == "" || expRaw == "" || sig == "" {
		return false
	}
	exp, err := strconv.ParseInt(expRaw, 10, 64)
	if err != nil || exp < s.now().Unix() {
		return false
	}
	expected := s.uploadSignature(filename, exp)
	return hmac.Equal([]byte(expected), []byte(sig))
}

func (s *Server) uploadSignature(filename string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(s.config.UploadSigningSecret))
	_, _ = mac.Write([]byte(filename))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) signMediaURLs(data any) any {
	if data == nil || s.config.UploadSigningSecret == "" {
		return data
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return data
	}
	signMediaTree(tree, s.signedUploadURL)
	return tree
}

func signMediaTree(node any, sign func(string, time.Duration) string) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if text, ok := child.(string); ok {
				if canonical := canonicalUploadPath(text); canonical != "" && !strings.Contains(text, "sig=") {
					value[key] = sign(canonical, defaultUploadURLTTL)
					continue
				}
			}
			signMediaTree(child, sign)
		}
	case []any:
		for index, child := range value {
			if text, ok := child.(string); ok {
				if canonical := canonicalUploadPath(text); canonical != "" && !strings.Contains(text, "sig=") {
					value[index] = sign(canonical, defaultUploadURLTTL)
					continue
				}
			}
			signMediaTree(child, sign)
		}
	}
}

func canonicalUploadPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.Contains(path, "://") {
		return ""
	}
	path = strings.SplitN(path, "?", 2)[0]
	if !strings.HasPrefix(path, "/uploads/") {
		return ""
	}
	filename := strings.TrimPrefix(path, "/uploads/")
	if filename == "" || strings.Contains(filename, "/") || filename == "." || filename == ".." {
		return ""
	}
	return "/uploads/" + filename
}
