# 文档索引

## 用户指南

- [上手指南](getting-started.md)（盘点 → 工作区 → build → 假 HOME → 真机部署）
- [CLI 命令参考](cli-reference.md)
- [跨设备迁移 runbook](runbooks/cross-device-transfer.md)（发布 → 搬运 → 取回 → 安装）
- [真机 dry-run runbook](runbooks/real-home-dry-run.md)
- [AI 资产管理踩坑清单](troubleshooting/ai-asset-pitfalls.md)

## 现行设计

- [项目协作说明](../CLAUDE.md)（Claude Code / Codex 共用单一事实源）
- [总体架构](architecture.md)
- [资产模型](asset-model.md)
- [安全与隐私](security.md)
- [工程流程：开发 / 测试 / 构建 / 部署 / 发布](development.md)
- [开发环境搭建 SOP](runbooks/development-environment.md)
- [MVP 路线图](roadmap.md)
- [假 HOME 闭环 runbook](runbooks/fake-home-loop.md)
- [Public 发布 runbook](runbooks/public-launch.md)
- [漏洞报告政策](../SECURITY.md)

契约 schema：`spec/inventory.schema.json`、`manifest.schema.json`、
`validation.schema.json`、`build.schema.json`。

## 架构决策

- [0001：文件优先与 adapter 分发](decisions/0001-file-first-adapter-distribution.md)
- [0002：多 Target 能力模型与可插拔适配](decisions/0002-multi-target-capability-adapters.md)
- [0003：CLI-first、Go Core 与产品界面演进](decisions/0003-cli-first-go-core-and-product-surfaces.md)
- [0004：MCP 原生配置所有权与安全写入](decisions/0004-native-mcp-config-ownership.md)
- [0005：MCP server 只读边界](decisions/0005-read-only-mcp-server-surface.md)
  （aiah **作为** MCP server 供 AI 工具调用；与 0004 主题不同）
- [0006：TUI 作为第一个交互界面，及其写操作边界](decisions/0006-tui-as-first-interactive-surface.md)
  （取代 ADR-0003 §5 的 Web UI 规划）
- [0007：不可变分发通道，传输不归 aiah 管](decisions/0007-immutable-channel-distribution.md)
  （`publish` / `pull` / `versions`；通道是普通目录，网络传输交给 git / rsync / U 盘）
- [0008：bootstrap 只编排取回与强制交互审阅](decisions/0008-interactive-bootstrap.md)
  （pull 前 TTY 预检；复用 TUI Phase C；无 `--yes`）

## 设计方案

- [TUI 技术方案](designs/tui-technical-design.md)（Phase A / B / C 已实现）

## 评审

- [2026-07-24 设计与实现评审](reviews/2026-07-24-design-implementation-review.md)
- [2026-07-25 MCP create-only 严格复审（ADR-0004 六条门槛）](reviews/2026-07-25-mcp-create-only-strict-review.md)
- [2026-07-27 Public readiness、仓库身份与 TS 边界评估](reviews/2026-07-27-public-readiness-assessment.md)

评审记录是时点快照；其中的待修项修复后应在原文标注结果，不回改结论。

## 调研

- [PromptHub 调研与采用边界](research/prompthub-assessment.md)
- [TUI 界面评估（参考 grok-build）](research/tui-surface-assessment.md)
- [产品形态与分发边界评估](research/product-form-and-distribution-assessment.md)
  （服务端 docker / 内网多用户 / curl 安装 / MCP server 四问收口；结论未冻结为 ADR）
- [外部参考](references.md)

## 许可证

- [第三方依赖许可证清单](licenses/third-party.md)

本项目以 Apache-2.0 发布：协议正文在仓库根 `LICENSE`，署名在 `NOTICE`，
与 PromptHub 的边界见 [security.md](security.md) §6。
