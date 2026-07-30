# TUI 界面评估：要不要做、做成什么样（2026-07-25）

- 触发：参考 [xai-org/grok-build](https://github.com/xai-org/grok-build)
  的全屏 TUI 使用体验，评估本项目是否需要同类界面
- 状态：**评估结论，未动手实现**
- 后续修订（2026-07-30）：本评估“不做设置面板”继续约束业务配置和控制面板；
  [N7 方案](../designs/settings-and-i18n.md)已接受语言、首选资产库预填和显示密度
  三项设备本地 UI 偏好；首页双语消息目录已建立，设置入口和偏好存储尚未实现
- 关联：[ADR-0003](../decisions/0003-cli-first-go-core-and-product-surfaces.md)
  （CLI-first、Phase 3.5 只读 Web UI）、2026-07-25 真机 dogfood

## 0. 结论先行

**建议做，但要换掉 ADR-0003 §5 的形态：把「Phase 3.5 本地只读 Web UI」改为
「本地 TUI」，Web UI 降级为更远期的可选项。**

三条理由，按权重：

1. **本产品的高价值时刻在新机器上**。装机、迁移、SSH 进一台服务器——这些场景里
   TUI 只需要那个已经下载好的单文件二进制；Web UI 需要本地 HTTP 服务、浏览器、
   端口和本地鉴权，而目标机器可能根本没有浏览器。
2. **TUI 不引入第二种语言**。ADR-0003 明确「TypeScript 只在真正建设 Web UI 或
   VS Code 扩展时引入」。TUI 用 Go 写，仓库保持单语言、单二进制、单发布链路。
3. **不开监听端口**。这个工具天天碰 secret 邻近的文件，少一个 localhost 服务就
   少一整类问题（端口占用、CORS/CSRF、本地鉴权、其他进程可达）。

但**不是现在开工**：ADR-0003 的五项门槛还差第 3 项（跨设备包发布/拉取链路），
且第 4 项刚刚才开始有证据。建议排在首个 dogfood 包真机 apply 之后。

## 1. 现在 CLI 确实痛在哪（本次会话的实测证据）

不是「界面更好看」，是有几件事**只能靠人肉 jq**：

| 场景 | 当前实际做法 | 证据 |
|---|---|---|
| 看清一台机器有什么 | `scan` 输出 6438 条（修分类缺口后仍 151 条），本次会话为了读它现写了 5 段 Python | 盘点报告 §1 |
| 从候选里挑包内容 | 24 个候选肉眼过一遍，再手写 7 条 YAML | 盘点报告 §2 |
| apply 前审 diff | runbook 强制「逐项检查 diff 且用户明确决定执行」，实际是读 22 行 JSON changes | `real-home-dry-run.md` §5 |
| findings 分诊 | 修复前 329 条 `suspected_secret`，靠 Counter 按目录聚合才看得出规律 | 盘点报告 §3 |

共同点：**都是「读大量结构化数据 + 做选择」**，正是 TUI 的强项，也正是 ADR-0003
门槛第 4 条描述的「难搜索、难预览、难比较」。反过来，CLI/JSON 在自动化、CI、
Agent 调用上依然不可替代——所以是**增加一个界面，不是替换 CLI**。

## 2. TUI vs Web UI vs 维持纯 CLI

| 维度 | TUI（建议） | 本地 Web UI（ADR-0003 现方案） | 维持纯 CLI |
|---|---|---|---|
| 新机器/SSH 可用性 | 单二进制即可 | 需浏览器 + 端口 | 可用 |
| 语言与仓库形态 | 保持纯 Go 单语言 | 引入 TypeScript，仓库变多语言 | 纯 Go |
| 分发 | 无新产物，`aiah ui` 子命令 | 需打包前端产物或起 dev server | 无 |
| 攻击面 | 无监听端口 | localhost 服务 + 鉴权/CORS 问题 | 最小 |
| 可视化上限 | 文本/表格/树/彩色 diff | 图表、富 diff、图片 | 无 |
| 无障碍与远程 | 终端可用，屏幕阅读器支持一般 | 浏览器无障碍生态成熟 | — |
| 开发成本 | 中（Go 生态成熟） | 高（前端 + 契约 + 构建链） | 零 |
| 测试 | Model/Update 纯函数 + golden 字符串 | 需 E2E 浏览器测试 | 现状 |

Web UI 唯一明显更强的是「富可视化」和「无障碍」，而本产品第一波需求是
**列表、树、过滤、勾选、diff**——终端完全够用。

## 3. 从 grok-build 参考什么、不参考什么

事实（2026-07-25 查证）：Rust，`ratatui 0.29` + `crossterm 0.28`，75 个
workspace crate，Apache-2.0，定位是「全屏 TUI 编码 agent，也支持 headless
跑 CI，以及经 ACP 嵌入编辑器」。

**值得参考的是交互语法**（截图里那套设置面板）：

- 全屏面板 + 分组列表（Appearance / Mouse / Models / Advanced…）；
- 右对齐显示当前值，`›` 表示可展开；
- 底栏常驻键位说明：`↑/↓/j/k` 导航、`g/G` 首尾、`Space/Enter` 切换、`→` 展开、
  `/` 搜索、`d` 重置、`F2/Esc` 关闭；
- 顶部 `/ to search`，即时过滤；
- 底部一行 tip。

这套语法几乎可以原样映射到我们的场景：把「设置项」换成「资产候选」，
把「toggle」换成「选进包 / 不选」，把「展开」换成「看资产文件列表和 findings」。

**不参考的是它的架构**：grok-build 是 agent harness（流式对话、工具审批、
ACP、PTY 控制），75 个 crate 里绝大多数与我们无关。我们要的是一个数据浏览器
加选择器，不是 agent 外壳。

**许可证边界**：grok-build 是 Apache-2.0，与本项目同协议，但
[security.md §7](../security.md) 对 PromptHub 立的规矩同样适用——
**参考公开行为与交互思路，不复制源码、不移植受版权保护的实现细节**。
Rust → Go 本来也没法照抄，但这条要写在开工文档里，避免有人「参考」成搬运。

## 4. 实现方案

### 4.1 范围与分期

**Phase A — 只读浏览（先做，价值最高）**

```text
aiah ui            # 默认进入 inventory 浏览
```

- 左树右详情：按 source（claude/codex/grok/shared）→ type → 资产分组；
- `/` 过滤路径与类型；`f` 只看有 finding 的；
- 详情面板显示资产文件列表、scope/portability/sensitivity、命中的 finding；
- 数据来自 `inventory.Scan`，**不新增任何分类规则**；
- 全程只读，不写任何目录。

**Phase B — manifest 组装（本次 dogfood 最直接的痛点）**

- 在候选列表上 `Space` 勾选，`w` 写出 `manifest.yaml` 到**工作区**；
- 只写工作区，永不写 `.claude`/`.codex`/`.grok`；
- 校验直接调 `validate`，错误就地显示。

**Phase C — diff 审阅与确认执行**

- 展示 `apply --dry-run` 的 changes，按 create/update/unchanged 分组，可逐条展开；
- `a` 触发真正 apply，**调用同一 `apply.Apply`**，返回后显示 `backupId`；
- 失败路径显示 finding 原文，不做二次解释。

明确不做：不做资产内容编辑器、不做 agent/对话、不做鼠标优先设计、
不做设置面板（本项目几乎没有可配置项，`aiah` 的配置是 manifest 本身）。

### 4.2 技术选型

| 选项 | 说明 | 结论 |
|---|---|---|
| `charmbracelet/bubbletea` + `bubbles` + `lipgloss` | Go 生态事实标准，Elm 架构（Model/Update/View 纯函数），MIT | **推荐** |
| `rivo/tview` + `gdamore/tcell` | 组件更「开箱即用」，命令式 API，测试较难 | 备选 |
| 自己写 tcell 渲染 | 无第三方 UI 依赖，但要自造滚动/焦点/布局 | 不建议 |

选 Bubble Tea 的关键理由是**可测试性**：`Update` 是纯函数，我们的状态本来就是
「一份 JSON 报告 + 光标位置 + 过滤条件」，可以用 golden 字符串锚定渲染结果，
符合本仓库「每个行为都要有测试锚定」的要求（Rust 那边的 ratatui 是同类定位）。

### 4.3 架构约束（硬要求）

1. **TUI 不得包含任何业务规则**：分类、路径安全、adapter 映射、备份、回滚全部
   调用现有 `internal/*` 函数，与 CLI 走同一条路径。ADR-0003 §2「UI 不得复制
   业务规则」原样适用。
2. **TUI 不直接触碰 harness 目录**：读走 `inventory.Scan`，写走 `apply.Apply`。
3. **JSON 契约不变**：TUI 是契约的第二个消费者，不是契约的理由。任何为 TUI
   加的字段，CLI JSON 里也得有。
4. **可降级**：非 TTY（管道、CI）时 `aiah ui` 直接报错退出并提示用 JSON 子命令，
   不做半吊子渲染。

### 4.4 代码落位

```text
cmd/aiah/main.go          # 加一个 case "ui"，其余不动
internal/tui/             # 新包：model.go / view.go / keys.go / inventory.go ...
                          # 只依赖 internal/inventory、internal/apply、internal/build
```

`cmd/aiah/main.go` 目前 327 行、扁平 switch，加一个分支即可，不需要引入
命令框架（clap/cobra 类）。

## 5. 代价与风险

- **依赖增加**：Bubble Tea 一族会新增约 6–10 个模块（MIT/BSD 系，与 Apache-2.0
  兼容），必须同步 `NOTICE` 与 `docs/licenses/third-party.md`——这是本项目
  已经立好的规矩。
- **二进制体积**：预计 +2–4MB，**需实测**，不要照抄这个估计。
- **Windows**：ADR-0003 已把 Windows 排在「先验证只读命令」，TUI 在 Windows
  终端的键位/颜色/尺寸行为要单独验收，不能用「编译通过」代替。
- **第二个界面的维护成本**：每加一个 finding code、一种资产类型，TUI 都要跟。
  缓解办法是 TUI 只渲染契约里已有的字段，不做特例分支。
- **过早做的风险**：现在 UI 的形态会被「盘点 + 组包」这一个场景绑架，而跨设备
  分发（Phase 3）还没跑通，它可能带来完全不同的交互需求（版本对比、拉取、校验）。
  这正是建议排在 dogfood 之后的原因。

## 6. 开工门槛

沿用 ADR-0003 的五条，逐条对当前状态：

| # | 门槛 | 现状 |
|---|---|---|
| 1 | apply/rollback 安全问题清零 | ✅ 2C 全部收口，MCP create-only 已通过严格复审 |
| 2 | CLI schema 与兼容策略稳定 | 🟡 schema 在 `spec/`，但尚未定版本兼容策略 |
| 3 | 跨设备不可变包发布/拉取链路跑通 | ❌ Phase 3 未开始 |
| 4 | 用户主要问题变成「难搜索、难预览、难比较」 | 🟡 本次 dogfood 首次出现该信号（见 §1） |
| 5 | UI 只调用 Core 契约，不复制部署逻辑 | ✅ 架构上可满足，见 §4.3 |

**结论：还差第 3 条，第 2、4 条部分满足。**建议顺序不变：先跑完首个 dogfood 包
的真机 apply，再做 Phase 3 分发，之后启动 TUI Phase A。若 dogfood 过程中
「读 JSON 找信息」的痛感持续出现，可以把 Phase A 提前——它是纯只读的，风险最低。

## 7. 对 ADR 的影响

本文只是评估，不改决策。真要启动时需要一份 **ADR-0006：以 TUI 作为第一个交互
界面**，明确取代 ADR-0003 §5 的「Phase 3.5 只读 Web UI」，并保留 Web UI 作为
远期可选项（真出现「需要图表/富 diff/团队共享」时再评估）。ADR-0003 的其余部分
（CLI-first、Go Core、不引入 npm launcher）不受影响，反而因为 TUI 不引入
TypeScript 而被进一步坐实。
