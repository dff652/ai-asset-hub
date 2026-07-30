# N7.1 diff/apply 双语目录检查点（2026-07-30）

- 状态：本地检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 前序检查点：`6d9ebdc`
- 本切片代码提交：`ad230fa`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

N7.1 已覆盖只读变更预览、typed `apply` 二次确认、应用中状态、应用结果、备份 ID
和撤销命令，共 239 个 typed `messageID`。简体中文兼容输出与既有写入安全边界
保持不变，English 也能完整展示预览、确认和结果。

语言设置仍未开放：`withLanguage` 只供包内测试，默认仍为 `zh-CN`，没有 locale
自动选择或偏好文件。

## 2. 本切片覆盖

- Diff loading/failed/blocked/成功摘要和变更树；
- create/update/unchanged/skipped 与 finding 详情；
- 二次确认页及其零写入说明；
- applying 状态、成功/失败/no-op 结果；
- targets、written、backupId、完整 rollback 命令和下一步安装检查；
- zh-CN/en 各两份 golden：变更预览与二次确认；
- English 应用结果对 backupId、rollback 命令和安装检查提示的断言。

Core finding code/message/path 原样展示，`apply` token 不翻译。TUI 仍复用
`apply.Diff` / `apply.Apply`，没有 shell out 或业务规则复制。

## 3. 安全回归

既有测试继续证明：

- 按 `a` 只能打开确认页，不能直接 Apply；
- 非完整 `apply` 必须拒绝；
- Diff 与 Core 报告一致且零写入；
- 真实 Apply 保留 targets，生成 backupId 并给出完整 rollback 命令；
- symlink target 和 MCP conflict 的 Core findings 不被隐藏或改写。

本切片有意破坏：

| 变异 | 实际失败 |
|---|---|
| 删除 English `diff.title` | `zh-CN=239 en=238`，声明键完整性同时失败 |
| 在 `diff_view.go` 重新写入 `— 目标` | 中文直写门禁报告该 literal |
| 把完整 `apply` 判断放宽为任意非空值 | `yes` 启动 Apply，错误确认拒绝测试失败 |

三项变异均已恢复。首次提交检查还捕获了 textinput 渲染产生的 golden 尾随空格；
测试现只规范化终端行尾填充，golden 文件自身无尾随空格，随后重新执行完整门禁。

```text
go test ./internal/tui -count=1
git diff --check
./scripts/check-local.sh
```

全部通过，包括全仓单测、race、lint、构建及 fake HOME
`validate → build → diff → apply → doctor → rollback` 闭环。

## 4. 剩余范围

排除 `messages_zh_cn.go` 后，当前仍有 7 个 production TUI 文件、219 行含中文
文本：

1. doctor/rollback；
2. migration 与发布/取回/换机检查；
3. version 与更新状态。

下一切片先做 doctor/rollback，并继续保留 typed `rollback` 和只针对当前安装的
恢复边界。
