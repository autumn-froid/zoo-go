package contentsecurity

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestVerifierAcceptsPlainCallback(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	verifier, err := NewVerifier(CallbackConfig{
		Token: "callback-token", AppID: "wx-test", Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	body := []byte(`<xml>
		<MsgType><![CDATA[event]]></MsgType>
		<Event><![CDATA[wxa_media_check]]></Event>
		<appid><![CDATA[wx-test]]></appid>
		<trace_id><![CDATA[trace-1]]></trace_id>
		<errcode>0</errcode>
		<errmsg><![CDATA[ok]]></errmsg>
		<result><suggest><![CDATA[pass]]></suggest><label>100</label></result>
	</xml>`)
	timestamp := fmt.Sprint(now.Unix())
	nonce := "nonce-1"
	query := url.Values{
		"timestamp": {timestamp},
		"nonce":     {nonce},
		"signature": {signature("callback-token", timestamp, nonce)},
	}
	request := httptest.NewRequest("POST", "/notify?"+query.Encode(), bytes.NewReader(body))
	notification, err := verifier.Verify(request, body)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if notification.TraceID != "trace-1" || notification.Suggest != SuggestPass || notification.Label != 100 {
		t.Fatalf("Verify() = %+v", notification)
	}
}

func TestVerifierRejectsBadSignature(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	verifier, err := NewVerifier(CallbackConfig{
		Token: "callback-token", Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	body := []byte(`<xml><Event>wxa_media_check</Event><trace_id>trace-1</trace_id></xml>`)
	query := url.Values{
		"timestamp": {fmt.Sprint(now.Unix())}, "nonce": {"nonce"}, "signature": {"bad"},
	}
	request := httptest.NewRequest("POST", "/notify?"+query.Encode(), bytes.NewReader(body))
	if _, err := verifier.Verify(request, body); err == nil {
		t.Fatal("Verify() accepted a bad signature")
	}
}

func TestVerifierDecryptsSafeModeCallback(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	key := []byte("12345678901234567890123456789012")
	encodingKey := strings.TrimRight(base64.StdEncoding.EncodeToString(key), "=")
	verifier, err := NewVerifier(CallbackConfig{
		Token: "callback-token", EncodingAESKey: encodingKey, AppID: "wx-test",
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	message := `<xml><Event><![CDATA[wxa_media_check]]></Event><appid><![CDATA[wx-test]]></appid>` +
		`<trace_id><![CDATA[trace-encrypted]]></trace_id><errcode>0</errcode>` +
		`<result><suggest><![CDATA[risky]]></suggest><label>20002</label></result></xml>`
	encrypted, err := encryptCallback(key, message, "wx-test")
	if err != nil {
		t.Fatalf("encryptCallback() error = %v", err)
	}
	body := []byte(`<xml><Encrypt><![CDATA[` + encrypted + `]]></Encrypt></xml>`)
	timestamp := fmt.Sprint(now.Unix())
	nonce := "nonce-safe"
	query := url.Values{
		"timestamp": {timestamp}, "nonce": {nonce}, "encrypt_type": {"aes"},
		"msg_signature": {signature("callback-token", timestamp, nonce, encrypted)},
	}
	request := httptest.NewRequest("POST", "/notify?"+query.Encode(), bytes.NewReader(body))
	notification, err := verifier.Verify(request, body)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if notification.TraceID != "trace-encrypted" || notification.Suggest != SuggestRisky || notification.Label != 20002 {
		t.Fatalf("Verify() = %+v", notification)
	}
}

func encryptCallback(key []byte, message, appID string) (string, error) {
	random := bytes.Repeat([]byte{0x7f}, 16)
	payload := append([]byte(nil), random...)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(message)))
	payload = append(payload, length...)
	payload = append(payload, []byte(message)...)
	payload = append(payload, []byte(appID)...)
	padding := aes.BlockSize - len(payload)%aes.BlockSize
	payload = append(payload, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	ciphertext := make([]byte, len(payload))
	cipher.NewCBCEncrypter(block, key[:block.BlockSize()]).CryptBlocks(ciphertext, payload)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
