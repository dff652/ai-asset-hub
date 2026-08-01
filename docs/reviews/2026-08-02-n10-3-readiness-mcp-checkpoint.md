# 2026-08-02 N10.3 迁移准备只读 MCP 检查点

- 状态：源码候选；待 review / CI / 合入 `dev`
- 范围：`aiah_migration_readiness`、ADR-0005 工具清单 7→8、write-nothing 覆盖
- 不在范围：MCP 写操作、N10.4 证据记录器、正式发布
- 设计：[N10：迁移准备检查](../designs/migration-readiness.md)
- ADR：[0005 只读 MCP](../decisions/0005-read-only-mcp-server-surface.md)

## 1. 已实现

- 新工具 `aiah_migration_readiness`：参数与 CLI 只读入口对齐（workspace、profile
  必填；manifest/home/project/backupEvidence/restoreExercise 可选）；
- handler 仅调用 `readiness.Inspect`，返回 `kind=migration-readiness` 报告；
- `inputSchema.additionalProperties=false`，统一 `readOnlyHint=true` 等注解；
- `TestToolCallsWriteNothing` 增加该工具参数，并快照 `.aiah/evidence` 目录；
- 未知字段、缺 profile、协议 tools/call 与 reference 不回显均有测试。

## 2. 验证

- `go test ./internal/mcp`
- 变异：跳过 Core / 工具调用写证据目录 → 目标测试变红后恢复
- `./scripts/check-local.sh`

## 3. 分类

| 层级 | 状态 |
|---|---|
| 源码候选 | 是 |
| 已合入 `dev` | 否（待 PR/CI） |
| 正式发布 | 否 |
| 安装包 MCP 验收 | 否 |

## 4. 未做

- 真实客户端模型级 dogfood（协议测试已覆盖 tools/list + tools/call）
- N10.4 与正式 Release
