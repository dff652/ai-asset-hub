<p align="center">
  <img src="assets/readme/hero.svg" width="100%" alt="AI Asset Hub：在 Claude、Codex、Grok 与版本化资产库之间安全管理 AI 编程资产">
</p>

<p align="center">
  <a href="https://github.com/dff652/ai-asset-hub/releases"><img alt="Release v0.1.6" src="https://img.shields.io/badge/release-v0.1.6-238636"></a>
  <img alt="Status Technical Preview" src="https://img.shields.io/badge/status-technical_preview-D29922">
  <img alt="Platform Linux amd64" src="https://img.shields.io/badge/platform-Linux_amd64-58A6FF">
  <a href="LICENSE"><img alt="License Apache 2.0" src="https://img.shields.io/badge/license-Apache--2.0-8B949E"></a>
</p>

AI Asset Hub（`aiah`）是面向个人与小团队的 **AI 编程资产管理器**。它把分散在
Claude、Codex、Grok 中的 Skills、Rules、Memory、Agents、Hooks 与 MCP 模板整理成
一份可版本管理的资产库，并提供写入前预览、安装检查、撤销和跨设备迁移。

> **当前边界：Technical Preview。** 最新公开版是 `v0.1.6`，安装与端到端验收范围
> 为 **Linux amd64**。任务首页、统一资产状态、更新/移出向导、连续应用和只读迁移
> 状态已经完成正式 Release 安装包验收。

## 立即开始

Linux amd64 一行安装：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh | sh
aiah
```

日常启动只需 `aiah`。`aiah ui` 仅作为兼容入口和高级参数入口保留，不需要新用户
记忆。安装器校验 Release SHA256 后原子替换到
`~/.local/bin`，不用 sudo，也不修改 shell profile；同版本复装零下载、零写入。
当前源码中的安装器默认固定为已验收的 `v0.1.6`。也可以固定版本和安装目录：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v0.1.6/scripts/install.sh |
  AIAH_VERSION=0.1.6 AIAH_INSTALL_DIR="$HOME/.local/bin" sh
```

aiah 不在后台升级。固定 tag 的安装器保留 tag 创建时的默认 pin，因此使用固定 tag
时仍应像上面一样显式设置 `AIAH_VERSION`。

先只读检查版本：

```bash
aiah update --check
```

它不会下载或替换二进制。已知问题：`v0.1.4` / `v0.1.5` 输出的推荐命令缺少
`AIAH_VERSION`；从这些版本升级到 `v0.1.6` 时请使用上方显式版本命令。bridge
边界和真实升级证据已在
[`v0.1.6` Release](https://github.com/dff652/ai-asset-hub/releases/tag/v0.1.6)
公开。`v0.1.6` 已包含修复，今后生成的命令会同时绑定 tag 和目标版本；旧二进制
不会被追溯修改。`aiah --update` 和不带 `--check` 的 `aiah update` 都不会执行升级。

直接执行远程脚本前，推荐先下载、阅读再运行。Release 裸二进制的手动安装方法见
[上手指南](docs/getting-started.md#安装)。

Linux amd64 已完成端到端行为验证。`v0.1.1` 中现存的 macOS、Windows 和 arm64
文件是历史交叉编译产物，未做对应平台原生验收，不属于当前支持范围；后续 Release
只发布 Linux amd64，其他平台通过原生验收后再恢复分发。

## 第一次使用：五步安全应用

这张图只表达“第一次把资产安全应用到本机”的主流程，不是全部功能地图。

<p align="center">
  <img src="assets/readme/usage-flow.svg" width="100%" alt="AI Asset Hub 五步安全流程：发现资产、整理资产库、检查并准备、预览变化、人工确认；只有最后一步写入目标工具目录">
</p>

只需要分清三个角色：

| 位置 | 角色 |
|---|---|
| `~/ai-assets/` | 可进入 Git 的资产库，也是唯一事实源 |
| `*.tar` | 从资产库生成的确定性、不可变安装包 |
| `~/.claude` / `~/.codex` / `~/.grok` | 目标工具目录，是可重建的安装结果 |

日常流程是：

```text
发现资产 → 整理资产库 → 检查并准备 → 预览变化 → 人工确认
```

前四步都不写 `.claude`、`.codex` 或 `.grok`；只有审阅变化后完整输入 `apply`
才会应用，并在写入前创建本机安装恢复点。随后运行安装检查，必要时可以撤销。
五步各自的输入、输出和写入边界见[上手指南](docs/getting-started.md#2-五步完成一次安全应用)。

其它常见流程：

| 你要做什么 | 简化流程 | 入口 |
|---|---|---|
| 日常维护 | 查看统一状态 → 更新/移出 → 预览 → 应用 | TUI |
| 检查与撤销 | 安装检查 → 判断漂移 → typed `rollback` | TUI / CLI |
| 跨设备迁移 | build/publish → 外部搬运 → versions/pull → diff/apply | TUI（开发候选）/ CLI |
| AI 与自动化 | MCP 只读查询，或 CLI JSON 编排 | `aiah mcp` / CLI |
| 升级 aiah | 只读检查 → 指定 Release → 校验并原子替换 | installer |

完整入口、写入边界和当前覆盖见[使用流程总览](docs/usage-flows.md)。

跨设备时，私有 Git/NAS 负责**资产库备份**，Git、NAS、U 盘等负责搬运不可变包；
`publish/pull` 不是双向同步，`.aiah/backups` 也只是本机**安装恢复点**。

## TUI：先看状态，再做操作

<p align="center">
  <img src="assets/readme/tui-proof-board.svg" width="100%" alt="AI Asset Hub v0.1.6 TUI 的任务首页、统一资产状态、应用结果和跨设备只读状态">
</p>

> 上图所示任务首页、统一状态、更新/移出、连续应用和迁移状态页已用 `v0.1.6`
> 正式 Release 安装包完成隔离 TTY dogfood。

在 `v0.1.6` 中，常用操作集中在一个任务首页：

- **整理本机资产**：查看未纳管、已纳管、待更新、仅库内和不可纳管状态；
- **预览并应用资产库**：自动检查资产、准备安装包并展示变化，最终仍需输入
  `apply`；
- **安装检查与撤销**：显示包版本、目标工具、漂移和恢复点；
- **迁移到其他设备**：只读比较资产库、当前安装和用户选择的分发通道；
- **关于与更新**：默认离线，只有明确触发才检查 GitHub Release。

`v0.1.6` 的迁移页仍是上图所示只读状态。当前开发候选已经加入 E3.2：按 `p`
选择资产组合并 typed `publish`，按 `v` 明确选择版本/profile 和已有输出目录；
取回后仍进入同一 diff，并且必须 typed `apply` 才写目标工具目录。aiah 不创建
通道目录、不自动取回“最新版”，也不接管 Git/NAS/rsync/U 盘传输；首次发布可在
用户明确选择的空目录中初始化通道索引和不可变包布局。

E3.3 开发候选继续增加 `e 换机检查`：选择资产组合后只读列出本机不迁移项、
缺失 secret、不支持或会丢弃资产的目标，以及 adapter 能力降级。检查不生成安装包、
不创建 `dist/`、不发布、不取回，也不应用资产；`v0.1.6` 公开版尚不包含该入口。

完整图解和首次操作见[上手指南](docs/getting-started.md)，TUI 产品用语与状态边界见
[产品体验方案](docs/designs/tui-product-experience-v2.md)。

## 人和 AI 都能使用

| 入口 | 用途 |
|---|---|
| TUI：`aiah` | 人工交互主入口；`aiah ui` 继续兼容 |
| `aiah scan` | 自动化或 JSON 方式盘点本机资产 |
| `aiah validate` / `aiah build` | 校验资产库并构建确定性资产包 |
| `aiah diff` / `apply` / `rollback` | 预览、应用和撤销 |
| `aiah publish` / `pull` / `versions` | 通过普通目录分发不可变资产包 |
| `aiah bootstrap` | pull 后进入强制交互 diff 与 typed `apply` |
| `aiah doctor` | 只读检查 journal、backup、deployment drift 与 MCP 前置状态 |
| `aiah update --check` | 用户触发的只读 Release 版本检查 |
| MCP：`aiah mcp` | 供 AI 工具调用的只读 `scan/validate/diff/doctor/version` |

MCP 当前不开放写操作，也尚未覆盖 TUI 已有的统一资产状态和迁移状态；这两项已进入
[后续计划](docs/roadmap.md)。完整参数见[命令参考](docs/cli-reference.md)，
跨设备操作见[迁移 runbook](docs/runbooks/cross-device-transfer.md)。

## 安全边界

- TUI 首页和本机资产页初始只读；只有明确选择并确认资产库后，才会创建或写资产库。
- 应用入口只来自显式 `--package`，或当前会话成功构建出的包。
- TUI 应用必须完整输入 `apply`；`bootstrap` 没有 `--yes` 或非交互旁路。
- TUI 撤销只针对安装检查识别到的当前安装，且必须完整输入 `rollback`；选择
  历史 backup 仍使用 CLI。
- TUI 发布必须完整输入 `publish`；取回必须明确选择版本/profile 和已有输出目录，
  不覆盖不同或残缺的同名产物，取回成功也不会跳过 diff/typed `apply`。
- MCP server 只暴露 `scan`、`validate`、`diff`、`doctor`、`version`，不暴露
  `build`、`apply` 或 `rollback`。
- MCP 原生配置采用 create-only 所有权；冲突时整单 fail-closed。
- 项目 `CLAUDE.md` / `AGENTS.md` 由项目 Git 管理；aiah 只盘点和报告，不自动对齐。

安全模型见[安全与隐私](docs/security.md)，漏洞报告见
[`SECURITY.md`](SECURITY.md)。

## 文档

- [上手指南](docs/getting-started.md)
- [使用流程总览](docs/usage-flows.md)
- [README 与 SVG 视觉验收 SOP](docs/runbooks/readme-visual-acceptance.md)
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
