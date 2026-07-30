# 使用流程总览

本页回答“aiah 可以完成哪些任务、应该从哪里进入、哪些步骤会写文件”。第一次使用
优先走本地五步主线；日常维护、安装恢复、跨设备、AI 接入和 aiah 自身升级是独立
流程，不应全部塞进一张图。

## 0. 先按任务选择流程

| 用户任务 | 简化流程 | 推荐入口 | 当前边界 |
|---|---|---|---|
| 第一次整理并应用 | 发现 → 整理 → 准备 → 预览 → 人工确认 | `aiah` | TUI 已覆盖；前四步不写目标工具目录 |
| 日常维护资产 | 查看统一状态 → 更新/移出 → 预览 → 应用 | `aiah` | `update/remove/apply` 都是显式操作，不做后台同步 |
| 检查或撤销安装 | 安装检查 → 判断漂移 → typed `rollback` | `aiah` 或 CLI | 恢复点只在本机使用，不等于资产库备份 |
| 迁移到其他设备 | 换机检查 → build/publish → 外部搬运 → versions/pull → 取回版本检查 → diff/apply | TUI（开发候选）/ CLI | E3.2/E3.3 已合入 `dev`，E3.4 候选绑定所选发布包；`v0.1.6` 仍为 E3.1；网络传输不归 aiah |
| AI 或自动化接入 | MCP 只读查询，或 CLI JSON 编排 | `aiah mcp` / CLI | MCP 不开放 build/apply/rollback |
| 安装或升级 aiah | 检查版本 → 显式安装指定 Release → 复核版本 | installer / `update --check` | 不后台自更新；这不是用户资产生命周期 |

## 1. 第一次整理并应用

这是 README 中五步 SVG 表达的**主流程**，目标是让新用户完成一次可审阅、可撤销的
本地安装：

```text
发现资产 → 整理资产库 → 检查并准备 → 预览变化 → 人工确认
```

| 阶段 | TUI 表达 | Core / CLI | 会写哪里 |
|---|---|---|---|
| 发现 | 本机 AI 资产 | `scan` | 零写入 |
| 整理 | 纳入、更新、移出 | `compose` / Workspace Core | 只写明确选择的资产库 |
| 准备 | 检查资产、准备安装包 | `validate` / `build` | 只写资产库的 `dist/` |
| 预览 | 变更预览 | `diff` / `apply --dry-run` | 目标工具目录零写入 |
| 确认 | 完整输入 `apply` | `apply` | 原子写目标，并先创建安装恢复点 |

完成后运行安装检查；需要恢复时再进入第 3 节。详细按键和首次操作见
[上手指南 §2](getting-started.md#2-五步完成一次安全应用)。

## 2. 日常维护资产

### 2.1 工具目录发生变化

```text
源端有更新 → 选择 update → 资产库完整替换 → 选择资产组合 → diff → typed apply
```

`update` 是一次明确的“源端 → 资产库”替换，只修改资产库；它不是监听文件变化，也
不是双向同步。替换后仍需独立审阅并确认应用。

### 2.2 直接维护资产库

资产库是可编辑事实源。直接修改 `assets/` 或 `manifest.yaml` 后：

```text
编辑资产库 → validate/build → diff → typed apply → doctor
```

不要把 `.claude`、`.codex`、`.grok` 继续当成长期源码，否则下次应用会覆盖这些
临时修改。

### 2.3 不再纳管某项资产

```text
选择已纳管资产 → 输入 remove → 从 manifest/资产库移出 → 重新预览
```

`remove` 不删除源端工具文件。执行前应使用 Git、NAS 快照或其它备份工具保护资产库。

## 3. 安装检查与撤销

```text
doctor → 正常 / 漂移 / 缺失 / 前置条件失败 → 必要时 typed rollback
```

- `doctor` 只读检查当前安装、journal、恢复点、文件漂移和 MCP 前置状态；
- TUI 只撤销安装检查识别到的当前安装，历史恢复点由 CLI 显式指定；
- `.aiah/backups` 是 apply 前的**本机安装恢复点**，不是资产库备份；
- 资产库历史与灾难恢复仍交给私有 Git、NAS 快照或用户选择的备份系统。

## 4. 跨设备迁移

跨设备包含两条不同链路：

1. **可编辑资产库迁移**：私有 Git、NAS 快照或移动介质负责备份和恢复事实源；
2. **不可变安装分发**：aiah build/publish，外部工具搬运，新设备
   versions/pull/bootstrap，再走 diff + typed `apply`。

```text
旧设备：换机前置检查 → 检查并生成分发包 → publish
传输层：Git / NAS / rsync / U 盘（aiah 不参与）
新设备：versions → pull → 取回版本检查 → diff → typed apply → doctor
```

TUI 的“迁移到其他设备”先只读比较资产库、当前安装和用户选择的普通目录通道。
当前 `dev` 的 E3.2 在同一页增加两条显式路径：

- 发布：`p` → 选择资产组合 → build → 核对包/通道 → typed `publish`；
- 取回：`v` → 明确选择版本/profile → 输入已有输出目录 → pull。

E3.3 在同一页增加第三条只读路径：

- 检查：`e` → 选择资产组合 → 查看全部目标、secret、本机不迁移项与问题；
- credential/session/cache/device-state 等按设计排除，只作提示；
- 缺失 secret、不支持目标和 adapter 丢弃是阻止项；adapter 降级需人工确认；
- 检查只针对当前设备和当前资产库/profile，不创建 `dist/`，也不替用户发布、
  取回或应用。

E3.4 候选把取回后的连续路径补成：

```text
pull → 绑定 name/version/profile/SHA256 → 目标设备检查
     → Enter → diff → typed apply → doctor
```

包级检查重新打开实际 `.tar`，并复用同一 target/adapter/secret/device-private
报告。坐标或摘要不匹配、有阻止项时不能进入 diff；检查通过也不会自动应用。

光标默认停在最后发布项只用于导航，不会自动取回；TUI 不比较版本号大小。取回不会
覆盖输出目录中不同或残缺的同名产物，完整同内容四件套才视为幂等。`v0.1.6` 公开版
仍只提供 E3.1 状态页，全部 CLI 命令继续兼容。完整命令见
[跨设备迁移 runbook](runbooks/cross-device-transfer.md)。

凭据、session、cache、数据库、device scope 和厂商运行时状态默认不迁移。密钥只在
目标设备 apply 时解析，不进入包、报告或恢复点 metadata。

## 5. AI 与自动化接入

### 5.1 AI 工具通过 MCP 读取状态

`aiah mcp` 暴露 `scan`、`validate`、`diff`、`doctor`、`version` 五个只读工具。
AI 可以盘点、校验和解释差异，但不能通过 MCP build、apply 或 rollback。

这是有意的权限边界：写操作必须继续由人或显式 CLI 自动化负责，并保留路径、diff、
确认和恢复证据。

### 5.2 脚本与 CI

需要机器可读结果时使用 CLI `--output json`。脚本可以显式调用完整 CLI，但写操作
仍需自己提供包、HOME/project、目标工具和错误处理；不要把 TUI 按键自动化当成稳定
API。

## 6. aiah 自身安装与升级

这条流程管理的是 `aiah` 二进制，不是 Skills、Rules 或 MCP 模板：

```text
update --check（只读） → 选择精确 Release → 校验 SHA256 → 原子替换 → version
```

aiah 不后台升级，也不修改 shell profile。安装和升级必须显式触发；同版本复装应
零下载、零替换。当前版本与已知升级提示问题见
[上手指南 §安装](getting-started.md#安装)和
[安装/升级 dogfood SOP](runbooks/install-upgrade-dogfood.md)。

## 7. README 为什么只放一张主流程 SVG

README 的 `usage-flow.svg` 只负责回答“第一次如何安全成功”，不是完整功能地图。
保留一张主流程图是有意设计：

- 新用户只需先理解一条从发现到人工确认的路径；
- 日常维护、迁移、MCP 和工具升级不是同一时序，强行合并会形成难以在手机端阅读的
  巨型图；
- 其它流程更依赖命令、当前实现状态和限制，应保留为可复制、可搜索、易更新的
  Markdown；
- 跨设备的独立生命周期图继续放在上手指南，TUI 证明板负责展示真实界面状态。

因此，“一张主流程 SVG + README 任务表 + 本页详细流程”足够；只有未来某条次要流程
成为主要入口，并且文字已经无法清楚表达时，才新增第二张 README 流程图。
