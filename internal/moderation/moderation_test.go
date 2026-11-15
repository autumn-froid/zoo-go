package moderation

import "testing"

func TestReviewRejectsBlockedTermsAfterNormalization(t *testing.T) {
	result := DefaultPolicy().Review("加 微 信 赌 博，私聊我", 0)
	if result.Action != ActionReject || result.Level != LevelHigh {
		t.Fatalf("Review() = %+v, want rejected high risk", result)
	}
	if len(result.Matches) == 0 {
		t.Fatal("Review() did not report matched terms")
	}
}

func TestReviewHoldsContactInfoAndExternalLinks(t *testing.T) {
	cases := []string{
		"联系我 13800138000",
		"详情看 https://example.com/a",
		"加我微信聊",
	}
	for _, content := range cases {
		result := DefaultPolicy().Review(content, 0)
		if result.Action != ActionReview {
			t.Fatalf("Review(%q) = %+v, want review", content, result)
		}
	}
}

func TestReviewRequiresImageReviewByDefault(t *testing.T) {
	result := DefaultPolicy().Review("今天猫猫很乖", 2)
	if result.Action != ActionReview || result.Level != LevelMedium {
		t.Fatalf("Review() = %+v, want medium-risk image review", result)
	}
}

func TestReviewAutoApprovesCleanTextWhenEnabled(t *testing.T) {
	policy := DefaultPolicy()
	policy.AutoApproveText = true
	result := policy.Review("今天猫猫很乖", 0)
	if result.Action != ActionApprove || result.Level != LevelLow {
		t.Fatalf("Review() = %+v, want auto approval", result)
	}
}

func TestReviewManualModeHoldsCleanText(t *testing.T) {
	result := DefaultPolicy().Review("今天猫猫很乖", 0)
	if result.Action != ActionReview {
		t.Fatalf("Review() = %+v, want manual review", result)
	}
}

func TestReviewLocalSeparatesLocalRiskFromAutoApprovePolicy(t *testing.T) {
	result := DefaultPolicy().ReviewLocal("今天猫猫很乖", 0)
	if result.Action != ActionApprove || result.Level != LevelLow {
		t.Fatalf("ReviewLocal() = %+v, want clean local result", result)
	}
	result = DefaultPolicy().ReviewLocal("联系我 13800138000", 0)
	if result.Action != ActionReview || result.Level != LevelMedium {
		t.Fatalf("ReviewLocal() = %+v, want local contact risk", result)
	}
}
