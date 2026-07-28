# ADR-0004：MCP 原生配置所有权与安全写入

- 状态：Accepted
- 实施：Phase 2C.3.2 已实现，**2026-07-25 通过严格复审**
  （[复审报告](../reviews/2026-07-25-mcp-create-only-strict-review.md)）
- 日期：2026-07-24（复审 2026-07-25）
- 关联：[ADR-0001](0001-file-first-adapter-distribution.md)、
  [ADR-0002](0002-multi-target-capability-adapters.md)

## 背景

Phase 2C.3 已实现 MCP 模板到 Claude、Codex、Grok 原生配置的合并原型，但 review
确认它还不能用于真机 apply：

1. 完整 JSON/TOML 被解码为 `map` 后重新编码；即使 MCP 容器没有语义变化，也可能
   改变整份文件的格式、键序，并丢失 TOML 注释。
2. 原生配置可能含用户已有 token/header；更新时必须完整备份才能精确 rollback，
   因而敏感内容可能进入本机 `.aiah/backups`。
3. Claude 的原型路径错误：user MCP 应位于 `~/.claude.json`，project MCP 应位于
   项目根 `.mcp.json`，而不是 `.claude/settings.json`。
4. MCP 与 hook 已成为 apply 前的串行特殊阶段；继续加入 rules/agents expand 会让
   `Apply` 成为状态和错误处理中心。

官方路径依据：

- [Claude Code MCP scopes](https://code.claude.com/docs/en/mcp)
- [Codex config basics](https://developers.openai.com/codex/config-basic)
- [Codex MCP](https://developers.openai.com/codex/mcp)
- [Grok MCP servers](https://docs.x.ai/build/features/mcp-servers)

## 决策

### 1. 区分 sidecar 与原生配置所有权

- `*/mcp/<name>.json` sidecar 是 aiah 管理的审计产物，可以正常 create/update/rollback。
- harness 原生配置是用户或 harness 管理的文件，aiah 默认不取得整文件所有权。
- 包含 MCP asset 不等于授权覆盖已有原生配置。

### 2. 固定 scope 到原生路径

| Target | Global/User | Project |
|---|---|---|
| Claude | `~/.claude.json` | `<project>/.mcp.json` |
| Codex | `~/.codex/config.toml` | `<project>/.codex/config.toml`（trusted project） |
| Grok | `~/.grok/config.toml` | `<project>/.grok/config.toml` |

Claude local scope 存在 `~/.claude.json` 的 project 子项中；首个安全修复切片不自动
生成 local scope，避免把绝对项目路径写进可移植资产。

### 3. 已有原生配置默认不修改

首个安全版本采用 create-only：

- 原生配置不存在：允许创建只含所需 MCP 容器的最小文件，作为一次性 bootstrap；
- 创建完成不代表 aiah 取得持续所有权；没有 managed 标记，之后一律按已有
  用户/harness 文件处理；
- 原生配置已存在且所需 server 已完全相同：不 stage、不写盘、不备份；
- 原生配置已存在且需要新增或改变内容：保留 sidecar，返回明确 finding，不修改
  原文件；
- 同名不同内容令整个 apply fail-closed，避免其它资产安装成功但 MCP 名称实际
  指向不同 server。

未来只有在提供显式 opt-in，并能保留 MCP 容器外的原始字节/注释时，才开放对已有
原生配置的 add-only 更新。不得以“语义可解析”为由整文件重编码。

### 4. 备份必须可逆，不做内容脱敏

精确 rollback 要求保留更新前原文，因此不能对 backup payload 做不可逆脱敏。
若未来显式更新原生配置：

- backup 目录与 payload 必须保持 `0700` / `0600`；
- finding 必须说明备份可能含原配置中的敏感值；
- 输出、日志、manifest 仍不得回显 backup 内容；
- runbook 必须禁止把 `.aiah/backups` 提交到 Git 或同步到不可信介质。

默认不修改已有原生配置，是降低该风险的第一道边界。备份加密与保留周期属于后续
独立能力，不阻塞 create-only。

### 5. Apply 只编排轻量策略流水线

apply 前变换采用一组小型 stage policy 函数：

```text
CompileTargets → MCP policy → Hook policy → plan → backup/write
```

每个 policy 接收 staged files 和安装上下文，返回新的 staged files 与 findings。
`Apply` 只负责顺序执行和统一 fail-closed；文件默认 mode 在 plan/write 边界解析，
hook policy 不修改非 hook 文件。不引入为单个实现服务的重量级接口。

## Phase 2C.3.1 验收门槛

1. Claude/Codex/Grok global 与 project 路径均有 fixture 和精确断言。
2. 已有非规范 JSON/TOML 且 MCP 语义相同时，原字节不变、`written == 0`、无 backup。
3. 已有原生配置需要新增 server 时，默认只产生 finding 和 sidecar，不改原文件。
4. 含真实敏感值的已有配置在默认流程中不进入 backup。
5. MCP/hook 通过统一 policy loop 执行；hook 不再给非 hook 文件补默认 mode。
6. `go test ./...`、`go test -race ./...`、`go vet ./...` 与假 HOME 闭环通过。

在以上门槛满足前：

- Phase 2C.3 标记为“原型完成、review blocked”；
- 含 MCP asset 的包不得对真实 HOME/project 执行非 dry-run apply；
- 不进入 Phase 2C.5。

## 实施记录

2026-07-24 已完成 Phase 2C.3.1：

- 三端 global/project native 路径由单一函数推导；
- create-only、identical 零 stage、existing skip warning 已落地；
- 已有敏感 native config 保持原字节且不进入 backup payload；
- MCP/hook 使用轻量 stage policy loop，mode 默认回到 plan/write 边界；
- 全量 test/race/vet、50 次关键包测试和两套假 HOME 闭环通过。

该实现随后收到契约收口意见，见
[设计与实现评审的后续状态](../reviews/2026-07-24-design-implementation-review.md#9-后续状态phase-2c32)。

## Phase 2C.3.2 契约收口

2026-07-24 已完成：

- 明确选择一次性 bootstrap（选项 C），不增加 managed ownership；
- finding 从历史 `mcp_merge_failed` 改为 `mcp_native_failed`；
- MCP policy 只提取 MCP 模板并追加 native 输出，不再按 scope 重排全部 staged；
- 缺少 `--home` / `--project` 统一由 plan 返回 missing-root finding；
- 新增「创建后包增加 server 仍 skip 且原字节不变」与 project MCP 包级测试；
- runbook 明示同名冲突会令整个 apply fail-closed。

## 严格复审结果（2026-07-25）

6 条验收门槛全部通过，实证方式与逐条证据见
[复审报告](../reviews/2026-07-25-mcp-create-only-strict-review.md)。含 MCP asset
的包解除「仅 dry-run」限制，继续遵循先假环境闭环、再 dry-run 和人工检查 diff 的
长期流程。

复审新发现三项 **fail-closed 过宽**（P6），均为拒绝写入方向、无安全风险，但会让
完全无辜的环境状态导致整单 apply 失败，真机 dogfood 前建议先决策：

- 已有配置把等价内容写成 `"args": []` 被判成同名冲突（假冲突）；
- 原生配置是软链（dotfiles 管理常见）→ 整单失败；
- 原生配置是 0 字节或非法 JSON → 整单失败。

放宽方向（需要显式决策，因为会移动本 ADR §3 的 fail-closed 边界）：只把「同名
server 真冲突」保留为 error，其余降为 `mcp_native_skipped` warning 并保留 sidecar；
比较前对空数组/空对象做同一规范化。

## 影响

正面影响：

- aiah 不会静默取得用户整份配置的所有权；
- 默认路径不会复制真实密钥到 backup；
- scope 与各 harness 实际加载路径一致；
- 后续 rules/agents policy 不再继续膨胀 `Apply`。

代价：

- 一次性 bootstrap 不能自动向已有原生配置（包括 aiah 曾创建的文件）追加 server；
- 用户需依据 finding 手动采用 sidecar，或等待显式安全 merge；
- 不同 harness 的路径和信任语义需要独立 fixture。
