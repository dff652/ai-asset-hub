# N7：偏好设置与中英文支持方案

- 状态：Proposed（仅方案，尚未实现）
- 日期：2026-07-30
- 目标阶段：TUI 产品体验 V2 E4
- 关联：[TUI 产品体验方案](tui-product-experience-v2.md)、
  [ADR-0006](../decisions/0006-tui-as-first-interactive-surface.md)、
  [安全与隐私](../security.md)

## 1. 结论

N7 应做，但必须按“**字符串目录先行，偏好持久化后置**”拆开实施。设置页只管理
设备本地的界面偏好，不成为第二份业务配置，也不替代首页、资产页和确认页必须展示的
状态。

首版只提供三项：

1. 语言：`auto` / `zh-CN` / `en`；
2. 首选资产库：只预填路径，每次会话仍由用户确认后才启用资产库操作；
3. 显示密度：`standard` / `detailed`，只控制技术明细的默认展开状态。

不在首版加入最近资产库自动历史、主题、自动升级、网络源、target/profile/scope、
secret 或任何“关闭校验/备份/确认”的开关。

当前代码**没有**设置页、偏好文件或语言切换。TUI 以中文为主，界面字符串分散在
多个 view/action 文件；仓库当前只有一份 inventory golden。本文描述的能力不能在
实现和验收完成前写成已支持。

## 2. 产品边界

### 2.1 设置页是什么

首页新增“偏好设置 / Preferences”任务入口。它回答：

- 界面使用什么语言；
- 路径输入框优先提示哪个资产库；
- 页面默认展开多少技术细节；
- 当前偏好保存到哪个本机文件。

进入设置页、切换选项或预览语言都不写文件。只有用户明确选择“保存偏好”后才允许
写偏好文件；退出或 `Esc` 放弃本次未保存修改。

### 2.2 设置页不是什么

设置页不得承载：

- 资产、profile、targets、scope、portability、sensitivity；
- 分发通道、安装包版本选择或 apply 目标；
- secret、token、Cookie、MCP env 实际值；
- 关闭校验、关闭备份、跳过 diff、自动确认或隐藏风险；
- 后台同步、自动发布、自动取回或自动升级；
- 当前安装、扫描结果和风险看板。

`manifest.yaml` 继续是资产业务配置唯一事实源。偏好文件是设备本地派生状态，不进入
资产包、不进入分发通道，也不由 MCP 读取或修改。

### 2.3 首选资产库不等于默认写入授权

ADR-0006 规定：没有明确确认资产库，就没有资产库写能力。N7 保留这条规则：

- `--workspace PATH` 仍是本次启动的显式选择，优先级最高，行为保持兼容；
- 保存的 `preferredAssetLibrary` 只用于首页提示和路径输入框预填；
- TUI 不因读取偏好而创建、打开或自动选择该目录；
- 用户仍须在本次会话按 Enter 确认路径，才调用现有
  `workspace.PrepareRoot`；
- 偏好中的路径失效、不可访问或落入受管工具目录时，只显示警告，不创建、不修复、
  不回退到猜测路径。

这样既减少重复输入，又不会把“曾经保存过一个路径”扩大成未来每次启动的写授权。

## 3. 偏好数据模型

### 3.1 文件位置

使用 Go `os.UserConfigDir()` 得到操作员配置目录，再拼接：

```text
<user-config-dir>/aiah/preferences.json
```

Linux 通常是 `${XDG_CONFIG_HOME:-$HOME/.config}/aiah/preferences.json`。它和
`aiah ui --home PATH` 的扫描/安装目标分离：`--home` 不得把偏好文件改写到被检查的
另一台设备目录。

测试必须注入临时配置路径，不能读写开发者真实配置目录。

### 3.2 v1 schema

```json
{
  "schemaVersion": 1,
  "language": "auto",
  "density": "standard",
  "preferredAssetLibrary": "/absolute/path/to/ai-assets"
}
```

规则：

- `schemaVersion` 必须恰好为 `1`；
- `language` 只接受 `auto`、`zh-CN`、`en`；
- `density` 只接受 `standard`、`detailed`；
- 首选资产库可省略；保存时必须是现有、可访问且不落入受管工具目录的安全目录，
  规范为解析软链后的绝对路径，不保存 `~`；
- 拒绝未知字段和尾随 JSON；
- 不保存最近打开文件、HOME/project、包路径、通道、凭据或运行状态。

首版不保存“最近使用的资产库列表”。自动维护历史意味着仅仅浏览一个目录也会写配置，
与显式保存边界冲突；真实使用证明需要该功能后再单独评估。

### 3.3 读取与失败策略

读取偏好永远只读：

- 文件不存在：使用安全默认值，不创建文件；
- JSON、版本或枚举非法：忽略整份文件，使用安全默认值，并在首页与设置页显示警告；
- 文件是软链、目录或权限过宽：拒绝读取敏感扩展字段的可能性，显示警告；
- 无效偏好不得阻止 inventory、doctor 或显式 `--workspace` 正常工作；
- 不静默覆盖损坏文件。用户必须在设置页明确选择保存/重置。

### 3.4 安全写入

保存偏好由独立的 `internal/preferences` Core 负责，TUI 不直接拼文件：

1. 解析并验证完整文档；
2. 确认 `aiah` 配置目录和目标文件不是软链；
3. 配置目录使用 `0700`，目标文件使用 `0600`；
4. 在同目录创建 `0600` 临时文件，完整写入、关闭后原子 rename；
5. 任一步失败都保留旧文件，清理本次临时文件；
6. 保存内容不得包含 secret，错误信息不得回显文件正文。

## 4. 语言解析与覆盖顺序

语言优先级：

```text
aiah ui --language
→ 已保存 language
→ auto 解析 LC_ALL / LC_MESSAGES / LANG
→ English
```

`auto` 对 `zh`、`zh_CN`、`zh-CN` 等中文 locale 选择 `zh-CN`，其它 locale 选择
`en`。不增加隐式联网语言包；两套目录都编译进同一个二进制。

显示密度优先级：

```text
aiah ui --density
→ 已保存 density
→ standard
```

CLI override 只作用于当前进程，不反写偏好。现有 CLI 子命令、JSON schema、MCP
报告和 typed confirmation token 保持不变；`apply`、`rollback`、`publish`、
`update`、`remove` 等确认词不翻译，避免脚本、文档和安全测试出现两套口令。

N7.1 建立字符串目录时仍以 `zh-CN` 作为兼容默认值；只有 N7.3 设置入口和 locale
golden 同时就绪后，才把无偏好启动切换到 `auto`。这是独立的产品决策门。

## 5. 字符串目录设计

### 5.1 目录与类型

界面层使用编译期定义的 `messageID`，两套内置 catalog 分文件维护：

```text
internal/tui/messages.go
internal/tui/messages_zh_cn.go
internal/tui/messages_en.go
```

View/action 代码只引用 message ID，不再直接放用户可见中英文句子。保持直接、无模板
引擎、无运行时语言包和无新网络依赖。

测试必须验证：

- zh-CN 与 en 的 ID 集合完全一致；
- 值非空，格式化占位符数量和类型一致；
- production TUI 文件中不再出现 catalog 之外的中文界面字面量；
- 未知 ID 在测试中失败，生产环境使用明确的英文 fallback，不显示空白；
- 排序、业务状态和写操作判断不依赖翻译后的文字。

英文单复数由小型 presentation helper 处理，不把复数规则塞进 Core，也不引入通用
模板系统。

### 5.2 Core finding 的边界

Core 的 `kind`、`code`、`severity`、JSON 字段和原始 `message` 是机器契约，不能因
TUI 语言改变。

TUI 可以按稳定 finding code 提供本地化标题和处置建议，同时：

- 路径、severity、code 和受影响对象保持原值；
- `detailed` 模式可展开 Core 原始 message；
- 未识别 code 时显示本地化“未知问题”加原始 message，不静默丢弃；
- 不在翻译层重新判断风险级别、可移植性或是否允许继续。

## 6. 显示密度

`density` 只改变默认展开状态，不能改变能否访问某项信息。

无论 `standard` 还是 `detailed`，以下信息始终可见：

- 当前任务和下一步；
- 资产库、包、通道和目标路径；
- package/version/profile/targets；
- 风险级别、finding 数量和阻止原因；
- create/update/remove/unchanged 数量；
- 将执行的写操作、确认词和是否联网；
- backup ID、rollback 入口和失败恢复建议；
- 迁移时绑定的发布坐标与摘要。

`detailed` 可以默认展开 finding code、`producedBy`、完整摘要、时间戳和逐项
unchanged/skipped；`standard` 可以默认折叠这些技术字段，但用户仍可手动展开。
确认页与阻止页不受密度影响。

## 7. TUI 交互

首页任务列表增加“偏好设置 / Preferences”，不占用当前状态区域。设置页使用稳定
方向键和 Enter 操作，不要求用户记新的 CLI。

建议流程：

```text
首页 → 偏好设置
→ 选择语言 / 显示密度
→ 可选：输入首选资产库
→ 预览本次界面
→ 查看保存路径和最终值
→ 明确保存；或 Esc 放弃
```

语言预览可以立即重绘当前设置页，但保存失败必须恢复为进入设置页前的有效偏好并显示
错误。首选资产库输入框明确提示“只预填，使用时仍需确认”。

## 8. 分阶段开发任务

### N7.0：契约冻结

- [ ] 评审本文的三项首版设置和排除项；
- [ ] 修订 ADR-0006 中“不得引入 TUI 私有设置存储”的绝对措辞，只允许本文定义的
  非业务偏好；
- [ ] 固化必要信息清单和偏好文件安全契约；
- [ ] 确认无偏好启动最终采用 `auto`，或决定继续默认 `zh-CN`。

出口：只有文档变化，不新增设置能力。

### N7.1：完整字符串目录，不开放切换

- [ ] 提取所有 production TUI 用户可见字符串为 typed IDs；
- [ ] 完成 zh-CN/en catalog、格式占位符与完整性测试；
- [ ] 为首页、inventory、diff/确认、doctor/rollback、migration、version/help
  增加双语 golden；
- [ ] 保持当前 zh-CN 默认行为，避免重构和行为切换同时发生。

出口：两套 catalog 完整，但用户仍看不到语言开关。

### N7.2：偏好 Core

- [ ] 新增 `internal/preferences` 的严格 load/validate/save；
- [ ] 注入 config path、locale env 和当前偏好，测试不接触真实用户目录；
- [ ] 覆盖不存在、非法 JSON、未知字段、错误版本、软链、权限、写入中断和原子替换；
- [ ] 做安全变异验证：放宽 mode、允许软链、直接覆盖和隐式自动保存均必须变红。

出口：Core 可用，但 TUI 尚不自动创建文件。

### N7.3：设置页与语言切换

- [ ] 首页加入设置入口、未保存修改、保存失败和重置状态；
- [ ] 支持 `auto` / `zh-CN` / `en`，并增加 `aiah ui --language` 临时覆盖；
- [ ] 明确 CLI override 不落盘；
- [ ] 设置文件只在用户明确保存后创建；
- [ ] 非 TTY、首次启动和只读浏览继续零写入。

出口：中英文可用；语言切换与偏好写入边界通过真实 TTY 验收。

### N7.4：密度与首选资产库预填

- [ ] 支持 `standard` / `detailed` 和 `aiah ui --density`；
- [ ] 必要信息矩阵逐屏测试，确认页/阻止页两种密度必须等价；
- [ ] 首选资产库只预填，缺失路径不创建，使用前仍需本次会话确认；
- [ ] `--workspace` 继续优先且保持现有兼容行为。

出口：设置降低重复操作，但不放宽任何写入授权。

### N7.5：发布候选验收

- [ ] `./scripts/check-local.sh`、`git diff --check` 和安全变异全部通过；
- [ ] zh-CN/en 在 100 列与 60 列覆盖核心页面和所有写入确认；
- [ ] fake HOME/fake config 证明无隐式写入或真实配置污染；
- [ ] Linux amd64 正式安装包完成首次启动、保存、重启、语言切换和损坏配置恢复；
- [ ] README/上手指南只在正式包验收后把“双语设置”标成已发布。

## 9. 验收矩阵

| 场景 | 必须结果 |
|---|---|
| 首次启动，无偏好文件 | 不创建配置；TUI 正常进入只读首页 |
| `LANG=zh_CN.UTF-8` + auto | zh-CN |
| `LANG=en_US.UTF-8` + auto | English |
| CLI language/density override | 仅本次有效，不改偏好文件 |
| 偏好 JSON 损坏/未知版本 | 安全默认值 + 可见警告；不自动覆盖 |
| 偏好文件或目录为软链 | 拒绝保存；旧文件和目标均不变 |
| 保存中途失败 | 原偏好逐字节不变，无残留临时文件 |
| 首选资产库不存在 | 只警告和预填，不创建目录 |
| standard 密度 | 路径、版本、targets、风险、变更、确认、恢复信息不缺失 |
| zh-CN/en 窄屏 | 不截断确认词、风险、恢复入口或必要路径 |
| bootstrap/deployment-only TUI | 复用同一语言目录，不意外开放设置写入 |
| CLI/JSON/MCP | 输出契约与 N7 前一致 |

## 10. 实施前需要确认的两个产品决定

推荐值如下：

1. 无偏好文件时语言使用 `auto`；中文 locale 显示简体中文，其余显示 English。
2. 首选资产库只做预填，不自动启用；最近资产库历史暂缓。

这两项确认后再进入 N7.1。当前阶段不创建偏好文件、不修改 TUI 行为。
