# N7.1 inventory 与资产库管理双语目录检查点（2026-07-30）

- 状态：本地检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 前序检查点：`cc0e806`
- 本切片代码提交：`4b68c9f`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

N7.1 已从首页扩展到 inventory、资产库选择/profile 输入、纳入/更新/移出确认和
相关帮助，共 199 个 typed `messageID`。中文默认界面与既有 Core/CLI 契约保持
兼容，English 能在包内测试中完整渲染这些页面。

这仍然不是“已支持中英文设置”：`withLanguage` 继续是 package-private，
`auto` locale、设置入口和偏好文件都未实现，也没有任何首次启动隐式写入。

## 2. 本切片覆盖

- inventory 的扫描/资产库状态、树、详情、过滤、风险视图、footer 和帮助；
- 资产库路径输入、profile 选择和对应错误/进度提示；
- 纳入资产库的结果、跳过原因和事务恢复提示；
- update/remove 的选择限制、typed confirmation、结果和失败恢复提示；
- deployment、doctor、version 的共享 help 导航文案；
- filter placeholder 在测试语言变化时同步刷新；
- 简体中文既有 inventory golden 和新增 English inventory golden。

`apply`、`update`、`remove`、`rollback` 等确认词，finding code 和 Core 载荷仍是
稳定协议值，不翻译。TUI 仍直接调用相同 Core，没有 shell out 或复制业务规则。

## 3. 完整性与安全门禁

目录门禁验证：

- `messages.go` 中每个声明的 `messageID` 必须同时存在于 `zh-CN` 和 `en`；
- 两套 catalog 键数、非空值和 `fmt` 占位符必须一致；
- 已迁移的 7 个生产文件不得重新出现中文 string literal；
- 默认语言仍为 `zh-CN`，English 测试语言必须同步输入框 placeholder；
- English 资产库路径、profile、remove 确认和纳入/更新结果均有断言。

有意破坏结果：

| 变异 | 实际失败 |
|---|---|
| 删除 English `inventory.title` | `zh-CN=199 en=198`，并报告声明键缺失 |
| 在 `inventory_view.go` 重新写入 `— 中文` | 中文直写门禁报告该 literal |
| 删除 `withLanguage` 的输入同步 | English placeholder 仍为中文，测试失败 |

三项变异均已恢复。恢复后执行：

```text
go test ./internal/tui -count=1
git diff --check
./scripts/check-local.sh
```

全部通过。完整门禁还覆盖全仓单测、race、lint、构建和 fake HOME
`validate → build → diff → apply → doctor → rollback` 闭环。

## 4. 剩余范围

排除 `messages_zh_cn.go` 后，当前仍有 9 个 production TUI 文件、260 行含中文
文本；这是 `rg` 文本行数，不是独立消息数量：

1. diff/apply 与确认；
2. doctor/rollback；
3. migration 与发布/取回/换机检查；
4. version 与更新状态。

N7.1 完成全部页面目录和 60/100 列双语验收后，才进入 N7.2 preference Core。
