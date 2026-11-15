# 实名、个人信息加密与协议留痕

## 1. 小范围试跑采用的证据链

试跑阶段采用以下组合，而不是仅信任客户端传来的 `agreementAccepted=true`：

1. 用户提交真实姓名、身份证件号码和实名核验授权。
2. 运营人员通过微信实名、持牌第三方核验服务或人工方式确认人证一致。
3. 宠物主发布任务前逐项确认健康告知、摄像头披露、影像使用范围和当前版本协议。
4. 宠托师报价前逐项确认实名真实性、上门记录规范和当前版本协议。
5. 服务端记录协议版本、正文摘要、确认项目、用户、角色、任务、报价、订单、时间、IP 和 User-Agent。
6. 成交时服务端再次校验双方协议版本一致且记录完整，否则拒绝成交。

该方案适合小范围试跑和争议留痕，不等同于可靠电子签名。高风险、高金额或需要更强举证效力的订单，应接入腾讯电子签、e签宝、法大大等持牌电子签名服务，并由律师确认最终协议文本。

## 2. 密钥配置

生产环境启动时必须配置：

```bash
PII_ENCRYPTION_KEY=<64 位十六进制字符串>
PII_HASH_KEY=<另一个 64 位十六进制字符串>
```

生成方式：

```bash
openssl rand -hex 32
openssl rand -hex 32
```

要求：

- 两个密钥必须不同，并和数据库备份分开保存。
- 丢失 `PII_ENCRYPTION_KEY` 后，已有实名和学籍资料无法解密。
- 更换密钥前必须先编写并验证数据重加密工具，不能直接替换环境变量。
- 不要把密钥写入代码、日志、工单、聊天记录或 Git。

`deploy/deploy.sh` 在服务器 `deploy/.env` 缺少密钥时会一次性生成并写回文件。自动生成适合单机试跑；正式运营建议使用云密钥管理服务或独立密钥托管。

## 3. 加密范围

当前使用 AES-256-GCM，随机 nonce，字段作用域作为 AAD；密文格式为 `enc:v1:<base64url>`。确定性查询使用带独立密钥的 HMAC-SHA256，不保存明文索引。

已加密字段：

- `identity_verifications.real_name_enc`
- `identity_verifications.id_number_enc`
- `student_verifications.real_name`
- `student_verifications.student_no`
- `student_verifications.major`
- `student_verifications.chsi_code`
- `owner_addresses.detail`
- `owner_addresses.location_name`
- `tasks.address_detail`
- `tasks.location_name`

身份证件号码额外保存掩码和 HMAC 查询值，用于后台核验与防止同一证件绑定多个账号。用户端只返回掩码，后台审核接口才返回完整号码并设置 `Cache-Control: no-store`。

启动时会自动迁移旧明文数据；已加密数据会先验证能否解密，再决定是否跳过迁移。迁移失败会阻止服务启动，避免带病运行。

## 4. 协议版本管理

协议正文和必选项在 `internal/agreement/agreement.go`。当前版本为 `2026-11-15`。

修改协议时必须同时提升版本号，例如改为 `2026-12-01`。服务启动会校验数据库中相同版本的内容摘要；如果正文发生变化但版本未变，启动会失败。历史协议记录不会被覆盖。

接口：

- `GET /v1/agreements/current?role=owner`
- `GET /v1/agreements/current?role=sitter`
- `GET /v1/orders/{id}/agreement`

## 5. 实名认证接口

- `GET /v1/identity/verification`：当前用户的实名状态与授权说明。
- `POST /v1/identity/verification`：提交姓名、身份证件号码和授权确认。
- `GET /v1/admin/identity-verifications?status=PENDING`：后台待审列表。
- `POST /v1/admin/identity-verifications/{id}/approve`：人工/第三方核验通过。
- `POST /v1/admin/identity-verifications/{id}/reject`：驳回并记录原因。

当前 `provider` 为 `MANUAL_REVIEW`。接入微信实名或持牌服务后，应仅在服务商返回明确成功结果时自动通过，并继续保留原始核验流水号、时间和响应摘要。

## 6. 上线前检查

- 配置并备份两个 PII 密钥。
- 验证数据库中的实名、学籍和地址字段不再以明文出现。
- 验证旧数据库能启动并完成迁移。
- 验证未实名用户不能发布任务和报价。
- 验证协议版本不一致时不能成交。
- 验证后台实名列表只对管理员开放，日志和错误信息不含身份证件号码。
- 对正式协议文本进行律师审阅，并评估是否接入可靠电子签名。
