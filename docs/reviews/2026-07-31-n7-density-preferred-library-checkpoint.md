# N7.4 密度与首选资产库预填检查点（2026-07-31）

- 状态：本地源码候选检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 前序检查点：`e4b62cb`
- 本切片代码提交：`bf36f6d`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

N7.4 已达到源码候选出口。偏好设置现可编辑：

1. 界面语言：`auto` / `zh-CN` / `en`；
2. 信息密度：`standard` / `detailed`；
3. 首选资产库：一个现有安全目录，只用于首页提示和路径框预填。

三项都是设备本地界面偏好，不是资产业务配置。它们不能改变 targets、profile、
校验、备份、diff、typed confirmation 或 MCP/CLI/JSON 契约。

## 2. 用户流程与边界

### 2.1 信息密度

`aiah ui --density standard|detailed` 是进程内临时覆盖，优先于保存值且不反写文件。
设置页选择密度只更新 draft 和本次预览；只有明确保存才持久化。

密度控制“新一轮变更预览的默认展开状态”，不控制信息是否存在：

| 页面/信息 | standard | detailed |
|---|---|---|
| create/update 与风险分组 | 默认展开 | 默认展开 |
| unchanged/skipped 逐项明细 | 默认折叠，可手动展开 | 默认展开，可手动收起 |
| 首页、资产页、安装检查、迁移、版本 | 完全相同 | 完全相同 |
| 阻止原因与受影响路径 | 完全相同 | 完全相同 |
| typed apply 确认页 | 完全相同 | 完全相同 |

密度默认只在新 diff 开始时应用，不能覆盖当前 diff 中用户手动展开/收起的状态。

### 2.2 首选资产库

设置页编辑首选资产库时只调用 `workspace.ValidateExistingRoot`：

- 允许选择现有、可访问且不与 `.agents` / `.claude` / `.codex` / `.grok`
  受管目录重叠的安全目录；
- 支持输入 `~/...`，保存时规范为解析软链后的绝对路径；
- 空输入清除首选值；
- 不存在、不安全或不可访问时留在编辑页显示错误；
- 编辑、取消和预览均不创建目录、不打开资产库、不更新当前 workspace、不写偏好。

保存后重新启动仍保持 `workspace=""`。首页显示“未选择 · 建议 …（使用前仍需确认）”，
任务入口把路径预填到“选择资产库”输入框；只有用户在本次会话按 Enter，才调用原有
`workspace.PrepareRoot`。因此，失效路径在启动和预填阶段不会被重新创建，而明确
Enter 仍保留既有“打开或创建用户刚确认的路径”语义。

显式 `aiah ui --workspace PATH` 继续最高优先：启动时准备该路径，保存的首选值只留在
偏好文档和警告中，不替换当前 workspace。

## 3. 测试与变异证据

新增覆盖：

- 保存 density 与 `--density` 临时覆盖，且覆盖不改偏好文件；
- standard/detailed 的可选 diff 明细差异；
- diff 摘要、分组、包名与 create/update/unchanged/skipped 数量始终可见；
- 首页、inventory、doctor、migration、version、阻止页与确认页两种密度逐字等价；
- 首选路径编辑只接受现有安全目录，失败时零创建、零 workspace、零偏好写入；
- 失效保存路径仍提示和预填，但启动不创建、不自动选择；
- 显式 `--workspace` 优先，失效首选路径不会被启动流程重建；
- CLI 无效 density 在启动 TUI 前作为用法错误拒绝；
- zh-CN/en 首页和设置页 golden 同步三项设置文案。

有意破坏后的实际失败：

| 变异 | 锚定结果 |
|---|---|
| standard 也默认展开 unchanged/skipped | 密度差异测试发现标准模式出现逐项路径 |
| 首选路径编辑改用 `PrepareRoot` | 不存在路径被创建，零创建测试失败 |
| 预填时直接赋值当前 workspace | 会话未确认已变成选中状态，预填边界测试失败 |

所有变异均已恢复。代码提交前执行：

```text
go test ./internal/tui ./cmd/aiah -count=1
./scripts/check-local.sh
git diff --check
```

全部通过。完整门禁包含全仓测试、race、vet、gofmt、golangci-lint、README 资产
检查，以及 fake HOME 的 build → diff → apply → doctor → rollback 闭环。

## 4. 隔离真实 PTY

使用源码构建二进制、临时 HOME 和临时 XDG config 逐键执行：

```text
English locale 首次启动
→ Preferences
→ Detailed
→ 输入一个已存在的首选资产库
→ 明确 Save preferences
→ 检查 JSON 与 0700/0600
→ 重启
→ 首页仍显示 Not selected + Suggested
→ 进入 Organize，只预填路径
→ Esc 取消，资产库仍未打开
```

保存结果为 `language=auto`、`density=detailed` 和规范化绝对首选路径；重启后的首页与
路径输入框均显示该路径，但没有自动进入 inventory 或获得资产库写能力。

这是源码候选真实 PTY 验收，不替代 N7.5 正式 Release 安装包 dogfood。

## 5. 下一步

进入 N7.5：

1. 在 zh-CN/en、100 列和 60 列检查核心页面及所有写入确认；
2. 汇总 fake HOME/fake config 的首次启动、损坏配置、保存失败和零污染证据；
3. 用 Linux amd64 正式安装包完成首次启动、三项保存、重启和损坏配置恢复；
4. 正式包通过后再更新 README 与上手指南，不提前宣称 public `v0.1.6` 已有该能力。
