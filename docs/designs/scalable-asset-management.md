# N8：规模化资产管理增强方案

- 状态：**N8.0 评估完成；N8.1 已通过 [PR #34](https://github.com/dff652/ai-asset-hub/pull/34)
  合入 `dev`，并随 public `v0.1.8` 发布；Release notes 曾遗漏该范围，见发布后审计**
- 日期：2026-08-01
- 关联：[资产模型](../asset-model.md)、
  [TUI 产品体验 V2](tui-product-experience-v2.md)、
  [不可变分发 ADR](../decisions/0007-immutable-channel-distribution.md)、
  [MCP 只读边界 ADR](../decisions/0005-read-only-mcp-server-surface.md)

## 1. 结论

N8 先增强现有文件优先资产库，不引入数据库、服务端、云账户或后台同步。
当前真实样本仍是小型个人资产库：本机盘点得到 22 个资产候选，显式选择的本机测试
资产库 manifest 登记 7 项资产（2 条 rules、5 个 skills）。
这个规模下，现有 source → type 分组和内存增量筛选足够；优先补齐筛选覆盖面、
manifest 来源字段的类型化读取，以及备份/恢复术语和验收门槛。

本阶段不增加 `tags`、`description`、`license` 字段。manifest v1 对未知字段使用
`additionalProperties: false`，直接给 v1 添加新字段会让旧版 aiah 拒绝新清单；如有
真实需求，应先设计 manifest v2 和兼容矩阵。

## 2. 评估证据

### 2.1 真实样本

2026-08-01 在开发机执行只读盘点，并只保留聚合统计：

```bash
go run ./cmd/aiah scan --home "$HOME" --project "$PWD" --output json
```

结果：

| 指标 | 数量 |
|---|---:|
| 资产总数 / 候选资产 | 22 / 22 |
| Entry 总数 | 129 |
| included / reported / excluded / unreadable | 33 / 13 / 83 / 0 |
| config / plugin / rules / skill | 5 / 3 / 3 / 11 |
| claude / codex / grok / shared | 7 / 4 / 6 / 5 |
| findings | 0 |

另只读解析显式测试资产库的 `manifest.yaml`：schemaVersion 1，7 项资产、1 个
profile。该目录只作为本次评估样本，不把它自动设为任何用户的首选资产库；公开文档
不记录设备上的绝对路径。

### 2.2 当前能力与缺口

| 主题 | 已有能力 | 本轮发现的缺口 |
|---|---|---|
| 查找 | TUI `/` 增量筛选、source/type 分组 | 只匹配路径、类型和文件；“仅在资产库”不参与过滤；已关联 finding 不能按 code/message 找到 |
| 来源 | manifest v1 schema 已有可选 `source.url/revision/importedAt` | Go typed manifest 未读取这些字段，TUI 也不能按它们筛选 |
| 文件证据 | schema 已有可选 `files.path/sha256`；build 会计算包内文件哈希 | Go typed manifest 未读取 manifest 声明的 `files`；声明值与构建实测值不是同一事实 |
| 备份 | 用户可用 Git/NAS 备份资产库；apply 生成本机恢复点 | 没有“资产库备份就绪”报告，也不能证明异机恢复成功 |
| 恢复 | pull/preflight/diff/apply/doctor/rollback 已形成受控链路 | 尚无单一恢复演练报告；网络传输仍由 Git/rsync/U 盘等外部工具负责 |

## 3. 术语与边界

### 3.1 检索资产

首版称“筛选资产”或“检索资产”，不称“全文搜索”。它只匹配结构化元数据、路径、
文件名和 findings，不索引资产正文，也不做语义检索。

### 3.2 来源信息

`source.url`、`source.revision`、`source.importedAt` 是资产库作者声明的来源信息：

- 方便定位原始仓库和导入版本；
- 不是下载记录、签名、可信证明或更新授权；
- 不能从 URL 推断许可证；许可证必须是未来单独审核并显式写入的字段。

盘点模型中的 `asset.source=claude|codex|grok|shared|project` 表示本机发现来源工具，
与 manifest 的上游 URL 不是同一个概念，界面和 API 不得混称。

### 3.3 三种容易混淆的恢复概念

| 名称 | 含义 | 当前责任方 |
|---|---|---|
| 资产库备份 | 保留 manifest、assets 和历史版本，并能从独立位置取回 | 私有 Git、NAS、快照或用户选择的备份工具 |
| 安装恢复点 | apply 前保存被覆盖的目标文件，用明确 `backupId` rollback | aiah 本机 `.aiah/backups` |
| 不可变分发 | publish/versions/pull 搬运已构建的固定版本包 | aiah 管包和校验；网络/介质传输由外部工具负责 |

因此 `.aiah/backups` 不能叫“资产库备份”，publish/pull 不能叫“双向同步”。

“资产库备份就绪”只表示 manifest 校验、profile 解析、构建准备和文件读取通过，
可生成确定性安装包；它不证明异地副本已经存在。“恢复验证”必须在隔离目标环境完成
取回、前置检查、diff、人工确认 apply、doctor，必要时再验证 rollback。

## 4. 分阶段实现

### N8.0：规模与边界评估（完成）

- 用真实样本记录资产数、Entry 数、类型、来源和 findings；
- 复核 manifest、TUI、CLI、MCP、build、publish/pull 和本机恢复点边界；
- 冻结“文件优先、无后台同步、无数据库、无自动网络传输”的当前方案。

### N8.1：统一筛选与来源读取（已随 `v0.1.8` 发布）

- TUI `/` 匹配路径、文件、来源工具、scope、type、portability、sensitivity、
  inventory 状态、feature、资产 ID、资产库路径、目标工具和资产库状态；
- “仅在资产库”条目遵循相同筛选，不再在无匹配结果时常驻；
- 已关联 inventory finding 和 catalog finding 可按 code、severity/message/path 检索；
- 支持用当前界面语言的资产库状态检索；
- typed manifest 读取 schema v1 已存在的 `source` 与 `files`，不改变 schema；
- 所有操作保持只读，不扫描资产正文，不增加持久化索引。

验收：中英文提示一致；匹配与不匹配、仅库内、来源字段、finding 均有测试；
`go test ./internal/tui ./internal/workspace` 与完整本地门禁通过。

2026-08-01 本地验收结果：

- `./scripts/check-local.sh` 全部通过，包括环境、许可证、发布/安装器/README 资产、
  全仓测试、race、构建和假 HOME 闭环；
- 真 TTY 裸 `aiah` 正常进入任务首页并可用 `q` 退出；`TERM=dumb` 按设计拒绝进入，
  这不是裸命令回归；
- 真 TTY 以显式测试资产库打开资产页，`/ source-changed` 和中文顶部术语
  `/ 待更新` 均准确筛出 10 个“源端有更新”资产；
- dogfood 先发现“待更新”与详情“源端有更新”的同义词缺口，修复后增加回归测试。

### N8.2：资产库备份就绪与恢复演练报告（需求已确认，转入 N10）

建议先做共享 Core 的只读报告，再按顺序暴露给 CLI、TUI、MCP：

1. `validate`：清单与契约有效；
2. `prepare`：profile、依赖、冲突、target、secret 前置条件通过；
3. `build`：生成包并记录坐标与 SHA256；
4. `backupEvidence`：只记录用户提供的 Git commit、快照 ID 或外部副本位置，不代替传输；
5. `restoreExercise`：在隔离 HOME/project 记录 pull/preflight/diff/apply/doctor/rollback 结果。

TUI 可显示“可构建 / 已有外部备份证据 / 已通过恢复演练”三个独立状态，不合并成一个
绿色“已备份”。MCP 首版继续只读；任何写入、发布或恢复动作仍由人操作 TUI/CLI。

2026-08-01 已确认该方向符合“AI 工具资产管理 + 换机迁移”定位。后续不在 N8 内继续
堆范围，改由 [N10：迁移准备检查](migration-readiness.md)冻结用户术语、只读报告
契约、CLI/TUI/MCP 分期与隔离恢复演练边界。

### N8.3：描述、标签和许可证（暂缓）

只有出现下列真实需求之一才启动：

- 同一资产需要跨 source/type 的稳定业务分类；
- 用户反复依赖路径猜测用途；
- 需要发布第三方资产且必须审计许可证；
- 多个资产库需要合并或去重。

启动前必须先完成 manifest v2 设计、v1/v2 读写兼容矩阵、升级/降级行为和 MCP 输出
版本化。标签是人工维护的分类，不从目录名自动生成；许可证是显式审核结果，不从 URL
自动推断。

### N8.4：索引、数据库或服务端（不启动）

仅当代表性资产库的实测数据证明文件优先方案不能满足体验目标时再评估，例如：

- 资产数量显著高于当前 22 项，并出现持续查找困难；
- TUI 内存过滤或重新盘点的延迟经过优化仍不可接受；
- 多进程并发编辑、团队权限或远程查询成为明确需求。

届时数据库也只能是可重建索引，`manifest.yaml + assets/` 仍是事实源；不得先把本地
SQLite 或服务端状态变成第二份权威数据。

## 5. 后续顺序

1. N8.1 已通过 [PR #34](https://github.com/dff652/ai-asset-hub/pull/34) 合入 `dev`，
   完整门禁、严格 diff review、真 TTY dogfood 和合并后 CI 已完成，并随
   `v0.1.8@21ef3fc` 发布；
2. 提交范围和敏感信息检查已通过，公开文档不含用户私有绝对路径；
3. 收集实际筛选关键词、资产规模和失败案例，不以假数据推动 schema 膨胀；
4. N8.2 需求已确认并转入 N10；先实现只读报告契约，不先做自动传输或证据写入；
5. N8.3/N8.4 保持触发式，不作为下一个版本的默认承诺。
