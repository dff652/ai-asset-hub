# N7.1 version 与完整双语目录检查点（2026-07-31）

- 状态：本地检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 前序检查点：`d53b605`
- 本切片代码提交：`71f2f7c`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

version 切片完成后，N7.1 达到代码出口：457 个 typed `messageID` 覆盖全部
production TUI 用户文案，排除中文 catalog 后含中文 string literal 为 0。简体中文
兼容默认保持不变，English catalog 和核心页面 golden 完整。

这不等于用户已经获得双语设置：`withLanguage` 仍只供包内测试，当前没有 locale
自动选择、设置页或偏好文件。public `v0.1.6` 也不包含本地分支上的 N7.1 改动。

## 2. 本切片覆盖

- aiah version、commit 和 build date；
- 当前资产安装的 package/version/profile/backup ID；
- 未检查、检查中、失败、已是最新、可更新、ahead、development 和未知状态；
- 只有按 `c` 后才触发的只读 GitHub Release 检查；
- Release URL 和窄屏可复制的精确版本升级命令；
- version 离线页和更新可用页各一份 zh-CN/en golden；
- English 状态、失败提示和版本 help 的独立断言；
- `version.go` / `version_view.go` 纳入中文 literal 门禁。

Core update status、版本值、URL 和升级命令保持原值，不因界面语言变化。TUI 不执行
升级命令，也不自动替换当前二进制。

## 3. 网络与执行边界

既有和新增测试继续证明：

- 打开 version 页只调本地 Doctor，不运行 Release 检查；
- 页面明确显示“尚未检查”和“打开 TUI 不会自动联网”；
- 只有用户按 `c` 才构造 update check command；
- 网络失败保持可见，不伪装为“已是最新”；
- 升级命令按固定四行展示，保留精确 `AIAH_VERSION`，但不自动执行；
- deployment-only 交互不开放 version 页面。

本切片有意破坏：

| 变异 | 实际失败 |
|---|---|
| 删除 English `version.title` | `zh-CN=457 en=456`，声明键完整性同时失败 |
| 在 `version_view.go` 重新写入 `— 版本` | 中文直写门禁报告该 literal |
| 打开 version 页时直接设置在线检查状态 | 离线边界测试报告页面已开始 update check |

三项变异均已恢复。

```text
go test ./internal/tui -count=1
git diff --check
./scripts/check-local.sh
```

全部通过，包括全仓单测、race、lint、构建及 fake HOME
`validate → build → diff → apply → doctor → rollback` 闭环。

## 4. N7 后续

N7.1 的完成只代表字符串目录与测试基础就绪。下一步按设计顺序执行：

1. N7.2：严格的 `internal/preferences` load/validate/save，仍不开放设置页；
2. N7.3：设置页、`auto` / `zh-CN` / `en` 和临时 CLI override；
3. N7.4：显示密度与仅预填的首选资产库；
4. N7.5：fake HOME/fake config、窄屏和正式安装包验收。

在 N7.3/N7.5 完成前，README 和发布说明不得把“双语设置”标为已支持或已发布。
