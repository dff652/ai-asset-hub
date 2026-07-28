# ADR-0002：多 Target 能力模型与可插拔适配

- 状态：Accepted
- 日期：2026-07-23
- 关联：[ADR-0001](0001-file-first-adapter-distribution.md)

## 背景

Phase 0 只读盘点已覆盖 Claude Code、Codex 与共享 `.agents`。本机还在使用
Grok Build（`~/.grok`），未来还可能加入 Cursor 等工具。

现状评估（2026-07-23）：

- **格式层部分重叠**：`SKILL.md`、项目 `AGENTS.md` / `CLAUDE.md`、共享
  `.agents/skills` 可被多工具加载。
- **路径层未覆盖 Grok**：扫描与部署均不认识 `~/.grok` / 项目 `.grok/`。
- **产品层仅承诺 Claude/Codex**：路线图 Phase 2 只有两端 adapter。
- **结论**：当前对 Grok 是「顺带兼容共享层」，不是一等支持。

若用 `if claude / codex / grok` 在 Core 中堆分支，端数增加时复杂度会变成
O(端点数 × 特性)，不可维护。

## 决策

在 ADR-0001「文件优先 + adapter」之上，冻结以下多端兼容结构。

### 1. 四层边界（禁止跨层泄漏）

```text
Canonical Source（assets/ + manifest + profiles）
        │ resolve / validate
Core IR（AssetUnit + Capability + SecretRef + TargetSet）
        │ per-target compile
Target Adapters（claude / codex / grok / … → staging only）
        │ shared apply
Device roots（~/.claude ~/.codex ~/.grok ~/.agents 项目目录…）
```

| 层 | 可以知道 | 不可以知道 |
|---|---|---|
| Source | 资产语义与中立路径 | 各工具 HOME 细节、TOML/JSON 方言 |
| Core | target id、能力矩阵、依赖冲突 | 直接写用户 HOME |
| Adapter | 单一 harness 的路径与格式 | 其它 harness 的目录 |
| Apply | staging → diff/backup/原子安装 | 资产业务语义 |

**新工具 = Target 注册表 + Probe（可选）+ Adapter + fixture**，不得在 Core
业务路径上增加工具名特判。

### 2. 用 Capability 建模，不用工具名散落分支

Core 只处理**能力**；每个 Target 声明自己支持哪些能力及读写根。

| Capability | 可移植性 | 说明 |
|---|---|---|
| `skill.skilmd` | 高 | `SKILL.md` 包 |
| `rules.markdown` | 中高 | 公共规则片段 |
| `rules.project_file` | 中 | `AGENTS.md` / `CLAUDE.md` 等 |
| `agent.definition` | 中低 | 格式差大 |
| `hook.lifecycle` | 低 | 事件模型与落盘格式差大 |
| `mcp.template` | 中 | 语义同，JSON/TOML/config 段不同 |
| `command.slash` | 中 | `commands/*.md` 等 |
| `persona` / `workflow` | 低 | 多为单家能力 |
| `memory.curated` | 中 | 仅整理后的长期知识 |
| `plugin.bundle` | 低 | 默认不当默认可迁移单元 |

Target 以声明文件注册（示意，非运行时格式终稿）：

```yaml
id: grok
roots:
  home: "{home}/.grok"
  project: "{project}/.grok"
  sharedSkills: "{home}/.agents/skills"
capabilities:
  skill.skilmd: { write: ["home/skills", "project/skills"] }
  rules.project_file: { write: ["project/AGENTS.md"] }
  mcp.template: { write: ["home/config.toml#mcp_servers"] }
exclude:
  - auth.json
  - sessions/**
  - logs/**
  - marketplace-cache/**
  - downloads/**
  - bundled/**
compatReads:
  - target: claude
    capabilities: [skill.skilmd, rules.project_file]
```

`compatReads` 表示「运行时也会加载其它工具目录」，只影响盘点的
`loadedBy` 关系，**不把对方目录当成第二份权威事实源**。

### 3. 可移植性分层（T0–T3）

| 层 | 内容 | 策略 |
|---|---|---|
| **T0 共享落点** | `~/.agents/skills`、可共享的项目规则 | 优先写共享根，避免 N 份拷贝 |
| **T1 高移植** | 中立 `SKILL.md` 正文、`rules/common` | 一源多 target 编译 |
| **T2 结构映射** | MCP 模板、agents 骨架、commands | 表驱动 adapter；可 degraded |
| **T3 专属** | personas、workflows、厂商 bundled、会话 | 单 target；不承诺互转 |

口诀：

1. 能进 T0 就不要复制三份。
2. T1 用 `targets/<id>.yaml` sidecar 扩展，不污染公共正文。
3. T3 必须显式 `targets: [grok]`（或对应 id），CLI/文档标明不可移植。
4. 新工具先填能力表，再写 adapter；表上全是 T3 则不承诺「全量迁移」。

### 4. 中立资产与 sidecar

- 公共正文（如 `SKILL.md`、`rules/common/*`）保持工具无关、可审查。
- 平台专属 frontmatter、权限、事件绑定放在资产旁 `targets/<id>.*`。
- Manifest 中资产声明 `targets`（或「所有支持该 capability 的已注册
  target」）与 `portability`。
- Profile 选择**资产集合**；Device profile 选择**本机安装哪些 target**
  以及 skill 权威根（例如 `skillRoot: shared`）。

### 5. Inventory：可插拔 Probe + 统一 Asset/Entry

扫描拆成 Probe，再合并为统一 IR（延续 Phase 0.1 的 Asset / Entry 分层）：

- `ProbeClaude` / `ProbeCodex` / `ProbeGrok` / `ProbeShared` / `ProbeProject`
- 每个 Probe 只负责：根路径、排除列表、单元规则、`source` 标签
- 合并层负责：同名冲突 finding、`loadedBy`、权威根、migration 摘要

硬规则：

- **迁移统计只数逻辑 Asset（candidate）**，不数「某工具碰巧能读到的路径」。
- Grok 兼容读取 Claude skills 时，记加载关系，不双计权威资产。
- 设备私有（auth、sessions、cache、marketplace-cache、bundled 默认等）永不进
  candidate。

### 6. 编译与安装

```text
manifest + profile + device
  → ResolvePlan（按 target 分组的 AssetUnit）
  → 各 Adapter.Compile → staging/<target>/
  → 共享 Diff / Backup / AtomicApply / Rollback
```

Adapter 概念接口：

- `Discover`（只读盘点，可与 Probe 共用）
- `Compile` → staging + `CompileReport`
- 可选 `Doctor`

`CompileReport` 必须包含：`emitted` / `dropped` / `degraded` / `secretRefs`。
不支持的 capability 要可见降级，禁止静默丢弃。

共享库可放 skill 解析、secret ref、staging/apply；**Adapter 之间不得互引**。

### 7. 权威根与冲突

| 场景 | 策略 |
|---|---|
| 同名 skill 出现在共享根与 `~/.grok/skills` | finding；apply 只维护 profile 指定的权威根 |
| 项目同时存在 `AGENTS.md` 与 `CLAUDE.md` | rules shadowing / multi-file finding |
| MCP 同名 server 多处配置 | 冲突 finding；禁止静默合并密钥 |
| 多端 compat 双读 | `loadedBy` ≠ 第二事实源 |

### 8. 安全（与端数无关）

- 资产包内无真实 Token；仅 `${secret:}` / `${ENV:}`。
- 每个 target 的 `exclude` 在 apply 时 deny-write。
- 默认不创建指向包外的软链接；staging 越界失败。
- 厂商 `bundled/`、marketplace 缓存、会话库默认不迁移。

### 9. 演进顺序（冻结优先级）

| 阶段 | 内容 |
|---|---|
| A | Target 注册表 + capability 枚举；manifest `targets` 可扩展 |
| B | Probe 插件化；现有 claude/codex/shared 迁入后加 Grok 盘点 |
| C | 共享 skill 权威根（`.agents`）；profile 可配置 |
| D | Adapter 接口；Claude/Codex 按接口落地；Grok 子集 adapter |
| E | T2 映射与 degraded 报告 |
| F | 更多工具 = 新 target 声明 + probe + adapter + fixture |

当前实现阶段仍以 Phase 0/1（盘点与 validate）为主；本 ADR 约束后续设计与
schema 演进，不要求立即实现全部 Probe/Adapter。

## 原因

- 端数会增加；能力集合相对稳定，适合作为 Core 稳定面。
- Grok 等工具已兼容读取 `.agents` / Claude 路径，必须区分「加载关系」与
  「权威资产」，否则盘点与迁移统计失真。
- 共享 skill 根（T0）把三端成本从「三份拷贝」降为「一份真相」。
- 显式 T3 与 CompileReport 降级，避免虚假「全工具无损迁移」承诺。
- 与 ADR-0001 一致：文件可审、adapter 显式处理差异、密钥与会话不进默认包。

## 影响

正面：

- 3 端与 N 端同一套模型，边界复杂度近似 O(端点数)。
- Phase 0 Asset/Entry 可直接延伸为多 Probe 合并 IR。
- 个人通用 Skill 可优先落在 `.agents`，Claude/Codex/Grok 同时受益。

代价：

- 需维护 target 注册表与能力矩阵文档。
- Manifest/inventory schema 的 `targets` / `source` 不能长期写死两家枚举。
- 每个新 harness 需要 fixture 与排除列表，测试矩阵变长。
- T2/T3 能力永远存在缺口，产品文案必须诚实。

## 不采用的方案

### Core 内按工具名堆特殊分支

短期快，长期每个新特性都要改公共路径，必然意大利面。

### 所有工具共用完全相同落盘文件

路径与元数据不同（尤其 hooks、MCP、agents）；逐字共享会把失败推迟到运行时。

### 把 Grok `bundled/` / marketplace-cache / sessions 当作用户资产

那是厂商运行时与设备状态，不是可移植事实源。

### 会话历史跨工具无损互转

超出资产编译器范围；与 ADR-0001 一致，默认不承诺。

### 默认 N 份 skill 镜像到每个 HOME

除非 profile 显式要求 mirror，否则以共享权威根为默认，避免分叉。

## 与 Grok Build 的当前关系（2026-07-24 更新）

| 问题 | 结论 |
|---|---|
| Skill 格式是否兼容 | 是（`SKILL.md`） |
| 项目规则是否兼容 | 是（`AGENTS.md` / `CLAUDE.md` 等） |
| aiah 是否已盘点 `~/.grok` | 否 |
| 是否已有 Grok adapter | 是，Phase 2B 落盘子集；尚未实现全量语义转换 |
| 共享 `.agents/skills` | 是有效重叠面；应作为 T0 优先策略 |
| 产品承诺 | 多端为已接受架构方向；实现按第 9 节阶段推进 |

## 后续文档与实现约束

- 架构总览见 [总体架构](../architecture.md)「多 Target」一节。
- 资产目录与 sidecar 约定见 [资产模型](../asset-model.md)。
- Grok 落盘子 Adapter 已实现；下一步优先补只读 Probe，不先扩展写入语义。
- 未完成本 ADR 阶段 A/B 前，不得宣称「支持 Grok 全量迁移」。
