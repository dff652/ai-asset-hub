# AI Asset Hub

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

面向个人与小团队的 AI 编程资产管理、打包和跨设备部署工具。

> **状态：Technical Preview。** 核心 CLI、只读 MCP、跨设备分发与 TUI 引导式
> 本地闭环已可用；当前安装和 Release 支持范围为 **Linux amd64**。

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

Linux amd64 一行安装：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh | sh
```

安装器默认固定 `v0.1.3`，下载 Release 的 `SHA256SUMS` 和 Linux amd64 二进制，校验后
在目标目录原子替换；默认安装到 `~/.local/bin`，不用 sudo，也不修改 profile。
已安装同版本时零下载、零写入。可以显式选择版本和目录：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh |
  AIAH_VERSION=0.1.3 AIAH_INSTALL_DIR="$HOME/.local/bin" sh
```

直接执行远程脚本前，推荐先下载、阅读再运行。Release 裸二进制的手动安装方法见
[上手指南](docs/getting-started.md#安装)。

Linux amd64 已完成端到端行为验证。`v0.1.1` 中现存的 macOS、Windows 和 arm64
文件是历史交叉编译产物，未做对应平台原生验收，不属于当前支持范围；后续 Release
只发布 Linux amd64，其他平台通过原生验收后再恢复分发。

## 五分钟上手

先记住三个目录：

| 目录 | 角色 |
|---|---|
| `~/ai-assets/` | 可进入 Git 的工作区，也是唯一事实源 |
| `*.tar` | `aiah build` 生成的不可变资产包 |
| `~/.claude` / `~/.codex` / `~/.grok` | adapter 生成的目标文件，不是源头 |

日常使用可以只启动一个 TUI：

```bash
aiah ui --home "$HOME"
```

进入后按 `w` 明确输入工作区路径，空格勾选资产，再按 `w` 写出；按 `b` 选择
profile 后会自动校验、构建并进入只读 diff。只有按 `a` 后完整输入 `apply` 才会写
目标目录。部署后运行 `aiah doctor` 自查；真正需要恢复时复制界面给出的 rollback
命令。

自动化、跨设备和假 HOME 演练仍使用 CLI。完整教程、manifest 属性与安全边界见
[上手指南](docs/getting-started.md)。

## 主要入口

| 入口 | 用途 |
|---|---|
| `aiah scan` / `aiah ui` | 只读盘点，或在 TUI 完成本地组装、构建与部署 |
| `aiah validate` / `aiah build` | 校验工作区并构建确定性资产包 |
| `aiah diff` / `apply` / `rollback` | 审阅、部署和恢复 |
| `aiah publish` / `pull` / `versions` | 通过普通目录分发不可变资产包 |
| `aiah bootstrap` | pull 后进入强制交互 diff 与 typed `apply` |
| `aiah doctor` | 只读检查 journal、backup、deployment drift 与 MCP 前置状态 |
| `aiah mcp` | 面向 AI 工具的只读 MCP server |

完整参数与行为见[命令参考](docs/cli-reference.md)，跨设备操作见
[迁移 runbook](docs/runbooks/cross-device-transfer.md)。

## 安全边界

- TUI 初始只读；只有显式传入 `--workspace`，或按 `w` 输入并确认路径，才会创建或写工作区。
- 部署入口只来自显式 `--package`，或当前会话成功构建出的包。
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
