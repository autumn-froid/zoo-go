package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"zoo-go/internal/contentsecurity"
	"zoo-go/internal/domain"
	"zoo-go/internal/store"
)

const machineMediaURLTTL = 15 * time.Minute

func (s *Server) communityPosts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(query(r, "limit", "20"))
	feed, err := s.store.CommunityPosts(currentUser(r).ID, query(r, "cursor", ""), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeData(w, feed)
}

func (s *Server) createCommunityPost(w http.ResponseWriter, r *http.Request) {
	var req domain.CreatePostRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	user := currentUser(r)
	var post domain.CommunityPost
	var err error
	if s.contentSecurity == nil {
		post, err = s.store.CreateCommunityPost(user.ID, req, s.now())
	} else {
		post, err = s.store.CreateCommunityPostForMachineReview(user.ID, req, s.now())
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if s.contentSecurity != nil {
		post = s.runMachineReview(r.Context(), user.ID, post)
	}
	s.writeDataStatus(w, http.StatusCreated, post)
}

func (s *Server) runMachineReview(ctx context.Context, userID string, post domain.CommunityPost) domain.CommunityPost {
	now := s.now()
	openID, err := s.store.UserOpenID(userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "load user openid for machine review", "post_id", post.ID, "error", err)
		return s.fallbackMachineReview(ctx, post, "读取用户标识失败，转人工复核", now)
	}
	verdict := store.MachinePass
	if strings.TrimSpace(post.Content) != "" {
		text, err := s.contentSecurity.CheckText(ctx, openID, post.Content)
		if err != nil {
			s.logger.WarnContext(ctx, "wechat text security check failed", "post_id", post.ID, "error", err)
			return s.fallbackMachineReview(ctx, post, "文字机器审核服务暂时不可用，转人工复核", now)
		}
		verdict = machineVerdict(text.Suggest)
	}
	updated, needsImages, err := s.store.ApplyMachineTextReview(post.ID, verdict, nil, now)
	if err != nil {
		s.logger.ErrorContext(ctx, "apply machine text review", "post_id", post.ID, "error", err)
		return post
	}
	if !needsImages {
		return updated
	}
	if s.publicBaseURL == "" {
		return s.fallbackMachineReview(ctx, post, "图片机器审核未配置公网访问地址，转人工复核", now)
	}
	if s.callbackVerifier == nil {
		return s.fallbackMachineReview(ctx, post, "图片机器审核未配置回调验签，转人工复核", now)
	}
	urls := make([]string, 0, len(updated.Images))
	ttl := s.mediaTTL
	if ttl > machineMediaURLTTL {
		ttl = machineMediaURLTTL
	}
	for _, image := range updated.Images {
		urls = append(urls, s.publicBaseURL+s.media.signedURL(image, now, ttl))
	}
	images, err := s.contentSecurity.SubmitImages(ctx, openID, urls)
	if err != nil {
		s.logger.WarnContext(ctx, "wechat image security submission failed", "post_id", post.ID, "error", err)
		return s.fallbackMachineReview(ctx, post, "图片机器审核提交失败，转人工复核", now)
	}
	if len(images) != len(updated.Images) {
		return s.fallbackMachineReview(ctx, post, "图片机器审核返回结果不完整，转人工复核", now)
	}
	submissions := make([]store.MachineImageSubmission, 0, len(images))
	for index, image := range images {
		if index >= len(updated.Images) {
			break
		}
		submissions = append(submissions, store.MachineImageSubmission{
			TraceID: image.TraceID, Path: updated.Images[index],
		})
	}
	updated, err = s.store.MarkMachineReviewing(post.ID, submissions, now, now.Add(s.mediaReviewTTL))
	if err != nil {
		s.logger.ErrorContext(ctx, "mark machine media reviewing", "post_id", post.ID, "error", err)
		return post
	}
	return updated
}

func (s *Server) fallbackMachineReview(ctx context.Context, post domain.CommunityPost, reason string, now time.Time) domain.CommunityPost {
	updated, _, err := s.store.ApplyMachineTextReview(post.ID, store.MachineUnknown, []string{reason}, now)
	if err != nil {
		s.logger.ErrorContext(ctx, "fallback machine review", "post_id", post.ID, "error", err)
		return post
	}
	return updated
}

func machineVerdict(suggest contentsecurity.Suggest) store.MachineVerdict {
	switch suggest {
	case contentsecurity.SuggestPass:
		return store.MachinePass
	case contentsecurity.SuggestRisky:
		return store.MachineRisky
	case contentsecurity.SuggestReview:
		return store.MachineReview
	default:
		return store.MachineUnknown
	}
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if err := s.store.CheckUploadAllowed(currentUser(r).ID, s.now()); err != nil {
		writeError(w, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, &uploadError{message: "请选择需要上传的图片"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil {
		writeError(w, err)
		return
	}
	if len(data) > 8<<20 {
		writeError(w, &uploadError{message: "图片不能超过 8MB"})
		return
	}
	mimeType := http.DetectContentType(data)
	extensions := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
		"image/gif":  ".gif",
	}
	extension, ok := extensions[mimeType]
	if !ok {
		writeError(w, &uploadError{message: "仅支持 JPG、PNG、WebP、GIF 图片"})
		return
	}
	filename := newUploadFilename() + extension
	path := s.uploadPath(filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		writeError(w, err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		writeError(w, err)
		return
	}
	if err := s.store.RecordUpload(currentUser(r).ID, filename, header.Filename, mimeType, int64(len(data)), s.now()); err != nil {
		_ = os.Remove(path)
		writeError(w, err)
		return
	}
	s.writeDataStatus(w, http.StatusOK, map[string]any{
		"url":      "/uploads/" + filename,
		"path":     "/uploads/" + filename,
		"filename": filename,
		"mimeType": mimeType,
		"size":     len(data),
	})
}

type uploadError struct {
	message string
}

func (e *uploadError) Error() string {
	return "UPLOAD_ERROR: " + e.message
}

func newUploadFilename() string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return "media_" + strings.TrimSpace(hex.EncodeToString(raw))
}
