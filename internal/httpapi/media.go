package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const defaultMediaURLTTL = time.Hour

type mediaSigner struct {
	key []byte
}

func newMediaSigner(key string) mediaSigner {
	return mediaSigner{key: []byte(key)}
}

func randomMediaSigningKey() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic("generate media signing key: " + err.Error())
	}
	return hex.EncodeToString(raw)
}

func (m mediaSigner) signedURL(value string, now time.Time, ttl time.Duration) string {
	uploadPath, ok := canonicalUploadPath(value)
	if !ok {
		return value
	}
	if ttl <= 0 {
		ttl = defaultMediaURLTTL
	}
	expiresAt := now.Add(ttl).Unix()
	return uploadPath + "?exp=" + strconv.FormatInt(expiresAt, 10) + "&sig=" + m.signature(uploadPath, expiresAt)
}

func (m mediaSigner) signature(uploadPath string, expiresAt int64) string {
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write([]byte(uploadPath))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(strconv.FormatInt(expiresAt, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m mediaSigner) valid(uploadPath string, expiresAt int64, signature string, now time.Time) bool {
	if expiresAt <= now.Unix() || signature == "" {
		return false
	}
	expected := m.signature(uploadPath, expiresAt)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func canonicalUploadPath(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Fragment != "" {
		return "", false
	}
	if !strings.HasPrefix(parsed.Path, "/uploads/") {
		return "", false
	}
	filename := strings.TrimPrefix(parsed.Path, "/uploads/")
	if !safeUploadFilename(filename) {
		return "", false
	}
	return "/uploads/" + filename, true
}

func safeUploadFilename(filename string) bool {
	return filename != "" && filename != "." && filename != ".." &&
		filename == path.Base(filename) && !strings.ContainsAny(filename, `/\`+"\x00")
}

func (s *Server) mediaHandler(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if !safeUploadFilename(filename) {
		http.NotFound(w, r)
		return
	}
	uploadPath := "/uploads/" + filename
	expiresAt, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	if err != nil || !s.media.valid(uploadPath, expiresAt, r.URL.Query().Get("sig"), s.now()) {
		writeJSON(w, http.StatusForbidden, errorEnvelope{Error: apiError{
			Code: "MEDIA_FORBIDDEN", Message: "图片链接无效或已过期",
		}})
		return
	}

	file, err := os.Open(s.uploadPath(filename))
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, err)
		return
	}
	if info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

var mediaStructFields = map[string]bool{
	"Avatar":       true,
	"Image":        true,
	"Images":       true,
	"Photos":       true,
	"AuthorAvatar": true,
}

var mediaMapFields = map[string]bool{
	"avatar":       true,
	"image":        true,
	"images":       true,
	"photos":       true,
	"authoravatar": true,
	"url":          true,
}

func signMediaValue(value any, signer mediaSigner, now time.Time, ttl time.Duration) any {
	if value == nil {
		return nil
	}
	signed := signReflectValue(reflect.ValueOf(value), signer, now, ttl)
	if !signed.IsValid() || !signed.CanInterface() {
		return value
	}
	return signed.Interface()
}

func signReflectValue(value reflect.Value, signer mediaSigner, now time.Time, ttl time.Duration) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		signed := signReflectValue(value.Elem(), signer, now, ttl)
		result := reflect.New(value.Type()).Elem()
		result.Set(signed)
		return result
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(signReflectValue(value.Elem(), signer, now, ttl))
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		valueType := value.Type()
		for index := 0; index < value.NumField(); index++ {
			field := result.Field(index)
			if !field.CanSet() {
				continue
			}
			if mediaStructFields[valueType.Field(index).Name] {
				signed := signMediaField(field, signer, now, ttl)
				if signed.IsValid() && signed.Type().AssignableTo(field.Type()) {
					field.Set(signed)
				}
				continue
			}
			signed := signReflectValue(field, signer, now, ttl)
			if signed.IsValid() && signed.Type().AssignableTo(field.Type()) {
				field.Set(signed)
			}
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(signReflectValue(value.Index(index), signer, now, ttl))
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(signReflectValue(value.Index(index), signer, now, ttl))
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key()
			item := iter.Value()
			if key.Kind() == reflect.String && mediaMapFields[strings.ToLower(key.String())] {
				signed := signMediaField(item, signer, now, ttl)
				if signed.IsValid() && signed.Type().AssignableTo(item.Type()) {
					result.SetMapIndex(key, signed)
				} else {
					result.SetMapIndex(key, item)
				}
				continue
			}
			signed := signReflectValue(item, signer, now, ttl)
			if signed.IsValid() && signed.Type().AssignableTo(item.Type()) {
				result.SetMapIndex(key, signed)
			} else {
				result.SetMapIndex(key, item)
			}
		}
		return result
	default:
		return value
	}
}

func signMediaField(value reflect.Value, signer mediaSigner, now time.Time, ttl time.Duration) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.String:
		result := reflect.New(value.Type()).Elem()
		result.SetString(signer.signedURL(value.String(), now, ttl))
		return result
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		signed := signMediaField(value.Elem(), signer, now, ttl)
		result := reflect.New(value.Type()).Elem()
		result.Set(signed)
		return result
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(signMediaField(value.Elem(), signer, now, ttl))
		return result
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(signMediaField(value.Index(index), signer, now, ttl))
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(signMediaField(value.Index(index), signer, now, ttl))
		}
		return result
	default:
		return value
	}
}
