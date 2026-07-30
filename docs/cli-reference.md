# CLI 命令参考

本文是用户侧命令入口摘要。JSON 契约见 `spec/`，安全写入流程见
[上手指南](getting-started.md)，设计理由见[架构决策](decisions/)。

## 命令总表

```bash
aiah
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
aiah ui [--home <path>] [--project <path>] [--workspace <path>] [--package <tar|dir>] [--targets claude,codex,grok]
aiah mcp
aiah update --check [--output text|json]
aiah version [--output text|json]
```

所有 JSON 报告与部署记录都带 `producedBy`，用于追踪生成它的二进制版本。

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
```

- 首页按用户任务组织为“整理本机资产”“预览并应用资产库”“安装检查与撤销”
  “迁移到其他设备”和“关于与更新”，不要求先理解内部阶段。
- “迁移到其他设备”当前是 E3.1 只读状态页：按 `c` 选择已有普通目录通道，
  比较资产库、当前安装和最近发布版本；不会创建通道、发布、取回或应用文件。
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

交互键与设计边界见 [TUI 技术方案](designs/tui-technical-design.md)。

## `mcp`

`aiah mcp` 在 stdio 上提供只读 MCP server，暴露：

- `aiah_scan`
- `aiah_validate`
- `aiah_diff`
- `aiah_doctor`
- `aiah_version`

不暴露 `build`、`apply` 或 `rollback`，因此“经 MCP server 零写入”是可测试的绝对
不变式。该子命令不接受 flag 或 operand。

Claude Code 接入：

```bash
claude mcp add aiah -- aiah mcp
```

Codex 与其它客户端配置 `command: "aiah"`、`args: ["mcp"]`。

## `update`

```bash
aiah update --check [--output text|json]
```

只读查询 GitHub latest release，报告当前版本、最新稳定版本、两者关系、Release URL
和绑定精确 tag 的升级命令。状态为 `current` / `update-available` / `ahead` /
`development`；开发构建不伪装成可比较的 Release。

该命令不下载、不替换二进制。`aiah --update` 不存在；`aiah update` 必须显式带
`--check`。真正升级仍由用户执行报告中的校验安装命令。

已知问题：`v0.1.4` / `v0.1.5` 输出的命令虽然绑定 tag，却没有给安装器显式传入
`AIAH_VERSION`；而 `v0.1.5` tag 内的安装器默认版本仍是 `v0.1.4`，直接执行推荐
命令会停留在旧版。升级到 `v0.1.5` 请使用：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v0.1.5/scripts/install.sh |
  AIAH_VERSION=0.1.5 sh
```

该命令已完成隔离升级验收。后续版本必须先修复命令生成和发布门禁，不能把“绑定
tag”误当成“安装目标版本已绑定”。

## `version`

```bash
aiah version [--output text|json]
```

Release 二进制报告版本、commit 与构建时间；直接 `go build` 的版本诚实地显示为
`dev`。
