# ADR-0009：统一资产状态与受控资产库写操作

- 状态：Accepted
- 日期：2026-07-30
- 关联：[TUI 产品体验 V2](../designs/tui-product-experience-v2.md)、
  [ADR-0006](0006-tui-as-first-interactive-surface.md)

## 背景

create-only 的 `compose` 能安全建立资产库，但用户无法判断源端与资产库是否一致，
也只能手工更新或删除。把这类操作称为“同步”会错误暗示后台、双向、冲突合并和删除
传播能力。

## 决策

1. Workspace Core 根据扫描结果、manifest 和正文内容统一计算五种状态：
   `unmanaged`、`managed`、`source-changed`、`library-only`、`blocked`。
2. `纳入` 保持 create-only；`更新` 是用户明确选择的源端 → 资产库完整替换；
   `移出` 删除资产库正文和 manifest/profile 引用，但不删除源端文件。
3. 更新和移出均检查路径边界与资产路径重叠，先暂存原内容，校验失败恢复操作前状态。
   仍被其他资产的 dependency/conflict 引用时拒绝移出。
4. TUI 只调用上述 Core；更新、移出分别要求完整输入 `update`、`remove`。
5. 成功后连续进入 profile、build 和 diff；写目标工具目录仍需独立输入 `apply`。

## 术语边界

- apply backup 对用户称“安装恢复点”，不称“资产库备份”；
- 资产库备份当前由 Git、NAS 快照或外部备份工具负责；
- publish/pull 是不可变版本分发，不称“双向同步”。

## 后续任务

- 应用成功页汇总目标工具、文件数、恢复点和建议下一步；
- 用正式安装包完成 E1/E2 真机 dogfood；
- E3 只把现有 publish/versions/pull/bootstrap 带入“跨设备分发”页面；
- E4 在完整字符串目录和安全 UI 偏好配置落地后再开放语言/设置。
