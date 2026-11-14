请对当前 Pull Request 中的 **Go 后端**变更做最终自动审查。

## 范围

仅关注这些路径的变更（忽略无关模块）：
- `cmd/`
- `internal/`
- `go.mod` / `go.sum`
- `.golangci.yml`
- `Dockerfile` / `deploy/` / `scripts/` 中与后端相关的部分

## 工作方式

1. 阅读完整 Diff，以及被改动符号的相关调用方、测试与领域文档。
2. 必要时阅读：
   - `docs/order-state-machine.md`（订单/支付/退款状态机）
   - `docs/student-verification.md`
   - `docs/dispatch-algorithm.md`
   - `docs/observability.md`
3. 不要修改代码，不要创建 commit / branch / 新 PR。
4. 若没有后端相关实质变更，发表一条简短 Review 说明「无后端审查项」即可。

## 审查重点

- 正确性与边界条件（空值、超时、重试、幂等）
- 并发与数据一致性（SQLite 事务、唯一索引、竞态）
- 支付/退款安全（回调验签、金额校验、状态机违规）
- 错误处理与日志（是否吞错、是否泄露敏感信息）
- API 契约破坏、向后兼容、鉴权与权限
- 测试缺口：高风险逻辑是否缺回归测试

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
Go 后端

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
5. 优先报告真实缺陷与风险；风格偏好等低优先级问题克制输出。
