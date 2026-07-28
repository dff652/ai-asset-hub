# TUI 技术方案（2026-07-25）

- 状态：**Phase A 已实现**（2026-07-26）、**Phase B/C 已实现**（2026-07-28，
  见 [ADR-0006](../decisions/0006-tui-as-first-interactive-surface.md)）；跨设备
  分发前置由 ADR-0007 满足
- 前置结论：[TUI 界面评估](../research/tui-surface-assessment.md)
  （做 TUI 不做 Web UI，为什么，以及五项门槛的对账）
- 约束来源：[ADR-0003](../decisions/0003-cli-first-go-core-and-product-surfaces.md)
  §2「UI 不得复制业务规则」、§5「先只读、再受控写入」

本文只回答「怎么做」。「要不要做、什么时候做」在评估文档里；分发前置满足后，
Phase A/B/C 已按顺序全部落地。

## 0. 定位：工作流操作台，不是控制面板

「控制面板 + 方便用户配置」这个定位**一半成立，一半有害**，必须拆开说：

**✅ 成立的一半——操作台**。浏览盘点、审阅 diff、执行 apply、看部署历史与回滚，
这些确实需要一个界面，就是 Phase A/C。

**⚠️ 有害的一半——「配置」**。这个工具**几乎没有属于自己的配置**：

- 它的「配置」就是 **manifest 文件**，躺在用户的资产工作区里、进 git、可 diff、
  可跨设备携带。TUI 可以**编辑那个文件**（Phase B），但绝不能引入一份 TUI 私有的
  设置存储——那会直接违反 ADR-0001「文件是事实源」；
- 项目已经刻意避免隐式配置：`--manifest` 一直要求显式传，roadmap 明确写了
  「首版不加入隐式默认发现」。一个设置面板会反过来鼓励把状态藏进不可见的地方；
- 一旦 TUI 能开关 manifest 之外的东西，两台设备的行为就会在没有 diff 的情况下分叉，
  而「可审计、可 diff、可回滚」正是这个产品存在的理由。

grok-build 的设置面板之所以合理，是因为它是一个**常驻的交互式 agent**，真有
per-user 偏好（主题、鼠标、模型、审批模式）。aiah 是**一次性执行的迁移工具**，
把那个用例照搬过来等于**发明本不该存在的配置**。要借鉴的是它的**交互语法**
（分组列表、右对齐值、`/` 搜索、底栏键位），不是「设置面板」这个概念。

一句话定位：**不是控制面板，是「盘点 → 组包 → 审阅 → 执行」这条既有工作流的
可视化操作台**。凡是想加进 TUI 的东西，先问一句「它落到哪个文件里、能不能 diff」。

## 1. 目标与非目标

**目标**：把三件今天只能靠人肉 `jq` 的事变成可交互操作——
浏览盘点结果、从候选里挑出包内容、apply 前审阅 diff。

**非目标**（写进代码注释，防止长成第二个产品）：

- 不做资产内容编辑器（改 SKILL.md 用你自己的编辑器）；
- 不做 agent / 对话 / 流式；
- 不做鼠标优先设计（键盘优先，鼠标可选支持滚轮）；
- **不做设置面板**——本工具的「配置」就是 manifest 本身，理由见 §0；
- 不做主题系统，只跟随终端配色 + 少量语义色。

## 2. 技术选型

| 依赖 | 版本 | 用途 | 协议 |
|---|---|---|---|
| `github.com/charmbracelet/bubbletea` | v1.3.10 | 事件循环与 Elm 架构 | MIT |
| `github.com/charmbracelet/bubbles` | v0.21.1 | key / textinput | MIT |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | 布局与样式 | MIT |
| `github.com/charmbracelet/x/term` | v0.2.2 | TTY 探测 | MIT |
| （传递依赖）`muesli/termenv`、`mattn/go-runewidth` 等 | — | 终端能力探测、宽字符 | MIT |

选它而不是 `tview`/`tcell` 的决定性理由是**可测试性**：Bubble Tea 的
`Update(msg) (Model, Cmd)` 是纯函数，本项目的界面状态本来就是「一份 JSON 报告 +
光标 + 过滤条件 + 勾选集合」，可以用表驱动测 `Update`、用 golden 字符串测 `View`，
符合 [development.md §2](../development.md) 的测试纪律。命令式 UI 框架做不到这点。

**落地前必须做的两件事**（本项目已有的规矩）：

1. 新依赖同步 `NOTICE` 与 `docs/licenses/third-party.md`（MIT 与 Apache-2.0 兼容）；
2. 实测二进制体积增量并记录（评估文档里 +2–4MB 是估计，**不要照抄**）。

Phase A 实测（Linux amd64、`CGO_ENABLED=0`、`-trimpath -ldflags "-s -w"`）：
基线 `4,821,154` bytes，加入 TUI 后 `5,841,058` bytes，增加 `1,019,904`
bytes（约 `0.97 MiB` / `21.2%`）。这是构建实测，不是原评估的估算。

## 3. 架构

```text
cmd/aiah/main.go            case "ui": → internal/tui.Run(opts)
                                          │
internal/tui/               Model ──Update──▶ Model ──View──▶ 终端
                                │
                                └── Cmd（异步）────┐
                                                   ▼
internal/inventory.Scan / internal/apply.Diff / internal/apply.Apply
internal/build.Build / internal/validate.Validate
```

四条硬约束：

1. **零业务规则**。TUI 不判断什么是 cache、不算路径安全、不做 adapter 映射、
   不写备份逻辑。它调用与 CLI **完全相同**的 `internal/*` 函数。
2. **同进程直调，不 shell out**。同一个二进制，直接调用包函数：没有子进程、
   没有版本偏斜、没有 JSON 往返开销。
3. **契约优先**。TUI 需要的任何字段，必须先存在于 JSON 报告里。不允许为 TUI
   单开一条数据通路，否则 CLI 与 TUI 会长出两套事实。
4. **非 TTY 直接失败**。管道、CI、无 TERM 时 `aiah ui` 报错退出并提示改用 JSON
   子命令，不做半吊子渲染。

耗时操作（`Scan` 在真机上要遍历上千条目）走 `tea.Cmd` 异步执行，界面先渲染
loading 态，避免阻塞事件循环。

## 4. 分期与验收

### Phase A：只读浏览（可现在做）

**实现状态：已完成。** CLI 为 `aiah ui [--home PATH] [--project PATH]`；非 TTY、
空 `TERM` / `TERM=dumb` 均在扫描前失败并提示改用 JSON 命令。

范围：inventory 浏览 + findings 分诊。

- 左侧树：`source（claude/codex/grok/shared）→ type → 资产`；没有对应 Asset 的
  finding（如 incomplete skill / broken symlink）进入独立组，不得因树只遍历
  Assets 而消失；
- 右侧详情：资产的文件列表、scope / portability / sensitivity、命中的 finding；
- `/` 增量过滤（匹配路径、类型与 finding）；`f` 只看有 finding 的项；`r` 重新扫描；
- 底栏常驻键位说明 + 计数（候选 / 排除 / findings）。

验收：真机 `~` 扫描后能在 3 次按键内定位到任一资产；全程零写入
（用 `find` 前后比对断言）；`Update` 表驱动测试覆盖过滤、光标边界、空结果。

### Phase B：manifest 组装

**实现状态：已完成（2026-07-28）。** CLI 为 `aiah ui --workspace PATH`；不给
`--workspace` 时界面保持只读、不显示复选框。落地时对本节原范围做了两处修正：

- 勾选后**同时把资产文件复制进工作区**再写 manifest。只写 manifest 不搬文件会让
  紧随其后的 `validate` 必然报 path 不存在，那种形态自相矛盾（ADR-0006 §2）。
- 已存在 manifest 走 `yaml.Node` 就地编辑，不经结构体重新序列化，注释、键序与未知
  字段都保留（ADR-0006 §4）；工作区已有文件 create-only 不覆盖。

范围：把「挑资产 → 写 manifest」从手写 YAML 变成勾选。

- `Space` 勾选候选资产，`w` 写出 `manifest.yaml` 到**工作区**；
- 写出后立即调 `validate.Validate`，错误就地显示、不落盘半成品；
- **只写工作区，永不写 `.claude` / `.codex` / `.grok`**；
- 已存在 manifest 时进入「编辑」模式：读入现有 assets，勾选状态预置。

验收：`~/ai-assets` 的现有 manifest 能被读入、改一项再写出，`git diff` 只有那一项
变化（键序与格式稳定）。

### Phase C：diff 审阅与执行

**实现状态：已完成（2026-07-28）。** CLI 为
`aiah ui --package PATH [--targets LIST]`；`--targets` 没有 `--package` 时拒绝。
前置由 [ADR-0007](../decisions/0007-immutable-channel-distribution.md) 满足，执行
边界见 [ADR-0006 §7](../decisions/0006-tui-as-first-interactive-surface.md)。

范围：把 runbook 里「人工逐项确认 diff」这一步搬进界面。

- 展示 `apply.Diff` 的 changes，按 create / update / unchanged / skipped 分组，
  可展开看单文件；
- `a` 只进入确认页；完整输入 `apply` 并按 Enter 才执行 `apply.Apply`；
- 完成后**显著展示 `backupId` 与包含全部安装根的回滚命令**；no-op 明示无需回滚；
- 失败时原样显示 finding，不做二次解释或美化；
- apply 期间吞掉所有按键；完成后重扫 inventory，避免返回旧视图。

验收：与 CLI 同包同参数的执行结果逐字段一致；失败路径（软链冲突 P9、
MCP 冲突 P6）在界面上可读且不吞信息。10 项变异验证全部判红；真实 PTY 已走通
diff → 输入 `apply` → 展示 backup / rollback → 退出。

`aiah bootstrap`（ADR-0008）直接复用这一阶段：先由 channel Core 取回包，再调用
同一 deployment model。TUI 退出 alternate screen 时把最终 diff/apply Core report
交回 CLI，使 backup/rollback 能持久留在普通终端；bootstrap 不另做确认逻辑。

## 5. 代码落位与状态模型

```text
cmd/aiah/main.go        +1 个 case，其余不动（现 327 行扁平 switch，不引入命令框架）
internal/tui/
  run.go                Run(opts)：TTY 探测、程序启动、错误退出码
  model.go              Model 定义与 Init/Update/View
  keys.go               键位绑定表（bubbles/key）
  inventory_view.go     Phase A 视图
  manifest_view.go      Phase B 视图
  diff_view.go          Phase C 视图
  styles.go             lipgloss 样式，语义色集中在此
```

状态草图（Phase A）：

```go
type Model struct {
    stage    stage           // browse | compose | diff
    report   inventory.Report // 唯一数据源，来自 Scan
    filter   string
    cursor   int
    expanded map[string]bool
    selected map[string]bool // Phase B：资产 logicalPath 集合
    status   status          // idle | loading | failed
    err      error
    width    int
    height   int
}
```

`report` 是唯一事实源，视图全部由它派生；过滤只影响渲染用的索引切片，**不改动
report 本身**，这样重新扫描与过滤互不干扰。

## 6. 交互语法

对齐 grok-build 的设置面板（参考交互，不复制实现，见评估文档 §3 的许可证边界）：

| 键 | 行为 |
|---|---|
| `↑` `↓` `j` `k` | 上下移动 |
| `g` `G` | 跳首 / 跳尾 |
| `→` `Enter` | 展开 / 进入详情 |
| `←` `Esc` | 收起 / 返回上一层 |
| `Space` | 勾选（Phase B） |
| `/` | 增量搜索，`Esc` 退出搜索 |
| `f` | 只看有 finding 的 |
| `r` | 重新扫描 |
| `?` | 键位帮助 |
| `q` `Ctrl+C` | 退出 |

布局：

```text
┌ aiah · inventory ─────────────────────────── 候选 19 · 排除 4 · findings 5 ┐
│ / 过滤…                                                                    │
│ ▾ shared                        │ home/.agents/skills/feature-dev          │
│   ▸ skill  feature-dev          │ type        skill                        │
│   ▸ skill  frontend-design      │ scope       global                       │
│ ▾ codex                         │ portability portable                     │
│   ▸ agent  code-architect.toml  │ files       2                            │
│   ▸ rules  default.rules        │   SKILL.md                               │
│ ▾ claude                        │   agents/openai.yaml                     │
│   ⚠ skill  weekly-report         │ findings    —                            │
└ ↑↓/jk 导航 · / 搜索 · f 只看告警 · r 重扫 · ? 帮助 · q 退出 ───────────────┘
```

## 7. 性能

真机首次扫描曾产生 6438 条目（修分类缺口后 151 条，但新机器上可能再次很大）。
两条要求：

- 列表只渲染可视区（`bubbles/viewport` 或自己切片），不做全量字符串拼接；
- `Scan` 异步执行并显示进度态；扫描期间界面可退出。

## 8. 测试策略

| 层 | 方法 |
|---|---|
| `Update` | 表驱动：给定 Model + Msg，断言下一个 Model（光标边界、过滤空结果、勾选幂等） |
| `View` | golden 字符串（固定宽高、关闭颜色），断言布局与计数行 |
| 集成 | 用 `testdata/home-*` fixture 跑 `Run` 的非交互路径；Phase C 用假 HOME 验证与 CLI 结果一致 |
| 反向 | 非 TTY 必须报错退出；只读阶段用 `find` 前后比对断言零写入 |

**不测**：颜色、真实终端尺寸变化、鼠标。

按 [development.md §2.1](../development.md)，每条安全相关断言仍需变异验证。

## 9. 风险与回退

| 风险 | 缓解 |
|---|---|
| 界面被「盘点+组包」这一个场景绑架，分发上线后要重做 | Phase A 只读、代码量小；B/C 明确等分发链路 |
| 第二个界面的维护成本（每加一个 finding code 都要跟） | TUI 只渲染契约已有字段，不写特例分支 |
| 依赖体积与供应链 | 只引 charm 三件套，实测体积并记录；新依赖照例进 NOTICE 与第三方清单 |
| Windows 终端行为差异 | 与 ADR-0003 §4 一致：先只声明 Linux/macOS 支持，Windows 需单独验收 |
| 与未来 Web UI 重复投入 | 两者共用同一 Core 契约；真要 Web UI 时 TUI 不必废弃（SSH 场景仍只有 TUI 可用） |

## 10. 工作量估算

| 阶段 | 估算 | 说明 |
|---|---|---|
| Phase A | 2–3 天 | 含测试与非 TTY 降级 |
| Phase B | 2 天 | manifest 读写与 round-trip 稳定性是主要成本 |
| Phase C | 2–3 天 | 二次确认、失败路径展示、与 CLI 结果一致性测试 |

先做 A，用真实 dogfood 打磨完再决定 B/C 的形态——不要一次性把三期都设计死。
