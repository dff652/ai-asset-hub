# 设计与实现评审（2026-07-24）

- 评审对象：dev 分支 `a11ab0a`（Phase 2C.3.1 严格 review 前状态）
- 评审方式：设计文档通读（README / architecture / 4 份 ADR / security / roadmap）
  + 两轮代码审查（写路径 `apply`/`adapter`/`pkgload`；读路径
  `inventory`/`workspace`/`validate`/`build`/`content`/CLI/e2e）
  + 现成二进制实测闭环（build → apply → 二次 apply → rollback）
  + Go 1.26.5 复跑 test / race / vet
- 审批结论（时点快照）：**Phase 2C.3.1 通过严格复审**；后续状态见 §9。
- 总体结论：**设计「优」、实现「良」**。核心承诺（幂等、确定性构建、回滚、
  fail-closed）实测与代码走查均成立；未发现可利用的高置信度安全漏洞。
  短板集中在三处「文档承诺 vs 实现」落差和若干测试锚定缺口，均可修、
  不阻塞当前阶段。

## 1. 实测验证（全部通过）

| 承诺 | 验证方式 | 结果 |
|---|---|---|
| apply 幂等 | 同包二次 apply | `written=0, unchanged=5` |
| 构建确定性 | 两次独立 build 比对 tar sha256 | 逐字节一致 |
| rollback 完整 | rollback 后统计非 `.aiah` 落盘文件 | 归零 |

## 2. 设计评估

问题定义与边界划分是项目最强的部分：README 明确「不是什么」，ADR 各锁死
一个真有分歧的决策。

- **ADR-0002**（Capability + 可插拔 Target）含金量最高：能力矩阵 T0–T3
  把散落的工具名分支收敛到 Target 边界；四层边界表是可执行约束；诚实承认
  T2/T3 永远有缺口。
- **ADR-0004**（MCP create-only）体现自我纠错：2C.3 原型 review 发现整文件
  重编码 / Claude 路径错误 / 敏感值进 backup 三个真问题后，回退到 create-only
  最小方案 + 明确复审门槛，而非硬推。「先收窄所有权再谈合并」的判断正确。
- **ADR-0003**（CLI-first Go Core）论证扎实：复杂度在路径/权限/原子写入而非
  UI 生态，拒绝 TS 重写与 npm launcher 的理由成立。
- 安全模型把资产安装按「代码部署」对待（密钥只存引用、hook 只落盘不执行、
  备份不脱敏但强制私有权限），威胁建模认真。

设计侧无值得反对的决策。

## 3. 实现评估

两轮审查一致结论：**无高置信度安全漏洞**。以下防线走查确认严密且与文档
声明一致：

- 软链接逃逸：plan 与 commit 各校验一次收窄 TOCTOU 窗口
  （`internal/apply/path.go` securePath 逐段 Lstat）；rollback 对 backup.json
  路径注入有测试锚定。
- tar 解包：拒绝 `..`/绝对路径/软硬链接成员 + 512MB/64MB/10000 成员三重上限
  （`internal/pkgload/`）；未压缩格式无放大攻击面。
- secret fail-closed：MCP env 敏感 key 强制 `${ENV:}`/`${secret:}` 引用；
  hook 内容过 `content.ContainsSecret`。
- MCP native config create-only：已存在只读比对从不重写；identical 零 stage、
  二次 apply 零 write 零 backup；与 ADR-0004 验收门槛逐条对上。
- 扫描零写入、secret 只报排除原因不回显哈希、schema embed 与 spec/ 防漂移、
  profile 依赖展开 fail-loud——均有负向测试直接锚定。

## 4. 待修问题（按优先级）

1. **build「原子发布」承诺不成立** ✅ **已修复**
   （`internal/build/pack.go:88-127`）：4 个产物逐个 rename，中途被杀会留下
   tar 在而 manifest/lock/sha256 缺的半成品。
   
   **修复**（2026-07-24）：跨设备 fallback 改用 WriteFile→.tmp→rename（优于
   裸 WriteFile），最后写入 `.{archive}.complete` marker 文件；消费方检查
   marker 确认完整性。测试：`TestBuildPublishIncludesCompletionMarker`。
2. **cache 排除用 basename 子串匹配** ✅ **已修复**
   （`internal/inventory/classify.go:139-141`）：`strings.Contains(base, "cache")`
   会误排除名字含 cache 的合法资产（如 `cache-warmer` skill 目录）并跳过整树。
   
   **修复**（2026-07-24）：删掉子串匹配行，保留路径分段精确匹配。
   测试：`TestCacheExclusionDoesNotMisclassifyAssets`。
   
   **后续订正**（2026-07-25）：该测试未经编译且未调用生产代码（假测试），已重写为
   直接调用 `policyFor` 并做变异验证；同时发现**修法本身有副作用**——只留精确分段
   匹配后，真机 `~/.claude/paste-cache`、`plugins/*/transcript-cache`、
   `stats-cache.json`、`~/.codex/models_cache.json` 都不再被排除，transcript-cache
   还被判为可打包的 plugin 资产。现规则见 roadmap 第一层第 2 条。
3. **apply-journal 只写不读且固定路径**（`internal/apply/write.go:20-51`）：
   上次失败保留的取证 journal 会被下次 apply 静默覆盖，
   「留档」承诺打折。配套问题：mid-commit 恢复成功时把真实失败原因
   （如 `codePathEscape`）折叠成泛化 `codeApplyFailedRollback`
   （write.go:137-164），削弱安全事件可观测性。
4. **测试锚定缺口** ✅ **已修复**（2026-07-25）：tar 内 `../` 成员名、软/硬链接
   成员被拒、超限成员、安装目标叶子节点软链接——代码正确但无直接单测固化，
   重构易无声退化。
   
   **修复**：全部锚定并逐条变异验证（删掉防线必须变红），另补超量成员。
   过程见 [2026-07-25 复审](2026-07-25-mcp-create-only-strict-review.md) §4。
5. **可维护性**：`.claude`/`.codex`/`.grok` 字面量在
   probe/classify/install_path 三处各一份，与踩坑清单第 11 条「路径不写死在
   Core」有张力，宜随 ADR-0002 阶段 A 收口为 Target 注册表。路径安全校验则
   面对不同语义：workspace/apply 使用本机路径，pkgload 校验 POSIX tar 成员；
   不应仅为去重强行合成一个函数。

## 5. 产品层取舍提醒

secret 检测命中即整包 fail、无豁免机制（代码库无 allowlist/suppress）。
这是潜在误报体验问题，但目前没有 dogfood 数据证明它已阻塞产品；字面量
`api_key=sk-xxx` 也不会命中当前最小长度规则。先通过真实资产收集误报样本。
若确需豁免，应绑定文件、规则与内容哈希并留下审计 finding，不能使用可放行
整个资产任意内容的宽泛开关；MCP 敏感字段仍不得绕过 Secret Ref 约束。

## 6. 建议排期

时点结论为 Phase 2C.3.1 通过本次严格复审；后续状态见 §9。原排期建议是先修
第 1、2 项，再进入真实 dry-run/dogfood；
两项都比 2C.5 三路合并更便宜且直接兑现已有承诺。第 3 项与 doctor/status
一起设计，第 4 项可作为低成本回归加固，第 5 项随 ADR-0002 阶段 A 收口。

## 7. 评审问答补充（2026-07-24）

**Q：资产统一在项目目录下管理是否合理？**
资产工作区与 aiah 仓库、部署目标是逻辑角色，不要求物理目录永不重合：
aiah 仓库只是工具代码（`testdata/` 是 fixture）；个人全局与跨项目复用资产
建议放独立私有 Git 工作区；项目专属资产仍可随目标项目 Git 管理，再由 aiah
只读盘点或显式导入。`aiah init <directory>` 脚手架可降低首次使用成本，但
首版不需要隐式默认发现规则。

**Q：需要界面吗？CLI 够吗？**
当前阶段 CLI 够；ADR-0003（CLI-first 非 CLI-only）评审后维持成立：核心
链路用户是开发者本人、操作低频，JSON 输出同时服务人/脚本/CI/Agent；
UI 增量价值集中在 inventory 浏览、diff 可视化、findings 审阅（皆只读），
按 Phase 3.5 只读 Web UI 形态押后。补充触发信号：dogfood 时反复用 jq
手工过滤 scan/diff 输出，是读 UI 需求出现的初步信号；是否启动仍以 ADR-0003
列出的五项门槛同时满足为准。

## 8. 环境备注

评审机非交互 shell 的默认 `PATH` 未包含 Go，但工具链实际位于
`$HOME/.local/go1.26.5/bin/go`。补充 PATH 后，`go test ./...`、
`go test -race ./...`、`go vet ./...` 均通过。

**2026-07-25 订正**：本节是准确的，`b274ce1` 的提交说明和 handoff 里「机器无 go
工具链」是**误判**——工具链一直在。为免再次踩坑，已把 `go`/`gofmt` 软链进
`~/.local/bin`（该目录本就在 profile 的 PATH 里），现在任何 shell 直接 `go version`
即可，不需要临时改 PATH。

## 9. 后续状态（Phase 2C.3.2）

后续严格 code review 重新打开 MCP 契约门槛：写路径安全结论不变，但要求清除
历史 merge 双叙事，并钉死 create 后的持续所有权。项目选择一次性 bootstrap
（选项 C），不引入 managed 标记；finding、policy 边界与测试已在 2C.3.2 收口。
本报告的时点结论保留，当前审批状态以 ADR-0004 和 handoff 为准。

**2026-07-25 更新**：严格复审已完成，六条门槛全部通过，另发现 3 项 fail-closed
过宽（P6）待决策。见 [2026-07-25 复审报告](2026-07-25-mcp-create-only-strict-review.md)。
