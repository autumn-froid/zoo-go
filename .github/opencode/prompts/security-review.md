请对当前 Pull Request 做 **Security Review**（安全专项审查）。

## 范围

审查本次 Diff 中所有可能引入安全风险的变更，尤其：
- Go 后端：`internal/httpapi/`、`internal/store/`、`internal/payment/`、`cmd/`
- 微信小程序：`miniprogram/`、`pet-feeding-miniapp/`
- 配置与部署：`.env*`、`deploy/`、`Dockerfile`、workflow secrets 引用方式

不要修改代码，不要创建 commit / branch / 新 PR。
若无明显安全相关变更，发表一条简短 Review 说明「无安全审查项」即可。

## 必查四类问题

### 1. 越权（Authorization / IDOR）
- 接口是否只校验“已登录”，未校验资源归属（task/order/user 是否属于当前用户）
- 主人/学生/管理员角色是否可互相访问对方数据或操作
- 路径参数 / body 中的 user_id、order_id 是否可被篡改越权
- 支付回调、退款、管理员接口是否缺少鉴权或签名校验
- 是否存在可跳过状态机校验的强制状态写入

### 2. SQL 注入（Injection）
- 是否用字符串拼接构造 SQL（即使部分参数来自内部也需警惕）
- 动态 ORDER BY / 表名 / 列名是否未白名单校验
- 原始查询、迁移脚本、管理查询是否可控输入直接进入 SQL
- 是否错误地把用户输入拼进 `LIKE`、`IN (...)` 等子句

### 3. Token 泄露
- Access token / session token 是否被写入日志、错误信息、埋点、截图文案
- 响应体是否多余返回 token、密钥、内部身份标识
- 小程序是否把 token 放进 URL query、分享路径、可被第三方读取的存储
- 测试/调试代码是否硬编码有效 token 或长期后门登录

### 4. 敏感信息
- API Key、微信密钥、支付商户密钥、私钥、证书是否进入 Diff 或仓库
- 身份证、手机号、地址、学生证明等 PII 是否过度返回给前端或日志
- `.env`、密钥文件、CI secret 是否被明文打印
- 错误响应是否泄露堆栈、SQL、内部路径

## 严重级别参考

- **Critical**：可直接越权读写他人资金/订单、可注入、密钥/Token 已进入可利用位置
- **High**：鉴权缺口明确、敏感数据可被非授权角色读取、高概率可利用
- **Medium**：需特定条件利用、防护不足但有缓解
- **Low**：加固建议、信息不充分暴露、防御性改进

## 输出格式（必须）

1. 通过 **GitHub Pull Request Review** 发布结果（不要只写普通 issue comment）。
2. 对每个具体问题在对应代码行添加 **中文** 行内评论，格式：

```text
[Critical|High|Medium|Low] 简短标题

类别：越权|SQL注入|Token泄露|敏感信息
问题：...
攻击面：...
建议：...
```

3. Review 正文使用中文，结构如下：

```markdown
## 模块
Security Review

## 结论
Approve / Comment / Request changes（存在 Critical/High 用 Request changes，仅 Medium/Low 用 Comment，无问题用 Approve）

## 摘要
1-3 句说明本次变更的安全风险。

## 发现
### Critical
- ...
### High
- ...
### Medium
- ...
### Low
- ...

## 覆盖检查
- 越权：已检查 / 不适用
- SQL 注入：已检查 / 不适用
- Token 泄露：已检查 / 不适用
- 敏感信息：已检查 / 不适用
```

4. 没有某级别问题时写「无」。
5. 只报安全问题；不要把普通 bug、风格问题混进本 Review，除非它们直接构成安全风险。
