# AI Asset Hub — 项目协作说明

本文件是本仓库 Claude Code 与 Codex 共用的项目说明，也是项目级 AI 指令的单一
事实源。Codex 通过用户配置
`project_doc_fallback_filenames = ["CLAUDE.md"]` 读取它；不要再生成同级
`AGENTS.md`，否则原生文件可能遮蔽 fallback，形成两份会漂移的规则。

## 项目定位

AI Asset Hub（`aiah`）是文件优先的 AI 编程资产与配置工具：在 Claude Code、Codex、
Grok 等目标之间盘点、校验、构建和**安全应用** Skills、Rules、Memory、Agents、
Hooks 与 MCP 模板。对外说法优先是「安全地改 AI 工具配置 / 多工具语义统一」，
不是通用 dotfile 同步器，也不是「跨设备搬运」单点产品。

核心原则：

- 纯文本资产是事实源；数据库或索引只能是可删除、可重建的派生物。
- 网盘、NAS、Git 或移动介质只负责传输不可变资产包；aiah 不实现网络传输。
- Core 与 CLI 使用 Go；TUI 只调用同一 Core，不复制业务规则。
- 密钥只在目标设备 apply 阶段解析，不进入包、报告、journal 或 backup metadata。
- 写入前必须能 diff，写入后必须能通过 `backupId` 回滚。

架构与产品边界见 `docs/architecture.md`、`docs/asset-model.md`、
`docs/decisions/`；产品取舍见
`docs/research/product-positioning-and-direction.md`。

## 开始工作

继续已有开发任务时先读：

1. `docs/development.md`：开发、测试、构建、发布硬约束。
2. `docs/roadmap.md`：当前状态、下一步与待决策项。
3. `docs/README.md`：按任务找到相关 ADR、设计或 runbook。

先执行只读检查：

```bash
git status --short
git fetch --prune origin
git log --oneline origin/dev -5      # dev 常有并行改动，别默认本地是最新
./scripts/dev-doctor.sh
```

`docs/roadmap.md` 的「下一阶段方向」是当前排期，「待决策」表是需要所有者拍板、
不得代为决定的事项。

开发可能跨设备进行，不要把用户名、绝对 HOME 或上一台机器的安装状态写成项目事实。

## 开发边界

- **`dev` 是受保护分支，不能直接 push**：要求 8 项状态检查且开启
  `required_linear_history`。所有改动走「功能分支 → PR → CI 全绿 → 合入 `dev`」；
  大里程碑才合 `main`。
- **合并只能用 rebase 或 squash**。线性历史保护会拒绝 merge commit，尽管仓库级
  设置允许它、历史上也存在 2-parent 的旧合并。改动包含多个实质不同的提交时用
  rebase 保留分离（安全修复应当在历史里独立可追溯）。
- **并行开发很密，PR 被 `dev` 追上是常态**：合并前若 `mergeStateStatus=BEHIND`，
  先 rebase 到最新 `origin/dev` 再推。写状态类文档时不要复制别处的可变状态
  （子任务进度、版本号），指向权威位置，否则 PR 还没合就已过时。
- push、tag、Release、仓库 visibility 等远端动作由所有者明确授权后执行；
  拿到针对具体操作的授权后可以执行，但 CI 未绿不打 tag。
- 尊重脏工作区，不回滚用户改动，不使用 destructive git 命令。
- 分类、路径安全、adapter 映射、备份和回滚逻辑只放在 `internal/*` Core。
- 项目专属 `CLAUDE.md` / `AGENTS.md` 由对应项目 Git 管理；aiah 默认只读盘点，
  不替项目静默初始化、改名或覆盖项目说明文件。
- 新增依赖必须同步 `NOTICE` 与 `docs/licenses/third-party.md`；不得直接引入
  GPL、AGPL 或 LGPL 依赖。
- 不做服务端或 Docker 运行时形态，不给只读 MCP surface 暴露 build、apply 或
  rollback。

## 测试与交付

修改 Go 代码后先运行最小相关测试，提交前必须运行：

```bash
./scripts/check-local.sh
```

新增安全或行为测试必须做变异验证：临时删掉被测防线或放回 bug，确认测试会变红，
然后恢复生产实现。恶意输入夹具必须内部自洽，避免只命中更外层检查。

不允许把没有在本机验证过的改动推进 CI。跨平台编译只证明可以构建，不等于 Windows
等平台的写入语义已经验收。

提交信息使用 Conventional Commit 风格，例如：

```text
feat(tui): review deployment diff
fix(apply): reject unsafe target path
docs(rules): define project instruction source
```

提交后报告验证结果、提交号、工作树状态和领先远端的提交数；没有授权不要 push。

## 已知流程陷阱

**发布后的 `main → dev` 文件树同步会删除 `dev` 独有的内容**，这是它的性质不是
意外：门禁就是两棵树完全一致，因此 `main` 切出之后才落到 `dev` 的改动会被抹掉。
新增文件可能因此变成没有任何引用的孤儿文档，比整份删掉更难发现。
同步前必须先跑 `docs/runbooks/release.md` §5 的第 0 步逐项决定去留。
实例：`v0.1.10` 的同步删掉了刚合入的路线章节与一条待决策。
