# N7.1 migration 双语目录检查点（2026-07-30）

- 状态：本地检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 前序检查点：`6dc4f8c`
- 本切片代码提交：`16ae4c9`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

N7.1 已覆盖跨设备状态对齐、换机前置检查、不可变发布、版本选择、显式取回和取回包
级检查，共 428 个 typed `messageID`。简体中文兼容输出与既有 E3 安全边界保持
不变，English 也能完整展示主要状态、写入边界、阻止路径和下一步。

语言设置仍未开放：`withLanguage` 只供包内测试，默认仍为 `zh-CN`，没有 locale
自动选择或偏好文件。

## 2. 本切片覆盖

- 资产库、当前安装、分发通道和版本对齐状态；
- E3.3 换机前置检查的 target、secret、本机不迁移项、finding 和摘要；
- 已有普通目录的分发通道选择及只读状态刷新；
- 生成包后的 typed `publish` 确认、发布结果与不可变覆盖边界；
- 按发布顺序查看版本、显式选择 release、已有输出目录取回；
- E3.4 name/version/profile/SHA256 绑定、包级目标设备检查和 diff 交接；
- zh-CN/en 各四份 golden：状态、换机检查、发布确认和版本列表；
- English 取回目录边界、包级阻止路径和错误确认词断言。

Core alignment/status/finding 和 release 坐标原样展示，`publish` / `apply` token
不翻译。TUI 继续直接调用 migration、channel、diff Core，没有 shell out、网络
传输实现或重复业务规则。

## 3. 安全回归

既有和新增测试继续证明：

- 状态与换机检查零写入，不生成 `dist/`，也不改资产库或目标 HOME；
- 通道路径确认只读且不创建缺失目录；
- 发布前必须完整输入 `publish`，错误确认不写通道；
- 取回只写明确选择的已有输出目录，同名残缺、不同内容或非普通文件不覆盖；
- 取回包先绑定 release 坐标与 SHA256，阻止项存在时不能进入 diff；
- 包检查通过也只进入只读 diff，仍须完整输入 `apply` 才写目标工具目录。

本切片有意破坏：

| 变异 | 实际失败 |
|---|---|
| 删除 English `migration.title` | `zh-CN=428 en=427`，声明键完整性同时失败 |
| 在 `migration_view.go` 重新写入 `— 版本` | 中文直写门禁报告该 literal |
| 把完整 `publish` 判断放宽为任意非空值 | `wrong` 启动 Publish，typed confirmation 测试失败 |

三项变异均已恢复。

```text
go test ./internal/tui -count=1
git diff --check
./scripts/check-local.sh
```

全部通过，包括全仓单测、race、lint、构建及 fake HOME
`validate → build → diff → apply → doctor → rollback` 闭环。

## 4. 剩余范围

排除 `messages_zh_cn.go` 后，当前只剩两个 production TUI 文件、29 行含中文文本：

1. `version.go`；
2. `version_view.go`。

下一切片完成 version 与更新状态目录。全目录完成后仍先评审 N7.1 出口，不直接开放
`auto` locale、设置页或偏好文件。
