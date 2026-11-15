package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

const encryptedPrefix = "enc:v1:"

// Cipher encrypts personally identifiable information before it is persisted.
// Encryption keys are never written to logs or returned in API responses.
type Cipher struct {
	aead    cipher.AEAD
	hashKey []byte
}

// NewCipher accepts a 32-byte encryption key encoded as hex, standard base64,
// base64url, or raw text. A separate lookup key is recommended; when omitted a
// domain-separated key is derived from the encryption key.
func NewCipher(encryptionKey, lookupKey string) (*Cipher, error) {
	key, err := ParseKey(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("parse PII encryption key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create PII cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create PII GCM: %w", err)
	}
	hashKey := lookupKey
	if strings.TrimSpace(hashKey) == "" {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte("zoo-go:pii:lookup-key:v1"))
		hashKey = hex.EncodeToString(mac.Sum(nil))
	}
	parsedHashKey, err := ParseKey(hashKey)
	if err != nil {
		return nil, fmt.Errorf("parse PII lookup key: %w", err)
	}
	return &Cipher{aead: aead, hashKey: parsedHashKey}, nil
}

// ParseKey accepts common secret encodings so deployment tooling can use
// openssl rand -hex 32 or openssl rand -base64 32 without custom transforms.
func ParseKey(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New("key is empty")
	}
	if len(value) == 64 {
		if decoded, err := hex.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(value); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if len([]byte(value)) == 32 {
		return []byte(value), nil
	}
	return nil, errors.New("key must be 32 bytes or a 64-character hex / base64 encoding")
}

func (c *Cipher) Encrypt(scope, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate PII nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), additionalData(scope))
	payload := append(nonce, sealed...)
	return encryptedPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

// Decrypt also accepts legacy plaintext so a rolling migration can read old
// rows before migratePII has converted them.
func (c *Cipher) Decrypt(scope, value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, encryptedPrefix))
	if err != nil {
		return "", errors.New("PII ciphertext is malformed")
	}
	if len(payload) < c.aead.NonceSize()+c.aead.Overhead() {
		return "", errors.New("PII ciphertext is too short")
	}
	nonce := payload[:c.aead.NonceSize()]
	sealed := payload[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, sealed, additionalData(scope))
	if err != nil {
		return "", errors.New("PII decryption failed")
	}
	return string(plaintext), nil
}

// Lookup returns a deterministic, keyed digest for equality checks and unique
// indexes. Callers should normalize the value before hashing it.
func (c *Cipher) Lookup(scope, normalized string) string {
	mac := hmac.New(sha256.New, c.hashKey)
	_, _ = mac.Write([]byte(scope))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encryptedPrefix)
}

func additionalData(scope string) []byte {
	return []byte("zoo-go:pii:v1:" + scope)
}

func MaskName(value string) string {
	runes := []rune(strings.TrimSpace(value))
	switch len(runes) {
	case 0:
		return ""
	case 1:
		return "*"
	case 2:
		return string(runes[0]) + "*"
	default:
		return string(runes[0]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1])
	}
}

func MaskIDNumber(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 7 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:3]) + strings.Repeat("*", len(runes)-7) + string(runes[len(runes)-4:])
}

func MaskStudentNo(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-4:])
}

func MaskPhone(value string) string {
	digits := make([]rune, 0, len(value))
	for _, char := range value {
		if unicode.IsDigit(char) {
			digits = append(digits, char)
		}
	}
	if len(digits) < 7 {
		return strings.Repeat("*", len(digits))
	}
	return string(digits[:3]) + strings.Repeat("*", len(digits)-7) + string(digits[len(digits)-4:])
}

// NormalizeIDNumber validates a mainland China resident identity number and
// returns the canonical uppercase form. It deliberately rejects malformed
// numbers before anything is encrypted or stored.
func NormalizeIDNumber(value string) (string, error) {
	value = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	if len(value) != 18 {
		return "", errors.New("identity number must be 18 characters")
	}
	for index, char := range value {
		if index == 17 && char == 'X' {
			continue
		}
		if char < '0' || char > '9' {
			return "", errors.New("identity number contains invalid characters")
		}
	}
	weights := [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checks := "10X98765432"
	sum := 0
	for index, char := range value[:17] {
		sum += int(char-'0') * weights[index]
	}
	if value[17] != checks[sum%11] {
		return "", errors.New("identity number checksum is invalid")
	}
	return value, nil
}
