<p align="center">
  <img src="assets/readme/hero.svg" width="100%" alt="AI Asset Hub：在 Claude、Codex、Grok 与版本化资产库之间安全管理 AI 编程资产">
</p>

<p align="center">
  <a href="https://github.com/dff652/ai-asset-hub/releases"><img alt="Release v0.1.4" src="https://img.shields.io/badge/release-v0.1.4-238636"></a>
  <img alt="Status Technical Preview" src="https://img.shields.io/badge/status-technical_preview-D29922">
  <img alt="Platform Linux amd64" src="https://img.shields.io/badge/platform-Linux_amd64-58A6FF">
  <a href="LICENSE"><img alt="License Apache 2.0" src="https://img.shields.io/badge/license-Apache--2.0-8B949E"></a>
</p>

AI Asset Hub（`aiah`）是面向个人与小团队的 **AI 编程资产管理器**。它把分散在
Claude、Codex、Grok 中的 Skills、Rules、Memory、Agents、Hooks 与 MCP 模板整理成
一份可版本管理的资产库，并提供写入前预览、安装检查、撤销和跨设备迁移。

> **当前边界：Technical Preview。** 最新公开版是 `v0.1.4`，安装与端到端验收范围
> 为 **Linux amd64**。仓库当前 `dev` 候选还包含尚未发布的任务首页、统一资产状态、
> 更新/移出向导和只读迁移状态页；下方证明板会明确标注这一区别。

## 立即开始

Linux amd64 一行安装：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh | sh
aiah ui
```

`v0.1.4` 使用 `aiah ui` 进入交互界面；当前 dev 候选已支持直接运行 `aiah`，正式
Release 前不把它写成已发布行为。

安装器默认固定 `v0.1.4`，校验 Release SHA256 后原子替换到 `~/.local/bin`，不用
sudo，也不修改 shell profile。同版本复装零下载、零写入。也可以固定版本和目录：

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
[上手指南](docs/getting-started.md#安装)。

Linux amd64 已完成端到端行为验证。`v0.1.1` 中现存的 macOS、Windows 和 arm64
文件是历史交叉编译产物，未做对应平台原生验收，不属于当前支持范围；后续 Release
只发布 Linux amd64，其他平台通过原生验收后再恢复分发。

## 从“散落配置”到“可恢复资产”

<p align="center">
  <img src="assets/readme/asset-lifecycle.svg" width="100%" alt="AI 资产从 Claude、Codex、Grok 进入版本化资产库，再经过预览应用或不可变分发迁移到其他设备">
</p>

只需要分清三个角色：

| 位置 | 角色 |
|---|---|
| `~/ai-assets/` | 可进入 Git 的资产库，也是唯一事实源 |
| `*.tar` | 从资产库生成的确定性、不可变安装包 |
| `~/.claude` / `~/.codex` / `~/.grok` | 目标工具目录，是可重建的安装结果 |

日常流程是：

```text
发现本机资产 → 加入资产库 → 检查并准备安装包 → 预览变化 → 输入 apply → 安装检查
```

跨设备时，私有 Git/NAS 负责**资产库备份**，Git、NAS、U 盘等负责搬运不可变包；
`publish/pull` 不是双向同步，`.aiah/backups` 也只是本机**安装恢复点**。

## TUI：先看状态，再做操作

<p align="center">
  <img src="assets/readme/tui-proof-board.svg" width="100%" alt="AI Asset Hub dev 候选 TUI 的任务首页、统一资产状态、应用结果和跨设备只读状态">
</p>

> 上图来自当前 **dev candidate**，不是 `v0.1.4` 正式安装包验收截图。公开版已具备
> TUI 本地闭环；任务首页、统一状态、更新/移出和迁移状态页将在正式 Release
> dogfood 后进入发布版。

当前 dev 候选把常用操作放在一个任务首页：

- **整理本机资产**：查看未纳管、已纳管、待更新、仅库内和不可纳管状态；
- **预览并应用资产库**：自动检查资产、准备安装包并展示变化，最终仍需输入
  `apply`；
- **安装检查与撤销**：显示包版本、目标工具、漂移和恢复点；
- **迁移到其他设备**：只读比较资产库、当前安装和用户选择的分发通道；
- **关于与更新**：默认离线，只有明确触发才检查 GitHub Release。

完整图解和首次操作见[上手指南](docs/getting-started.md)，TUI 产品用语与状态边界见
[产品体验方案](docs/designs/tui-product-experience-v2.md)。

## 人和 AI 都能使用

| 入口 | 用途 |
|---|---|
| TUI：`aiah ui` | 公开版的人工交互入口；当前 dev 候选也支持裸 `aiah` |
| `aiah scan` | 自动化或 JSON 方式盘点本机资产 |
| `aiah validate` / `aiah build` | 校验资产库并构建确定性资产包 |
| `aiah diff` / `apply` / `rollback` | 预览、应用和撤销 |
| `aiah publish` / `pull` / `versions` | 通过普通目录分发不可变资产包 |
| `aiah bootstrap` | pull 后进入强制交互 diff 与 typed `apply` |
| `aiah doctor` | 只读检查 journal、backup、deployment drift 与 MCP 前置状态 |
| `aiah update --check` | 用户触发的只读 Release 版本检查 |
| MCP：`aiah mcp` | 供 AI 工具调用的只读 `scan/validate/diff/doctor/version` |

MCP 当前不开放写操作，也尚未覆盖 dev 候选新增的统一资产状态和迁移状态；这两项已
进入[后续计划](docs/roadmap.md)。完整参数见[命令参考](docs/cli-reference.md)，
跨设备操作见[迁移 runbook](docs/runbooks/cross-device-transfer.md)。

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
