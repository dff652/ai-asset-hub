# ADR-0006：TUI 作为第一个交互界面，及其写操作边界

- 状态：Accepted
- 实施：Phase A 已实现（2026-07-26）；Phase B/C 于 2026-07-28 落地
- 日期：2026-07-28
- 取代：[ADR-0003](0003-cli-first-go-core-and-product-surfaces.md) §5（本地只读
  Web UI → 本地 TUI）
- 关联：[TUI 界面评估](../research/tui-surface-assessment.md)、
  [TUI 技术方案](../designs/tui-technical-design.md)、
  [ADR-0005](0005-read-only-mcp-server-surface.md)（另一条 agent 面的只读边界）

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
  TUI 可以编辑那个文件，但**不得引入 TUI 私有的设置存储**。

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

### 2.1 没有 `--workspace` 就没有写能力

`aiah ui` 默认**只读**：不给 `--workspace PATH` 时界面不显示复选框、空格不可勾选、
`w` 明确拒绝并提示如何开启。**没有默认工作区路径**——猜一个写入目标正是本项目最
不该做的事。

这条把写能力做成了结构性的、由用户显式开启的开关，而不是一个总是在场、靠运行时
判断兜底的功能。

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
