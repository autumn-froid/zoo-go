请对当前 Pull Request 中的 **微信小程序前端**变更做最终自动审查。

## 范围

优先关注：
- `miniprogram/`（原生微信小程序，主交付物）
- 若有变更：`pet-feeding-miniapp/`（原型/辅助前端，需遵循其 `AGENT_RULES.md`）

忽略纯 Go 后端无关文件。

## 工作方式

1. 阅读完整 Diff，以及相关页面、组件、`app.js`、`utils/` 与配置。
2. 必要时对照后端 API / 鉴权约定（例如 `internal/httpapi` 中相关接口）。
3. 不要修改代码，不要创建 commit / branch / 新 PR。
4. 若没有小程序相关实质变更，发表一条简短 Review 说明「无小程序审查项」即可。

## 审查重点

- 登录 / token 存储与失效处理（`wx.login`、本地 storage、请求头）
- `wx.request` 错误处理、超时、非 2xx、业务 error code
- 页面生命周期与状态竞态（重复提交、过期回调）
- 用户可见文案、空态/加载态/失败态是否完整
- 微信能力使用是否正确（权限、兼容、隐私合规）
- 硬编码密钥、调试后门、把敏感信息打进日志/UI
- `pet-feeding-miniapp` 变更还需检查 Tailwind/组件规范与路由约定

## 输出格式（必须）

1. 通过 **GitHub Pull Request Review** 发布结果（不要只写普通 issue comment）。
2. 对每个具体问题在对应代码行添加 **中文** 行内评论，格式：

```text
[Critical|High|Medium|Low] 简短标题

问题：...
影响：...
建议：...
```

3. Review 正文使用中文，结构如下：

```markdown
## 模块
微信小程序前端

## 结论
Approve / Comment / Request changes（按最高严重级别选择：存在 Critical/High 用 Request changes，仅 Medium/Low 用 Comment，无问题用 Approve）

## 摘要
1-3 句说明本次变更风险。

## 发现
### Critical
- ...
### High
- ...
### Medium
- ...
### Low
- ...

## 测试建议
- ...
```

4. 没有某级别问题时写「无」。
5. 优先报告真实缺陷与风险；纯样式偏好克制输出。
