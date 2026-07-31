# N7.2 偏好 Core 检查点（2026-07-31）

- 状态：本地检查点；尚未 push、未创建 PR、未发布
- 分支：`feat/n7-i18n-catalog`
- 前序检查点：`0caebd6`
- 本切片代码提交：`23b0b8e`
- 发布边界：public `v0.1.6` 不包含本检查点

## 1. 结论

N7.2 已达到 Core 出口：新增 `internal/preferences`，严格处理设备本地语言、
显示密度和首选资产库三项非业务偏好。配置路径、locale 和当前偏好均可注入，
测试不接触真实用户配置目录。

这不等于设置功能已开放。当前 TUI 没有设置入口，`aiah` / `aiah ui` 没有接入
偏好读取或保存，中文仍是兼容默认。只有后续设置页明确调用 `Save` 时，偏好文件才
允许创建。

## 2. Core 契约

### 2.1 读取

- 默认位置由 `os.UserConfigDir()/aiah/preferences.json` 解析，与扫描/安装
  `--home` 分离；
- 文件或目录不存在时返回 `auto` / `standard`，不创建目录或文件；
- 拒绝未知字段、尾随 JSON、错误 schema、错误语言和错误密度；
- 配置目录必须为真实 `0700` 目录，文件必须为真实 `0600` 普通文件；
- 目录或文件为软链、权限过宽、正文过大或不可读时忽略整份配置并返回稳定 warning
  code，错误不回显正文；
- 已保存的首选资产库后来失效时，语言和密度仍有效，原路径继续供未来设置页预填，
  同时返回警告；路径不会被创建、修复或猜测替代。

`Load` 的失败策略是“安全默认值 + 可见警告”，因此偏好损坏不能阻止 inventory、
doctor 或用户显式指定的工作区。

### 2.2 保存

- 先校验完整 v1 文档，再进行任何配置目录写入；
- 首选资产库必须是现有可访问目录，拒绝 `~`、相对路径和 HOME/project 下的
  `.agents` / `.claude` / `.codex` / `.grok`，并规范为解析软链后的绝对路径；
- 未注入 HOME 时使用操作员 `os.UserHomeDir()`，避免调用方漏传后放宽 HOME 下的
  受管目录边界；
- 只接受真实 `0700` 配置目录和真实 `0600` 既有文件；新目录按 `0700` 创建；
- 在目标同目录创建 `0600` 临时文件，完整写入、同步并关闭后才原子 rename；
- rename 前任一步失败时，旧文件逐字节不变，本次临时文件被清理。

工作区 Core 新增 `ValidateExistingRoot`，与 `PrepareRoot` 复用同一候选路径和
受管目录判断，但前者永不创建目录。首选资产库预填和只读检查必须走前者，不能把
路径校验扩大成写授权。

### 2.3 解析

纯 Core `Resolve` 已固定优先级：

```text
process language override
→ current preferences
→ LC_ALL / LC_MESSAGES / LANG
→ English
```

显示密度按 process override → current preferences → `standard`。override 只影响
返回的进程内有效值，不保存配置。N7.3 接入前，这些规则尚不影响实际 TUI。

## 3. 测试与变异证据

覆盖场景：

- 配置不存在且零写入；
- 非法 JSON、未知字段、尾随 JSON、错误版本和错误枚举；
- 文件软链、目录软链、`0644` 文件和 `0755` 配置目录；
- 失效首选资产库只警告、不创建；
- 受管工具目录拒绝与软链路径规范化；
- 首次保存的 `0700` / `0600`；
- 保存中断保留旧文件且无 stage 残留；
- 成功替换后只能读到完整新文档；
- locale 优先级、当前偏好、process override 和非法注入状态。

有意破坏生产实现后的实际失败：

| 变异 | 测试为何变红 |
|---|---|
| 删除 `0700` / `0600` 精确 mode | 过宽目录和文件被读取或覆盖 |
| 保存目标由 `Lstat` 改为 `Stat` | 文件软链被接受 |
| rename 前直接写目标文件 | 中断钩子观察到旧文件已改变 |
| 缺失配置时从 `Load` 调 `Save` | 只读加载创建了配置目录 |
| `ValidateExistingRoot` 创建缺失目录 | 只读 workspace 校验产生写入 |
| 未注入 HOME 时不解析操作员 HOME | `~/.claude` 被接受为首选资产库 |
| 删除 `DisallowUnknownFields` | 含 `recent` 的扩展文档被接受 |
| 删除 schema version 检查 | v2 文档被当作 v1 读取和保存 |
| 忽略 language override | override 优先级和非法 override 测试同时失败 |

所有变异均已恢复。恢复后执行：

```text
go test ./internal/preferences ./internal/workspace -count=1
go test ./...
go test -race ./internal/preferences ./internal/workspace
go vet ./...
./scripts/check-local.sh
git diff --check
```

全部通过。完整门禁包括全仓单测、race、vet、gofmt、golangci-lint、README 资产
检查和 fake HOME `validate → build → diff → apply → doctor → rollback` 闭环。

## 4. 下一步

按设计进入 N7.3，而不是直接启用所有偏好：

1. 首页增加“偏好设置 / Preferences”入口；
2. 启动时只读加载偏好并展示损坏/失效警告；
3. 提供 `auto` / `zh-CN` / `en` 和 `aiah ui --language` 临时覆盖；
4. 只有用户明确选择“保存偏好”才调用 Core `Save`；
5. 退出、取消、非 TTY、首次只读浏览和 CLI override 保持零写入；
6. 设置入口、首次保存、保存失败回退、重启恢复和 zh-CN/en golden 通过后，再进入
   N7.4 的显示密度与首选资产库预填。

在 N7.3 和正式安装包验收完成前，README 与 Release 说明仍不得声称“双语设置已
支持”。
