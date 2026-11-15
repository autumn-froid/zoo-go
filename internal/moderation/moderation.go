package moderation

import (
	"regexp"
	"strings"
	"unicode"
)

type Action string

const (
	ActionApprove Action = "APPROVE"
	ActionReview  Action = "REVIEW"
	ActionReject  Action = "REJECT"
)

type Level string

const (
	LevelLow    Level = "LOW"
	LevelMedium Level = "MEDIUM"
	LevelHigh   Level = "HIGH"
)

type Result struct {
	Action  Action
	Level   Level
	Reasons []string
	Matches []string
}

type Policy struct {
	AutoApproveText    bool
	RequireImageReview bool
	BlockedTerms       []string
}

func DefaultPolicy() Policy {
	return Policy{
		AutoApproveText:    false,
		RequireImageReview: true,
		BlockedTerms:       append([]string(nil), defaultBlockedTerms...),
	}
}

func (p Policy) Review(content string, imageCount int) Result {
	compact := compactText(content)
	matches := []string{}
	for _, term := range p.blockedTerms() {
		normalized := compactText(term)
		if normalized == "" {
			continue
		}
		if strings.Contains(compact, normalized) {
			matches = append(matches, term)
		}
	}
	if len(matches) > 0 {
		return Result{
			Action:  ActionReject,
			Level:   LevelHigh,
			Reasons: []string{"内容包含平台禁止的违法、低俗或伤害动物信息"},
			Matches: matches,
		}
	}

	reasons := []string{}
	level := LevelLow
	if urlPattern.MatchString(widthNormalized(content)) {
		reasons = append(reasons, "包含外部链接或站外导流信息")
		level = LevelMedium
	}
	if phonePattern.MatchString(compact) {
		reasons = append(reasons, "疑似包含手机号等联系方式")
		if level == LevelLow {
			level = LevelMedium
		}
	}
	if idPattern.MatchString(compact) {
		reasons = append(reasons, "疑似包含身份证号等敏感个人信息")
		level = LevelHigh
	}
	for _, term := range reviewTerms {
		if strings.Contains(compact, compactText(term)) {
			reasons = appendUnique(reasons, "包含站外联系或交易引导")
			if level == LevelLow {
				level = LevelMedium
			}
			break
		}
	}
	if imageCount > 0 && p.RequireImageReview {
		reasons = append(reasons, "包含图片，需人工复核图片内容")
		if level == LevelLow {
			level = LevelMedium
		}
	}
	if len(reasons) > 0 {
		return Result{Action: ActionReview, Level: level, Reasons: reasons}
	}
	if !p.AutoApproveText {
		return Result{Action: ActionReview, Level: LevelMedium, Reasons: []string{"新动态需人工审核"}}
	}
	return Result{Action: ActionApprove, Level: LevelLow}
}

func (p Policy) blockedTerms() []string {
	if len(p.BlockedTerms) == 0 {
		return defaultBlockedTerms
	}
	return p.BlockedTerms
}

func compactText(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		if r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff' {
			continue
		}
		if r >= 0xFF01 && r <= 0xFF5E {
			r -= 0xFEE0
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || r == 0x3000 {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func widthNormalized(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		if r >= 0xFF01 && r <= 0xFF5E {
			r -= 0xFEE0
		} else if r == 0x3000 {
			r = ' '
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

var (
	phonePattern = regexp.MustCompile(`(?:86)?1[3-9]\d{9}`)
	idPattern    = regexp.MustCompile(`\d{17}[\dx]`)
	urlPattern   = regexp.MustCompile(`(?i)(https?://|www\.|[a-z0-9-]+\.(?:com|cn|net|org|io|me|top|xyz)\b)`)
)

var defaultBlockedTerms = []string{
	"色情", "裸聊", "约炮", "成人视频", "涉黄",
	"赌博", "博彩", "六合彩", "洗钱", "刷单", "杀猪盘", "诈骗",
	"毒品", "冰毒", "海洛因", "大麻", "摇头丸",
	"枪支", "弹药", "炸药", "雷管",
	"代开发票", "办假证", "假证", "黑客", "木马", "钓鱼网站",
	"虐猫", "虐狗", "毒狗", "捕杀", "活体投喂",
}

var reviewTerms = []string{
	"微信", "vx", "威信", "薇信", "加我", "私聊", "手机号", "留个电话",
	"扫码", "二维码", "支付宝", "转账", "闲鱼", "淘宝", "私下交易", "现金交易",
	"加群", "招代理", "兼职", "出售", "转让", "代购", "配种", "繁殖", "活体",
}
