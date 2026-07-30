# ADR-0006：TUI 作为第一个交互界面，及其写操作边界

- 状态：Accepted
- 实施：Phase A 已实现（2026-07-26）；Phase B/C、Phase D1 引导式本地闭环与
  Phase D2 Doctor/当前部署回滚于 2026-07-28 落地；Phase D3 版本与只读
  Release 检查于 2026-07-29 落地；N7.0 于 2026-07-30 接受非业务 UI 偏好边界，
  偏好存储和语言切换尚未实现
- 日期：2026-07-28
- 取代：[ADR-0003](0003-cli-first-go-core-and-product-surfaces.md) §5（本地只读
  Web UI → 本地 TUI）
- 关联：[TUI 界面评估](../research/tui-surface-assessment.md)、
  [TUI 技术方案](../designs/tui-technical-design.md)、
  [ADR-0005](0005-read-only-mcp-server-surface.md)（另一条 agent 面的只读边界）、
  [N7 偏好设置与中英文支持方案](../designs/settings-and-i18n.md)

## 背景

ADR-0003 §5 规划的第一个交互界面是「本地、只读的 Web UI」（`aiah ui` 起
`127.0.0.1:<port>`）。评估后改为**本地 TUI**：不引入 TypeScript、不开监听端口、
新机器与 SSH 上单二进制即可用，边际成本比 Web 低一个数量级。Phase A 只读浏览已
实现并完成真机 TTY dogfood，未发现需要修复的问题。

Phase B 要让界面第一次**写文件**，因此必须先把边界写下来——ADR-0003 §5 对写操作
UI 提出的五项门槛是针对 Web UI 形态写的，需要按 TUI 形态重新表述，并明确 B 与 C
各自受哪一条约束。

## 决策

### 1. 第一个交互界面是本地 TUI，不是 Web UI

取代 ADR-0003 §5。理由见评估文档；此处只固化结论：

- Core 与 TUI 都留 Go，**不转 TypeScript**；TS 只在将来真做 Web UI 或编辑器扩展
  时作为契约客户端出现；
- 界面**不得复制** apply / rollback / 编译逻辑，只调用 Core；
- 定位是**工作流操作台，不是控制面板**：本工具的「配置」就是 manifest 文件本身，
  TUI 可以编辑那个文件，但**不得引入 TUI 私有的业务设置存储**。

### 1.1 N7 只允许三项设备本地 UI 偏好

N7.0 接受一个窄例外：语言、首选资产库预填和显示密度可以成为设备本地 UI 偏好。
它们不进入 manifest、资产包、分发通道或 MCP，也不能改变 Core 规则。

- 首选资产库只预填路径，不自动选择、创建或启用写能力；
- 最近资产库历史不保存，避免只读浏览产生隐式写入；
- 显示密度只改变默认展开状态，不得隐藏路径、版本、targets、风险、变更、确认词、
  摘要或恢复信息；
- 无偏好时最终采用 `auto`：中文 locale 使用 `zh-CN`，其它使用 `en`；
- 偏好文件只有用户明确保存时才创建；必须严格 schema、`0600`、原子替换、拒绝
  软链且不含 secret；
- CLI 临时覆盖不反写偏好；typed confirmation token、CLI/JSON/MCP 契约不翻译。

这不是控制面板准入。任何新增偏好都必须再次修订本 ADR，不能借设置页加入业务事实、
自动联网、绕过校验/备份/确认或隐藏决策信息。

### 2. Phase B 的写入面 = 只有工作区

Phase B 勾选 HOME 盘点结果后，做两件事：

1. 把资产文件**复制进工作区** `<workspace>/assets/<容器>/<名字>`；
2. 在 `<workspace>/manifest.yaml` 登记这些资产并加进 profile。

「只写工作区，永不写 `.claude` / `.codex` / `.grok`」这条规则约束的是**工具目录**；
往用户自己指定的工作区里复制文件不违反它，反而是唯一能走完「装完 aiah → 勾一勾
→ 有了工作区」这条从零到一路径的形态。只写 manifest 不搬文件会让紧随其后的
`validate` 必然报 path 不存在，那种形态自相矛盾。

**工具目录零写入是行为级不变式**，与 ADR-0005 同款：测试对真实 `.claude` /
`.codex` / `.grok` 做前后快照比对，任何越界都变红。

### 2.1 没有明确确认工作区就没有写能力

`aiah ui` 默认**只读**：不给 `--workspace PATH` 时界面不显示复选框、空格不可勾选。
用户按 `w` 后必须自己输入路径并回车确认，TUI 才创建/打开该目录并开启勾选能力；
也可以在启动时显式给出 `--workspace PATH`。非 TTY 检查发生在目录创建之前。

**没有默认工作区路径**，输入框中的 `~/ai-assets` 只是 placeholder，不会自动采用。
这条把写能力做成由用户显式确认的开关，而不是一个总是在场、靠运行时判断兜底的
功能。

工作区不得等于或位于 HOME/project 的 `.agents`、`.claude`、`.codex`、`.grok`
目录内；软链指向这些目录也拒绝。路径确认阶段在创建目录前检查，`workspace.Compose`
再次检查，避免「用户把工具目录误当工作区」绕过 §2 的零写入不变式。

### 3. 工作区文件 create-only

已存在的工作区文件**不覆盖**，记 `workspace_file_exists` finding 交给人判断。
工作区是用户手写内容的所在地；界面按一次勾选就静默覆盖它，与本项目「不删除、不
覆盖未知文件」的既有立场冲突。

### 4. manifest 就地改，保留注释与键序

已存在的 manifest 通过 `yaml.Node` **就地编辑**，不经由 `Document` 结构体
round-trip 重新序列化。原因：manifest 是用户手写的 YAML，重新序列化会丢注释、
打乱键序、并吃掉本版本还不认识的字段，让 `git diff` 无法审阅。

定位不到 `assets` 序列或目标 profile 时**fail-closed**：不写、报 finding、提示
手工修，不猜测结构。

### 5. 事务化：先复制、后验证、再落盘；失败回滚

顺序固定为：

```text
规划 → 复制文件（记录本次真正创建的） → 写临时 manifest → validate
     → 通过则原子 rename；失败则删临时文件并回滚本次创建的文件
```

回滚**只删本次创建的**，绝不碰原本就存在的工作区内容。`validate` 失败时磁盘回到
操作前的状态，不留半成品——这正是设计文档「不落盘半成品」的要求。

### 6. 属性推导 fail-closed，不猜

从 inventory 资产推导 manifest 条目（`id` / `type` / `targets` / `scope` /
`portability` / `sensitivity`）时，任何一项推不出合法值就**跳过该资产并报
finding**，不用默认值蒙混：

- 只有 `skill` / `rules` / `agent` / `hook` / `memory` 五类可登记，其余
  （`config`、`credential`、`session`、`cache`、`device-state`…）不是可迁移资产；
- `sensitivity: secret` 的资产**永不**登记；
- `scope: device-private` 不登记（device scope 永不 apply，登记没有意义）；
- `portability: unknown` 保守取 `adapter-required`（走 adapter 按端编译，比原样
  分发安全）；`sensitivity: unknown` 保守取 `private`；
- `id` 由类型与名字派生并必须匹配 schema 的 pattern，派生不出就跳过。

推导规则是**建议值**，不是终审：用户可以在写出的 manifest 里改，TUI 不会再回来
覆盖它（见 §3 与 §4）。

### 7. Phase C 在跨设备分发门槛满足后落地

ADR-0003 五项门槛中的第 3 条（跨设备不可变包发布 / 拉取链路跑通，即 roadmap
第 9 项）已由 [ADR-0007](0007-immutable-channel-distribution.md) 满足。该门槛针对
的是**执行部署**的界面：

- **Phase A（只读）不受限** —— 已实现；
- **Phase B（只写工作区）不受限** —— 它写的是资产源头，不是任何设备的部署；
- **Phase C（界面里跑 `apply`）曾受限** —— 第 9 项完成后才启动，现已实现。

Phase C 复用 `apply.Diff` / `apply.Apply`，不复制业务规则。执行前必须完整输入
`apply` 再按 Enter；完成后显著展示 `backupId` 与包含所有安装根的回滚命令；失败时
原样显示 finding，不做美化。10 项变异验证和真实 PTY diff → apply → rollback 提示
链路均已通过。

### 8. Phase D1 只串联既有本地 Core

为降低首次使用门槛，TUI 在工作区写出后增加一条本地引导链：

```text
明确工作区 → 勾选并 compose → 选择 manifest profile → build 到 <workspace>/dist
           → 自动进入既有 diff → typed apply
```

- profile 仍来自 `manifest.yaml`；TUI 不保存偏好或私有设置；
- 构建直接调用 `build.Build`，输出目录固定在已确认工作区的 `dist/`；
- 成功后只把生成的 archive 路径交给既有 Phase C，不复制 diff/apply/确认逻辑；
- publish/pull、doctor 和真正执行 rollback 不纳入本批，继续使用 CLI。

工作区未创建、构建成功却未绑定 archive、`--targets` 在无显式 package 时丢失三项
变异均能使对应行为测试变红；审查修复又增加工具目录禁入、compose 后旧包失效、重建
失败后旧包失效三项变异验证。恢复后全量门禁通过。真实 PTY 已走通路径确认 → compose
→ profile → build → 自动 diff，并在 apply 前退出。

### 9. Phase D2 只维护 Doctor 识别到的当前部署

普通 `aiah ui` 增加 `h` Doctor 页，直接调用 `apply.Doctor`，展示当前 deployment、
backup 数量、drift 与 Core 原始 findings。Doctor 通过且当前 deployment 有
`backupId` 时，才开放 `x` rollback；用户必须完整输入 `rollback`，TUI 再调用
`apply.Rollback`。成功后同时刷新 Doctor 与 inventory。

本阶段不在界面里列出或猜选历史 backup；要恢复历史版本仍用 CLI 显式传
`--backup`。`aiah bootstrap` 复用 Phase C 的 diff/apply 结果契约，因此不附加
Doctor/rollback 维护入口，避免改变 bootstrap 的退出与报告语义。

### 10. Phase D3 版本页默认离线，更新检查必须由用户触发

普通 `aiah ui` 增加 `v` 版本页，显示当前 aiah 版本、commit、构建时间，以及
Doctor 识别到的当前资产 deployment 包版本。打开页面只读取本地状态，**不联网**；
只有用户再按 `c` 才调用与 CLI `aiah update --check` 相同的 `internal/update`
只读 Core。

检查只读取 GitHub latest release 元数据，不下载、不替换当前二进制。若有新版本，
界面展示绑定精确 Release tag 的安装命令；真正升级仍由用户退出 TUI 后显式执行。
`bootstrap` 继续保持 deployment-only，不暴露版本页。

## 讨论过但不采用的方案

### 保留 ADR-0003 §5 的本地只读 Web UI

需要 HTTP 服务、端口、浏览器与（一旦要做得好看）TypeScript 工具链，且在 SSH 与
新装机器上不如单二进制直接。只读浏览这个需求 TUI 已经满足。

### Phase B 只写 manifest，文件让用户自己搬

写入面确实更小，但写完 `validate` 必然失败，把一个本该闭环的操作变成"写一半再去
命令行补"。详见 §2。

### 覆盖已存在的工作区文件（带确认）

即使加确认，也是让界面获得覆盖用户手写内容的能力。create-only + finding already
把信息给到了人，人可以自己决定删了重来。

### 用 `Document` 结构体重新序列化 manifest

实现最简单，但丢注释、乱键序、吃掉未知字段，直接违反设计文档「`git diff` 只有那
一项变化」的验收标准。

## 影响

正面：

- 「从零到一」第一次能在界面里走完，不再要求用户手写 YAML；
- 工具目录零写入有行为级测试兜底，不依赖 review 自律；
- manifest 保持人类可读、可审阅、可手改；
- Phase C 只在分发门槛满足后落地，执行边界有行为测试兜底。

代价：

- `yaml.Node` 就地编辑比重新序列化复杂，节点定位失败要显式 fail-closed；
- 属性推导保守，用户可能需要手工调 `targets` 或 `portability`；
- 事务回滚增加了一条必须测试的失败路径。
