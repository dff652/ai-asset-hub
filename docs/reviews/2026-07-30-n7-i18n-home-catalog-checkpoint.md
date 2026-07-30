# N7.0 决策与 N7.1 首页双语目录检查点（2026-07-30）

- 状态：本地检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 方案提交：`32e7373`
- 首页目录提交：`617b464`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

N7.0 的产品和安全边界已经冻结，N7.1 的首页首个垂直切片已经实现并通过完整本地
门禁。当前**不能**宣称 TUI 已支持中英文切换：只有首页具备两套消息，默认仍为
`zh-CN`，`withLanguage` 只供包内测试使用；设置页、`auto` locale 选择和偏好文件
均不存在。

## 2. 已冻结的产品边界

1. 无偏好时最终使用 `auto`：中文 locale → `zh-CN`，其它 → `en`；N7.3 前继续
   保持 `zh-CN` 兼容默认。
2. 首选资产库只预填，不自动选择、创建或授权写入；不保存最近资产库历史。
3. 首版只允许语言、首选资产库预填、显示密度三项设备本地 UI 偏好。
4. 偏好不能进入 manifest、资产包、MCP 或 Core 业务规则，也不能隐藏必要决策信息。
5. 偏好文件只能在用户明确保存时创建，并须满足严格 schema、`0600`、原子替换、
   拒绝软链和不含 secret；本检查点尚未创建该文件或写入逻辑。

## 3. 已实现的首页切片

- `language` 和 typed `messageID`；
- `zh-CN` / `en` 两套首页 catalog，各 40 个消息；
- 选择目录缺项时回退 English，English 也缺项时显示显式 `[missing:<id>]`；
- 首页任务、状态、帮助和“整理本机资产”提示不再直写中文；
- 首页状态标签和任务标题按终端显示宽度对齐；
- 简体中文与 English 两份 100 列 golden；
- 现有中文测试继续证明默认行为未切换。

该切片只改变 TUI 展示层；没有 shell out，没有复制 Core 规则，也没有改变
apply/rollback/publish/pull 的确认与写入边界。

## 4. 验证证据

提交前执行：

```text
go test ./internal/tui -count=1
./scripts/check-local.sh
git diff --check
```

结果全部通过。`check-local` 覆盖开发环境、license/release/install/README 资产检查、
全仓单元与集成测试、race、gofmt、构建和 fake HOME
`validate → build → diff → apply → doctor → rollback` 闭环。

有意破坏验证：

| 变异 | 必须失败的门禁 | 实际失败 |
|---|---|---|
| 删除 English `home.footer` | catalog 完整性 | `zh-CN=40 en=39` |
| 在 `home_view.go` 重新写入 `aiah 首页` | 已迁移文件禁止中文直写 | 报告该 Han string literal |
| 把默认语言改成 `en` | 中文兼容默认和现有首页测试 | 默认语言断言及中文首页断言同时失败 |

每项变异都已恢复，恢复后 `internal/tui` 全套测试和完整本地门禁再次通过。

## 5. 未完成范围与下一步

排除 `messages_zh_cn.go` 后，当前 `rg` 仍在 15 个 production TUI 文件中找到
434 行含中文文本；这是文本行数，不是独立消息数量。N7.1 后续按以下顺序推进：

1. 共享输入框、通知和基础导航；
2. inventory 与资产库管理；
3. diff/apply 确认和 doctor/rollback；
4. migration、version 与全部 help；
5. 60/100 列的全页面双语 golden 和中文直写门禁。

只有 N7.1 全页面目录完成后才进入 `internal/preferences`；只有 preference Core
通过独立安全变异后才开放设置页和语言切换。
