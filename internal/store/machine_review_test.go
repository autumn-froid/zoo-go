package store

import (
	"strings"
	"testing"
	"time"

	"zoo-go/internal/domain"
	"zoo-go/internal/moderation"
)

func createMachineReviewPost(t *testing.T, st *Store, owner domain.Profile, content string, imageNames ...string) domain.CommunityPost {
	t.Helper()
	now := time.Now()
	images := make([]string, 0, len(imageNames))
	for _, name := range imageNames {
		if err := st.RecordUpload(owner.ID, name, name, "image/jpeg", 128, now); err != nil {
			t.Fatalf("RecordUpload(%s) error = %v", name, err)
		}
		images = append(images, "/uploads/"+name)
	}
	post, err := st.CreateCommunityPostForMachineReview(owner.ID, domain.CreatePostRequest{
		Content: content, Images: images,
	}, now)
	if err != nil {
		t.Fatalf("CreateCommunityPostForMachineReview() error = %v", err)
	}
	return post
}

func TestMachineReviewApprovesPostAfterAllImagesPass(t *testing.T) {
	st := newTestStore(t)
	owner := createTestUser(t, st, "机审通过用户")
	post := createMachineReviewPost(t, st, owner, "今天猫猫很乖", "review-pass-1.jpg", "review-pass-2.jpg")
	now := time.Now()
	post, needsImages, err := st.ApplyMachineTextReview(post.ID, MachinePass, nil, now)
	if err != nil {
		t.Fatalf("ApplyMachineTextReview() error = %v", err)
	}
	if !needsImages || post.ModerationStatus != domain.PostPending {
		t.Fatalf("post after text pass = %+v, needsImages=%v", post, needsImages)
	}
	post, err = st.MarkMachineReviewing(post.ID, []MachineImageSubmission{
		{TraceID: "trace-pass-1", Path: "/uploads/review-pass-1.jpg"},
		{TraceID: "trace-pass-2", Path: "/uploads/review-pass-2.jpg"},
	}, now, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("MarkMachineReviewing() error = %v", err)
	}
	if post.ModerationStatus != domain.PostMachineReviewing {
		t.Fatalf("machine reviewing status = %s", post.ModerationStatus)
	}
	if err := st.HandleMachineMediaResult("trace-pass-1", MachinePass, 100, 0, "ok", now); err != nil {
		t.Fatalf("first image result error = %v", err)
	}
	post, _ = st.communityPostByID(post.ID, owner.ID)
	if post.ModerationStatus != domain.PostMachineReviewing {
		t.Fatalf("post approved before all images passed: %s", post.ModerationStatus)
	}
	if err := st.HandleMachineMediaResult("trace-pass-2", MachinePass, 100, 0, "ok", now); err != nil {
		t.Fatalf("second image result error = %v", err)
	}
	post, _ = st.communityPostByID(post.ID, owner.ID)
	if post.ModerationStatus != domain.PostApproved || post.ModerationReason != "" {
		t.Fatalf("post after all images passed = %+v", post)
	}
}

func TestMachineReviewRiskyImageRejectsPost(t *testing.T) {
	st := newTestStore(t)
	owner := createTestUser(t, st, "机审拒绝用户")
	post := createMachineReviewPost(t, st, owner, "上传了一张图片", "review-risky.jpg")
	now := time.Now()
	post, needsImages, err := st.ApplyMachineTextReview(post.ID, MachinePass, nil, now)
	if err != nil || !needsImages {
		t.Fatalf("ApplyMachineTextReview() = %+v, %v", post, err)
	}
	post, err = st.MarkMachineReviewing(post.ID, []MachineImageSubmission{
		{TraceID: "trace-risky", Path: "/uploads/review-risky.jpg"},
	}, now, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("MarkMachineReviewing() error = %v", err)
	}
	if err := st.HandleMachineMediaResult("trace-risky", MachineRisky, 20002, 0, "ok", now); err != nil {
		t.Fatalf("HandleMachineMediaResult() error = %v", err)
	}
	post, _ = st.communityPostByID(post.ID, owner.ID)
	if post.ModerationStatus != domain.PostRejected || post.RiskLevel != "HIGH" {
		t.Fatalf("risky image post = %+v", post)
	}
}

func TestExpiredMachineReviewFallsBackToManualReview(t *testing.T) {
	st := newTestStore(t)
	owner := createTestUser(t, st, "机审超时用户")
	post := createMachineReviewPost(t, st, owner, "机审可能超时", "review-timeout.jpg")
	now := time.Now()
	post, _, err := st.ApplyMachineTextReview(post.ID, MachinePass, nil, now)
	if err != nil {
		t.Fatalf("ApplyMachineTextReview() error = %v", err)
	}
	post, err = st.MarkMachineReviewing(post.ID, []MachineImageSubmission{
		{TraceID: "trace-timeout", Path: "/uploads/review-timeout.jpg"},
	}, now, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("MarkMachineReviewing() error = %v", err)
	}
	expired, err := st.ExpireMachineReviews(now.Add(2 * time.Minute))
	if err != nil {
		t.Fatalf("ExpireMachineReviews() error = %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired reviews = %d, want 1", expired)
	}
	post, _ = st.communityPostByID(post.ID, owner.ID)
	if post.ModerationStatus != domain.PostPending || !strings.Contains(post.ModerationReason, "人工复核") {
		t.Fatalf("expired review post = %+v", post)
	}
	if err := st.HandleMachineMediaResult("trace-timeout", MachinePass, 100, 0, "ok", now.Add(3*time.Minute)); err != nil {
		t.Fatalf("late image result error = %v", err)
	}
	post, _ = st.communityPostByID(post.ID, owner.ID)
	if post.ModerationStatus != domain.PostPending {
		t.Fatalf("late callback approved an expired review: %+v", post)
	}
}

func TestLateMachineReviewPassDoesNotApproveExpiredPost(t *testing.T) {
	st := newTestStore(t)
	owner := createTestUser(t, st, "迟到回调用户")
	post := createMachineReviewPost(t, st, owner, "回调可能迟到", "review-late.jpg")
	now := time.Now()
	post, _, err := st.ApplyMachineTextReview(post.ID, MachinePass, nil, now)
	if err != nil {
		t.Fatalf("ApplyMachineTextReview() error = %v", err)
	}
	post, err = st.MarkMachineReviewing(post.ID, []MachineImageSubmission{
		{TraceID: "trace-late", Path: "/uploads/review-late.jpg"},
	}, now, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("MarkMachineReviewing() error = %v", err)
	}
	if err := st.HandleMachineMediaResult("trace-late", MachinePass, 100, 0, "ok", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("late image result error = %v", err)
	}
	post, _ = st.communityPostByID(post.ID, owner.ID)
	if post.ModerationStatus != domain.PostPending || !strings.Contains(post.ModerationReason, "人工复核") {
		t.Fatalf("late pass approved an expired review: %+v", post)
	}
	var reviewStatus string
	if err := st.db.QueryRow(`SELECT status FROM community_media_reviews WHERE trace_id = ?`, "trace-late").Scan(&reviewStatus); err != nil {
		t.Fatalf("load media review status: %v", err)
	}
	if reviewStatus != mediaReviewExpired {
		t.Fatalf("late review status = %s, want EXPIRED", reviewStatus)
	}
}

func TestDuplicateMachineReviewDoesNotOverwriteResult(t *testing.T) {
	st := newTestStore(t)
	owner := createTestUser(t, st, "重复回调用户")
	post := createMachineReviewPost(t, st, owner, "重复回调", "review-duplicate.jpg")
	now := time.Now()
	post, _, err := st.ApplyMachineTextReview(post.ID, MachinePass, nil, now)
	if err != nil {
		t.Fatalf("ApplyMachineTextReview() error = %v", err)
	}
	post, err = st.MarkMachineReviewing(post.ID, []MachineImageSubmission{
		{TraceID: "trace-duplicate", Path: "/uploads/review-duplicate.jpg"},
	}, now, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("MarkMachineReviewing() error = %v", err)
	}
	if err := st.HandleMachineMediaResult("trace-duplicate", MachinePass, 100, 0, "ok", now); err != nil {
		t.Fatalf("first image result error = %v", err)
	}
	if err := st.HandleMachineMediaResult("trace-duplicate", MachineRisky, 20002, 0, "late duplicate", now.Add(time.Second)); err != nil {
		t.Fatalf("duplicate image result error = %v", err)
	}
	post, _ = st.communityPostByID(post.ID, owner.ID)
	if post.ModerationStatus != domain.PostApproved {
		t.Fatalf("duplicate callback changed approved post: %+v", post)
	}
	var reviewStatus string
	if err := st.db.QueryRow(`SELECT status FROM community_media_reviews WHERE trace_id = ?`, "trace-duplicate").Scan(&reviewStatus); err != nil {
		t.Fatalf("load media review status: %v", err)
	}
	if reviewStatus != mediaReviewPass {
		t.Fatalf("duplicate review status = %s, want PASS", reviewStatus)
	}
}

func TestMachineTextReviewKeepsLocalContactRiskForHumans(t *testing.T) {
	st := newTestStore(t)
	owner := createTestUser(t, st, "本地风险用户")
	post := createMachineReviewPost(t, st, owner, "加微信聊 13800138000", "review-local-risk.jpg")
	updated, needsImages, err := st.ApplyMachineTextReview(post.ID, MachinePass, nil, time.Now())
	if err != nil {
		t.Fatalf("ApplyMachineTextReview() error = %v", err)
	}
	if needsImages || updated.ModerationStatus != domain.PostPending || !strings.Contains(updated.ModerationReason, "联系方式") {
		t.Fatalf("local risk post = %+v, needsImages=%v", updated, needsImages)
	}
}

func TestMachineTextPassCanAutoApproveCleanPost(t *testing.T) {
	st := newTestStore(t)
	policy := moderation.DefaultPolicy()
	policy.AutoApproveText = true
	st.SetModerationPolicy(policy)
	owner := createTestUser(t, st, "自动通过用户")
	post := createMachineReviewPost(t, st, owner, "今天猫猫很乖")
	updated, needsImages, err := st.ApplyMachineTextReview(post.ID, MachinePass, nil, time.Now())
	if err != nil {
		t.Fatalf("ApplyMachineTextReview() error = %v", err)
	}
	if needsImages || updated.ModerationStatus != domain.PostApproved {
		t.Fatalf("auto-approved post = %+v, needsImages=%v", updated, needsImages)
	}
}
