# 2026-08-02 N10.2 迁移准备 TUI 检查点

- 状态：源码候选；本地完整门禁与变异验证已通过，待 review / CI / 合入 `dev`
- 范围：TUI「换机与备份」页面、首页入口、中英文 message catalog、零写入测试
- 不在范围：MCP、证据记录器、隔离恢复演练编排、正式发布
- 设计：[N10：迁移准备检查](../designs/migration-readiness.md)
- 前置：N10.1 已合入 `dev`（`093a619` / PR #43）

## 1. 已实现

- 首页任务 **换机与备份**；进入后选择 profile，调用同一 `readiness.Inspect` Core；
- 页面展示三项独立状态（可以打包 / 已记录外部副本 / 恢复已验证）与迁移前置、
  下一步文案；状态同时输出稳定枚举码与本地化标签，不依赖颜色；
- 证据路径仅指向资产库 `.aiah/evidence/`，打开与刷新为零隐式写入；
- 可用 `b` / `x` 显式指定或清除证据相对路径后再检查；
- 360/900 宽度保留枚举码与关键文案；中英文 golden 与 message catalog 对齐。

## 2. 验证

- `go test ./internal/tui`
- 三项变异：跳过 Core、打开时隐式写证据、去掉状态枚举码 → 目标测试变红后恢复
- `./scripts/check-local.sh` 全绿

## 3. 分类

| 层级 | 状态 |
|---|---|
| 源码候选 | 是（本检查点） |
| 已合入 `dev` | 否（待 PR/CI） |
| 正式发布 | 否 |
| 安装包验收 | 否 |

## 4. 未做

- N10.3 `aiah_migration_readiness` MCP
- N10.4 自动证据记录 / 隔离演练编排
- 正式 tag / installer pin / 安装包 TUI 验收
