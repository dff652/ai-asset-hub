<p align="center">
  <img src="assets/readme/hero.svg" width="100%" alt="AI Asset Hub：在 Claude、Codex、Grok 之间安全地统一与应用 AI 编程资产">
</p>

<p align="center">
  <a href="https://github.com/dff652/ai-asset-hub/releases"><img alt="Release v0.1.10" src="https://img.shields.io/badge/release-v0.1.10-238636"></a>
  <img alt="Status Technical Preview" src="https://img.shields.io/badge/status-technical_preview-D29922">
  <img alt="Platform Linux and macOS" src="https://img.shields.io/badge/platform-Linux_%7C_macOS-58A6FF">
  <a href="LICENSE"><img alt="License Apache 2.0" src="https://img.shields.io/badge/license-Apache--2.0-8B949E"></a>
</p>

AI Asset Hub（`aiah`）帮你**安全地改 AI 工具配置**，并在 Claude、Codex、Grok
之间统一 Skills、Rules、Agents、Hooks 与 MCP 模板。

它不是又一个把文件铺到 `$HOME` 的通用 dotfile 工具：它理解各工具目录语义，
在写入前给出 diff，写入后可 doctor / rollback，并避免把 token 打进包。个人可先
盘点与安全应用；小团队可把同一份规范做成不可变包再分发。跨设备迁移是能力之一，
**不是**主打卖点。

定位与后续取舍见
[产品定位与后续方向](docs/research/product-positioning-and-direction.md)。

> **当前边界：Technical Preview。** 最新公开版是 `v0.1.10`，安装与端到端验收范围
> 为 **Linux amd64 与 macOS**（Windows 未做原生写入验收）。N10.1–N10.3 迁移准备检查
> 已发布；N10.4 自动证据记录器明确延期。当前优先真实使用与反馈，而不是继续扩
> 功能清单。

## 先看问题：零承诺只读盘点

已安装时，先只读扫描本机 AI 资产（不写盘、不执行 hook、不改任何工具目录）：

```bash
aiah scan
aiah scan --output json
```

输出会列出候选资产与 findings（例如疑似 secret、软链资产）。**先看到问题，再决定
是否整理资产库。** 完整安装与五步安全应用见下节与
[上手指南](docs/getting-started.md)。

## 立即开始

一行安装（Linux amd64、macOS Apple Silicon 与 Intel）：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh | sh
aiah scan
aiah
```

日常交互入口是 `aiah`（任务首页）。`aiah ui` 仅作兼容与高级参数入口。安装器校验
Release SHA256 后原子替换到 `~/.local/bin`，不用 sudo，也不改 shell profile；
同版本复装零下载、零写入。源码默认 pin 为已验收的 `v0.1.10`。也可固定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v0.1.10/scripts/install.sh |
  AIAH_VERSION=0.1.10 AIAH_INSTALL_DIR="$HOME/.local/bin" sh
```

aiah 不在后台升级。固定 tag 的安装器保留 tag 创建时的默认 pin，因此使用固定 tag
时仍应像上面一样显式设置 `AIAH_VERSION`。

先只读检查版本：

```bash
aiah update --check
```

它不会下载或替换二进制。`v0.1.6 → v0.1.7` 已首次完整证明程序生成的推荐命令、
精确 tag、显式 `AIAH_VERSION`、SHA256 校验和原子替换形成闭环。仅 `v0.1.4` /
`v0.1.5` 的旧二进制仍受历史命令缺陷影响；它们不会被追溯修改，直接按上方
`v0.1.10` 显式版本命令升级即可。`aiah --update` 和不带 `--check` 的
`aiah update` 都不会执行升级。

直接执行远程脚本前，推荐先下载、阅读再运行。Release 裸二进制的手动安装方法见
[上手指南](docs/getting-started.md#安装)。

支持范围按**验收强度**分档，而不是按「能不能编译」：

| 档位 | 平台 | 依据 |
|---|---|---|
| 完整支持 | `linux/amd64`、`darwin/arm64` | 各自在专属 CI runner 上跑完整套：测试、`vet`、gofmt、假 HOME 闭环与安装器回归 |
| 交叉编译 + 冒烟 | `darwin/amd64` | 纯 Go `CGO_ENABLED=0` 二进制在同一 OS 的不同架构间只差代码生成，而 macOS 语义已在 arm64 验过 |
| 不发布 | `linux/arm64`、Windows | 前者仅在 CI 编译健康检查中构建；Windows 的 chmod、shebang 与配置根语义没有任何验收覆盖 |

发布二进制等于给出支持承诺，所以不发布的平台一个字节都不上传，安装器也会在下载前拒绝。

**macOS Gatekeeper**：用上面的 `curl | sh` 安装**不受影响**（curl 不打 quarantine
属性）。若改从 Release 页面用浏览器下载，首次执行会被拦，需
`xattr -d com.apple.quarantine ./aiah_<version>_darwin_arm64` 或右键打开。
二进制未做 Apple 签名与公证。

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
| 只想先摸清本机 | `scan` 只读盘点 | CLI |
| 安全改配置 | 整理资产库 → 预览 → typed `apply` → doctor | TUI |
| 日常维护 | 查看统一状态 → 更新/移出 → 预览 → 应用 | TUI |
| 检查与撤销 | 安装检查 → 判断漂移 → typed `rollback` | TUI / CLI |
| 换机前对账 | `readiness` / TUI「换机与备份」 | CLI / TUI / MCP |
| 跨设备分发 | build/publish → 外部搬运 → pull → diff/apply | TUI / CLI |
| AI 与自动化 | MCP 只读查询，或 CLI JSON 编排 | `aiah mcp` / CLI |
| 升级 aiah | 只读检查 → 指定 Release → 校验并原子替换 | installer |

完整入口、写入边界和当前覆盖见[使用流程总览](docs/usage-flows.md)。

跨设备时，私有 Git/NAS 负责**资产库备份**，Git、NAS、U 盘等负责搬运不可变包；
`publish/pull` 不是双向同步，`.aiah/backups` 也只是本机**安装恢复点**。

## TUI：先看状态，再做操作

<p align="center">
  <img src="assets/readme/tui-proof-board.svg" width="100%" alt="AI Asset Hub v0.1.7 TUI 的任务首页、统一资产状态、安全应用结果和跨设备连续迁移">
</p>

> 上图所示任务首页、统一状态、更新/移出、连续应用和迁移流程已用 `v0.1.7`
> 正式 Release 安装包完成隔离 TTY 与双设备 dogfood。

在 `v0.1.7` 中，常用操作集中在一个任务首页：

- **整理本机资产**：查看未纳管、已纳管、待更新、仅库内和不可纳管状态；
- **预览并应用资产库**：自动检查资产、准备安装包并展示变化，最终仍需输入
  `apply`；
- **安装检查与撤销**：显示包版本、目标工具、漂移和恢复点；
- **换机与备份**：只读汇总能否打包、外部副本证据与恢复演练证据及下一步；
- **迁移到其他设备**：换机检查、发布、版本选择、取回、目标检查和安全应用；
- **关于与更新**：默认离线，只有明确触发才检查 GitHub Release；
- **偏好设置**：选择中文/English、显示密度和只用于提示与预填的首选资产库。

迁移页按 `p` 选择资产组合并 typed `publish`，按 `v` 明确选择版本/profile 和
已有输出目录，按 `e` 检查当前资产库/profile。取回后先检查所选发布包与目标设备，
通过后才允许进入同一 diff，并且必须 typed `apply` 才写目标工具目录。aiah 不创建
通道目录、不自动取回“最新版”，也不接管 Git/NAS/rsync/U 盘传输；首次发布可在
用户明确选择的空目录中初始化通道索引和不可变包布局。

`e 换机检查`选择资产组合后只读列出本机不迁移项、
缺失 secret、不支持或会丢弃资产的目标，以及 adapter 能力降级。检查不生成安装包、
不创建 `dist/`、不发布、不取回，也不应用资产。取回后的包级检查再绑定
name/version/profile/SHA256；任一不匹配都阻止进入 diff。

完整图解和首次操作见[上手指南](docs/getting-started.md)，TUI 产品用语与状态边界见
[产品体验方案](docs/designs/tui-product-experience-v2.md)。

## 人和 AI 都能使用

| 入口 | 用途 |
|---|---|
| TUI：`aiah` | 人工交互主入口；`aiah ui` 继续兼容 |
| `aiah init` | 脚手架一个资产工作区；create-only、幂等，不做隐式发现 |
| `aiah scan` | 自动化或 JSON 方式盘点本机资产 |
| `aiah validate` / `aiah build` | 校验资产库并构建确定性资产包 |
| `aiah diff` / `apply` / `rollback` | 预览、应用和撤销 |
| `aiah publish` / `pull` / `versions` | 通过普通目录分发不可变资产包 |
| `aiah bootstrap` | pull 后进入强制交互 diff 与 typed `apply` |
| `aiah doctor` | 只读检查 journal、backup、deployment drift 与 MCP 前置状态 |
| `aiah update --check` | 用户触发的只读 Release 版本检查 |
| MCP：`aiah mcp` | 供 AI 工具调用的只读盘点、统一资产状态、迁移状态/准备、diff 与安装检查 |

`v0.1.9` 起，`aiah init` 会在写入前拒绝 `.agents`、`.claude`、`.codex`、`.grok`
及其后代，也拒绝解析后落入这些目录的软链接别名；`.claude-workspace` 等安全同名前缀
不受影响。`v0.1.8` 的历史边界与修复证据见
[发布后审计](docs/reviews/2026-08-01-v0.1.8-post-release-audit.md)和
[v0.1.9 正式验收](docs/reviews/2026-08-01-v0.1.9-release-acceptance.md)。

公开版 `v0.1.10` 提供 8 个只读 MCP 工具，含 `aiah_asset_status`、
`aiah_migration_status` 与 `aiah_migration_readiness`。MCP 不开放任何写操作。
完整参数见[命令参考](docs/cli-reference.md)，客户端接入与验收见
[MCP runbook](docs/runbooks/mcp-client-acceptance.md)，跨设备人工操作见
[迁移 runbook](docs/runbooks/cross-device-transfer.md)。

## 安全边界

- TUI 首页和本机资产页初始只读；只有明确选择并确认资产库后，才会创建或写资产库。
- 应用入口只来自显式 `--package`，或当前会话成功构建出的包。
- TUI 应用必须完整输入 `apply`；`bootstrap` 没有 `--yes` 或非交互旁路。
- TUI 撤销只针对安装检查识别到的当前安装，且必须完整输入 `rollback`；选择
  历史 backup 仍使用 CLI。
- TUI 发布必须完整输入 `publish`；取回必须明确选择版本/profile 和已有输出目录，
  不覆盖不同或残缺的同名产物，取回成功也不会跳过 diff/typed `apply`。
- MCP server 只暴露 8 个只读查询工具，不暴露 `build`、资产库写操作、
  `publish/pull`、`apply`、`rollback` 或证据写入。
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
