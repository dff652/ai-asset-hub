# 2026-07-30 E3.4 发布包绑定与跨设备验收检查点

## 1. 结论与产品边界

E3.4 候选补上了 E3.3 明确保留的缺口：目标设备检查现在绑定用户实际选择并取回的
不可变发布包，而不是新设备上可能不存在或版本不同的资产库。

TUI 连续路径为：

```text
versions → 明确选择 name/version/profile → pull 到已有目录
→ 绑定 SHA256 的取回版本检查 → Enter → diff → typed apply → doctor
```

安全边界不变：

- pull 只写显式输出目录，检查和 diff 不写 HOME；
- 坐标、SHA256、包完整性或目标检查任一失败，不能进入 diff；
- 检查通过也不会自动 apply，仍须审阅变化并完整输入 `apply`；
- aiah 不实现 Git/NAS/rsync/U 盘传输，不比较版本号大小，不自动选择最新版；
- secret 值不进入包、预检报告、journal 或 backup metadata；
- “恢复旧版本”是显式选择旧坐标后再次检查/diff/apply，不是静默降级；
- 本阶段不新增公开 CLI/MCP 写接口。

## 2. 实现与规则归属

- `pkgload.Open` 在读取同一 tar 文件描述符时计算完整 archive SHA256，避免迁移层
  重新打开路径计算一份可能漂移的摘要；
- `migration.InspectPackagePreflight` 复用 `pkgload.Open`、adapter 编译、
  `apply.InspectSecretReferences` 和 inventory 分类；
- `PreflightSubject` 明确报告来源是 `workspace` 还是 `package`，包级报告包含
  name、version、profile、archive 名和 SHA256；
- TUI 只保存 pull 返回的已选发布坐标、加载状态和连续导航，不复制校验规则；
- `apply.Options.ExpectedSHA256` 让 diff 与最终 apply 继续绑定同一发布包；
  bootstrap 也把 pull 返回的 SHA 传入既有 Phase C；
- `channel.json` 每条记录必须使用标准
  `packages/<name>/<version>/<profile>` 路径，archive、SHA256、坐标和唯一性必须
  自洽；
- publish/pull 对通道目录逐级 `Lstat`，拒绝路径中的软链；恶意索引不能越出
  通道根读取外部发布树。

## 3. 验收矩阵

| 场景 | 夹具/门禁 | 当前结果 |
|---|---|---|
| 两台隔离设备 | 源资产库 build/publish，复制完整通道，目标 pull/preflight/diff/apply/doctor | 定向测试通过 |
| 同版本同内容 | 重复 publish、重复 pull | 幂等且目录摘要不变 |
| 同坐标不同内容 | v1 坐标重新 build 不同内容后 publish | fail-closed |
| 显式旧版本恢复 | 目标设备依次应用 v1→v2→显式 v1 | 内容和 deployment 版本恢复为 v1 |
| 发布中断恢复 | 发布树已存在、索引缺失后重发相同包 | 不重写发布树，只重建索引 |
| apply 中途失败 | 既有 `TestCommitFailureRestoresCommittedFiles` | 自动恢复已提交文件并清 journal |
| apply 恢复失败 | 既有 `TestFailedRecoveryKeepsJournal` + Doctor | 保留 journal，显式暴露人工处理 |
| 恶意索引越界 | 自洽外部发布树 + `../trusted/...` 索引 | 拒绝，输出目录零写入 |
| 目录软链 | 标准索引 + profile 目录软链到外部有效发布 | 拒绝，输出目录零写入 |
| 包篡改/替换 | archive、旁路 SHA、index SHA、可解析性既有夹具 | 任一不一致均拒绝并清理本次产物 |

## 4. 当前验证证据

定向测试已通过：

```text
go test ./internal/channel ./internal/migration ./internal/tui
```

其中新增端到端夹具
`TestE34TwoDeviceTransferIdempotencyAndExplicitOldVersionRestore` 完成两设备、
幂等、冲突和 v1→v2→v1 闭环；TUI 测试确认 pull 后先进入包级检查，有阻止项时
Enter 不会进入 diff，检查/diff 前目标 HOME 不变。

### 4.1 安全变异验证

三项临时变异均按预期变红，随后恢复：

1. 移除 channel release 标准路径匹配：
   `TestChannelIndexRejectsNonCanonicalReleaseRecords/path_mismatch` 失败并显示
   非标准路径被接受；
2. 跳过 `apply.Options.ExpectedSHA256` 比较：
   `TestDiffRejectsAPackageOutsideTheSelectedDigest` 失败并生成了本应阻止的变更计划；
3. 移除 TUI `preflightReport.Ok` 连续门：
   `TestPulledPackageBlockersPreventEnteringDiff` 失败并进入 diff。

恢复后对应定向测试与完整门禁重新通过；变异代码不在候选 diff 中。

### 4.2 完整门禁与严格维护性审查

最终文件树两次运行 `./scripts/check-local.sh` 均通过，覆盖开发环境、许可证、
installer/Release/README 资产检查、全量 test、race、vet、gofmt、
golangci-lint 和假 HOME 闭环。

按严格维护性审查重构后：

- archive SHA 由 `pkgload` 在读取同一 tar 文件描述符时计算，migration 不重复打开；
- TUI 移除一次性 package-mode 布尔，使用实际 pull 上下文判断检查来源；
- workspace/package 共用同一 preflight Core，expected SHA 继续约束 diff/apply；
- E3.4 channel 与 migration 夹具拆成独立文件；无文件越过 1000 行；
- 修复迁移子页显示 `m 首页` 但按键无效，以及已完成检查仍显示“正在检查”的问题。

严格审查未发现剩余 P0/P1 结构性问题。

### 4.3 隔离真 TTY

使用开发二进制、复制后的普通目录通道、独立 workspace 和 fake HOME 驱动真实 tmux
PTY：

```text
aiah → 迁移到其他设备 → 选择复制后的通道 → 查看/选择 2026.07.1
→ 输入已有 incoming 目录 → 取回版本检查 → Enter → 变更预览
→ 输入 apply → 安装检查
```

源包与取回包 SHA256 均为
`4c0556b24564a618c6a838e5bc5c6c857131f108138cfa9e40e54dcd3c490261`。最终 TUI
显示：

```text
当前安装 是 · 备份 1 · 未变化 5 · 已修改 0 · 缺失 0 · 风险与问题 0
```

CLI Doctor 对账为 `ok=true`、deployment `2026.07.1`；真实 HOME 未参与。

## 5. 交付前剩余门禁

本地实现、变异、完整门禁、严格 review 和隔离 TTY 已完成。远端交付仍需：

1. 提交候选；
2. 推送并创建面向 `dev` 的 PR；
3. 等待最终 head 的 push 与 pull_request CI 全绿。

公开版 `v0.1.6` 不包含 E3.2–E3.4；Release 能力状态必须与开发候选分开报告。
