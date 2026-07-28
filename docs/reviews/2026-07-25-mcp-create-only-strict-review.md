# Phase 2C.3.1 / 2C.3.2 严格复审：MCP create-only 契约（2026-07-25）

- 复审对象：dev `8b0bda8`（本次修复后的树；MCP 相关代码自 `f45bff0` 未变）
- 复审依据：[ADR-0004](../decisions/0004-native-mcp-config-ownership.md)
  「Phase 2C.3.1 验收门槛」6 条 + 2C.3.2 契约收口 7 项
- 复审方式：代码走查（`internal/apply/{mcp_policy,policy,plan,write,path}.go`、
  `internal/adapter/mcp*.go`）+ **真实二进制实证**（假 HOME/project 下逐条门槛跑，
  含边界构造）+ go 1.26.5 全量 test / race / vet + 假 HOME 闭环脚本
- **结论：6 条门槛全部通过**。新发现 3 个同类问题，均为「拒绝写入」方向，
  不构成安全风险，但把 create-only 从「跳过原生配置、继续装其它资产」变成了
  「整单装不了」，属产品行为问题，列为 P6 待决策。

## 1. 门槛逐条结论与证据

| # | 门槛 | 结论 | 证据 |
|---|---|---|---|
| 1 | 三端 global/project 路径有 fixture 和精确断言 | ✅ | `TestNativeMCPConfigPath` 覆盖全部 6 组合；policy 级 global 三端（`TestMCPPolicyCreatesMissingNativeFiles`）与 project 三端（`TestMCPPolicyUsesProjectNativePaths`）；apply 级 project 包（`TestProjectMCPPackageBootstrapAndConflict`）。路径表与 ADR §2 逐行一致 |
| 2 | 已有非规范 JSON/TOML 且语义相同 → 原字节不变、无 native 写、无 backup | ✅ | 实证：手写含 tab/空格/额外键的 `.claude.json`，apply 后 sha256 前后一致，findings 只有 `mcp_native_unchanged`，`backups/**` 下无 `.claude.json` |
| 3 | 已有配置需新增 server → 只 finding + sidecar，不改原文件 | ✅ | 实证：已有 `.codex/config.toml` 含用户注释与无关 server，apply 后字节不变，`mcp_native_skipped`（warning），sidecar `.codex/mcp/example.json` 正常落盘 |
| 4 | 含真实敏感值的已有配置不进 backup | ✅ | 实证：`grep -r existing-user-secret .aiah/` 无命中；已有单测 `phase2b_test.go` 断言 backup 路径不存在 |
| 5 | MCP/hook 走统一 policy loop；hook 不给非 hook 文件补 mode | ✅ | `applyStagePolicies` 顺序执行 + 统一 fail-closed；实证：apply 后仅两个 hook 文件带 +x，原生配置为 `600` |
| 6 | test / race / vet + 假 HOME 闭环 | ✅ | 本会话 go 1.26.5 全绿；`scripts/demo-apply-scan-loop.sh` 闭环通过（apply 5 → rollback 归零） |

补充实证（非门槛但属契约）：

- **一次性 bootstrap 语义成立**：创建后二次 apply `written=0 / unchanged=13`，
  findings 为 `mcp_native_unchanged`，不追加、不接管。
- **rollback 会清掉 bootstrap 出来的原生文件**：`removed=15`，`.claude.json` 消失。
- **同名冲突整单 fail-closed**：`.mcp.json` 已有同名不同 command 时 `ok=false`，
  sidecar 也不落盘，原文件字节不变。
- **device scope** MCP 模板不会静默漏过：policy 不处理它，但 `planInstall` 对
  sidecar 报 `device_scope_blocked` 错误，整单失败。

## 2. 新发现（P6，同一类：fail-closed 过宽）

三种「已有原生配置处于非典型状态」会让**整个 apply 失败**，而不是按 create-only
的本意跳过 MCP、保留 sidecar、继续安装其它资产。实证命令见 §5。

| 编号 | 触发条件 | 现象 | 判断 |
|---|---|---|---|
| P6-a | 已有配置把等价内容写成 `"args": []`（包内该字段为空被省略） | `mcp_native_failed` error，整单失败 | **假冲突**。`serverToMap` 省略空值，`jsonEquivalent` 逐字节比 JSON，空数组 ≠ 缺省。ADR §3 要 fail-closed 的是「同名不同内容」，这里语义相同 |
| P6-b | `~/.claude.json` 是软链（dotfiles / stow 管理很常见） | `mcp_native_failed` error，整单失败 | `readExistingMCPConfigs` 用 `Lstat` 判非常规文件即 `errUnsafePath`。**只读比对场景不必致命**：create-only 本就不会写它 |
| P6-c | `~/.claude.json` 存在但是 0 字节 / 非法 JSON | `mcp_native_failed` error，整单失败 | 同上。文件坏掉是用户侧状态，不该阻断技能、规则、hook 的安装 |

三者都**不是安全问题**（方向是拒绝写入，不会误改用户文件），但会让首次真机
dogfood 在完全无辜的环境状态上卡死，且 finding 文案（"unreadable or unsafe"）
无法自解释。

建议（需产品决策，因为 ADR-0004 已 Accepted）：

1. 把「已有原生配置无法解析 / 非常规文件类型」从 error 降为 `mcp_native_skipped`
   warning，保 sidecar，其它资产照装；**只保留「同名 server 真冲突」为 error**。
2. P6-a 单独修：比较前对 desired / current 做同一规范化（空数组、空对象、空字符串
   视同缺省），再比 JSON。这条即使不改 1，也应该修，否则等价配置被判冲突。
3. finding 文案区分三种情况（软链 / 解析失败 / 真冲突），给出「怎么办」。

未擅自修改：这三条会放宽 ADR-0004 的 fail-closed 边界，属决策而非缺陷修复。

## 3. 观测性小项（非阻塞）

- `summary.staged` 与 `create+update+unchanged+skipped` 不对账：`collapsePlans`
  会把「同路径同内容」的重复 staged 项静默去重（内容不同则报 `path_collision`
  错误，行为正确）。实测 staged=16 / changes=15。建议 summary 增加
  `collapsed` 字段或在 schema 注明，否则读报告的人会怀疑丢文件。
- `collapsePlans` 只比 `SHA256`/`Body`，不比 `Mode`：同路径同内容但 mode 不同时
  取先到者。当前无资产能触发，记录备查。

## 4. P3 / P4 / P5 评审结论

### P3：apply journal 只写不读 + mid-commit 恢复吞真实 failure code —— 确认成立

代码坐实两点：

1. `writeJournal`（`internal/apply/write.go:20`）固定写
   `.aiah/apply-journal.json`，且在**每次 apply 开头**执行。上一次失败时
   `recoverCommitted` 明确保留 journal 作为取证（finding 文案写着 "the apply
   journal was preserved"），但下一次 apply 会直接覆盖它——取证承诺形同虚设。
2. `recoverCommitted`（`write.go:157-163`）在恢复成功时，把 finding 一律改写为
   `apply_failed_rollback`，丢掉原始 code（如 `path_escape` 这种安全事件），
   只把原 paths 追加进去。

修复方向（与 `aiah doctor` 一起做，维持 roadmap 排期）：journal 按 backup ID
版本化（`apply-journal-<backupID>.json`），或开工前发现未结 journal 即 fail-loud
要求先跑 doctor；恢复时保留原始 finding 与 `apply_failed_rollback` **两条**，
不做折叠。

**修复状态（2026-07-26）**：已随 `aiah doctor` 收口。journal 改为按 backup ID
版本化，成功清理只删除本次 journal，不会覆盖其它失败取证；mid-commit 自动恢复
同时保留原始 finding 与 `apply_failed_rolled_back`。两条防线均做过变异验证。

### P4：测试锚定缺口 —— 本次已补齐并做变异验证

新增锚点：tar 内 `../`/绝对路径成员、软链接成员、硬链接成员、超限成员、
超量成员、安装目标**叶子**软链接。每条都用「删掉对应防线，测试必须变红」验证过。

过程中的教训值得记一笔：**第一版锚点全部是假的**——恶意成员不在 lock 里，
loader 用「assets/ 成员必须在 lock 中」这条更外层的检查挡掉了，于是把 typeflag、
成员数、成员大小三道防线逐个删掉，测试依然全绿。改成「恶意包内部自洽
（manifest/lock 与成员对齐）」后才真正锚住。超限成员那条必须真造 64MiB 才有效，
仅伪造 header 会被当成截断包拒绝、什么也锚不住（已用 `-short` 跳过）。

### P5：三处路径字面量 —— 维持暂缓，但补充一条现场证据

`.claude` / `.codex` / `.grok` 字面量目前散在 probe / classify / install_path，
维持随 ADR-0002 阶段 A 收口的判断。本次新增一条同类证据：cache 排除规则里
「哪些目录名算资产容器」（`skills`/`agents`/`rules`/`hooks`）也是硬编码，
阶段 A 做 Target 注册表时应一并纳入，否则每加一端就多一处漂移点。

## 5. 复现命令

```bash
export PATH="$HOME/.local/bin:$PATH"   # go 1.26.5 已软链到此
go test ./... && go test -race ./... && go vet ./...
./scripts/demo-apply-scan-loop.sh

# 门槛 2/3/4 与 P6 边界（假 HOME，不碰真实 $HOME）
aiah build --manifest testdata/workspace-2b/manifest.yaml --profile personal --out /tmp/d
# 已有非规范但语义相同的 .claude.json → 期望 mcp_native_unchanged、字节不变
# 已有缺 server 的 .codex/config.toml → 期望 mcp_native_skipped、sidecar 落盘
# 已有 "args": [] / 软链 / 0 字节的 .claude.json → 当前会 mcp_native_failed 整单失败（P6）
```

## 6. 审批

- Phase 2C.3.1 / 2C.3.2：**通过严格复审**。
- 含 MCP asset 的包解除「仅 dry-run」限制，但继续遵循 ADR-0004 的长期流程：
  先假 HOME 闭环 → 再 `--dry-run` 看 diff → 人工确认后 apply。
- P6 三项在真机 dogfood 前**建议先决策**：dogfood 环境如果命中 P6-b（`~/.claude.json`
  是软链）或 P6-c，第一次真机 apply 就会直接失败。
