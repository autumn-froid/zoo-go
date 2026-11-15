package contentsecurity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Notification struct {
	Event   string
	AppID   string
	TraceID string
	Suggest Suggest
	Label   int
	ErrCode int
	ErrMsg  string
}

type NotificationVerifier interface {
	Verify(request *http.Request, body []byte) (Notification, error)
}

type CallbackConfig struct {
	Token          string
	EncodingAESKey string
	AppID          string
	MaxAge         time.Duration
	Now            func() time.Time
}

type Verifier struct {
	token  string
	aesKey []byte
	appID  string
	maxAge time.Duration
	now    func() time.Time
}

func NewVerifier(config CallbackConfig) (*Verifier, error) {
	token := strings.TrimSpace(config.Token)
	if token == "" {
		return nil, errors.New("wechat content security callback token is required")
	}
	var aesKey []byte
	encodingKey := strings.TrimSpace(config.EncodingAESKey)
	if encodingKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(encodingKey + "=")
		if err != nil || len(decoded) != 32 {
			return nil, errors.New("WECHAT_CONTENT_SECURITY_AES_KEY must be a 43-character EncodingAESKey")
		}
		aesKey = decoded
	}
	maxAge := config.MaxAge
	if maxAge <= 0 {
		maxAge = 10 * time.Minute
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Verifier{
		token: token, aesKey: aesKey, appID: strings.TrimSpace(config.AppID),
		maxAge: maxAge, now: now,
	}, nil
}

func (v *Verifier) Verify(request *http.Request, body []byte) (Notification, error) {
	query := request.URL.Query()
	timestamp := strings.TrimSpace(query.Get("timestamp"))
	nonce := strings.TrimSpace(query.Get("nonce"))
	if timestamp == "" || nonce == "" {
		return Notification{}, errors.New("missing callback timestamp or nonce")
	}
	if err := v.verifyTimestamp(timestamp); err != nil {
		return Notification{}, err
	}

	var encryptedEnvelope struct {
		Encrypt string `xml:"Encrypt"`
	}
	if err := xml.Unmarshal(body, &encryptedEnvelope); err != nil {
		return Notification{}, fmt.Errorf("decode callback envelope: %w", err)
	}

	plain := body
	if strings.EqualFold(query.Get("encrypt_type"), "aes") || strings.TrimSpace(encryptedEnvelope.Encrypt) != "" {
		if len(v.aesKey) == 0 {
			return Notification{}, errors.New("encrypted callback received without an EncodingAESKey")
		}
		encrypted := strings.TrimSpace(encryptedEnvelope.Encrypt)
		if encrypted == "" {
			return Notification{}, errors.New("encrypted callback payload is empty")
		}
		expected := signature(v.token, timestamp, nonce, encrypted)
		if !secureEqual(expected, query.Get("msg_signature")) {
			return Notification{}, errors.New("invalid callback msg_signature")
		}
		decrypted, err := decryptCallback(v.aesKey, encrypted)
		if err != nil {
			return Notification{}, err
		}
		plain = []byte(decrypted)
	} else {
		expected := signature(v.token, timestamp, nonce)
		if !secureEqual(expected, query.Get("signature")) {
			return Notification{}, errors.New("invalid callback signature")
		}
	}

	var envelope struct {
		MsgType string `xml:"MsgType"`
		Event   string `xml:"Event"`
		AppID   string `xml:"appid"`
		TraceID string `xml:"trace_id"`
		ErrCode int    `xml:"errcode"`
		ErrMsg  string `xml:"errmsg"`
		Result  struct {
			Suggest string `xml:"suggest"`
			Label   int    `xml:"label"`
		} `xml:"result"`
	}
	if err := xml.Unmarshal(plain, &envelope); err != nil {
		return Notification{}, fmt.Errorf("decode callback payload: %w", err)
	}
	if envelope.MsgType != "" && !strings.EqualFold(envelope.MsgType, "event") {
		return Notification{}, errors.New("callback message is not an event")
	}
	if envelope.Event != "" && envelope.Event != "wxa_media_check" {
		return Notification{}, errors.New("callback event is not wxa_media_check")
	}
	if v.appID != "" && envelope.AppID != "" && !strings.EqualFold(v.appID, envelope.AppID) {
		return Notification{}, errors.New("callback appid does not match")
	}
	if strings.TrimSpace(envelope.TraceID) == "" {
		return Notification{}, errors.New("callback trace_id is empty")
	}
	suggest := normalizeSuggest(envelope.Result.Suggest)
	if envelope.ErrCode != 0 {
		suggest = SuggestUnknown
	}
	return Notification{
		Event: envelope.Event, AppID: envelope.AppID, TraceID: envelope.TraceID,
		Suggest: suggest, Label: envelope.Result.Label,
		ErrCode: envelope.ErrCode, ErrMsg: envelope.ErrMsg,
	}, nil
}

func (v *Verifier) verifyTimestamp(value string) error {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return errors.New("invalid callback timestamp")
	}
	delta := v.now().Sub(time.Unix(seconds, 0))
	if delta < 0 {
		delta = -delta
	}
	if delta > v.maxAge {
		return errors.New("callback timestamp is outside the allowed window")
	}
	return nil
}

func signature(parts ...string) string {
	sorted := append([]string(nil), parts...)
	sort.Strings(sorted)
	sum := sha1.Sum([]byte(strings.Join(sorted, "")))
	return hex.EncodeToString(sum[:])
}

func secureEqual(expected, actual string) bool {
	if expected == "" || actual == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func decryptCallback(key []byte, value string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode encrypted callback: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(ciphertext) == 0 || len(ciphertext)%block.BlockSize() != 0 {
		return "", errors.New("encrypted callback has an invalid length")
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:block.BlockSize()]).CryptBlocks(plain, ciphertext)
	plain, err = unpadPKCS7(plain, block.BlockSize())
	if err != nil {
		return "", err
	}
	if len(plain) < 20 {
		return "", errors.New("encrypted callback payload is too short")
	}
	messageLength := int(binary.BigEndian.Uint32(plain[16:20]))
	if len(plain) < 20+messageLength {
		return "", errors.New("encrypted callback message length is invalid")
	}
	return string(plain[20 : 20+messageLength]), nil
}

func unpadPKCS7(value []byte, blockSize int) ([]byte, error) {
	if len(value) == 0 || len(value)%blockSize != 0 {
		return nil, errors.New("invalid PKCS7 payload length")
	}
	padding := int(value[len(value)-1])
	if padding == 0 || padding > blockSize || padding > len(value) {
		return nil, errors.New("invalid PKCS7 padding")
	}
	for _, item := range value[len(value)-padding:] {
		if int(item) != padding {
			return nil, errors.New("invalid PKCS7 padding")
		}
	}
	return value[:len(value)-padding], nil
}
