# UGC 动态内容审核

社区动态的风险分为文字和图片两类。当前版本采用“本地硬规则拦截 + 微信文字/图片机审 + 媒体签名访问 + 人工兜底”的保守策略：机器通过后才自动公开，机器服务异常、结果不确定或超时一律转人工。完整需求、状态机、流程图和分阶段验收见 [`ugc-moderation-requirements.md`](ugc-moderation-requirements.md)。

## 1. 当前实现

发布流程：

```text
发布动态
  → 校验内容和图片数量
  → 校验图片必须由当前用户上传
  → 校验账号状态和当日发帖/上传限额
  → 本地文字规则检测
      → 命中硬违规：直接拒绝，不入库
      → 命中疑似风险：进入 PENDING
      → 未命中风险：进入微信文字机审
          → risky：REJECTED
          → pass + 无图 + 自动通过开启：APPROVED
          → pass + 有图：提交异步图片审核，进入 MACHINE_REVIEWING
          → review / 超时 / 服务异常：PENDING 人工复核
  → 图片回调全部 pass：APPROVED
  → 任一图片 risky：REJECTED
  → 任一图片 review / error / 超时：PENDING 人工复核
  → 运营后台人工通过 / 驳回
  → APPROVED 才出现在其他用户的动态流
  → 图片通过短时签名 URL 访问，未签名地址返回 403
```

关键行为：

- `community_posts.moderation_status` 取值为 `PENDING`、`MACHINE_REVIEWING`、`APPROVED`、`REJECTED`。
- 动态流只返回 `APPROVED`，但发布者可以看到自己的待审核和已驳回动态。
- 旧数据迁移时自动标记为 `APPROVED`，避免升级后历史动态突然消失。
- 含图片的动态在机审开关开启时先进入 `MACHINE_REVIEWING`，只有所有图片均返回 `pass` 才自动通过。
- 后台通过或驳回都会写入审计日志，驳回原因会展示给发布者。
- `GET /uploads/*` 不再允许匿名直接读取；API 返回的上传 URL 带有 `exp` 和 `sig` 签名参数，默认 1 小时有效。
- 新用户每日最多发布 3 条动态，普通用户 10 条；每日最多上传 30 张图片；`DISABLED` 账号禁止发布和上传。
- 待审队列默认按 `HIGH → MEDIUM → LOW → UNKNOWN` 排序，同风险级别先处理等待更久的内容。
- 图片机审记录保存在 `community_media_reviews`，按微信 `trace_id` 幂等更新；30 分钟未回调自动转人工，超时不等于通过。

## 2. 文字风险

本地规则在 `internal/moderation` 中实现，包含：

- 全角字符、空格、标点、零宽字符归一化，降低“加 微 信”“赌 博”这类绕过。
- 硬拦截词：色情、赌博、诈骗、毒品、枪爆、假证、洗钱、虐宠等。
- 站外导流和联系方式：URL、手机号、身份证号、微信/VX、支付宝、转账、二维码、私下交易等，进入人工复核。
- 图片动态进入人工复核。

本地规则适合做第一层兜底，不适合作为唯一合规能力。生产环境建议接入：

| 方案 | 适用场景 | 说明 |
|---|---|---|
| 微信 `security.msgSecCheck` | 微信小程序 UGC 文字 | 官方接口，需要 `access_token`、场景值和用户 `openid`，适合文本主审核 |
| 腾讯云内容安全 TMS | 文字审核 | 分类更细，适合需要申诉、置信度和人工复核队列的场景 |
| 阿里云内容安全 Green | 文字审核 | 支持多场景标签和自定义词库 |
| 本地 DFA 词库（如 `github.com/importcjj/sensitive`） | 前置过滤 | 速度快、成本低，但词库维护和变体识别需要持续投入 |

建议组合：本地硬规则快速拒绝明显违规内容，微信或云厂商接口给出风险分，低风险自动放行，中高风险进入人工队列。

当前 P1 已接入微信 `msgSecCheck`：设置 `UGC_MACHINE_REVIEW_ENABLED=true` 后，发布接口会使用用户 `openid` 调用文字机审。`risky` 直接驳回，`review`、超时或接口异常进入人工队列；本地联系方式/导流规则仍然优先保留人工复核。

## 3. 图片风险

图片内容审核需要依赖视觉模型或云服务，Go 标准库无法判断图片语义。可选方案：

| 方案 | 适用场景 | 说明 |
|---|---|---|
| 微信 `security.mediaCheckAsync` | 小程序图片/音频 | 异步回调，需要图片公网可访问 URL 和回调地址，适合当前小程序体系 |
| 腾讯云图片内容安全 IMS | 图片审核 | 支持色情、暴恐、政治敏感、广告等标签 |
| 阿里云内容安全图片审核 | 图片审核 | 支持多场景、机审+人审 |
| 自建模型 | 大规模或特殊品类 | 成本高，需要标注、训练和持续评估 |

接入异步图片审核时，建议把动态状态扩展为：

```text
PENDING → MACHINE_REVIEWING → APPROVED / REJECTED / PENDING（人工复核）
```

回调必须验签，并按 `trace_id` 或本地业务 ID 做幂等更新。图片审核未返回前，动态不能出现在公开动态流。

当前 P1 已接入微信 `mediaCheckAsync`：

- 后端为每张图片生成 15 分钟有效的签名公网 URL，提交后保存微信 `trace_id`。
- 微信回调地址配置为 `/v1/wechat/content-security/notify`。
- 支持明文签名和 `EncodingAESKey` 安全模式；回调时间戳窗口 10 分钟。
- 单张图片 `risky` 立即驳回整条动态；全部 `pass` 才自动通过；`review`、错误或 30 分钟超时转人工。
- 运营后台可按 `MACHINE_REVIEWING` 筛选处理中的机审动态。

## 4. 配置

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `UGC_AUTO_APPROVE_TEXT` | `false` | 纯文字且未命中风险时是否自动通过；接入可靠文本审核前建议保持 `false` |
| `UGC_REQUIRE_IMAGE_REVIEW` | `true` | 未启用机审时，含图片是否必须人工复核；P1 开启后由图片回调决定 |
| `UGC_BLOCKED_TERMS` | 空 | 追加自定义硬拦截词，英文逗号分隔 |
| `UPLOAD_SIGNING_KEY` | 空 | 媒体 URL 的 HMAC 签名密钥；生产环境必须设置高强度随机值 |
| `UGC_MACHINE_REVIEW_ENABLED` | `false` | 是否启用微信文字/图片机审；缺省关闭，保持 P0 人工模式 |
| `PUBLIC_BASE_URL` | 空 | 微信拉取图片使用的公网 HTTPS 根地址，例如 `https://api.example.com` |
| `WECHAT_CONTENT_SECURITY_TOKEN` | 空 | 微信内容安全回调 Token，必须与小程序后台消息推送配置一致 |
| `WECHAT_CONTENT_SECURITY_AES_KEY` | 空 | 可选。微信消息推送安全模式的 43 位 EncodingAESKey |

## 5. 上线前还需要补充

P0/P1 已完成本地私有媒体短时签名、发布频控、停用账号禁发、审核队列排序，以及微信文字机审、异步图片机审、验签回调、超时降级和幂等更新。距离完整治理闭环还需要：

- 增加同设备/同 IP 风控、重复正文/图片检测、用户举报入口和已通过内容强制下架。
- 明确审核 SLA、分级标准、申诉流程和违规账号处置策略。
- 审核日志记录真实操作人、来源 IP、请求 ID 和审核前后状态。
- 将本地私有盘升级为私有对象存储，并限制签名 URL 的有效期和访问日志。
