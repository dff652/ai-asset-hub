# 2026-08-01 N10.1 迁移准备 Core/CLI 检查点

- 状态：本地源码候选通过完整门禁；尚未 commit、push、合并或发布
- 范围：共享只读 Core、JSON schema、CLI 与文档
- 不在范围：TUI、MCP、证据记录器、隔离恢复演练编排
- 设计：[N10：迁移准备检查](../designs/migration-readiness.md)

## 1. 已实现

- `internal/readiness` 复用 `build.Prepare` 与 `migration.InspectPreflight`，分别报告
  构建就绪和迁移前置条件；
- `selectionSHA256` 对 resolved `PackageManifest` 及所选文件 SHA256 做规范摘要，
  资产正文变化会使旧证据失配；
- 外部副本和恢复演练证据只从当前资产库 `.aiah/evidence/` 中显式读取；
- 证据未知字段、重复/多文档、疑似 Secret、特殊文件、越界路径和软链接 fail closed；
- 恢复演练同时绑定 name/version/profile、selection SHA256、package SHA256 和完整
  target 集合；`shared` 作为共享权威根的机器 target 保留；
- `aiah readiness` 提供 text/JSON 输出。缺证据是 `attention`，构建或迁移前置阻止
  是 `blocked` 并返回非零；
- `spec/migration-readiness.schema.json` 约束稳定报告字段和状态枚举。

## 2. 复审中修正的契约问题

### 2.1 构建就绪不等于 Secret 已可用

`build` 不解析目标设备 Secret。第一版设计把缺少 Secret 写进“不能打包”会改变既有
语义，现已拆成：

- `packageReadiness`：manifest/profile/依赖/冲突/所选内容；
- `migrationPreflight`：target、adapter dropped/degraded、Secret 和设备私有项。

因此缺少 Secret 可以与“可以打包”同时存在，但整体迁移准备为 `blocked`。

### 2.2 `shared` 不是第四个 AI 工具

真实 Core 会把共享权威根作为 `shared` target 返回。schema 与恢复证据必须保留该
机器值，后续 TUI 显示为“共享资产”，不能丢弃，也不能把它宣传成独立工具。

### 2.3 恢复证据必须匹配完整 target 集合

仅匹配资产摘要仍可能让“只演练 Claude”的记录覆盖 Claude/Codex/shared 选择。
候选实现现要求证据 targets 与当前迁移前置报告排序后完全一致。

## 3. 安全边界

- Core、CLI 均不创建 `.aiah/evidence/`；
- 相对证据名解析到 `<workspace>/.aiah/evidence/`，绝对路径也必须落在同一目录；
- 路径链任何组件是软链接、最终对象不是普通文件或文件超过 1 MiB 均拒绝；
- 资产选择无法建立 subject 时，已提供证据记为 `unchecked`，不会继续读取；
- 完整外部 reference 不进入报告，只返回 12 位摘要；
- HOME、project、资产库和证据目录在只读测试前后内容与 mode 不变；
- 不访问网络，不 build/publish/pull/apply/rollback，不扩展 MCP 权限。

## 4. 变异验证

六项生产/契约防线均被临时放宽或破坏，目标测试按预期失败，随后恢复：

| 变异 | 捕获结果 |
|---|---|
| 把证据边界从 `.aiah/evidence/` 放宽到整个资产库 | 资产库根下的合法证据被错误接受，边界测试失败 |
| 选择摘要忽略实际内容 | 修改所选规则文件后摘要不变，内容绑定测试失败 |
| 允许证据未知字段 | 带未知字段的完整合法证据被错误接受，严格解析测试失败 |
| 跳过恢复证据 target 集合对账 | 单 target 演练被错误标为 passed，绑定测试失败 |
| 允许 `--manifest` 越出资产库 | 外部 manifest 被错误读取，资产库边界测试失败 |
| 把报告 targets 写死为当前实现枚举 | 含未知 target 的阻止报告不再符合 schema，前向兼容测试失败 |

这些失败不是编译错误或由外层校验偶然阻止，分别命中目标业务防线。

## 5. 验证结果

已通过：

```bash
go test ./internal/readiness ./cmd/aiah
go test ./...
go vet ./...
./scripts/check-gofmt.sh
./scripts/check-local.sh
git diff --check
```

完整本地门禁包含 test、race、vet、gofmt、golangci-lint、安装器/README/许可检查和
假 HOME build→diff→apply→doctor→rollback 闭环，全部通过。真实 CLI 还在空临时
HOME 上运行 text/JSON 两种输出，调用前后目录保持为空。

## 6. 尚未完成

- 当前结果只是源码候选，不属于 public `v0.1.9`；
- 尚未经过提交后 CI、PR review、main 候选或正式安装包验收；
- N10.2 TUI“换机与备份”页面尚未实现；
- N10.3 `aiah_migration_readiness` MCP 工具尚未实现；
- N10.4 证据记录器与隔离恢复演练尚未实现，用户当前只能提供符合契约的私有证据
  文件，CLI 不替用户生成。
