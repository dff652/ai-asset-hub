# N10：迁移准备检查

- 状态：N10.1–N10.3 已随 public `v0.1.10` 发布；N10.4 明确延期
- 日期：2026-08-02
- 来源：N8.2「资产库备份就绪与恢复演练报告」
- 关联：[跨设备迁移方案](cross-device-migration-and-version-sync.md)、
  [不可变分发 ADR](../decisions/0007-immutable-channel-distribution.md)、
  [MCP 只读边界](../decisions/0005-read-only-mcp-server-surface.md)

## 1. 要解决的问题

AI Asset Hub 已能检查资产库、构建不可变包、发布/取回版本、预览并应用到另一台
设备，也能用 `doctor` 和 `rollback` 验证安装结果。但这些能力分散在不同页面和
报告里，用户仍难以直接回答三个换机前问题：

1. **这份资产库现在能不能打包？**
2. **打包结果有没有放到机器之外？**
3. **这份副本是否真的恢复成功过？**

N10 把已有只读检查组合成一份「迁移准备检查」，并把外部备份证据与恢复演练证据
作为两个独立事实展示。它不是新的同步引擎，也不把“写过一个路径”冒充“已经备份”。

## 2. 用户术语

TUI 和用户文档使用下列名称；JSON/API 保留稳定、可机器读取的字段名。

| 用户看到的名称 | JSON 维度 | 准确含义 |
|---|---|---|
| 可以打包 | `packageReadiness` | manifest、profile、依赖、冲突和所选资产内容满足 `build.Prepare`；**没有实际生成包** |
| 已记录外部副本 | `backupEvidence` | 用户提供了与当前资产库或包绑定的 Git commit、快照 ID、对象地址或离线介质说明；**aiah 没有验证远端可用性** |
| 恢复已验证 | `restoreExercise` | 有一份与明确包 SHA256、profile 和目标绑定的隔离恢复演练结果，且 apply、doctor、rollback 门禁通过 |

目标支持、degraded/dropped、设备私有项和 Secret 是否可用另列为
`migrationPreflight`。例如缺少目标设备 Secret 时，资产库仍可能“可以打包”，但整体
迁移准备必须是 `blocked`；不能为了得到一个简单状态而篡改 `build` 的真实语义。

不使用下列容易误导的绿色总状态：

- 「已备份」：aiah 不负责传输，无法仅凭本机文件证明备份存在；
- 「已同步」：当前没有后台同步、双向合并或冲突自动解决；
- 「可恢复」：没有真实恢复演练时只能说“具备打包条件”；
- 「全部安全」：设备私有状态和 Secret 仍需用户按提示单独处理。

首页入口建议命名为 **换机与备份**，主操作命名为 **检查迁移准备**。已有
「跨设备」发布、取回和应用流程保留，不另造第二套迁移向导。

## 3. 三个状态必须独立

报告不能用一个 `ok` 抹平三个不同事实：

| 可以打包 | 外部副本 | 恢复演练 | 用户应看到的结论 |
|---|---|---|---|
| 否 | 任意 | 任意 | 先修复资产库或前置条件 |
| 是 | 未提供 | 未提供 | 可以打包；尚无外部副本和恢复证据 |
| 是 | 已记录 | 未提供 | 已记录副本；建议做一次隔离恢复演练 |
| 是 | 未提供 | 已通过 | 演练记录存在，但仍需补充外部副本位置 |
| 是 | 已记录 | 已通过 | 本次选择的迁移准备证据完整 |

上表假定 `migrationPreflight` 没有阻止项。顶层 `ok=false` 表示构建或迁移前置被
阻止；证据缺失、失配、失败或无效不会单独把 `ok` 置为 `false`。迁移准备完整度
另用 `level: blocked | attention | ready` 表达：

- `blocked`：`packageReadiness` 或 `migrationPreflight` 为 `blocked`；
- `attention`：可以打包且迁移前置条件不阻止，但有 degraded 项，或至少一项证据
  缺失、失配、失败或过期；
- `ready`：构建与迁移前置条件通过，两个证据维度也与当前 subject 精确绑定并通过。

## 4. N10.1 共享 Core 只读契约

第一阶段只增加共享 Core 报告，不创建包、不写证据、不访问网络：

```go
package readiness

type Options struct {
    WorkspaceRoot          string
    ManifestPath           string
    Profile                string
    Home                   string
    Project                string
    BackupEvidencePath     string // 可选，只读 JSON/YAML 文件
    RestoreExercisePath    string // 可选，只读 JSON 文件
}

func Inspect(Options) (Report, error)
```

Core 必须复用已有实现：

- `workspace.Inspect` / `validate`：资产库与 schema；
- `build.Prepare`：profile、依赖、冲突和可构建性；
- `migration.InspectPreflight`：target、degraded/dropped、Secret 引用和设备私有项；
- `pkgload` / channel release identity：已有包或演练记录的 SHA256 绑定；
- `apply.Doctor`：演练目标的安装健康；不得自行复制这些规则。

### 4.1 报告骨架

```json
{
  "schemaVersion": 1,
  "kind": "migration-readiness",
  "producedBy": "aiah 0.x.y+commit",
  "ok": true,
  "level": "attention",
  "subject": {
    "name": "personal",
    "version": "1.2.3",
    "profile": "default",
    "selectionSHA256": "..."
  },
  "packageReadiness": {
    "status": "ready",
    "assetCount": 12,
    "fileCount": 20
  },
  "migrationPreflight": {
    "status": "ready",
    "targets": ["claude", "codex", "grok", "shared"],
    "degradedItems": 0,
    "missingSecrets": 0
  },
  "backupEvidence": {
    "status": "missing"
  },
  "restoreExercise": {
    "status": "missing"
  },
  "findings": []
}
```

稳定状态枚举：

- `packageReadiness.status`: `ready | blocked`；
- `migrationPreflight.status`: `ready | attention | blocked`；
- `backupEvidence.status`: `missing | recorded | mismatch | invalid | unchecked`；
- `restoreExercise.status`: `missing | passed | failed | mismatch | invalid | unchecked`。

`unchecked` 只在资产选择本身已阻止、无法建立 subject 时使用；此时即使传入证据路径
也不读取，避免在更外层契约失败后继续扩大读取面。

`selectionSHA256` 是排序后的 resolved `PackageManifest`（其中含所选文件 SHA256）的
规范摘要，不是原始 `manifest.yaml` 的文件哈希。这样资产正文变化会让证据失配，
同时只读检查不需要先写出 tar。实际恢复演练仍必须绑定构建产物的 archive SHA256。

数组必须稳定排序；未知字段 fail closed；所有路径先做根内、软链和普通文件检查。
报告不得包含 Secret 值、认证文件内容、HOME 绝对路径或外部副本凭据。
`shared` 是共享权威根的机器 target；TUI 显示为“共享资产”，不把它描述成独立
AI 工具。target 字段沿用 manifest 的可扩展 ID 规则，不在报告 schema 中枚举当前
实现；否则一个尚无 adapter 的 target 会让本应解释阻止原因的报告自身失效。

### 4.2 证据不是事实源

外部证据文件只描述“用户把什么放到了哪里”，不进入发布包，不改变
`manifest.yaml + assets/` 的事实源地位。最小字段为：

```yaml
schemaVersion: 1
kind: backup-evidence
subject:
  name: personal
  version: 1.2.3
  profile: default
  selectionSHA256: "..."
copy:
  type: git-commit # git-commit | snapshot | object | offline-media
  reference: "..."
recordedAt: "2026-08-01T00:00:00Z"
```

`reference` 是不透明标识，不执行、不联网、不展开 shell。凭据、Cookie、Token、挂载
密码和私有拓扑备注不得写入该文件。报告默认只返回副本类型、记录时间和 reference
的摘要，不向 MCP 回显完整 reference。若 subject 与当前选择不一致，状态必须是
`mismatch`，不能显示“已记录外部副本”。

N10.1 只读取资产库内 `.aiah/evidence/` 下由用户明确指定的普通文件；该目录不进入
manifest、资产包或 Inventory candidate。路径不得越出此目录，也不得经过软链接。
CLI/TUI/MCP 共用这一限制，避免只读 MCP 借任意路径参数读取其它本机文件。N10.1
不会创建 `.aiah/evidence/`；未来记录器属于独立写操作。

恢复演练证据还必须绑定：

- package name/version/profile/SHA256；
- targets；
- 隔离 HOME/project 标记；
- pull/preflight/diff/apply/doctor/rollback 每一步结果；
- `producedBy` 与完成时间。

仅手写 `passed: true` 不构成通过证据。N10.1 可以读取并校验现有演练记录，但正式
记录器必须等 N10.4 的隔离演练编排完成后再开放。

## 5. 分阶段产品入口

### N10.1：Core + CLI 只读报告

建议命令：

```bash
aiah readiness \
  --workspace <asset-library> \
  --profile <profile> \
  [--backup-evidence <file>] \
  [--restore-exercise <file>] \
  [--output text|json]
```

CLI 默认只输出检查结果和下一步，不写文件。`--output json` 必须由独立
`spec/migration-readiness.schema.json` 约束。没有 profile 时不得猜测第一个 profile；
交互选择属于 TUI，CLI 应明确报错。

### N10.2：TUI「换机与备份」页面

页面按三个状态卡展示，并提供连续但不自动执行的下一步：

1. 不能打包 → 返回资产库问题或换机前置检查；
2. 可以打包 → 进入现有构建流程；
3. 没有外部副本证据 → 说明如何用 Git/NAS/网盘/U 盘传输；
4. 没有恢复演练 → 进入 N10.4 隔离演练说明或向导；
5. 三项通过 → 展示绑定的 profile、targets、SHA256 与检查时间。

TUI 只编排同一 Core。读取证据前展示路径；任何未来的“记录证据”都必须是独立、
显式写操作，不能在打开页面或刷新状态时隐式落盘。

### N10.3：MCP 只读查询

候选工具名：`aiah_migration_readiness`。输入与 CLI 只读参数对应，输出同一 Report。
它不得 build、publish、pull、apply、rollback、创建证据或访问网络。

开放前必须同步 ADR-0005、工具 schema、7→8 工具清单和
`TestToolCallsWriteNothing` 的所有可达目录；还要做一次真实客户端协议与零写入验收。

### N10.4：隔离恢复演练

这是后续写操作阶段，不与 N10.1 一起实现。它只允许在用户明确选择的隔离
HOME/project 下编排现有 pull/preflight/diff/apply/doctor/rollback；开始前显示精确
目录并 typed confirm。完成后生成可审计记录，失败也保留步骤结果，不能把部分成功
写成 `passed`。

N10.4 不允许：

- 对真实 HOME 静默演练；
- 自动上传包或证据；
- 跳过 diff、doctor 或 rollback；
- 将 Secret 解析值写入演练记录；
- 由 MCP 触发。

## 6. 验收门槛

### Core / CLI

- 同一输入产生稳定 JSON，符合 schema；
- manifest/profile/依赖/冲突准确路由到 `packageReadiness`，target/Secret/设备私有项
  准确路由到 `migrationPreflight`；
- 证据缺失、损坏、subject/SHA 失配分别可见；
- 证据路径的软链逃逸、特殊文件和超大文件 fail closed；
- fake HOME、asset library、project 和证据目录在只读调用前后 tree diff 为零；
- CLI text 与 JSON 含义一致，JSON 枚举不随界面语言变化；N10.2 再通过既有 TUI
  message catalog 提供完整中英文，不在 CLI 内复制一套翻译状态机；
- 新行为测试完成变异验证，完整 `./scripts/check-local.sh` 通过。

### TUI

- 第一屏能回答三项状态和下一步；
- 360/900 宽度无截断、状态不只靠颜色表达；
- 刷新和打开页面零隐式写入；
- 仍复用现有构建、跨设备和应用页面，不复制业务流程。

### MCP

- 工具 schema `additionalProperties: false`、`readOnlyHint: true`；
- 直接协议能返回同一报告；
- fake HOME/project/workspace/evidence/output 全部零写入；
- Claude Code、Codex、Grok 客户端结果按“握手”和“模型实际调用”分别记录。

## 7. 明确不做

- 后台同步、定时任务、冲突自动合并；
- 云存储、Git 托管或 NAS 客户端；
- 自动判断某个远端副本一定可恢复；
- 把 `.aiah/backups` 安装恢复点当成换机备份；
- 数据库、账号系统、服务端控制面；
- MCP 写操作。

## 8. 实施顺序与停止条件

1. N10.1 Core/CLI 只读报告；
2. 在真实资产库上收集“缺失/失配/已记录”三类结果；
3. N10.2 TUI 页面；
4. N10.3 MCP 只读查询；
5. 只有用户确实需要自动生成恢复演练证据时才启动 N10.4。

任一阶段若需要改变 manifest schema、写入资产库、访问网络或把 MCP 扩成写操作，
必须停止并另做 ADR；不能借“迁移准备”顺带扩大权限。

## 9. N10.1 源码候选检查点（2026-08-01）

Core、schema、CLI、零写入/安全测试和六项变异验证已完成，完整本地门禁通过。
实现过程中把构建就绪与 Secret/target 前置条件拆开，补入 `shared` 机器 target，并让
恢复证据精确匹配当前 target 集合。当前仍是未发布源码候选；详细证据、限制和未完成
范围见 [N10.1 检查点](../reviews/2026-08-01-n10-1-readiness-core-cli-checkpoint.md)。
