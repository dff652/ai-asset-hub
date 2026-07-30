# 2026-07-30 E3.3 换机前置检查候选检查点

## 1. 结论与边界

E3.3 已完成本地候选实现：用户在 TUI“迁移到其他设备”页按 `e`，明确选择当前
资产库中的资产组合后，可只读查看：

- 当前设备上按设计不迁移的 credential、session、cache、device-state 等项目；
- MCP `${ENV:...}` / `${secret:...}` 引用在当前设备是否可用；
- profile 中的目标是否有内置 adapter；
- 每个 adapter 将生成的文件数、丢弃项和能力降级项；
- 所有阻止项、警告及其路径。

检查不 build、不创建 `dist/`、不发布、不取回、不写目标工具目录，也不自动 apply。
`device-private` 只作排除说明；adapter degraded 是需确认警告；缺失 secret、不支持
目标和 adapter dropped 是阻止项。

本检查只绑定**当前资产库和所选 profile**，结果只描述**运行检查的当前设备**。
源、目标设备应分别检查。它不伪装成通道中任意历史发布包的逐包验收；选定发布包
绑定、双设备和失败恢复验收继续属于 E3.4。当前候选尚未进入公开 Release，
`v0.1.6` 不包含此入口。

## 2. 实现复用与结构审查

实现保持 Core/TUI 分层：

- `build.Prepare` 在内存中复用 workspace 校验、依赖、profile 和 target 选择；
  `build.Build` 再基于同一结果生成产物，避免前置检查复制 build 规则；
- `migration.InspectPreflight` 编排 inventory、build、adapter 和 secret Core，
  统一计算 readiness、摘要与 findings；
- `apply.InspectSecretReferences` 丢弃解析值，报告只保留 provider、引用名、
  影响目标和可用性；
- TUI 只维护 profile 选择、加载状态、光标与文案；完整 Core 报告通过可导航列表
  展示，不把 adapter/secret 判断复制进界面层。

按严格维护性审查核对了新增分支、文件行数和规则归属：没有文件越过 1000 行，
没有新增 TUI 私有业务规则，也没有发现需要阻止候选提交的结构回归。

## 3. 本地验证证据

### 3.1 定向与完整门禁

定向回归通过：

```text
go test ./internal/migration ./internal/tui ./internal/apply ./internal/build
```

完整门禁通过：

```text
./scripts/check-local.sh
```

覆盖开发环境、许可证、Release/installer/README 资产检查、全量 test、race、vet、
gofmt、golangci-lint 和假 HOME build→diff→apply→doctor→rollback 闭环。
首次门禁只报一项 `S1016` 结构体转换建议；修正后完整门禁重新全绿。

### 3.2 安全变异验证

两项临时变异均按预期变红，随后立即恢复：

1. 在 `InspectPreflight` 临时注入 `.preflight-mutation` 写入：
   `TestInspectPreflightReportsBlockersWarningsAndDevicePrivateWithoutWriting`
   通过前后目录摘要发现新增文件并失败；
2. 临时反转缺失 secret 的阻止条件：同一测试因缺少
   `preflight_missing_secret` finding 失败。

恢复后定向回归和完整门禁重新通过；变异代码不在候选 diff 中。

### 3.3 隔离真 TTY

使用 `testdata/workspace-valid` 临时副本、fake HOME 和 tmux 真实 PTY 驱动开发
二进制：

```text
aiah → 迁移到其他设备 → e 换机检查 → personal
```

页面实测显示：

```text
可继续：未发现阻止项
目标 3 · 不支持 0 · 丢弃 0 · 降级 0 · secret 缺失 0/0 · 本机不迁移 1
目标工具 · claude / codex / shared
本机不迁移 · home/.codex/auth.json
```

退出后核对：

- 临时资产库没有 `dist/`；
- fake HOME 中测试前已有的 `auth.json` SHA256 不变；
- 真实 HOME 和真实工具目录未参与；
- tmux 会话正常退出，隔离目录已移入回收站。

## 4. 远端与合并状态

本检查点创建时仅证明本地候选。仍需：

1. 按逻辑拆分实现与文档提交；
2. 推送开发分支并创建面向 `dev` 的 PR；
3. 等待 PR 当前最终 head 的 push 与 pull_request CI 全绿；
4. 严格复核未提交/未推送差异；
5. 由仓库所有者明确决定是否合并。

不能用 PR #24 或本地绿灯替代 E3.3 最终 head 的远端 CI；未获新的合并授权前不合并。
