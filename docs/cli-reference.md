# CLI 命令参考

本文是用户侧命令入口摘要。JSON 契约见 `spec/`，安全写入流程见
[上手指南](getting-started.md)，设计理由见[架构决策](decisions/)。

## 命令总表

```bash
aiah
aiah init <directory> [--name <name>] [--version <version>] --output json
aiah scan --home <path> [--project <path>] --output json
aiah validate --manifest <path> [--root <path>] --output json
aiah build --manifest <path> --profile <name> --out <dir> [--root <path>] --output json
aiah diff --package <tar|dir> [--home <path>] [--project <path>] [--targets claude,codex,grok] --output json
aiah apply --package <tar|dir> [--home <path>] [--project <path>] [--targets claude,codex,grok] [--dry-run] --output json
aiah rollback [--home <path>] [--project <path>] [--backup <id>] --output json
aiah publish --package <tar> --channel <dir> --output json
aiah pull --channel <dir> --name <name> [--version <v>] [--profile <p>] --out <dir> --output json
aiah versions --channel <dir> [--name <name>] --output json
aiah bootstrap --channel <dir> --name <name> [--version <v>] [--profile <p>] --out <dir> [--home <path>] [--project <path>] [--targets claude,codex,grok]
aiah doctor [--home <path>] [--project <path>] --output json
aiah ui [--home <path>] [--project <path>] [--workspace <path>] [--package <tar|dir>] [--targets claude,codex,grok] [--language auto|zh-CN|en] [--density standard|detailed]
aiah mcp
aiah update --check [--output text|json]
aiah version [--output text|json]
```

所有 JSON 报告与部署记录都带 `producedBy`，用于追踪生成它的二进制版本。

## `init`

```bash
aiah init ~/ai-assets --output json
aiah init ~/ai-assets --name team-shared --version 2026.08.1 --output json
```

脚手架一个资产工作区：`manifest.yaml` 加上 adapter 实际读取的五个目录
（`assets/{agents,hooks,mcp,rules,skills}`）。不创建任何没有消费者的目录。

这是唯一带位置参数的子命令（其余全部只用 flag），因为 `init <directory>` 是同类
工具的通用形态。flag 放在目录前后都可以。

| 行为 | 说明 |
|---|---|
| create-only | **已存在的 manifest 永不改写。** 重跑只补缺失目录，报告里列进 `existing` |
| 幂等 | 第二次运行 `created` 为空 |
| 确定性 | 默认版本是固定的 `0.1.0`，不是当天日期；两次 init 产物逐字节一致 |
| fail-closed | 路径被非目录或软链占用时拒绝，且一个字节都不写 |

`--name` 默认由目录名归一（`My AI Assets` → `my-ai-assets`）。归一不出合法名时
**要求你显式传** `--name` 而不是猜——这个值会进 manifest 和包文件名，猜错是永久的。

产物零手改即可通过 `validate`。`build` 需要先加入第一个资产：空 profile 会报
`empty_selection`，打一个空包没有意义。

**init 只是建工作区，不让它「被找到」。** 后续命令仍须显式 `--manifest`，不存在
隐藏状态决定某条命令作用在哪个资产库上。

## `scan`

只读盘点 Claude Code、Codex、Grok 与共享资产目录，输出
`spec/inventory.schema.json` 对应的确定性 JSON：

```bash
aiah scan --home "$HOME" [--project <project>] --output json
```

扫描不写目录、不执行 hook、不跟随逃逸软链接。报告路径使用 `home/...` 或
`project/...`，不泄露真实绝对路径；凭据、会话、cache 和数据库只报告排除原因。

## `validate` 与 `build`

```bash
aiah validate --manifest <manifest.yaml|json> [--root <workspace>] --output json
aiah build --manifest <manifest.yaml|json> --profile <name> \
  --out <dir> [--root <workspace>] --output json
```

`validate` 只读检查 schema、依赖/冲突/profile、路径逃逸、软链接、疑似密钥、
二进制和大小边界。`build` 复用相同检查，按 profile 选择资产，输出带 profile 的
tar、manifest、lock 与 SHA256 sidecar。输出采用临时目录 + rename，不产生半成品。

## `diff`、`apply` 与 `rollback`

```bash
aiah diff --package <tar|dir> --home "$HOME" [--project <project>] \
  [--targets claude,codex,grok] --output json
aiah apply --dry-run --package <tar|dir> --home "$HOME" --output json
aiah apply --package <tar|dir> --home "$HOME" --output json
aiah rollback --home "$HOME" [--project <project>] --backup <id> --output json
```

`diff` 永远只读；`apply --dry-run` 走与真实 apply 相同的计划路径。真实 apply 只写
adapter 声明的目标，事务记录和 backup 放在 meta 根下的 `.aiah/`。中途失败会尝试
自动回滚；恢复不完整时保留 journal 供 doctor 报告。

Scope 规则：

- `global` 使用 `--home`；
- `project` 使用 `--project`；
- `device` 永不部署；
- 可同时给出 home 与 project。

主要映射：

| 包内路径 | Claude | Codex | Grok | Shared |
|---|---|---|---|---|
| `assets/skills/<name>/…` | `.claude/skills/…` | `.codex/skills/…` | `.grok/skills/…` | `.agents/skills/…` |
| `assets/rules/…` | `.claude/rules/…` | `.codex/rules/…` | `.grok/rules/…` | — |
| `assets/agents/…` | `.claude/agents/…` | `.codex/agents/…` | `.grok/agents/…` | — |
| `assets/hooks/…` | `.claude/hooks/…` | `.codex/hooks/…` | `.grok/hooks/…` | — |
| `assets/mcp/…` | sidecar + Claude native config | sidecar + Codex native config | sidecar + Grok native config | — |

MCP 模板允许完整字段值使用 `${ENV:NAME}` / `${env:NAME}` /
`${secret:path}`。解析只发生在 apply 计划阶段；包与 sidecar 保留引用，只有设备
本地 native config 含解析值。native config create-only，已存在同名冲突时整单
fail-closed。

## `doctor`

```bash
aiah doctor [--home <path>] [--project <path>] --output json
```

只读检查未结 journal、残留 stage、backup metadata/payload、deployment drift 与
MCP 原生配置前置状态。新格式部署按记录的 SHA256 和 mode 报告
`unchanged` / `locally-modified` / `missing`；旧记录缺少 hash/mode 时明确返回
`drift_unavailable`，不会猜测健康。

`scripts/dev-doctor.sh` 检查开发工具链，与面向用户资产状态的 `aiah doctor` 不同。

## `publish`、`pull` 与 `versions`

```bash
aiah publish --package <tar> --channel <dir> --output json
aiah versions --channel <dir> [--name <name>] --output json
aiah pull --channel <dir> --name <name> [--version <v>] \
  [--profile <p>] --out <dir> --output json
```

Channel 是普通目录，可以位于 U 盘、挂载的 NAS/网盘或 Git checkout。aiah 不负责
网络传输，只负责布局、不可变性和两端完整性校验。

同一 `(name, version, profile)` 内容相同则 publish 幂等，内容不同则拒绝，没有
`--force`。省略 `--version` 取最近发布的条目，不比较或猜测版本号大小。
`pull` 不覆盖输出目录中的同名产物：完整且逐字节相同的四件套视为幂等；残缺或
任一内容不同则在写入前整单拒绝。

完整流程见[跨设备迁移 runbook](runbooks/cross-device-transfer.md)。

## `bootstrap`

```bash
aiah bootstrap --channel <dir> --name <name> [--version <v>] \
  [--profile <p>] --out <dir> --home "$HOME" \
  [--project <project>] [--targets claude,codex,grok]
```

`bootstrap` 在 pull 前要求真实 TTY，取回后复用 TUI 部署视图。用户必须审阅 diff
并完整输入 `apply`；没有 `--yes`、`--force` 或环境变量旁路。取消或 diff 失败不写
HOME，但已验证的包保留在显式 `--out`。

## `aiah` / `ui`

在交互终端直接运行 `aiah` 会打开任务首页；`aiah ui` 作为兼容入口保留：

```bash
aiah                                            # 推荐：打开任务首页
aiah ui --home "$HOME"                          # 兼容：同一任务首页
aiah ui --home "$HOME" --workspace ~/ai-assets # 直接打开资产库
aiah ui --package <tar|dir> --home "$HOME"     # 高级：直接审阅并应用安装包
aiah ui --language en                           # 仅本次进程使用 English
aiah ui --density detailed                      # 仅本次展开全部可选技术明细
```

- 首页按用户任务组织为“整理本机资产”“预览并应用资产库”“安装检查与撤销”
  “迁移到其他设备”“关于与更新”和“偏好设置”，不要求先理解内部阶段。
- `v0.1.7` 的偏好设置支持语言、显示密度和首选资产库预填。语言可选
  `auto` / `zh-CN` / `en`，密度可选 `standard` / `detailed`；密度只改变可选
  技术明细的默认展开状态，不隐藏路径、版本、目标、风险、确认或恢复信息。
- 首选资产库只接受已存在的安全目录，并只用于首页提示和路径框预填；每次会话仍须
  用户确认后才启用资产库，不会自动创建、打开或选择。
- 启动时只读加载
  `${XDG_CONFIG_HOME:-$HOME/.config}/aiah/preferences.json`；文件不存在不创建，
  损坏或权限不安全时使用安全默认值并在首页/设置页告警。
- 设置页选择语言只预览；`Esc` / `m` 放弃。只有明确选择“保存偏好”才以
  `0700` 目录、`0600` 文件原子保存。`--language` / `--density` 只覆盖本次
  进程，永不反写。
- “迁移到其他设备”先只读比较资产库、当前安装和通道；可按 `p`
  选择资产组合并 typed `publish`，按 `v` 查看全部发布坐标并明确选择版本/profile
  与已有输出目录；按 `e` 选择资产组合并零写入检查本机排除项、secret 和 adapter
  兼容性。取回后先按 name/version/profile/SHA256 检查确切发布包和
  目标设备，通过后由用户按 Enter 进入同一 diff/typed `apply`；不会直接写目标
  工具目录。
- 用户界面把 workspace 称为“资产库”：它是跨工具资产的可编辑事实源。CLI flag、
  manifest schema 和 API 仍保留 `workspace`，避免破坏兼容性。
- 没有 `--workspace` 时初始只读；进入需要资产库的任务后，必须明确输入并确认路径
  才创建或打开资产库。
- HOME/project 下的 `.agents`、`.claude`、`.codex`、`.grok` 不能作为资产库。
- “加入资产库”只写显式资产库，create-only，不写工具目录。
- 资产库内选择安装方案后，复用 `build` Core 准备安装包，成功后自动进入变更预览。
- 没有 `--package` 且当前会话尚未成功准备安装包时，不开放应用入口。
- `--targets` 同时适用于显式包和当前会话构建出的包。
- 应用前必须完整输入 `apply`；成功后显示 `backupId` 和 rollback 命令。
- 普通 TUI 可按 `h` 运行只读安装检查；检查通过且存在当前部署时，按 `x`
  并完整输入 `rollback` 可回滚当前部署。历史 backup 仍需 CLI 显式指定。
- `bootstrap` 只复用 diff/apply 部署视图，不开放 Doctor/rollback 维护入口。
- 普通 TUI 可按 `v` 查看 aiah 构建身份和当前资产部署版本；版本页按 `c`
  才请求 GitHub Release 元数据，打开 TUI 不会自动联网。
- TUI 内按 `m` 随时返回任务首页。
- stdin/stdout 不是 TTY，或 `TERM` 为空/`dumb` 时直接失败。

普通用户启动时只需记住 `aiah`。`ui` 子命令没有删除，因为脚本和旧文档需要兼容，
且 `--package` / `--workspace` / `--home` 等高级直达参数仍挂在该子命令下。

交互键与设计边界见 [TUI 技术方案](designs/tui-technical-design.md)。

## `mcp`

`aiah mcp` 在 stdio 上提供只读 MCP server，暴露：

- `aiah_asset_status`
- `aiah_scan`
- `aiah_validate`
- `aiah_diff`
- `aiah_doctor`
- `aiah_migration_status`
- `aiah_version`

`aiah_asset_status` 必须传 `workspace`，可选 `manifest`、`home`、`project`；
它比较源端与资产库，返回 `unmanaged`、`managed`、`source-changed`、
`library-only`、`blocked`。

`aiah_migration_status` 必须传 `workspace`，可选 `manifest`、`channel`、`home`、
`project`；它返回资产库、当前安装、普通目录通道和版本对齐状态。

不暴露 `build`、资产库纳入/更新/移出、`publish/pull`、`apply` 或 `rollback`，
因此“经 MCP server 零写入”是可测试的绝对不变式。该子命令不接受 flag 或 operand。

Claude Code 接入：

```bash
claude mcp add aiah -- aiah mcp
```

Codex、Grok 与其它客户端配置 `command: "aiah"`、`args: ["mcp"]`。可复制配置、
隔离 fake HOME 和三客户端验收步骤见
[MCP 客户端接入 runbook](runbooks/mcp-client-acceptance.md)。

## `update`

```bash
aiah update --check [--output text|json]
```

只读查询 GitHub latest release，报告当前版本、最新稳定版本、两者关系、Release URL
和绑定精确 tag 的升级命令。状态为 `current` / `update-available` / `ahead` /
`development`；开发构建不伪装成可比较的 Release。

该命令不下载、不替换二进制。`aiah --update` 不存在；`aiah update` 必须显式带
`--check`。真正升级仍由用户执行报告中的校验安装命令。

历史问题：`v0.1.4` / `v0.1.5` 输出的命令虽然绑定 tag，却没有给安装器显式传入
`AIAH_VERSION`。从这些版本升级到当前版请使用：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v0.1.7/scripts/install.sh |
  AIAH_VERSION=0.1.7 sh
```

`v0.1.6` 二进制已把生成格式修复为：

```text
.../v<version>/scripts/install.sh | AIAH_VERSION=<version> sh
```

并增加精确字符串与 TUI 窄屏可复制性回归。`v0.1.6 → v0.1.7` 已首次完成旧版
实际命令逐字断言、真实升级、幂等复装及版本/commit/SHA256 对账。该修复不能追溯
改变 `v0.1.4` / `v0.1.5`。

## `version`

```bash
aiah version [--output text|json]
```

Release 二进制报告版本、commit 与构建时间；直接 `go build` 的版本诚实地显示为
`dev`。
