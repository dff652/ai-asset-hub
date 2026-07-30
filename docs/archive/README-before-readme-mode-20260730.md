# README 重构前快照（2026-07-30）

> 这是 README mode 重构前的完整历史快照，仅用于对比，不是当前使用说明或项目状态
> 的事实源。当前说明请回到仓库根目录的 [`README.md`](../../README.md)。
>
> 原文件 SHA256：
> `745f782e5bbb89992b2173f478c7b10ec86959db0e4fd199b2fa94a96496a9a4`

---

# AI Asset Hub

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](../../LICENSE)

面向个人与小团队、多 AI 工具与多设备的 AI 编程资产管理器：把散落在
Claude、Codex、Grok 中的
skills、rules、agents 等整理成可版本管理的资产库，并安全地预览、应用、检查、
撤销，以及迁移到新工具或新设备。

> **状态：Technical Preview。** 核心 CLI、只读 MCP、跨设备分发与 TUI 引导式
> 本地闭环已可用；当前安装和 Release 支持范围为 **Linux amd64**。

AI Asset Hub 解决 Skills、Rules、Memory、Agents、Hooks 与 MCP 模板散落在
Claude Code、Codex、Grok 等工具目录，难以审计、迁移和回滚的问题。

## 它怎么工作

```text
资产库（Git 中的唯一事实源）
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
- 写入前能 diff，写入后返回安装恢复点 `backupId` 并支持回滚；这不替代资产库备份。
- aiah 不实现用户资产的网络传输、服务端或后台 daemon；版本检查仅在用户显式触发时
  读取 GitHub Release 元数据。

架构与格式详见[总体架构](../architecture.md)和[资产模型](../asset-model.md)。

## 安装

Linux amd64 一行安装：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh | sh
```

安装器默认固定 `v0.1.4`，下载 Release 的 `SHA256SUMS` 和 Linux amd64 二进制，校验后
在目标目录原子替换；默认安装到 `~/.local/bin`，不用 sudo，也不修改 profile。
已安装同版本时零下载、零写入。可以显式选择版本和目录：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh |
  AIAH_VERSION=0.1.4 AIAH_INSTALL_DIR="$HOME/.local/bin" sh
```

升级时重复执行第一条安装命令即可：脚本中的默认版本只会在对应 Release 校验通过后
更新；本机版本较旧时会校验 SHA256 后原子替换，同版本则直接退出。要固定或切换到
某个版本，显式设置 `AIAH_VERSION`。aiah 不在后台自动升级。

先只读检查版本：

```bash
aiah update --check
```

它不会下载或替换二进制；发现新版本时会输出绑定到精确 Release tag 的安装命令。
`aiah --update` 和不带 `--check` 的 `aiah update` 都不会执行升级。

直接执行远程脚本前，推荐先下载、阅读再运行。Release 裸二进制的手动安装方法见
[上手指南](../getting-started.md#安装)。

Linux amd64 已完成端到端行为验证。`v0.1.1` 中现存的 macOS、Windows 和 arm64
文件是历史交叉编译产物，未做对应平台原生验收，不属于当前支持范围；后续 Release
只发布 Linux amd64，其他平台通过原生验收后再恢复分发。

## 五分钟上手

先记住三个角色：

| 目录 | 角色 |
|---|---|
| `~/ai-assets/` | 可进入 Git 的资产库，也是唯一事实源 |
| `*.tar` | `aiah build` 生成的不可变资产包 |
| `~/.claude` / `~/.codex` / `~/.grok` | adapter 生成的目标文件，不是源头 |

日常使用只需输入：

```bash
aiah
```

交互终端会进入任务首页；`aiah ui` 继续作为兼容入口。首页按用户目标提供：

- **管理本机资产**：统一查看未纳管、已纳管、源端有更新和仅在资产库的项目，
  并显式纳入、更新或移出；
- **预览并应用资产库**：检查资产、准备安装包、预览变化，再确认应用；
- **安装检查与撤销**：检查当前安装和漂移，必要时撤销上次安装；
- **迁移到其他设备**：只读比较资产库、当前安装和已有分发通道的版本状态；
- **关于与更新**：查看版本；只有再次按 `c` 才联网检查 Release。

资产库不按 AI 工具拆目录。`assets/` 保存资产正文，`manifest.yaml` 用每项资产的
`targets` 区分目标工具，`dist/` 保存生成的不可变包。完整图解见
[上手指南的资产库与操作流程](../getting-started.md#21-tui-里的资产库与操作流程)。
纳入、更新或移出成功后会连续进入资产组合和变更预览；写工具目录仍需输入
`apply`。私有 Git/NAS 负责资产库备份，`.aiah/backups` 只是本机安装恢复点，
`publish/pull` 则是不可变版本分发，不是双向同步。

自动化、跨设备和假 HOME 演练仍使用 CLI。完整教程、manifest 属性与安全边界见
[上手指南](../getting-started.md)。

## 主要入口

| 入口 | 用途 |
|---|---|
| `aiah` / `aiah ui` | 进入任务首页；`ui` 是兼容入口 |
| `aiah scan` | 自动化或 JSON 方式盘点本机资产 |
| `aiah validate` / `aiah build` | 校验资产库并构建确定性资产包 |
| `aiah diff` / `apply` / `rollback` | 预览、应用和撤销 |
| `aiah publish` / `pull` / `versions` | 通过普通目录分发不可变资产包 |
| `aiah bootstrap` | pull 后进入强制交互 diff 与 typed `apply` |
| `aiah doctor` | 只读检查 journal、backup、deployment drift 与 MCP 前置状态 |
| `aiah update --check` | 用户触发的只读 Release 版本检查 |
| `aiah mcp` | 面向 AI 工具的只读 MCP server |

完整参数与行为见[命令参考](../cli-reference.md)，跨设备操作见
[迁移 runbook](../runbooks/cross-device-transfer.md)。

## 安全边界

- TUI 首页和本机资产页初始只读；只有明确选择并确认资产库后，才会创建或写资产库。
- 应用入口只来自显式 `--package`，或当前会话成功构建出的包。
- TUI 应用必须完整输入 `apply`；`bootstrap` 没有 `--yes` 或非交互旁路。
- TUI 撤销只针对安装检查识别到的当前安装，且必须完整输入 `rollback`；选择
  历史 backup 仍使用 CLI。
- MCP server 只暴露 `scan`、`validate`、`diff`、`doctor`、`version`，不暴露
  `build`、`apply` 或 `rollback`。
- MCP 原生配置采用 create-only 所有权；冲突时整单 fail-closed。
- 项目 `CLAUDE.md` / `AGENTS.md` 由项目 Git 管理；aiah 只盘点和报告，不自动对齐。

安全模型见[安全与隐私](../security.md)，漏洞报告见
[`SECURITY.md`](../../SECURITY.md)。

## 文档

- [上手指南](../getting-started.md)
- [命令参考](../cli-reference.md)
- [文档总索引](../README.md)
- [总体架构](../architecture.md)
- [资产模型](../asset-model.md)
- [工程流程](../development.md)
- [MVP 路线图](../roadmap.md)

## 开发

项目采用 CLI-first、Go Core；TUI 复用同一个 Core，不复制业务规则。新设备先运行
`./scripts/dev-doctor.sh`，提交前运行 `./scripts/check-local.sh`。完整约束见
[工程流程](../development.md)和根 [`CLAUDE.md`](../../CLAUDE.md)。

## 许可证

本项目以 [Apache-2.0](../../LICENSE) 发布，版权署名见 [NOTICE](../../NOTICE)，
第三方许可见[第三方依赖许可证清单](../licenses/third-party.md)。
