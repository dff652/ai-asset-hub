# N7.3 设置页与语言切换检查点（2026-07-31）

- 状态：本地源码候选检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 前序检查点：`b3b76d6`
- 本切片代码提交：`0fffb3e`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

N7.3 已达到代码与源码候选 TTY 出口：普通 TUI 首页新增“偏好设置 /
Preferences”，用户可选择 `auto`、`zh-CN` 或 `en`，也可用
`aiah ui --language auto|zh-CN|en` 临时覆盖本次进程。

偏好仍是设备本地、非业务状态。设置页不包含资产、targets、profile、secret，也
不能关闭校验、备份、diff 或 typed confirmation。显示密度和首选资产库预填尚未
开放，留给 N7.4。

## 2. 用户流程与边界

### 2.1 启动

真实 TTY 检查发生在偏好读取之前。通过后按以下顺序解析语言：

```text
aiah ui --language
→ 已保存 language
→ LC_ALL / LC_MESSAGES / LANG
→ English
```

- 无偏好文件时按 locale 显示，但不创建配置目录或文件；
- 损坏 JSON、未知版本、软链、权限不安全或失效首选资产库不会阻止 TUI；
- 首页显示“偏好文件需要处理”，设置页再给出本地化原因；
- deployment-only / bootstrap 复用同一语言解析，但不提供设置写入口；
- CLI JSON、MCP、typed `apply` / `rollback` / `publish` 等机器契约不翻译。

### 2.2 设置页

设置页当前提供五行操作：

1. 自动：按 locale 选择；
2. 简体中文；
3. English；
4. 保存偏好；
5. 重置全部本机偏好。

选择语言会立即预览当前设置页，但不写文件。`Esc` / `m` 放弃 draft 并恢复进入
设置页前的有效语言。重置也只修改 draft；它会清除 language、density 和
preferred library 到安全默认值，因此界面明确说明“仍需保存”。

只有用户选择“保存偏好”才调用 `internal/preferences.Save`。成功后清除 warning，
失败则恢复进入设置页前的有效偏好并显示错误，不回显文件正文。N7.3 修改 language
时保留尚未开放编辑的 density 和 preferred library。

`--language` 是最高优先级的进程内覆盖，不反写偏好；即使用户在设置页明确保存，
本次进程仍继续服从 override。

## 3. 测试与变异证据

新增覆盖：

- 首次启动 + 中文 locale / English locale；
- 已保存语言优先于 locale；
- CLI override 优先且文件逐字节不变；
- 语言预览、Esc 取消、显式保存和重启恢复；
- 保存失败恢复旧有效语言；
- language 保存保留 density / preferred library；
- 失效首选资产库经“重置 → 明确保存”修复；
- 损坏配置在首页和设置页可见；
- 设置页 zh-CN/en golden、首页第六项和 help；
- 非 TTY 在偏好读取/写入及 workspace 创建前失败；
- 无效 `--language` 在启动 TUI 前以用法错误拒绝。

有意破坏后的实际失败：

| 变异 | 测试为何变红 |
|---|---|
| 启动读取后自动 `Save` | 首次启动创建偏好目录 |
| 选择语言时直接 `Save` | 预览阶段出现偏好文件 |
| 取消时不恢复 runtime language | Esc 后仍显示预览语言 |
| 保存失败时不恢复 runtime language | 错误后仍显示未保存语言 |
| 选择语言先重建默认文档 | density 与 preferred library 被误清空 |
| CLI override 写回 document | 原偏好文件字节发生变化 |
| 重置时直接 `Save` | 用户尚未选择保存，文件已改变 |
| 跳过无效 language 校验 | `--language fr` 启动 TUI |
| 非 TTY 检查前写默认偏好 | 管道调用创建偏好目录 |
| 删除 English 设置入口消息 | catalog 数量与声明键完整性失败 |

所有变异均已恢复。恢复后：

```text
go test ./internal/tui ./cmd/aiah -count=1
go test ./...
go test -race ./internal/tui ./cmd/aiah
go vet ./...
./scripts/check-local.sh
git diff --check
```

全部通过。完整门禁包含全仓测试、race、vet、gofmt、golangci-lint、README 资产
检查和 fake HOME 应用/撤销闭环。

## 4. 隔离真实 PTY

使用临时 HOME、临时 XDG config 和本地构建二进制执行：

```text
中文 locale 首次启动
→ 首页进入偏好设置
→ 选择 English（此时仍无文件）
→ 明确保存
→ 检查配置目录 0700、文件 0600、language=en
→ 保持中文 locale 重启
→ 首页恢复为 English
```

脚本中的路径、内容、mode、保存结果和重启语言断言全部通过，临时目录随后删除。
这是源码候选的真实 TTY 验收，不是正式 Release 安装包验收；后者仍属于 N7.5。

## 5. 下一步

进入 N7.4：

1. 接入 `standard` / `detailed` 和 `aiah ui --density`；
2. 用逐屏必要信息矩阵证明两种密度不隐藏路径、风险、变更、确认或恢复信息；
3. 设置首选资产库，但只预填路径输入框；
4. 缺失或不安全路径只警告、不创建，使用前仍须本次会话确认；
5. 显式 `--workspace` 继续最高优先且保持兼容。

N7.4 完成后再进入 N7.5 的 100/60 列、fake HOME/fake config 和正式 Linux amd64
安装包验收。README 与上手指南在正式包通过前不宣称该能力已发布。
