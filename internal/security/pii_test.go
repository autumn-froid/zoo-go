package security

import (
	"strings"
	"testing"
)

func TestPIICipherRoundTripAndScopeBinding(t *testing.T) {
	cipher, err := NewCipher(strings.Repeat("a", 64), strings.Repeat("b", 64))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	plaintext := "张三 11010519491231002X"
	encrypted, err := cipher.Encrypt("identity", plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if !IsEncrypted(encrypted) || strings.Contains(encrypted, plaintext) {
		t.Fatalf("encrypted value = %q", encrypted)
	}
	decrypted, err := cipher.Decrypt("identity", encrypted)
	if err != nil || decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, %v", decrypted, err)
	}
	if _, err := cipher.Decrypt("another-scope", encrypted); err == nil {
		t.Fatal("decryption with a different scope unexpectedly succeeded")
	}
	legacy, err := cipher.Decrypt("identity", "legacy plaintext")
	if err != nil || legacy != "legacy plaintext" {
		t.Fatalf("legacy Decrypt() = %q, %v", legacy, err)
	}
}

func TestPIICipherRejectsTamperingAndUsesDeterministicLookup(t *testing.T) {
	cipher, err := NewCipher(strings.Repeat("c", 64), strings.Repeat("d", 64))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	encrypted, err := cipher.Encrypt("student.chsi", "ABC123")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	tampered := encrypted[:len(encrypted)-1] + "A"
	if tampered == encrypted {
		tampered = encrypted[:len(encrypted)-1] + "B"
	}
	if _, err := cipher.Decrypt("student.chsi", tampered); err == nil {
		t.Fatal("tampered ciphertext unexpectedly decrypted")
	}
	if cipher.Lookup("student.chsi", "ABC123") != cipher.Lookup("student.chsi", "ABC123") {
		t.Fatal("lookup hash is not deterministic")
	}
	if cipher.Lookup("student.chsi", "ABC123") == cipher.Lookup("student.chsi", "ABC124") {
		t.Fatal("different values produced the same lookup hash")
	}
}

func TestPIIMaskingAndIdentityValidation(t *testing.T) {
	if got := MaskName("张三丰"); got != "张*丰" {
		t.Fatalf("MaskName() = %q", got)
	}
	if got := MaskIDNumber("11010519491231002X"); got != "110***********002X" {
		t.Fatalf("MaskIDNumber() = %q", got)
	}
	if got := MaskStudentNo("20240001"); got != "****0001" {
		t.Fatalf("MaskStudentNo() = %q", got)
	}
	if got := MaskPhone("138 0013 8000"); got != "138****8000" {
		t.Fatalf("MaskPhone() = %q", got)
	}
	normalized, err := NormalizeIDNumber("11010519491231002x")
	if err != nil || normalized != "11010519491231002X" {
		t.Fatalf("NormalizeIDNumber() = %q, %v", normalized, err)
	}
	if _, err := NormalizeIDNumber("110105194912310021"); err == nil {
		t.Fatal("invalid identity checksum was accepted")
	}
}
