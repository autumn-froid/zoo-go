package agreement

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

const (
	RoleOwner  = "OWNER"
	RoleSitter = "SITTER"

	ItemService = "service_agreement"

	IdentityConsentVersion = "2026-11-15"
)

const IdentityConsentContent = `实名核验授权说明：我授权平台及其依法选择的核验服务商，为账号安全、交易准入、反欺诈和争议处理目的，核验我提交的姓名与身份证件号码，并保存核验结果、时间及必要的安全日志。身份证件号码将加密存储，后台访问遵循最小权限并留存审计记录；除法律法规另有规定或为处理争议所必需外，不会向无关第三方提供。`

type Item struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type Definition struct {
	Version       string `json:"version"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	ContentHash   string `json:"contentHash"`
	RequiredItems []Item `json:"requiredItems"`
}

var ownerRequiredItems = []Item{
	{Code: "owner_health_truth", Label: "我已如实告知宠物健康状况、攻击史、用药和过敏情况。"},
	{Code: "owner_camera_disclosure", Label: "我已披露室内摄像头及录音设备；服务期间不得用于与服务无关的用途。"},
	{Code: "owner_media_consent", Label: "我同意宠托师为履约和争议处理拍摄必要照片，并仅在订单范围内使用。"},
	{Code: ItemService, Label: "我已阅读并同意《上门宠物照护服务三方确认书》。"},
}

var sitterRequiredItems = []Item{
	{Code: "sitter_identity_truth", Label: "我确认实名信息真实，并同意平台为核验、风控和争议处理留档。"},
	{Code: "sitter_visit_record", Label: "我同意按订单要求签到、拍摄上门与完成凭证，并如实记录异常。"},
	{Code: ItemService, Label: "我已阅读并同意《上门宠物照护服务三方确认书》。"},
}

const termsContent = `《上门宠物照护服务三方确认书》

一、服务范围
平台提供宠物主与宠托师之间的信息匹配、订单留痕和争议协助，不提供动物诊疗、寄养或运输服务。宠物主与宠托师是服务的直接提供与接受方，平台不是宠物医疗或人身财产的保险人。

二、宠物主告知义务
宠物主应在下单前如实告知宠物疾病、传染病、攻击史、过敏、用药、饮食禁忌、逃逸风险、室内摄像头及可能影响服务安全的事项。因隐瞒或错误告知造成的损失，由宠物主依法承担相应责任。

三、宠托师履约义务
宠托师应按约定时间、清单和记录规范上门，不得擅自改变服务范围、转委托他人、带走宠物或财物，不得对宠物实施暴力。发现宠物异常、房屋安全风险或与描述不符时，应立即停止危险操作并联系宠物主和平台。

四、摄像头与影像
宠物主应披露室内摄像头和录音设备，影像仅限照护、安全和争议处理使用。宠托师应按订单要求拍摄必要的工作凭证，不得拍摄与履约无关的隐私，不得将影像用于宣传、传播或商业用途。

五、责任划分
宠物自身疾病突发、既往病史、主人未披露的风险，按宠物主过错和因果关系处理；宠托师违反操作规范、擅自处分财物或重大过失造成损失的，由宠托师承担相应责任；平台已履行审核、留痕、提示和争议协助义务的，在法律规定范围内不承担直接服务责任。

六、异常与事故处理
发生宠物受伤、走失、死亡、人身伤害或财物损失时，双方应立即救助、保护现场并通知平台。平台可依据订单记录、聊天记录、照片、签到记录和双方陈述组织协商；协商不成，任何一方可依法向有管辖权的机构主张权利。

七、个人信息
实名信息、身份证件号码、学籍材料和详细地址仅用于身份核验、履约安全和争议处理。平台采用加密存储、最小权限访问和操作留痕；未经授权不得向无关第三方披露，法律法规另有规定的除外。

八、生效与留档
双方在发布任务、提交报价或确认成交前勾选确认本确认书，即表示已阅读、理解并同意。平台保存协议版本、内容摘要、确认时间、网络地址和设备信息，作为订单履约与争议处理凭证。`

func Current(role string) (Definition, bool) {
	var items []Item
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case RoleOwner:
		items = append([]Item(nil), ownerRequiredItems...)
	case RoleSitter:
		items = append([]Item(nil), sitterRequiredItems...)
	default:
		return Definition{}, false
	}
	version := "2026-11-15"
	title := "上门宠物照护服务三方确认书"
	return Definition{
		Version: version, Title: title, Content: termsContent,
		ContentHash:   hashDefinition(version, title, termsContent, items),
		RequiredItems: items,
	}, true
}

func IdentityConsentHash() string {
	sum := sha256.Sum256([]byte(IdentityConsentVersion + "\n" + IdentityConsentContent))
	return hex.EncodeToString(sum[:])
}

func Validate(role, version string, accepted []string) error {
	definition, ok := Current(role)
	if !ok {
		return errors.New("agreement role is invalid")
	}
	if strings.TrimSpace(version) != definition.Version {
		return errors.New("agreement version is outdated")
	}
	present := make(map[string]bool, len(accepted))
	for _, item := range accepted {
		present[strings.TrimSpace(item)] = true
	}
	for _, item := range definition.RequiredItems {
		if !present[item.Code] {
			return errors.New("required agreement item is missing")
		}
	}
	return nil
}

func NormalizedItems(role string, accepted []string) []string {
	definition, ok := Current(role)
	if !ok {
		return []string{}
	}
	present := make(map[string]bool, len(accepted))
	for _, item := range accepted {
		present[strings.TrimSpace(item)] = true
	}
	result := make([]string, 0, len(definition.RequiredItems))
	for _, item := range definition.RequiredItems {
		if present[item.Code] {
			result = append(result, item.Code)
		}
	}
	sort.Strings(result)
	return result
}

func hashDefinition(version, title, content string, items []Item) string {
	var builder strings.Builder
	builder.WriteString(version)
	builder.WriteByte('\n')
	builder.WriteString(title)
	builder.WriteByte('\n')
	builder.WriteString(content)
	codes := make([]string, 0, len(items))
	for _, item := range items {
		codes = append(codes, item.Code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		builder.WriteByte('\n')
		builder.WriteString(code)
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}
