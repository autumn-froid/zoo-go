package httpapi

import (
	"testing"
	"time"

	"zoo-go/internal/domain"
)

func TestSignMediaValueOnlySignsMediaFields(t *testing.T) {
	signer := newMediaSigner("test-media-signing-key")
	now := time.Unix(1_800_000_000, 0)
	post := domain.CommunityPost{
		Content: "/uploads/this-is-content.jpg",
		Author: domain.CommunityAuthor{
			Avatar: "/uploads/avatar.jpg",
		},
		Images: []string{"/uploads/post.jpg", "https://example.com/remote.jpg"},
	}
	signed, ok := signMediaValue(post, signer, now, time.Hour).(domain.CommunityPost)
	if !ok {
		t.Fatal("signed value is not a community post")
	}
	if signed.Content != post.Content {
		t.Fatalf("content was modified: %q", signed.Content)
	}
	if canonical, _ := canonicalUploadPath(signed.Author.Avatar); canonical != "/uploads/avatar.jpg" || signed.Author.Avatar == post.Author.Avatar {
		t.Fatalf("avatar was not signed: %s", signed.Author.Avatar)
	}
	if canonical, _ := canonicalUploadPath(signed.Images[0]); canonical != "/uploads/post.jpg" || signed.Images[0] == post.Images[0] {
		t.Fatalf("post image was not signed: %s", signed.Images[0])
	}
	if signed.Images[1] != post.Images[1] {
		t.Fatalf("external image was modified: %s", signed.Images[1])
	}
}

func TestMediaSignatureRejectsTampering(t *testing.T) {
	signer := newMediaSigner("test-media-signing-key")
	now := time.Unix(1_800_000_000, 0)
	expiresAt := now.Add(time.Hour).Unix()
	signature := signer.signature("/uploads/post.jpg", expiresAt)
	if !signer.valid("/uploads/post.jpg", expiresAt, signature, now) {
		t.Fatal("valid signature was rejected")
	}
	if signer.valid("/uploads/other.jpg", expiresAt, signature, now) {
		t.Fatal("signature was accepted for another path")
	}
	if signer.valid("/uploads/post.jpg", expiresAt, signature, now.Add(2*time.Hour)) {
		t.Fatal("expired signature was accepted")
	}
}
