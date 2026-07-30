# 文档索引

## 用户指南

- [上手指南](getting-started.md)
  （发现资产 → 整理资产库 → 检查并准备 → 预览变化 → 人工确认）
- [使用流程总览](usage-flows.md)
  （首次使用、日常维护、撤销、跨设备、MCP/自动化与工具升级）
- [CLI 命令参考](cli-reference.md)
- [跨设备迁移 runbook](runbooks/cross-device-transfer.md)（发布 → 搬运 → 取回 → 安装）
- [真机 dry-run runbook](runbooks/real-home-dry-run.md)
- [工具安装、升级与 TUI dogfood](runbooks/install-upgrade-dogfood.md)
- [全部 Runbook / SOP 及缺口](runbooks/README.md)
- [AI 资产管理踩坑清单](troubleshooting/ai-asset-pitfalls.md)

## 现行设计

- [项目协作说明](../CLAUDE.md)（Claude Code / Codex 共用单一事实源）
- [总体架构](architecture.md)
- [资产模型](asset-model.md)
- [安全与隐私](security.md)
- [工程流程：开发 / 测试 / 构建 / 部署 / 发布](development.md)
- [开发环境搭建 SOP](runbooks/development-environment.md)
- [发版 runbook](runbooks/release.md)
- [MVP 路线图](roadmap.md)（当前能力矩阵、发布边界与后续任务 N0–N8）
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
- [0009：统一资产状态与受控资产库写操作](decisions/0009-controlled-asset-library-mutations.md)
  （纳入/更新/移出、事务恢复、typed confirmation；不是双向同步）

## 设计方案

- [TUI 技术方案](designs/tui-technical-design.md)（Phase A / B / C / D1 / D2 / D3 已实现）
- [TUI 产品体验与导航方案 V2](designs/tui-product-experience-v2.md)
  （任务首页、友好术语、默认启动、CLI/TUI 边界与设置/i18n 分期）
- [跨设备迁移与受控版本对齐方案](designs/cross-device-migration-and-version-sync.md)
  （换机迁移、不可变分发、E3 状态模型与未来同步准入条件）

## 评审

- [2026-07-30 v0.1.5 候选就绪检查点](reviews/2026-07-30-v0.1.5-candidate-readiness.md)
- [2026-07-30 E1 / E2 / E3.1 严格实现复审](reviews/2026-07-30-e1-e2-e3-1-strict-review.md)
- [2026-07-29 TUI 产品体验方案 V2 评审](reviews/2026-07-29-tui-product-experience-v2-review.md)
- [2026-07-24 设计与实现评审](reviews/2026-07-24-design-implementation-review.md)
- [2026-07-25 MCP create-only 严格复审（ADR-0004 六条门槛）](reviews/2026-07-25-mcp-create-only-strict-review.md)
- [2026-07-27 Public readiness、仓库身份与 TS 边界评估](reviews/2026-07-27-public-readiness-assessment.md)

评审记录是时点快照；其中的待修项修复后应在原文标注结果，不回改结论。

## 调研

- [PromptHub 调研与采用边界](research/prompthub-assessment.md)
- [TUI 界面评估（参考 grok-build）](research/tui-surface-assessment.md)
- [产品形态与分发边界评估](research/product-form-and-distribution-assessment.md)
  （服务端 docker / 内网多用户 / curl 安装 / MCP server 四问收口；结论未冻结为 ADR）
- [beautify-github-readme 适用性评估](research/beautify-github-readme-assessment.md)
  （asset-only 与 README mode 已进入 `v0.1.5` 默认分支；主 SVG 只承担首次使用
  流程，其它任务由 README 任务表和[使用流程总览](usage-flows.md)覆盖）
- [外部参考](references.md)

## 许可证

- [第三方依赖许可证清单](licenses/third-party.md)

本项目以 Apache-2.0 发布：协议正文在仓库根 `LICENSE`，署名在 `NOTICE`，
与 PromptHub 的边界见 [security.md](security.md) §6。
