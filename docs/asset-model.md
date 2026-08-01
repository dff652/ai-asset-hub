# 资产模型

## 0. 资产工作区在哪里（角色边界）

三个目录角色正交，勿混淆（2026-07-24 评审问答澄清）：

| 目录 | 角色 |
|---|---|
| aiah 仓库 | 纯工具代码 + 测试夹具（`testdata/` 只是 fixture，不是资产库） |
| 资产工作区（本节 `ai-assets/`） | 个人全局与跨项目复用资产的事实源；由用户经 `--manifest <path>` 指定，workspace root 默认 = manifest 所在目录（`--root` 可覆盖）；建议放独立私有 Git 仓库 |
| `--home` / `--project` | 部署目标（写 `~/.claude` 等）或扫描对象；`--project` 指接收 project-scope 资产的目标工程，项目专属资产仍可随该项目 Git 管理 |

正式个人资产不放在 aiah 工具仓库内。资产工作区与部署目标是逻辑角色，不要求
物理目录永不重合；项目专属规则仍以项目仓库为事实源，默认只读盘点或显式导入。
已知便利性缺口：尚无 `aiah init <directory>` 脚手架（见 roadmap
「当前优先级」）；首版继续显式传入 `--manifest`，不做隐式工作区发现。

## 1. 建议目录

```text
ai-assets/
├── assets/
│   ├── skills/
│   │   └── example-skill/
│   │       ├── SKILL.md
│   │       ├── references/
│   │       └── scripts/
│   ├── rules/
│   │   ├── common/
│   │   ├── claude/
│   │   └── codex/
│   ├── memory/
│   │   ├── personal/
│   │   └── projects/
│   ├── agents/
│   ├── hooks/
│   └── mcp/templates/
├── profiles/
│   ├── personal.yaml
│   ├── work.yaml
│   └── devices/
├── adapters/
├── manifest.yaml
└── lock.json
```

## 2. Manifest

示意格式：

```yaml
schemaVersion: 1
name: personal-ai-assets
version: 2026.07.1

assets:
  - id: skill.karpathy-guidelines
    type: skill
    path: assets/skills/karpathy-guidelines
    targets: [claude, codex]

  - id: rules.personal-defaults
    type: rules
    path: assets/rules/common/personal-defaults.md
    targets: [claude, codex]

profiles:
  personal:
    include:
      - skill.karpathy-guidelines
      - rules.personal-defaults
```

manifest v1 已定义：

- 稳定资产 ID；
- 依赖及冲突；
- 目标平台；
- 作用域：global、project、device；
- 安全等级；
- 可选来源 `source.url/revision/importedAt`；
- 可选文件清单与 SHA-256。

来源字段是资产库作者声明的追踪信息，不是可信、签名或许可证证明；inventory 的
Claude/Codex/Grok“来源工具”与 manifest 的上游 URL 也不是同一概念。描述、标签、
许可证和平台专属覆盖尚未进入 v1；增加这些字段前必须先设计版本兼容，详见
[N8 规模化资产管理增强方案](designs/scalable-asset-management.md)。

## 3. 盘点模型

Phase 0 inventory 明确区分两个层次：

- `Asset` 是可迁移的逻辑单元。例如包含 `SKILL.md` 的整个目录只算一个
  Skill，插件根目录只算一个 Plugin；
- `Entry` 是磁盘路径事实，记录文件类型、大小、权限、SHA-256、软链接和排除原因。

普通中间目录不进入 Entry，也不参与候选资产统计。被排除、无权限或软链接目录
仍保留 Entry，但目录大小固定为 `0`，避免把文件系统目录元数据当成资产大小。
Skill 资产必须由 `skills/<name>/SKILL.md` 这一直接、常规文件建立；软链接只作为
被排除的 Entry 报告，不能建立资产。`skills/<name>/` 下存在文件但没有合规
`SKILL.md` 时，文件降级为 `reported/unknown`，同时输出 `incomplete_skill`
finding，不产生 Skill Asset。

扫描管线固定为：

```text
路径策略 → Entry 内容检查 → Asset 合并 → findings / summary
```

`summary.candidateByType` 只统计状态为 `candidate` 的 Asset。未知文件只作为
`reported` Entry；含疑似密钥或不可读文件的资产不会成为迁移候选。Inventory
中的 portability 只表达 `adapter-required`、`excluded` 或 `unknown`；
`portable` 保留给经过 validate/build 确认后的 manifest 资产。

## 4. Rules

Rules 使用“公共规则片段 + 平台扩展”：

```text
rules/common/coding.md
rules/claude/permissions.md
rules/codex/sandbox.md
```

Adapter 负责生成最终 `CLAUDE.md`、`AGENTS.md` 或其它 target 规则入口。不得要求各端规则文件永远逐字相同。

多端时公共片段放 `rules/common/`，平台扩展放 `rules/<target>/`。能力分层与
Target 注册见
[ADR-0002](decisions/0002-multi-target-capability-adapters.md)：rules 拆成两个
capability——`rules.markdown`（公共片段，移植性中高）与 `rules.project_file`
（`CLAUDE.md` / `AGENTS.md` 这类**唯一项目入口**，移植性中）。

### 4.1 `CLAUDE.md` 与 `AGENTS.md` 的处理

这两个文件不是「配置」，而是 `type: rules` 资产的一个**特殊落点**：装到哪由
adapter 按 target 决定，不由文件名决定装成什么。相关实现分散在盘点、编译与
findings 三层，本节是它们的索引。

**盘点：按文件名归属工具。** `internal/inventory/classify.go::sourceFor` 把
`CLAUDE.md` 判给 `SourceClaude`、`AGENTS.md` 判给 `SourceCodex`，不看所在目录；
`internal/inventory/probe.go` 只在**项目根**探测这两个名字，HOME 侧的全局规则
经 `.claude/` 等前缀识别。

**编译：按 target 重定向，但绝不改名。**
`internal/adapter/map.go::mapRulesFile`：

| 工作区文件 | → claude | → codex | → grok |
|---|---|---|---|
| `assets/rules/CLAUDE.md` | `CLAUDE.md`（工具根） | `rules/CLAUDE.md` + `degraded` | `CLAUDE.md`（工具根） |
| `assets/rules/AGENTS.md` | `rules/AGENTS.md` + `degraded` | `AGENTS.md`（工具根） | `AGENTS.md`（工具根） |
| 其它 `assets/rules/*.md` | `rules/<rest>` | `rules/<rest>` | `rules/<rest>` |

**`CLAUDE.md` 装到 Codex 时不会被改名成 `AGENTS.md`**，而是降级放进 `rules/`
并记一条 `degraded`。理由：项目文档是唯一且有语义的入口，自动改名等于替 Codex
决定「这就是你的项目文档」，会静默覆盖它原有的事实源。宁可降级并显式告知。
Grok 两个名字都收在工具根，因为它两种都认。

**冲突：shadowing finding 有两级严重度。**
`internal/inventory/report.go` 的 `FindingRulesShadowing`：

| 条件 | 严重度 | 含义 |
|---|---|---|
| 项目同时有 `CLAUDE.md` 与 `AGENTS.md` | `info` | 请确认哪一个才是预期规则源 |
| 且 Codex 配了 `project_doc_fallback_filenames: ["CLAUDE.md"]` | `warning` | Codex 可能遮蔽已配置的 `CLAUDE.md` fallback |

第二种由 `internal/inventory/config.go` 探测的 `FeatureCodexProjectDocFallback`
触发，并可把项目状态判为 `ProjectConflicted` / `ProjectDualToolConfigured`。
「复用 `CLAUDE.md` 当 Codex 项目文档、不另写 `AGENTS.md`」是真实存在的工程约定，
在这类项目上新增 `AGENTS.md` 会让原设计失效——排障视角见
[踩坑 #2](troubleshooting/ai-asset-pitfalls.md)。

**scope 决定装哪个根。** `scope: project` 写项目根，`scope: global` 写
`~/.claude/` 等用户根；二者互斥，`internal/apply/phase2b_test.go` 断言
project scope 的 `CLAUDE.md` **不得同时**出现在 `home/.claude/` 下。

**与 `/init` 生成物的边界。** `claude /init`、Codex 等工具生成的项目说明文件是
**那个项目自己的事实源**，由它自身的 Git 与安装器管理，AI Asset Hub 不接管、
不改写。aiah 负责的是跨项目复用的通用规则：在工作区维护 `rules/common/`，
经 adapter 一源多端编译分发。

**项目级单一事实源策略。** 当 Codex 已配置
`project_doc_fallback_filenames = ["CLAUDE.md"]` 时，项目可以只提交根
`CLAUDE.md`，让 Claude Code 与 Codex 读取同一个文件；此时不应再运行会生成同级
`AGENTS.md` 的初始化。两个文件都不存在时，先从仓库事实整理、人工审阅并提交一份
`CLAUDE.md`，再验证 Codex fallback，而不是先生成两份独立摘要。若团队不能保证
每个 Codex 环境都配置 fallback，则必须显式选择另一种仓库策略（例如由中立源生成
双端文件并用 CI 检查漂移），不能假装缺少 `AGENTS.md` 已被自动解决。

这仍是**项目作者策略，不是 aiah 当前写能力**：aiah 只扫描并报告 missing /
shadowing；不会自动删除 `AGENTS.md`、修改用户 Codex 配置或把一个文件复制成另一个。
未来若提供对齐功能，默认也应先给只读计划，并要求显式选择单源 fallback 或受控
双文件生成，禁止猜测。

**未决：`~/.claude/CLAUDE.md` 尚未拆分**（待决策 D7）。全局那份通常混了三类
内容——工具无关的工程原则、Claude 专属的协作规则、本机私有的路径与端口——
必须人工拆成通用 / 平台专属 / 本机私有三份后才能进包。

## 5. Skills

`SKILL.md` 是优先的公共格式（T0/T1）。平台专属元数据放 sidecar：

```text
example-skill/
├── SKILL.md
├── targets/
│   ├── claude.yaml
│   ├── codex.yaml
│   └── grok.yaml
└── references/
```

这样可以：

- 保持主要说明可移植；
- 避免某一端专属 frontmatter 干扰其它端；
- 支持各平台元数据扩展；
- 在构建时明确报告能力降级（`degraded` / `dropped`）。

个人通用 Skill 默认优先部署到 **共享权威根** `~/.agents/skills`（T0），供
Claude / Codex / Grok 等已扫描该路径的工具共用，而不是默认复制三份到各
HOME。仅当 Device profile 显式要求 mirror 时才写入 `~/.claude/skills`、
`~/.grok/skills` 等。

## 6. Memory

Memory 只管理整理后的长期知识：

| 类型 | 处理方式 |
|---|---|
| 通用个人偏好 | 进入 `memory/personal/` |
| 项目架构、规则 | 优先进入项目文档 |
| 可复用排障经验 | 文档或显式调用 Skill |
| 临时会话上下文 | 不进入资产库 |
| 原生会话数据库 | 默认不迁移 |
| 敏感个人信息 | 加密私库，默认不部署 |

Memory 必须声明适用范围，避免把某个项目或客户的信息注入其他项目。

## 7. MCP

MCP 只保存可移植模板（包内 `assets/mcp/*.json`）：

```yaml
id: mcp.example
command: example-server
env:
  API_TOKEN: ${secret:example/api-token}
```

apply 时：

- 仍写入各端 sidecar `*/mcp/<name>.json` 供审计；
- sidecar 保留引用；计划 native config 前解析 `env` 中占满整个值的引用：
  `${ENV:NAME}` / `${env:NAME}` 读取非空环境变量，`${secret:path}` 调用
  `pass show -- path` 并取首行；
- provider 缺失、结果不存在或为空时整单 fail-closed，任何资产都不写；
- native 路径按 scope/target 选择：Claude `~/.claude.json` / `.mcp.json`，
  Codex `.codex/config.toml`，Grok `.grok/config.toml`；
- native config 不存在时可做一次性 bootstrap；创建完成后也按已有用户文件处理，
  后续包版本只报告、不修改；
- 已有同名且内容相同视为零写入；同名冲突 fail-closed；
- 只允许 `${ENV:…}` / `${secret:…}` / `${env:…}` 引用，原始密钥拒绝。

构建产物、报告、日志、journal 与 backup 元数据不得包含解析后的密钥值。设备本地
native config 是必须含真值的派生状态，inventory 会将其作为
`suspected_secret` 排除；可迁移事实源始终是只含引用的 sidecar。
完整所有权和备份边界见 ADR-0004。

## 8. Profile 与设备差异

Profile 表达“需要哪些**资产**”；Device profile 表达“这台机器安装哪些
**target**、skill 权威根在哪”，而不是复制一份资产：

```yaml
extends: personal
targets: [claude, codex]
includeTags: [coding, docs]
exclude:
  - skill.company-private
device:
  os: linux
```

机器名、绝对路径和凭据属于设备配置，不能写回通用资产。
