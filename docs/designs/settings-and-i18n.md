# N7：偏好设置与中英文支持方案

- 状态：N7.0–N7.5 源码候选已实现；完整双语消息目录、严格偏好 Core、设置页、
  语言切换、显示密度、首选资产库预填和 100/60 列验收已于 2026-07-31 完成；
  正式安装包验收尚未完成
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

当前分支源码候选已提供“偏好设置 / Preferences”入口，可编辑三项首版偏好，并支持
`aiah ui --language` / `--density` 临时覆盖。无偏好文件时实际按 locale 选择中文或
English；读取始终只读，只有用户在设置页明确选择“保存偏好”才创建或替换文件。

保存的首选资产库只在首页提示并预填路径框，TUI 保持“未选择”，直到用户在本次会话
按 Enter。public `v0.1.6` 不包含 N7 改动；README 与上手指南须等 N7.5 正式安装包
验收后，才能把这些能力标成已发布。

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

N7.4 首个实现把密度接到现有变更预览的可选逐项明细：create/update 和风险分组始终
展开，`standard` 默认折叠 unchanged/skipped，`detailed` 默认展开；用户仍可手动
展开或收起。首页、资产页、安装检查、迁移、版本、阻止页和 typed confirmation
渲染不读取 density，因此两种密度保持逐字等价。未来扩展更多技术字段时必须先扩大
必要信息矩阵，不能借“标准模式”隐藏路径、风险或恢复信息。

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

设置页现已开放语言、显示密度和首选资产库。首选资产库编辑只调用
`workspace.ValidateExistingRoot`：允许清空或选择现有安全目录，不创建、不打开，
也不把草稿变成当前 workspace。“重置全部本机偏好”会把三项恢复为安全默认值，
但重置本身仍不写文件，必须再次明确选择“保存偏好”。

## 8. 分阶段开发任务

### N7.0：契约冻结

- [x] 评审本文的三项首版设置和排除项；
- [x] 修订 ADR-0006 中“不得引入 TUI 私有设置存储”的绝对措辞，只允许本文定义的
  非业务偏好；
- [x] 固化必要信息清单和偏好文件安全契约；
- [x] 确认无偏好启动最终采用 `auto`。

出口：只有文档变化，不新增设置能力。

### N7.1：完整字符串目录，不开放切换

- [x] 建立 typed `messageID`、`zh-CN` / `en` catalog 和 English fallback；
- [x] 首页 40 个消息完整迁移，首页 production 文件不再直写中文；
- [x] 首页 zh-CN/en golden、catalog key/格式占位符完整性和中文直写门禁；
- [x] inventory、工作区/profile 输入、纳入/更新/移出和相关 help 完整迁移；
- [x] inventory zh-CN/en golden、输入框语言同步和资产库操作英文验收；
- [x] diff/apply、二次确认、应用结果和恢复提示完整迁移；
- [x] diff/apply zh-CN/en 预览与确认 golden，English 结果与 typed `apply` 验收；
- [x] doctor/rollback、检查结果、阻止提示和恢复结果完整迁移；
- [x] doctor/rollback zh-CN/en 检查与确认 golden，English 结果与 typed
  `rollback` 验收；
- [x] migration 状态、换机检查、typed `publish`、版本选择、显式取回和包级检查
  完整迁移；
- [x] migration zh-CN/en 状态、换机检查、发布确认、版本列表 golden，以及
  English 取回目录和包级阻止路径验收；
- [x] version 本机信息、当前资产安装、显式 Release 检查、失败与升级命令完整迁移；
- [x] version 离线/更新可用 zh-CN/en golden，English 状态、失败与 help 验收；
- [x] 提取所有 production TUI 用户可见字符串为 typed IDs；
- [x] 为首页、inventory、diff/确认、doctor/rollback、migration、version 增加
  双语核心页面/确认页 golden，并覆盖 help English 验收；
- [x] 保持当前 zh-CN 默认行为，避免重构和行为切换同时发生。

出口：两套 catalog 完整，但用户仍看不到语言开关。

2026-07-30 检查点：`617b464` 完成首页首个垂直切片，`./scripts/check-local.sh`
通过。缺翻译键、首页重新直写中文、默认语言误改为 English 三项变异都能使对应
测试失败。该检查点仍有 15 个 production TUI 文件、434 行含中文文本待迁移；这里按
`rg` 行数统计，不等同于 434 个独立消息。`withLanguage` 保持 package-private，
因此用户界面仍只有兼容默认的简体中文，且不会创建偏好文件。

第二检查点 `4b68c9f` 把目录扩展到 199 个 typed 消息，完成 inventory 与资产库
管理切片。删除 English `inventory.title`、重新直写中文、取消语言切换时的输入框
同步三项变异均能使门禁失败，完整 `check-local` 通过。排除中文 catalog 后，剩余
9 个 production TUI 文件、260 行含中文文本；下一步依次迁移 diff/apply、
doctor/rollback、migration 和 version。用户仍看不到语言开关，也不会创建偏好文件。

第三检查点 `ad230fa` 把目录扩展到 239 个 typed 消息，完成 diff/apply 切片。
删除 English `diff.title`、重新直写中文、放宽 typed `apply` 三项变异均能使门禁
失败；确认输入框的终端填充空格已在 golden 比较中规范化，仓库文件本身保持
`git diff --check` clean。排除中文 catalog 后剩余 7 个 production TUI 文件、
219 行含中文文本；下一步迁移 doctor/rollback。

第四检查点 `29ab586` 把目录扩展到 277 个 typed 消息，完成 doctor/rollback
切片。Doctor 继续只读；只有检查通过并存在当前安装 backup ID 才开放撤销，且必须
完整输入 `rollback`。删除 English `health.title`、重新直写中文、把确认判断放宽为
任意非空值三项变异均能使门禁失败，完整 `check-local` 通过。排除中文 catalog 后
剩余 5 个 production TUI 文件、181 行含中文文本；下一步迁移 migration，再迁移
version。用户仍看不到语言开关，也不会创建偏好文件。

第五检查点 `16ae4c9` 把目录扩展到 428 个 typed 消息，完成 migration 全流程
切片。状态读取、换机前置与包级检查继续零写入；发布仍须完整输入 `publish`，取回
仍只写用户显式选择的已有输出目录，包检查通过后仍须进入 diff 并完整输入 `apply`。
删除 English `migration.title`、重新直写中文、把 typed `publish` 放宽为任意非空值
三项变异均能使门禁失败，完整 `check-local` 通过。排除中文 catalog 后仅剩
`version.go` / `version_view.go` 两个 production 文件、29 行含中文文本；下一步完成
version，仍不开放语言开关或创建偏好文件。

第六检查点 `71f2f7c` 把目录扩展到 457 个 typed 消息，完成 version 并达到 N7.1
出口。打开版本页仍只读取本机程序与当前安装，只有用户按 `c` 后才运行只读 GitHub
Release 检查；升级命令只展示，不自动执行或替换二进制。删除 English
`version.title`、重新直写中文、让打开页面直接进入在线检查状态三项变异均能使门禁
失败，完整 `check-local` 通过。排除中文 catalog 后，production TUI 含中文 literal
为 0；下一步进入 N7.2 偏好 Core，而不是直接声明双语设置已可用。

### N7.2：偏好 Core

- [x] 新增 `internal/preferences` 的严格 load/validate/save；
- [x] 注入 config path、locale env 和当前偏好，测试不接触真实用户目录；
- [x] 覆盖不存在、非法 JSON、未知字段、错误版本、软链、权限、写入中断和原子替换；
- [x] 做安全变异验证：放宽 mode、允许软链、直接覆盖和隐式自动保存均必须变红。

出口：Core 可用，但 TUI 尚不自动创建文件。

2026-07-31 检查点 `23b0b8e` 达到 N7.2 出口：

- `Load` 对缺失文件返回安全默认值且零写入；损坏 JSON、未知字段、尾随 JSON、
  错误 schema/枚举、软链和过宽权限均使用稳定 warning code，不阻止其它 Core；
- 失效的首选资产库保留原路径用于未来设置页预填，同时报告警告，不创建、不修复；
- `Save` 只接受完整 v1 文档，复用 `workspace.ValidateExistingRoot` 的受管工具目录
  边界；配置目录和文件分别固定为 `0700` / `0600`，同目录临时文件写完、同步、
  关闭后才原子 rename；
- 中断测试证明 rename 前失败时旧文件逐字节不变且无临时文件残留；
- `Resolve` 已实现 CLI override → 当前偏好 → 注入 locale → English 的纯 Core
  优先级，但尚未接入命令行或 TUI；
- 放宽 mode、跟随软链、直接覆盖、隐式自动保存、允许只读校验创建目录、删除未知
  字段/schema/override 防线等变异均按预期失败，恢复后完整
  `./scripts/check-local.sh` 通过。

因此仓库现在存在“可复用的偏好存储 Core”，但不存在“已启用的偏好功能”。普通
`aiah` / `aiah ui` 行为、中文兼容默认和零隐式配置写入保持不变；下一步必须先做
N7.3 设置页和显式保存动作，不能从 Core 完成推导为用户功能已发布。

### N7.3：设置页与语言切换

- [x] 首页加入设置入口、未保存修改、保存失败和重置状态；
- [x] 支持 `auto` / `zh-CN` / `en`，并增加 `aiah ui --language` 临时覆盖；
- [x] 明确 CLI override 不落盘；
- [x] 设置文件只在用户明确保存后创建；
- [x] 非 TTY、首次启动和只读浏览继续零写入。

出口：中英文可用；语言切换与偏好写入边界通过真实 TTY 验收。

2026-07-31 检查点 `0fffb3e` 达到 N7.3 出口：

- TTY 检查先于偏好加载；首次启动、非 TTY、只读浏览和 CLI override 均零写入；
- 启动按 `--language` → 保存值 → `LC_ALL` / `LC_MESSAGES` / `LANG` →
  English 解析实际界面语言；
- 首页新增“偏好设置 / Preferences”和损坏配置警告，设置页展示保存路径、未保存
  状态、稳定 warning code 的本地化说明和临时 override 边界；
- 选择语言立即预览，`Esc` / `m` 放弃并恢复进入前有效语言；只有“保存偏好”调用
  `internal/preferences.Save`；
- 保存失败恢复进入设置页前的有效偏好；语言保存保留 N7.4 才编辑的 density 和
  preferred library；
- 重置先只修改 draft，明确保存后才替换损坏配置或清除失效首选资产库；
- zh-CN/en 首页与设置页 golden、CLI flag、locale、重启恢复和九类写入边界变异
  均通过；完整 `./scripts/check-local.sh` 通过。

隔离真实 PTY 使用临时 HOME/XDG config 完成
`选择 English → 明确保存 → 检查 0700/0600 → 中文 locale 重启仍为 English`。
这是源码候选验收，不替代 N7.5 的正式 Release 安装包 dogfood。

### N7.4：密度与首选资产库预填

- [x] 支持 `standard` / `detailed` 和 `aiah ui --density`；
- [x] 必要信息矩阵逐屏测试，确认页/阻止页两种密度必须等价；
- [x] 首选资产库只预填，缺失路径不创建，使用前仍需本次会话确认；
- [x] `--workspace` 继续优先且保持现有兼容行为。

出口：设置降低重复操作，但不放宽任何写入授权。

2026-07-31 检查点 `bf36f6d` 达到 N7.4 源码候选出口：

- 设置页从五行扩展为语言、信息密度、首选资产库、保存和重置四个分区；三项仍共用
  完整 v1 draft，预览、取消、保存失败恢复和显式保存边界不变；
- `--density` 复用 `preferences.Resolve`，CLI override 只作用于当前进程且偏好文件
  逐字节不变；
- 密度只在新一轮 diff 开始时设置默认展开状态，不覆盖用户在当前 diff 中手动展开或
  收起的状态；
- 首选路径编辑复用只读 `workspace.ValidateExistingRoot`；不存在或不安全路径保持
  编辑状态并显示本地化错误，不创建目录、不更新 workspace、不写偏好；
- 已保存但失效的路径继续显示警告、首页建议和输入框预填；只有用户按 Enter 后才走
  原有 `workspace.PrepareRoot` 明确打开/创建流程；
- 显式 `--workspace` 仍在启动时优先准备，保存的路径不会替换它；
- 首页、inventory、doctor、migration、version、阻止页和确认页的密度等价测试，
  diff 必要摘要/分组测试，以及 zh-CN/en 首页/设置页 golden 均通过；
- 标准密度误展开、编辑首选路径时创建目录、预填即自动选择三项变异均能使测试失败；
  恢复后完整 `./scripts/check-local.sh` 和隔离真实 PTY 保存/重启/预填验收通过。

这是本地源码候选，不是 public Release。N7.5 仍需完成 100/60 列核心页面和写入确认
检查、fake HOME/config 汇总验收，以及 Linux amd64 正式安装包 dogfood。

### N7.5：发布候选验收

- [x] `./scripts/check-local.sh`、`git diff --check` 和安全变异全部通过；
- [x] zh-CN/en 在 100 列与 60 列覆盖核心页面和所有写入确认；
- [x] fake HOME/fake config 证明无隐式写入或真实配置污染；
- [x] Linux amd64 本地 release-style 候选完成首次启动、保存、重启、语言切换和
  损坏配置恢复；
- [ ] Linux amd64 正式安装包完成首次启动、保存、重启、语言切换和损坏配置恢复；
- [ ] README/上手指南只在正式包验收后把“双语设置”标成已发布。

2026-07-31 N7.5 源码候选检查点：

- 新增 zh-CN/en × 100/60 列验收，覆盖首页、inventory、diff 预览与阻止页、
  doctor、migration 状态/版本列表/换机前置检查、version 和 settings；
- `apply`、`rollback`、`publish`、`update`、`remove` 五类写入确认均展示完整
  包/资产库/通道作用域、目标、版本或备份、选中数量和稳定英文确认词；
- 详情在双栏宽度不足时自动改为列表加详情的单栏布局，必要路径和 64 位 SHA256
  不再用省略号隐藏；首选资产库在设置页另列完整当前值；
- fake HOME/config 汇总验收证明首次启动不创建偏好目录，显式保存后重启生效，
  损坏 JSON 安全回退且原文件不自动覆盖，测试范围之外的哨兵目录逐字节不变；
- 两项新增安全变异分别禁用“详情放不下时改为单栏”和移除 apply 确认包路径，
  均能让 N7.5 验收测试失败，恢复后重新通过；
- 本地 `0.1.7-dev.n7.5` Linux amd64 release-style 候选通过 SHA、ELF/版本自检和
  60 列真实 PTY；设置保存产生 `0700` 目录与 `0600` 文件，重启恢复已保存中文和
  detailed，临时 English/standard override 不改文件，损坏配置显示警告且保持原文。

最后一项使用当前工作树构建的本地候选，不是 GitHub Release 产物，也没有执行安装器
或改动真实用户配置。因此它只能证明“候选可验”，不能把正式安装包勾为完成；详细证据
见 [N7.5 源码候选验收记录](../reviews/2026-07-31-n7-release-candidate-acceptance.md)。

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

## 10. 已确认的产品决定

1. 无偏好文件时语言使用 `auto`；中文 locale 显示简体中文，其余显示 English。
2. 首选资产库只做预填，不自动启用；最近资产库历史暂缓。

两项已于 2026-07-30 确认。N7.1 保持 `zh-CN` 兼容默认；N7.3 已在真实启动路径
启用 `auto`，但无偏好文件时仍不创建配置。只有设置页明确保存才写入本机偏好文件。
