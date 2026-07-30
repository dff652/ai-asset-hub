# N7.1 doctor/rollback 双语目录检查点（2026-07-30）

- 状态：本地检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 前序检查点：`0b6b96f`
- 本切片代码提交：`29ab586`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

N7.1 已覆盖只读安装检查、当前安装撤销入口、typed `rollback` 二次确认、撤销中
状态和结果，共 277 个 typed `messageID`。简体中文兼容输出与既有恢复安全边界
保持不变，English 也能完整展示检查、确认、阻止原因和恢复结果。

语言设置仍未开放：`withLanguage` 只供包内测试，默认仍为 `zh-CN`，没有 locale
自动选择或偏好文件。

## 2. 本切片覆盖

- Doctor loading/failed/正常/风险状态、摘要、当前安装详情、drift 和 finding；
- 仅在 Doctor 通过且当前 deployment 存在 backup ID 时显示撤销入口；
- 没有当前安装、检查未完成或检查未通过时的明确阻止提示；
- typed `rollback` 确认页、撤销中状态和成功/失败结果；
- zh-CN/en 各两份 golden：安装检查与撤销确认；
- English 恢复结果和阻止提示的独立断言。

Core status、finding code/message/path 原样展示，`rollback` token 不翻译。Doctor
继续只读；TUI 仍复用 `apply.Doctor` / `apply.Rollback`，没有 shell out 或业务
规则复制。成功撤销后沿用既有逻辑刷新 Doctor 与 inventory。

## 3. 安全回归

既有和新增测试继续证明：

- Doctor 只读，不因浏览或刷新产生目标目录写入；
- Doctor 失败、报告不健康或没有当前 installation backup 时不能撤销；
- 历史 backup 不在 TUI 猜测或自动选择，只能由 CLI 显式指定；
- 非完整 `rollback` 必须拒绝；
- 真实 Rollback 调用 Core，完成后刷新安装检查与统一资产状态。

本切片有意破坏：

| 变异 | 实际失败 |
|---|---|
| 删除 English `health.title` | `zh-CN=277 en=276`，声明键完整性同时失败 |
| 在 `health_view.go` 重新写入 `— 备份` | 中文直写门禁报告该 literal |
| 把完整 `rollback` 判断放宽为任意非空值 | `yes` 启动 Rollback，错误确认拒绝测试失败 |

三项变异均已恢复。

```text
go test ./internal/tui -count=1
git diff --check
./scripts/check-local.sh
```

全部通过，包括全仓单测、race、lint、构建及 fake HOME
`validate → build → diff → apply → doctor → rollback` 闭环。

## 4. 剩余范围

排除 `messages_zh_cn.go` 后，当前仍有 5 个 production TUI 文件、181 行含中文
文本：

1. migration、发布/取回与换机检查；
2. version 与更新状态。

下一切片先迁移 migration，最后迁移 version。完成全目录前不开放语言设置或
`auto` locale，也不创建偏好文件。
