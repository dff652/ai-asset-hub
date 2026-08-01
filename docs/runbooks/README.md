# Runbook / SOP 索引

本目录只收可重复执行、已有真实实现支撑的流程。一次性决策写 ADR，时点结论写
review；尚未实现的能力只列缺口，不先写一份无法验证的“未来 SOP”。

## 已固化

| 场景 | 入口 | 状态 |
|---|---|---|
| 新设备开发环境 | [development-environment.md](development-environment.md) | bootstrap、工具链与只读基线已实跑 |
| 假 HOME 资产闭环 | [fake-home-loop.md](fake-home-loop.md) | CI 与本机共同执行 |
| 真实 HOME 安全预演 | [real-home-dry-run.md](real-home-dry-run.md) | 默认只读；真写有人工门槛 |
| 跨设备资产分发 | [cross-device-transfer.md](cross-device-transfer.md) | publish/pull/bootstrap 与校验闭环 |
| MCP 客户端接入与只读验收 | [mcp-client-acceptance.md](mcp-client-acceptance.md) | 只读 MCP 协议、Claude/Codex/Grok 握手、模型调用与零写入边界 |
| 工具安装、升级与 TUI dogfood | [install-upgrade-dogfood.md](install-upgrade-dogfood.md) | Linux amd64；`v0.1.8→v0.1.9` 严格升级/init/TTY/MCP 回归通过；完整偏好与双设备闭环最近在 v0.1.7 实跑 |
| 工具自身发版 | [release.md](release.md) | main CI → tag → Release → 下载验收 |
| README 与 SVG 视觉验收 | [readme-visual-acceptance.md](readme-visual-acceptance.md) | 语义核对、视觉 token、自动门禁与 900/360 人工检查 |
| 首次 public 切换 | [public-launch.md](public-launch.md) | 一次性历史流程，已完成；保留作取证 |

## 仍需固化

### P0：包格式跨版本兼容矩阵

这不是先补文档就算完成。要先用历史 Release 二进制和固定 fixture 回答：

- 旧 `aiah` 读取新包如何失败；
- 新 `aiah` 读取旧包支持到什么程度；
- `schemaVersion` 何时必须提升；
- build / diff / apply / rollback 的跨版本验收组合。

先落兼容测试与 golden 包，再写 runbook；否则 SOP 没有可执行事实。

### P1：新增平台原生准入

macOS、Windows、arm64 目前只有交叉编译，不具备安装/写入语义验收。任一平台准备
重新进入 Release 前，应固化该平台的：

- 原生文件权限、原子替换和路径语义；
- HOME/project adapter 行为；
- apply/rollback 与 TUI 真终端验收；
- 安装入口、签名和卸载方式。

### P1：新增 Target adapter 准入

Cursor 等新 Target 仍明确暂缓。实现 ADR-0002 阶段 A 后，再固化能力声明、inventory、
compose、apply/rollback、MCP/hook 边界与跨设备 fixture；当前不写空流程。

## 暂不拆独立 SOP

- Release 故障回退已在 [release.md §5](release.md#5-出问题怎么退)，出现首个真实
  事故且现有表不足时再升级为 incident runbook。
- Secret Provider 的设备前置与失败处理已在
  [real-home-dry-run.md](real-home-dry-run.md) 和
  [cross-device-transfer.md](cross-device-transfer.md)；没有必要复制第三份。
- Homebrew/Scoop 尚未实现；渠道存在后再写维护 SOP。
