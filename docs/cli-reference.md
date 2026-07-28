# CLI 命令参考

本文是用户侧命令入口摘要。JSON 契约见 `spec/`，安全写入流程见
[上手指南](getting-started.md)，设计理由见[架构决策](decisions/)。

## 命令总表

```bash
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

## `ui`

引导式本地流程和三个显式入口共用同一个 Core：

```bash
aiah ui --home "$HOME"                         # 引导式：盘点→工作区→build→diff/apply
aiah ui --home "$HOME" --workspace ~/ai-assets # Phase B：组装工作区
aiah ui --package <tar|dir> --home "$HOME"     # Phase C：审阅并部署
```

- 没有 `--workspace` 时初始只读；按 `w` 明确输入并确认路径后才创建/打开工作区。
- HOME/project 下的 `.agents`、`.claude`、`.codex`、`.grok` 不能作为工作区。
- Phase B 只写显式工作区，create-only，不写工具目录。
- 工作区内按 `b` 选择 profile，复用 `build` Core 写入 `dist/`，成功后自动进入 diff。
- 没有 `--package` 且当前会话尚未成功构建时，不显示部署入口。
- `--targets` 同时适用于显式包和当前会话构建出的包。
- Phase C 必须完整输入 `apply`；成功后显示 `backupId` 和 rollback 命令。
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

## `version`

```bash
aiah version [--output text|json]
```

Release 二进制报告版本、commit 与构建时间；直接 `go build` 的版本诚实地显示为
`dev`。
