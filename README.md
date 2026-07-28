# AI Asset Hub

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

面向个人与小团队的 AI 编程资产管理、打包和跨设备部署工具。

> **状态：Technical Preview。** 核心 CLI、只读 MCP、跨设备分发与 TUI Phase C
> 已可用；安装脚本和 Windows 写入行为验收仍在路线图中。

AI Asset Hub 解决 Skills、Rules、Memory、Agents、Hooks 与 MCP 模板散落在
Claude Code、Codex、Grok 等工具目录，难以审计、迁移和回滚的问题。

## 它怎么工作

```text
资产工作区（Git 中的唯一事实源）
        │
        ├── validate：只读校验
        ├── build：构建不可变资产包
        ▼
分发通道（普通目录，由 Git / NAS / U 盘等搬运）
        │
        ├── pull / bootstrap
        ├── diff：写入前审阅
        ▼
Claude Code / Codex / Grok 目标目录
        └── apply → doctor → rollback
```

核心保证：

- 纯文本资产是事实源，索引和目标目录都是可重建的派生物。
- 包不可变；同版本不同内容会被拒绝，不提供 `--force`。
- 密钥只在目标设备 apply 时解析，不进入包、报告、journal 或 backup metadata。
- 写入前能 diff，写入后返回 `backupId` 并支持回滚。
- aiah 不实现网络传输、服务端或后台 daemon。

架构与格式详见[总体架构](docs/architecture.md)和[资产模型](docs/asset-model.md)。

## 安装

首个公开版本是
[`v0.1.1`](https://github.com/dff652/ai-asset-hub/releases/tag/v0.1.1)。
从 Release 下载与平台匹配的裸二进制和 `SHA256SUMS`，**校验后**再安装：

| 系统 | amd64 | arm64 |
|---|---|---|
| Linux | `aiah_0.1.1_linux_amd64` | `aiah_0.1.1_linux_arm64` |
| macOS | `aiah_0.1.1_darwin_amd64` | `aiah_0.1.1_darwin_arm64` |
| Windows | `aiah_0.1.1_windows_amd64.exe` | `aiah_0.1.1_windows_arm64.exe` |

Linux amd64 示例：

```bash
(
set -eu
AIAH_VERSION=0.1.1
AIAH_ASSET="aiah_${AIAH_VERSION}_linux_amd64"
AIAH_BASE="https://github.com/dff652/ai-asset-hub/releases/download/v${AIAH_VERSION}"
AIAH_TMP="$(mktemp -d)"
trap 'rm -rf "$AIAH_TMP"' EXIT

curl -fL "$AIAH_BASE/$AIAH_ASSET" -o "$AIAH_TMP/$AIAH_ASSET"
curl -fL "$AIAH_BASE/SHA256SUMS" -o "$AIAH_TMP/SHA256SUMS"
AIAH_EXPECTED="$(awk -v name="$AIAH_ASSET" '$2 == name { print $1 }' \
  "$AIAH_TMP/SHA256SUMS")"
AIAH_ACTUAL="$(sha256sum "$AIAH_TMP/$AIAH_ASSET" | awk '{ print $1 }')"
test -n "$AIAH_EXPECTED" && test "$AIAH_ACTUAL" = "$AIAH_EXPECTED"

mkdir -p "$HOME/.local/bin"
install -m 0755 "$AIAH_TMP/$AIAH_ASSET" "$HOME/.local/bin/aiah"
"$HOME/.local/bin/aiah" version
)
```

Linux amd64 已完成端到端行为验证。其他目标经过交叉编译和产物校验，但在对应平台
完成原生验收前，不把“有二进制”表述为“完整支持”。macOS 可用
`shasum -a 256` 校验；Windows 写入语义仍待单独验收。

## 五分钟上手

先记住三个目录：

| 目录 | 角色 |
|---|---|
| `~/ai-assets/` | 可进入 Git 的工作区，也是唯一事实源 |
| `*.tar` | `aiah build` 生成的不可变资产包 |
| `~/.claude` / `~/.codex` / `~/.grok` | adapter 生成的目标文件，不是源头 |

最短安全流程：

```bash
# 1. 只读盘点；也可运行 aiah ui 浏览
aiah scan --home "$HOME" --output json

# 2. 在工作区准备 manifest.yaml 与 assets/
#    也可用 aiah ui --workspace ~/ai-assets 交互组装

# 3. 校验并构建
aiah validate --manifest ~/ai-assets/manifest.yaml --output json
aiah build --manifest ~/ai-assets/manifest.yaml --profile personal \
  --out /tmp/aiah-dist --output json

# 4. 先看 diff，再部署
aiah diff --package /tmp/aiah-dist/<package>.tar --home "$HOME" --output json
aiah apply --package /tmp/aiah-dist/<package>.tar --home "$HOME" --output json

# 5. 自查；需要时使用 apply 返回的 backupId 回滚
aiah doctor --home "$HOME" --output json
aiah rollback --home "$HOME" --backup <id> --output json
```

首次写真实 HOME 前，必须先完成假 HOME 闭环和 dry-run。完整步骤、manifest
属性与常见边界见[上手指南](docs/getting-started.md)。

## 主要入口

| 入口 | 用途 |
|---|---|
| `aiah scan` / `aiah ui` | 只读盘点，或在 TUI 浏览资产 |
| `aiah validate` / `aiah build` | 校验工作区并构建确定性资产包 |
| `aiah diff` / `apply` / `rollback` | 审阅、部署和恢复 |
| `aiah publish` / `pull` / `versions` | 通过普通目录分发不可变资产包 |
| `aiah bootstrap` | pull 后进入强制交互 diff 与 typed `apply` |
| `aiah doctor` | 只读检查 journal、backup、deployment drift 与 MCP 前置状态 |
| `aiah mcp` | 面向 AI 工具的只读 MCP server |

完整参数与行为见[命令参考](docs/cli-reference.md)，跨设备操作见
[迁移 runbook](docs/runbooks/cross-device-transfer.md)。

## 安全边界

- TUI 只有显式传入 `--workspace` 才能写工作区；只有 `--package` 才会出现部署入口。
- TUI 部署必须完整输入 `apply`；`bootstrap` 没有 `--yes` 或非交互旁路。
- MCP server 只暴露 `scan`、`validate`、`diff`、`doctor`、`version`，不暴露
  `build`、`apply` 或 `rollback`。
- MCP 原生配置采用 create-only 所有权；冲突时整单 fail-closed。
- 项目 `CLAUDE.md` / `AGENTS.md` 由项目 Git 管理；aiah 只盘点和报告，不自动对齐。

安全模型见[安全与隐私](docs/security.md)，漏洞报告见
[`SECURITY.md`](SECURITY.md)。

## 文档

- [上手指南](docs/getting-started.md)
- [命令参考](docs/cli-reference.md)
- [文档总索引](docs/README.md)
- [总体架构](docs/architecture.md)
- [资产模型](docs/asset-model.md)
- [工程流程](docs/development.md)
- [MVP 路线图](docs/roadmap.md)

## 开发

项目采用 CLI-first、Go Core；TUI 复用同一个 Core，不复制业务规则。新设备先运行
`./scripts/dev-doctor.sh`，提交前运行 `./scripts/check-local.sh`。完整约束见
[工程流程](docs/development.md)和根 [`CLAUDE.md`](CLAUDE.md)。

## 许可证

本项目以 [Apache-2.0](LICENSE) 发布，版权署名见 [NOTICE](NOTICE)，第三方许可见
[第三方依赖许可证清单](docs/licenses/third-party.md)。
