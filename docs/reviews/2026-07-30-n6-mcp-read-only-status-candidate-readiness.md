# 2026-07-30 N6 MCP 只读状态与客户端验收检查点

## 1. 结论与边界

N6 候选把 AI 客户端可见的 MCP 表面从公开版 `v0.1.6` 的 5 个基础工具扩为
7 个只读工具：

- `aiah_asset_status`：把当前扫描结果与一个显式资产库合成统一资产状态；
- `aiah_migration_status`：对齐资产库、当前安装与一个可选的既有发布通道。

两个工具只编排现有 `inventory`、`workspace`、`migration` Core，不复制 TUI 规则。
MCP 仍不开放 build、资产库管理、publish、pull、apply 或 rollback，也不把“可调用”
误写成“可代用户修改”。

每个工具都通过 `tools/list` 声明：

```text
readOnlyHint=true
destructiveHint=false
idempotentHint=true
openWorldHint=false
```

输入 schema 使用 `additionalProperties=false`，handler 同时拒绝未知参数。两个状态工具
要求显式传入 `workspace`，不会猜测或创建资产库。

## 2. 实现与测试覆盖

- `aiah_asset_status` 先调用 `inventory.Scan`，再把资产列表交给
  `workspace.Catalog`，返回 `kind=asset-catalog`；
- `aiah_migration_status` 直接调用 `migration.Inspect`，返回
  `kind=migration-status`，并保留用户传入的可选 channel；
- 注册表和协议测试把 7 个工具视为完整、按名称排序的只读边界；
- `TestToolCallsWriteNothing` 对每个工具调用前后比较 fake HOME、project、资产库、
  通道和包目录的正文及 mode；
- 定向测试覆盖 Core 报告类型、显式资产库、通道选择、schema 与 annotations。

定向测试：

```text
go test ./internal/mcp ./internal/workspace ./internal/migration
```

通过。

## 3. 安全变异验证

五项临时变异均按预期变红，随后恢复：

1. `aiah_asset_status` 向资产库写入 `.mutation-marker`：
   `TestToolCallsWriteNothing` 报告 `created: .mutation-marker`；
2. `aiah_asset_status` 错误返回原始 inventory：
   `TestAssetStatusReturnsTheUnifiedCoreReport` 报告类型为 `inventory.Report`，
   期望 `workspace.CatalogReport`；
3. `aiah_migration_status` 丢弃 channel 参数：
   `TestMigrationStatusReturnsTheCrossDeviceCoreReport` 报告
   `Channel.Selected=false`；
4. 把 `readOnlyHint` 改为 false：
   `TestToolsListReportsSchemas` 报告只读 annotations 不完整；
5. CLI stdio 注册表把 `aiah_asset_status` 错接到 scan handler：
   `TestRunMCPCallsAssetStatusOverStdio` 收到工具级参数错误而失败。

恢复后 `go test ./internal/mcp -count=1` 重新通过；变异代码不在候选 diff 中。

## 4. 真实客户端验收

验收只使用 `/tmp` 下的候选二进制、fixture HOME、fixture 资产库和 disposable 配置。
直接 JSON-RPC 基线依次完成 initialize、tools/list 和两个新工具调用；客户端请求
`2025-11-25`，server 返回其支持的正式修订 `2025-06-18`，7 个工具与两个报告均正常。

| 客户端 | 版本 | 协议/握手 | 模型级调用 |
|---|---|---|---|
| Claude Code | 2.1.198 | `mcp get aiah` 为 `Connected` | blocked：组织策略在模型请求前返回 403，未产生工具调用 |
| Codex CLI | 0.145.0 | 发现候选 server | `aiah_asset_status` → `asset-catalog`, `ok=true` |
| Grok | 0.2.114 | protocol `2025-06-18`、7 tools、healthy | `aiah_migration_status` → `migration-status`, `ok=true` |

Codex 使用临时会话与只读 sandbox；Grok 使用 disposable project 配置，并只在自己创建、
内容已知的临时目录关闭 folder trust。Claude 的 `Connected` 只证明客户端握手，
不能替代模型调用成功；须在组织策略解除后补测。

全部调用后，fake HOME 和资产库仍与原始 fixture 一致。可复现步骤见
[MCP 客户端接入与只读验收](../runbooks/mcp-client-acceptance.md)。

## 5. 严格维护性审查

严格审查重点检查 Core 归属、handler 分支、文件规模、重复抽象和只读边界：

- 状态业务规则仍归 `workspace.Catalog` 与 `migration.Inspect`，MCP 只负责参数解析、
  错误分级和返回 Core 报告；
- 没有向 server 增加模式开关、写操作 dispatcher 或 MCP 私有状态模型；
- 最大改动文件 `internal/mcp/tools.go` 为 400 行，没有文件接近 1000 行门槛；
- annotations 在统一 descriptor 构造处添加，没有分散到各工具；
- 未发现需要阻止候选交付的结构性、边界或可维护性问题。

## 6. 候选交付状态

公开版 `v0.1.6` 仍只有 5 个基础只读工具；N6 已进入 `dev`，但仍不等同于已发布
能力。

最终文件树运行 `./scripts/check-local.sh` 通过，覆盖开发环境、许可证、
installer/Release/README 资产检查、全量 test、race、vet、gofmt、
golangci-lint 和 fake HOME build/diff/apply/doctor/rollback 闭环。

最终候选二进制重新完成直接 JSON-RPC 对账：protocol `2025-06-18`、7 个 schema
和 annotations、`asset-catalog`、`migration-status` 全部符合预期，fixture HOME
与资产库前后不变。

[PR #27](https://github.com/dff652/ai-asset-hub/pull/27) 面向 `dev` 创建，初始
交付 head `5ce80a86cbdf` 的 push CI
[`30534934851`](https://github.com/dff652/ai-asset-hub/actions/runs/30534934851)
和 pull_request CI
[`30534951487`](https://github.com/dff652/ai-asset-hub/actions/runs/30534951487)
各 9/9 job 全绿，共 18/18。

本证据更新只修改项目文档；推送后仍以 PR 最终 head 的两组 GitHub checks 为合入
门禁，不能用前一提交的绿灯替代。PR 合并和 Release 继续由项目所有者明确决定。

所有者确认后，最终 head `b2e50139a4c9` 的 push CI
[`30535101642`](https://github.com/dff652/ai-asset-hub/actions/runs/30535101642)
和 pull_request CI
[`30535103745`](https://github.com/dff652/ai-asset-hub/actions/runs/30535103745)
各 9/9 job 全绿。PR #27 于 2026-07-30 squash 合入
`dev@9eedd7bde8a9`；合并后的主线 CI
[`30549858137`](https://github.com/dff652/ai-asset-hub/actions/runs/30549858137)
共 9/9 job 全绿。

因此 N6 的本地、候选最终 head 和合并后门禁均已闭合。Claude 模型级调用仍因组织
策略 403 未执行；这是如实保留的外部补测项，不影响已验证的 server 协议、Claude
客户端握手、Codex/Grok 模型调用或 MCP 零写入边界。
