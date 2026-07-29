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
| 工具安装、升级与 TUI dogfood | [install-upgrade-dogfood.md](install-upgrade-dogfood.md) | Linux amd64；既有 Release 升级与 D2/D3 候选已实跑，新版发布后须再验收 |
| 工具自身发版 | [release.md](release.md) | main CI → tag → Release → 下载验收 |
| 首次 public 切换 | [public-launch.md](public-launch.md) | 一次性历史流程，已完成；保留作取证 |

## 仍需固化

### P0：包格式跨版本兼容矩阵

这不是先补文档就算完成。要先用历史 Release 二进制和固定 fixture 回答：

- 旧 `aiah` 读取新包如何失败；
- 新 `aiah` 读取旧包支持到什么程度；
- `schemaVersion` 何时必须提升；
- build / diff / apply / rollback 的跨版本验收组合。

先落兼容测试与 golden 包，再写 runbook；否则 SOP 没有可执行事实。

### P0：完整 TUI 发布验收

当前安装升级 SOP 已覆盖 D2/D3，现有设计文档记录了 Phase A–D3 dogfood，但尚缺一份
把 D1 的 `workspace → compose → build → diff → apply`、D2 和 D3 串成一个发布
候选人工清单的独立 SOP。下一次 TUI 工作流发生实质变化时应补齐，避免为本次发布
重复抄现有步骤。

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
