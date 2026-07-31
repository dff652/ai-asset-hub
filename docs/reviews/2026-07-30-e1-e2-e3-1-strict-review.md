# E1 / E2 / E3.1 严格实现复审

- 日期：2026-07-30
- 范围：任务首页、统一资产状态与资产库写操作、连续应用向导、跨设备只读状态、
  README 视觉与对应测试/文档
- 结论：**本地复审通过；发现的两项 P1 已修复，无剩余 P0/P1**
- 边界：这是当前工作树的开发态结论，不等于提交、CI、Release 或正式安装包验收

## 1. 已发现并修复

### P1-01：无法安全读取的资产库正文被误报为“待更新”

`workspace.Catalog` 在正文比较返回 finding 时仍按 `matches=false` 归类为
`source-changed`。这会让符号链接、非普通文件或读取失败的资产显示成可更新状态，
弱化 fail-closed 边界。

修复后：

- 只要安全比较产生 finding，该项统一进入 `blocked / 不可纳管`；
- 不再计入 `sourceChanged / 待更新`；
- TUI 不提供更新勾选；
- 新测试用资产库符号链接夹具固定该行为。

变异验证：临时恢复旧分支后，
`TestCatalogBlocksUnreadableLibraryAssetInsteadOfOfferingUpdate` 按预期失败，
显示 `SourceChanged:1, Blocked:0`；恢复修复后通过。

### P1-02：首页缺少自动安装状态与必要上下文

任务首页原先只自动扫描本机资产，“当前安装”长期显示“尚未检查”，且健康结果不包含
安装包、版本和目标工具。用户必须先进入子页面才能判断当前状态。

修复后：

- 常规 TUI 启动并行执行只读 `scan` 与 `doctor`；
- 未选择资产库时显示发现项与扫描问题；
- 选择资产库后显示未纳管、已纳管、待更新、仅库内、不可纳管和本机问题；
- 当前安装显示安装包、版本、目标工具和健康结论；
- deployment-only / bootstrap 路径保持原有边界，不自动启用维护入口。

变异验证：临时移除启动时 `doctor` 后，
`TestHomeInitRunsReadOnlyInventoryAndInstallationChecks` 按预期失败；恢复后通过。
该测试同时比较启动前后 fake HOME 树，证明两项检查零写入。

## 2. 维护性调整

`internal/tui/model.go` 在 E1–E3.1 后达到 972 行，继续加入 E3.2 会扩大根状态机。
本次把 tree row 类型、分组/过滤/选择和 cursor 逻辑提取到
`internal/tui/inventory_rows.go`，把加入资产库入口归回 `compose.go`。

结果：

- `model.go` 从 972 行降到 595 行；
- inventory 行构造与根消息状态机分离；
- 没有改变 Core 或 TUI 行为，现有 golden、单元测试和 race 测试均通过。

## 3. 状态信息与 Settings 边界

必要信息采用渐进披露，但不可隐藏：

- 首页：资产库路径、统一资产计数、不可纳管/扫描问题、当前安装包与版本、目标工具、
  健康结论；
- 写入前：资产组合、包路径/版本、目标工具、变更数量、风险、确认词；
- 写入后/迁移：`backupId`、rollback 可用性、资产库/安装/通道路径与版本、SHA256、
  冲突原因。

未来 Settings 只保存非业务 UI 偏好。2026-07-30 的
[N7 Proposed 方案](../designs/settings-and-i18n.md)把首版收紧为语言、首选资产库
预填和显示密度，不自动保存最近历史，也不再设置单独的“技术字段展开”开关。任何
设置都不得隐藏路径、版本、目标工具、风险、变更、确认或恢复信息，也不得关闭校验、
备份、确认或开启隐式联网。

## 4. 验证证据

- `go test ./internal/tui ./internal/workspace`：通过；
- `./scripts/check-local.sh`：通过，包含环境检查、license/release/install guard、
  全量测试、race、gofmt、lint 和 fake HOME validate/build/diff/apply/doctor/
  rollback 闭环；
- 隔离 tmux 真 TTY：裸 `aiah` 首页显示扫描结果和“尚无受管安装”，`q` 干净退出；
- README 三张静态 SVG：900px / 360px 渲染和人工检查通过；
- SVG 均有 `<title>` / `<desc>`，无脚本、远程资源、外部字体或 `foreignObject`；
- README 及关联文档的本地链接检查、`git diff --check`：通过。

## 5. 仍未完成

- 没有 commit、push、tag 或 Release；
- `v0.1.4 → 下一版` 的正式安装器升级、幂等复装、rollback 与 GitHub CI 尚未执行；
- E3.2 连续发布/取回向导、E3.3 换机前置检查、MCP 新状态工具、E4 Settings/i18n
  仍在后续任务中；
- README 已完成本地重构，但 GitHub 实际页面验收要在 push 后进行。

## 6. 建议提交拆分

1. E1：任务首页、裸 `aiah` 入口、友好文案与对应测试/文档；
2. E2：统一资产状态、纳入/更新/移出、事务恢复、ADR-0009 与连续向导；
3. E3.1：迁移只读 Core/TUI、状态模型、测试与跨设备方案；
4. N0 hardening：首页自动检查、blocked 修复、状态信息规范与状态机拆分；
5. README mode：三张项目原生 SVG、README 重构、历史快照与评估文档。

拆分时应逐组复跑相关测试，最后再跑完整 `check-local`；本评审不授权或执行提交。
