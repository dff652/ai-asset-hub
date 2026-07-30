# E3.2 连续迁移向导候选检查点

- 日期：2026-07-30
- 基线：`dev@3b4856629cb5`
- 开发分支：`feat/tui-cross-device-wizard`
- 结论：**本地候选验证通过；PR 与远端 CI 待收口，尚未发布**
- 关联：[跨设备迁移方案](../designs/cross-device-migration-and-version-sync.md)、
  [ADR-0007](../decisions/0007-immutable-channel-distribution.md)、
  [迁移 runbook](../runbooks/cross-device-transfer.md)

## 1. 候选范围

本候选只增加 E3.2 的人工连续编排，不增加传输协议或后台同步：

```text
发布端：迁移页 → p → 选择资产组合 → build → 核对包/通道 → typed publish
取回端：迁移页 → v → 明确选择版本/profile → 已有输出目录 → pull
        → 只读 diff → typed apply
```

- 通道和输出目录都由用户明确输入，TUI 不创建目录；
- 空通道目录只在 typed publish 成功后初始化 `channel.json` 与不可变包布局；
- 版本列表按发布顺序显示，光标默认位于最后一项，但不自动 pull；
- pull 成功只进入现有 Phase C，仍须完整输入 `apply` 才写目标工具目录；
- 不 shell out，不实现 Git、rsync、NAS、WebDAV、U 盘或其它网络/传输客户端；
- device-private、secret 和 adapter 换机前置摘要仍属于 E3.3。

公开版 `v0.1.6` 仍只有 E3.1 只读迁移状态页，本候选不是已发布能力。

## 2. 实现与严格审查结果

### 2.1 TUI 编排

- `p` 复用现有 profile 与 `build.Build`，build 消息携带用途，成功后进入独立
  typed publish 确认；
- `v` 调用 `channel.List`，只列当前资产库坐标；用户按键选定后才进入输出目录输入；
- pull 调用 `channel.Pull`，成功后把包路径交给已有 `apply.Diff` / typed apply，
  没有复制部署规则；
- publish/pull 运行期间阻止其它按键，错误和 Core finding 留在当前页面。

严格可维护性审查发现根 `Model` 不应继续堆叠迁移字段。最终把 E3.1/E3.2 状态收拢为
一个 `migrationFlow`，并将消息、动作和子状态机拆到 `migration_actions.go`。
当前关键文件行数：

| 文件 | 行数 | 职责 |
|---|---:|---|
| `internal/tui/model.go` | 596 | 根消息路由与共享状态 |
| `internal/tui/migration.go` | 123 | E3.1 状态读取、通道路径与后续动作衔接 |
| `internal/tui/migration_actions.go` | 371 | E3.2 发布、版本选择、取回与 Phase C 交接 |
| `internal/tui/migration_view.go` | 308 | E3 状态、确认和输入视图 |

没有新增依赖，没有修改 manifest/package schema，也没有引入 TUI 私有 Core。

### 2.2 pull 输出安全

旧 `channel.Pull` 会用 `os.WriteFile` 覆盖输出目录中的同名产物，失败清理还可能删除
预先存在的文件。候选改为：

- 写入前一次性读取四件套源内容并检查全部目标；
- 完整、逐字节相同、且全部为普通文件时幂等返回；
- 残缺、任一内容不同、符号链接或其它非普通文件时整单拒绝；
- 真正创建时使用 `O_EXCL`，关闭检查到写入之间的并发覆盖窗口；
- 失败清理只移除本次调用成功创建的文件。

新增覆盖不同内容、残缺同内容、完整幂等和输出符号链接四类回归。

## 3. 本地验证

### 3.1 常规门禁

下列命令在最终文档提交前的工作树通过：

```bash
go test ./internal/channel ./internal/tui
go test ./...
go vet ./...
git diff --check
./scripts/check-local.sh
```

完整门禁包含开发环境诊断、许可与 Release/installer/README 资产检查、Go 全量测试、
race、vet、gofmt、golangci-lint 和 fake HOME
validate/build/diff/apply/doctor/rollback 闭环。

### 3.2 变异验证

两道关键边界都确认测试能抓住真实回归：

1. 临时绕过 pull 的内容/残缺检查并把 exclusive create 改为 truncate，
   `TestPullRefusesToOverwriteOutputArtifacts` 按预期失败：

   ```text
   err = <nil>, want ErrChannelBlocked
   ```

2. 临时反转 typed publish 判断，
   `TestMigrationPublishRequiresTypedConfirmation` 按预期失败：

   ```text
   wrong confirmation started publish: confirming=false notice="" command nil=false
   ```

两次变异均已立即恢复；恢复后定向测试、全量测试和完整门禁重新通过。

### 3.3 隔离真 TTY 闭环

使用 `testdata/workspace-valid` 的临时副本、空通道目录、空 incoming 目录和 fake HOME，
通过 tmux 真实 PTY 驱动开发二进制：

```text
迁移页 → 选择通道 → p → personal → typed publish
→ v → 明确选择 2026.07.1/personal → 输入 incoming
→ diff → typed apply → Doctor → typed rollback
```

实测结果：

- publish 后通道有 1 个版本，通道与资产库版本一致；
- pull 后进入变更预览，显示 `create=5`，此时 fake HOME 仍为空；
- typed apply 写入 5 项并显示 `backupId`；
- Doctor 显示 `unchanged=5`、`locally-modified=0`、`missing=0`、0 个问题；
- typed rollback 恢复 0 项、移除 5 项；
- 最终 CLI Doctor 为 `ok=true`、`deployment=false`，保留 1 个安装恢复点记录。

全过程只写临时目录，未读取或修改真实 HOME 中的工具资产。

## 4. 尚未完成

- 尚未提交或推送当前开发分支；
- 尚未创建面向 `dev` 的 PR；
- push / pull_request CI 尚未运行；
- 尚未合并，也不属于任何 Release。

上述远端结果补齐并再次通过最终提交的本地门禁后，E3.2 才能标记为 dev 候选完成。
下一阶段是 E3.3 换机前置检查，不在本 PR 扩展范围。
