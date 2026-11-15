# 运营管理平台

运营后台由 Go 服务直接托管，入口为：

```text
http://127.0.0.1:8080/admin/
```

## 1. 登录方式

后台使用服务端 `ADMIN_TOKEN` 进行访问验证。登录成功后，令牌只保存在当前标签页的 `sessionStorage` 中，关闭标签页即失效。

本地开发：

```bash
DEV_AUTH=true ADMIN_TOKEN=dev-admin ./scripts/run-local.sh
```

图片和审核材料通过短时签名 URL 访问，生产环境还需要配置独立的媒体签名密钥：

```bash
UPLOAD_SIGNING_KEY="$(openssl rand -hex 32)" ./scripts/run-local.sh
```

未设置时服务会生成进程级临时密钥，服务重启后旧图片 URL 会失效；多实例部署必须使用同一个固定密钥。

生产环境必须使用高强度随机值，并通过 HTTPS 暴露服务：

```bash
ADMIN_TOKEN="$(openssl rand -hex 32)" ./scripts/run-local.sh
```

所有后台 API 都要求请求头：

```http
X-Admin-Token: <ADMIN_TOKEN>
```

不要把 `ADMIN_TOKEN`、`WECHAT_APP_SECRET` 或微信支付私钥写进小程序、前端源码或公开仓库。

## 2. 功能模块

| 模块 | 能力 |
|---|---|
| 运营概览 | 查看用户、待审学籍、待审退款、进行中订单、完成订单、今日订单和成交金额 |
| 派单对比 | 使用真实待派任务和已认证学生位置，对比 5km 内贪心与 KM 的匹配结果 |
| 学籍审核 | 按状态筛选学籍申请；核对姓名、学校、学号、验证码后通过或驳回 |
| 退款审核 | 查看退款申请；通过后调用微信原路退款，或填写原因驳回 |
| 用户管理 | 按姓名、用户 ID、学校、手机号搜索；停用或恢复平台账号 |
| 订单管理 | 按状态查看订单、交易双方、宠物、服务时间、金额和支付状态 |
| 动态审核 | 复核社区图文动态；含图片的新动态默认进入人工审核队列 |
| 审计日志 | 查看学籍审核、退款审核和账号处置记录 |

停用账号会立即删除该用户全部登录会话，后续登录返回 `403`。恢复账号后，用户可重新走微信登录。停用原因最多 200 字，恢复时会清空停用原因。

## 3. 审核规则

学籍审核通过时默认设置一年有效期，也可以按学信网报告有效期设置日期。驳回必须填写具体、可执行的原因，学生会看到该原因并可以重新提交。

退款通过前应核对：

1. 订单仍处于允许退款的已支付状态。
2. 服务尚未开始；迟到支付退款按系统规则单独处理。
3. 退款金额与原支付金额一致。
4. 退款原因与订单实际履约情况一致。

退款接口会执行事务状态迁移和幂等保护，后台按钮不能绕过这些校验。

社区动态审核规则：

1. 新动态默认进入 `PENDING`，只有 `APPROVED` 会出现在其他用户的动态流中。
2. 发布者可以在社区页看到自己的待审核或已驳回动态，以及后台填写的驳回原因。
3. 命中违法、低俗、赌博诈骗、伤害动物等硬规则时，接口直接返回 `400`，内容不会入库。
4. 图片无法用本地规则判断内容，默认必须人工复核；生产环境应接入微信 `mediaCheckAsync` 或云内容审核后再考虑自动放行。
5. 图片必须由发布者本人上传，防止复用他人上传文件或站外 URL。
6. 待审队列按 `HIGH → MEDIUM → LOW → UNKNOWN` 排序，同风险按等待时长升序，优先处理高风险和等待更久的内容。
7. 新用户每日最多 3 条动态，普通用户 10 条，每日最多上传 30 张图片；超限返回 `429`。
8. `DISABLED` 账号禁止发布动态和上传图片；审核台图片使用短时签名 URL，过期后刷新队列即可重新获取。

更完整的需求、状态机、流程图和分阶段验收见 [`ugc-moderation-requirements.md`](ugc-moderation-requirements.md)，文本/图片检测选型见 [`ugc-moderation.md`](ugc-moderation.md)。

## 4. 派单对比

“派单对比”读取当前报价未截止的竞价任务和已认证、已上报定位的学生，生成 5km 内的候选边，分别运行贪心算法与 KM（匈牙利算法）。页面展示匹配数、总得分、总距离和逐条分配明细。

当前模块是只读预演：

1. 不会自动选中报价或创建订单。
2. 不会锁住学生时段、发起支付或改变订单状态。
3. 不向页面返回任务或学生的精确经纬度。

综合分、距离规则、算法差异和自动派单所需条件见 [`dispatch-algorithm.md`](dispatch-algorithm.md)。

## 5. 管理 API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/v1/admin/overview` | 运营概览 |
| GET | `/v1/admin/dispatch/compare?limit=` | 贪心与 KM 只读派单对比 |
| GET | `/v1/admin/users?q=&status=` | 用户查询，状态支持 `ALL`、`ACTIVE`、`DISABLED` |
| POST | `/v1/admin/users/{id}/status` | 停用或恢复用户，请求体为 `{"status":"DISABLED","reason":"..."}` |
| GET | `/v1/admin/orders?status=` | 订单查询 |
| GET | `/v1/admin/community/posts?status=` | 动态审核查询，状态支持 `PENDING`、`APPROVED`、`REJECTED`、`ALL` |
| POST | `/v1/admin/community/posts/{id}/approve` | 通过动态审核 |
| POST | `/v1/admin/community/posts/{id}/reject` | 驳回动态，请求体为 `{"reason":"..."}` |
| GET | `/v1/admin/audit-logs?limit=` | 审计日志查询 |
| GET | `/v1/admin/student-verifications?status=` | 学籍申请查询 |
| POST | `/v1/admin/student-verifications/{id}/approve` | 通过学籍认证 |
| POST | `/v1/admin/student-verifications/{id}/reject` | 驳回学籍认证 |
| GET | `/v1/admin/refunds?status=` | 退款申请查询 |
| POST | `/v1/admin/refunds/{id}/approve` | 审核通过并发起微信退款 |
| POST | `/v1/admin/refunds/{id}/reject` | 驳回退款申请 |

## 6. 生产边界

当前 `ADMIN_TOKEN` 是单管理员 MVP 边界，不是完整的企业后台身份体系。正式上线前至少应补充：

- 独立运营账号、密码或企业微信/SSO 登录。
- 角色权限控制（RBAC），区分审核员、客服、财务和超级管理员。
- MFA/二次认证、登录失败限流、令牌轮换与强制下线。
- 反向代理层的 HTTPS、IP 白名单、访问频率限制和 WAF。
- 审计日志写入独立、防篡改的存储，并记录操作人账号、来源 IP 和请求 ID。
- 敏感资料脱敏、最小权限展示、导出审批和数据保留策略。
- 派单对比目前只考虑距离、评分和完成经验；若用于真实自动派单，还需要位置时效、可接单状态、时段冲突、唯一领取和通知补偿。

当前审计日志的 `actor` 固定为 `admin`。接入管理员账号体系后，应改为真实操作人 ID，并保证审核动作与审计写入处于同一事务或可靠事件链路中。
