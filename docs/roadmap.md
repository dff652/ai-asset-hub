# MVP 路线图

## 当前优先级（2026-07-24 评审后）

依据：[2026-07-24 设计与实现评审](reviews/2026-07-24-design-implementation-review.md)。
判断基准：当前最大风险不是代码质量，而是缺少真实使用闭环（本机 Claude 全局资产
未统一、跨设备包未开始，正是立项要解决的事）。

### 第一层：兑现已有承诺（当前迭代）

1. ✅ **已完成**（2026-07-25）：Phase 2C.3.1 / 2C.3.2 create-only 契约
   **通过严格复审**，6 条门槛逐条有实证；含 MCP asset 的包解除「仅 dry-run」
   限制。复审同时发现 3 项 fail-closed 过宽（P6，见下）。报告：
   reviews/2026-07-25-mcp-create-only-strict-review.md。
2. ✅ **已完成**（2026-07-25）：`b274ce1` 的测试缺陷已修复，全量
   test / race / vet 绿。
   - P1（build marker）：生产代码走查无误；测试里脆弱的扩展名再推导已改为
     跟随 `publishArtifacts` 命名。
   - P2（cache 排除）：假测试重写为直接调用 `policyFor`，并做变异验证。
   - **P2 的修法本身有副作用，已一并修正**：只留精确分段匹配后，真机
     `~/.claude/paste-cache`、`plugins/*/transcript-cache`、`stats-cache.json`、
     `~/.codex/models_cache.json` 全部不再被排除，其中 transcript-cache 还被
     判为可打包的 plugin 资产。现规则＝分段等于 cache/caches/.cache 或以
     `-cache`/`_cache` 结尾；带扩展名时只认状态类扩展名（避免误伤插件源码
     `context-cache.ts` 与 `references/CACHE.md`）；`skills`/`agents`/`rules`/
     `hooks` 的直接子项豁免，保住 `skills/go-cache` 这类资产名。
2b. ✅ **已完成**（2026-07-25）：开源协议定为 **Apache-2.0**，个人署名
   `Copyright 2026 ff.dou`。已落地 `LICENSE`（官方正文）、`NOTICE`、
   `docs/licenses/third-party.md`、README 徽章与章节、`security.md` §7 改写。
   依赖侧全部 MIT / Apache-2.0 / BSD，无 copyleft 传染。
3. ✅ **已完成**（2026-07-25，评审 P4）：tar 内 `../`/绝对路径成员、软/硬链接
   成员、超限成员、超量成员、安装目标叶子软链接均已锚定，每条都做了变异验证
   （删掉对应防线必须变红）。教训：第一版锚点因恶意成员不在 lock 中、被更外层
   的「assets/ 成员必须在 lock 里」挡掉，把 typeflag / 成员数 / 成员大小三道
   防线逐个删掉测试仍全绿——恶意包必须**内部自洽**才锚得住。
3b. **P6（本次复审新发现）：MCP create-only 的 fail-closed 过宽**，需决策后再改。
   已有原生配置写成 `"args": []`（语义等价却判成同名冲突）、是软链
   （dotfiles 管理常见）、或是 0 字节/非法 JSON 时，当前一律让**整单 apply 失败**，
   而不是跳过 MCP、保留 sidecar、继续安装其它资产。三者都不是安全问题。
   放宽方向与影响见复审报告 §2。
   
   **不是 dogfood 前置**（2026-07-25 实测本机）：`~/.claude.json` 是 70KB 普通
   文件、合法 JSON、无 `mcpServers` 键；两个 `config.toml` 也都是普通文件——
   三种触发条件都不命中。真正会遇到的是 create-only 的**固有代价**：原生配置
   已存在时平台不写它，只落 sidecar + `mcp_native_skipped` warning，MCP server
   要自己抄进去。第一个 dogfood 包建议先不含 MCP asset。
   P6-a（`"args": []` 假冲突）与环境无关，仍建议单独修。

### 第二层：产品可用

4. 第一次真机 dogfood：统一本机 Claude 全局资产到 `.agents` 共享根（T0 落地；
   设备盘点确认该层「部分完成」）。现有业务项目的项目级资产按既定策略不接管，
   只做只读盘点当兼容样本。
   
   **第一步（只读盘点）已完成**（2026-07-25）：候选资产零 finding，
   第一个包可以建；真机输出暴露 4 个分类缺口，其中 3 个当天已修。
   - ✅ **4a. 设备状态排除表按 target 收口**：原表只按 Claude 命名写，Codex 的
     `shell_snapshots`（下划线）/`.tmp`/`packages`/`attachments`/`ipc`、Claude 的
     `backups`/`ide`/`plugins` 运行态、Grok 的 `docs`、Codex 的 `skills/.system`
     全没覆盖。现改为按 harness 根声明的一张表（固定前缀 + 任意深度两类），
     P5 落地时整体搬到 Target 上。
   - ✅ **4b. 软链资产补 finding**：新增 `symlinked_asset` warning，同一软链
     skill 在三个根中不再静默消失。
   - 效果（同机复扫）：总条目 6438 → 151，included 5527 → 24，
     `suspected_secret` 329 → 2，候选资产 24 → 19（移出的 3 项全是
     `~/.codex/.tmp/plugins/*`，真实资产一个没少）。
   - 4c. `~/.grok/skills/*` 至少 3 个与 `bundled/skills/` 逐字节相同，应判定为
     `bundled-copy` 不进候选。**待做**（需读 `bundled/` 做内容比对）。
   - 4d. 拆 `~/.claude/CLAUDE.md`（通用 / Claude 专属 / 本机私有），人工活。
   
   **✅ 真机 apply 已完成**（2026-07-25，报告 §5.6）：工作区 `~/ai-assets/`，
   两次 apply 共写 14 个文件（3 个 skill 进 Codex 与 Grok），`update=0`，
   `.claude` 未动，幂等复验 `written=0`，两个 backup 可回滚。
   **首次真机 dogfood 跑通。**
   
   dogfood 暴露的产品问题（新，待排期）：
   - ✅ **P7 已修**（2026-07-25）：扩散只对 `portability: portable` 的 skill/rules
     生效，`adapter-required` 的资产（如 Codex 的 `default.rules` 命令审批 DSL）
     不再进 Grok；仍会扩散的部分记进 `CompileReport.Degraded`，不再静默。
     真机复验：`--targets codex,grok` 一次 apply 即得正确结果，
     **原先「拆成两次 apply」的绕开办法已不需要**。
   - ✅ **P8 已修**（2026-07-25）：产物名带 profile
     （`<name>-<version>-<profile>.tar`），从源头消除覆盖而非事后检测。
   - ✅ **P9 已改进**（2026-07-25）：仍然拒绝写入（不跟随软链是对的），但从
     误导性的 `path_escape` 改为独立的 `symlink_target`，文案直接告诉用户
     「把软链换成真实文件或目录再 apply」。**没有**加「自动替换软链」的 opt-in：
     那等于删用户自己建的东西，与「不删除未知文件」冲突，需要单独决策。
5. ✅ **`aiah doctor` 已完成（2026-07-26）**：消费 journal / 残留 stage /
   backup 完整性 / 部署漂移。
   同时处理评审 P3：journal 按 backup ID 版本化不互覆，mid-commit 恢复保留
   原始失败 code。

   **实现前复评：优先级上调，排在安装脚本之前。** 首次真机 apply 后机器上已经
   在产生 journal 与 backup，而当时没有检视手段。
   - **最有价值的一项是「部署漂移」**：用户 apply 后手改了
     `~/.claude/skills/<x>`，下次 apply 会发生什么？现在无法回答。lock 里有每个
     文件的 hash，比对即可给出 `unchanged` / `locally-modified` / `missing`
     三态。这是从「装得上」到「敢长期用」的那一步。
   - 顺带覆盖：残留 stage 目录、backupId 对应文件是否仍可回滚、hook 可执行位
     drift，**以及提前探测 D1 的三个 fail-closed 触发条件**（原生配置是软链 /
     0 字节 / `"args": []`）——让用户在整单 apply 失败**之前**就知道，比事后读
     `mcp_native_failed` 好得多。
   - **先只做 `doctor`，暂不拆 `aiah status`**：两者职责会重叠，等 doctor 输出
     稳定后再决定是否抽轻量摘要。
   - **命名风险**：`scripts/dev-doctor.sh` 查的是**开发者工具链**（Go /
     golangci-lint），`aiah doctor` 查的是**用户资产与部署状态**，两回事。
     文档必须写明，否则一定有人跑错。
   - **落地结果**：新 deployment 兼容保留旧 `files`，并新增 hash/mode
     `fileStates`；旧记录明确报 `drift_unavailable`，不伪装健康。未结 journal /
     损坏 backup / 非法 deployment 为 error；漂移与 MCP 前置风险为 warning。
     P3、hash、mode、backup payload、journal/stage、MCP 空 args 与软链边界均完成
     变异验证。真实 HOME 只读 dogfood：2 个 backup 完整、0 journal、0 stage；
     旧部署 7 个文件如实记为 `unchecked`，检查前后 `.aiah` 内容 hash/mode 不变。
6. `aiah init <directory>`：脚手架 `ai-assets/` 目录与 manifest 模板；首版
   继续显式传 `--manifest`，不加入隐式默认发现。
7. ✅ **`aiah bootstrap` 最小闭环已实现（2026-07-28）**：pull 前要求真实 TTY，
   取回后复用 TUI Phase C，必须完整输入 `apply`；取消不写 HOME、包保留在显式
   `--out`，成功持久展示 backup/rollback。边界固化为 ADR-0008。
8. dogfood 若证实 secret 规则存在高频误报，再设计绑定文件、规则与内容哈希的
   窄豁免；默认仍 fail-closed，MCP 敏感字段不得豁免。

### 第三层：Phase 3 跨设备（核心价值）

9. 分发与拉取最小闭环。
   ✅ **已实现（2026-07-28）**，语义固化为
   [ADR-0007](decisions/0007-immutable-channel-distribution.md)。
   - `aiah publish` / `aiah pull` / `aiah versions`；通道是**一个普通目录**，
     可以在 U 盘、挂载的 NAS/网盘或 git checkout 里。
   - **aiah 不做网络传输**：把字节搬过网络是 git / rsync / gh / U 盘的事，这正是
     architecture.md §4 早写下的分工。aiah 负责它们都不负责的部分——不可变性、
     布局、完整性校验。零新增依赖。
   - 布局 `packages/<name>/<version>/<profile>/`，键是三元组（profile 必须进
     路径，否则重蹈 P8 的 profile 互覆）；`channel.json` 追加顺序即发布顺序。
   - **不可变**：同坐标逐字节相同 → 幂等；内容不同 → 拒绝，不提供 `--force`。
     改内容就换版本号，这正是版本号的用途。
   - **两端校验**：发布前核对源包摘要且必须能被 `pkgload.Open` 读回（决不发布
     损坏的包）；拉取后核对落地文件，不一致就删掉并 fail-closed。发布走临时目录
     + rename，中途失败不留半个版本。
   - **不发明版本序**：`2026.07.1` 与 `2026.07.10` 的字典序是错的，manifest 也
     不保证 semver。省略 `--version` 解析为**最近发布**的那个并在报告里回报，
     不做版本号比较。
   - `pull` 不碰 HOME：取回与安装是两步，中间那步是人看 diff。
   - 10 项变异验证全部变红；真机跨「设备」闭环
     build → publish → pull → apply 走通。
10. ✅ **Secret Provider 已落地（2026-07-28）**：环境变量 + `pass`。

    **2026-07-28 核查：这不是「尚未开始的新功能」，是一个已上线功能里的正确性缺口。**
    修复前 `${ENV:...}` / `${secret:...}` **只被校验、从不被解析**——
    `internal/adapter/mcp.go::isSecretRef` 只确认它是引用而非明文，全仓库没有
    任何地方展开它。装完之后原生配置里躺着字面量 `${ENV:GITHUB_TOKEN}`，
    MCP server 拿到的就是这串字符本身，**含 MCP 资产的包跨设备之后不工作**。
    该路径一直没暴露，是因为首次 dogfood 明确建议「第一个包先不含 MCP asset」。
    - `${ENV:NAME}` / `${env:NAME}` 读取非空环境变量；`${secret:path}` 调
      `pass show -- path` 并只取首行。只解析 MCP `env` 中占满整个值的引用。
    - 解析发生在 apply 计划阶段，`diff` / `--dry-run` 走同一路径；无法解析时
      `mcp_native_failed`，整单零写入。
    - sidecar 保留引用，只有设备 native config 含解析值；报告、日志、journal、
      backup 元数据均不含真值。后续 scan 会把 native config 作为
      `suspected_secret` 排除，防止回流资产包。
    - 环境变量、`pass`、fail-closed、sidecar 不变和四类脱敏边界均有回归测试；
      变异验证见交接记录。
11. 发布工程（2026-07-25 起分步落地）：
    - ✅ **版本可追溯**：`internal/version` + `scripts/build.sh` 注入 ldflags；
      `aiah version`；四类报告与部署记录都带 `producedBy`。包内 manifest 暂不带
      （`DisallowUnknownFields` 会让旧二进制读不了新包，需抬 `schemaVersion`）。
    - ✅ **CI 补齐**：gofmt 门禁、golangci-lint、闭环脚本、跨平台构建矩阵
      （只证明可构建，不等于语义已验证，见 ADR-0003 §4）。
    - ✅ **发版流水线**：`scripts/release-build.sh`（Linux amd64 + 项目/第三方许可材料 +
      `SHA256SUMS` + 版本自检，本机已验证）+ `.github/workflows/release.yml`
      （监听 `v*` tag）。早期版本曾发布六平台交叉编译产物；当前仅分发完成原生
      端到端验收的 Linux amd64，其他目标保留在 CI 作为可构建性检查。
      发布逻辑放脚本不放 YAML，因为 YAML 没法本地跑。
    - ✅ **`docs/runbooks/release.md`**：pre-release checklist、本地预演、打 tag、
      发布后校验、回退表、已知缺口（无签名 / 无兼容矩阵 / 只做到 Releases 一步）。
    - ✅ **首次发版 `v0.1.0`（2026-07-26）**：tag 指向 `a95a1651785b`，
      Release workflow 全绿；线上六平台二进制、许可材料、SHA256 与 host 版本
      自检均通过。验收证据见 [development.md §5](development.md)。
    - ✅ **Actions Node.js 24 迁移（2026-07-26）**：首次 Release 报告 Node.js 20 action
      弃用告警。dev 已把 checkout / setup-go / action-gh-release 升到明确声明
      `node24` 的 v7 / v7 / v3；Node.js 20 的 golangci-lint-action 被相同 Go
      toolchain 的直接 `go install` 取代，以保留固定的 lint v1.62.2。YAML 解析、
      上游 runtime metadata、源码安装 lint 与完整本地门禁均已通过；迁移提交的
      dev CI 8 个 job 全绿且 annotations 为 0；Release-only 的 action-gh-release
      已随 public `v0.1.1` 正常 tag 实跑通过。
    - ✅ **Linux amd64 安装脚本 `scripts/install.sh`**（ADR-0003 §3 分发顺序
      第 4 项）。当前源码默认固定 `v0.1.6`；校验 Release `SHA256SUMS` 后才安装，同目录
      stage 后原子替换，不先删旧版本，不用 sudo、不改 profile，同版本零下载。
      校验复用 `scripts/_sha256.sh`；网络隔离测试覆盖校验失败保旧、重复 checksum、
      幂等、原子替换，以及 macOS/arm64 在下载前被拒绝。Windows/macOS/arm64
      安装入口须在对应平台完成原生验收后再提供。已发布 `v0.1.4` / `v0.1.5`
      二进制仍生成缺少 `AIAH_VERSION` 的命令；`v0.1.6` 已完成 bridge Release、
      legacy no-op 与显式版本升级验收，当前源码 pin 随发布后收口更新到 `v0.1.6`。
    - ⬜ 包格式兼容矩阵：旧包被新 aiah 读到什么程度（同时是 ADR-0003 UI 门槛
      第 2 条的前置）。

### 接口：MCP server（AI 工具接入）

15. **`aiah mcp`：把只读子集暴露为 MCP server**，兑现 ADR-0003 §1 承诺的
    「Agent 接口」。
    ✅ **已实现（2026-07-28）**，边界固化为
    [ADR-0005](decisions/0005-read-only-mcp-server-surface.md)。
    - `v0.1.6` 暴露 5 个工具：`aiah_scan` / `aiah_validate` / `aiah_diff` /
      `aiah_doctor` / `aiah_version`；当前源码候选已通过 N6 增加统一资产状态和迁移状态。
    - **不暴露 `apply` 与 `rollback`**：二者写 HOME，而 Claude Code 自身在读
      `~/.claude`——让 agent 改自己的运行时配置，harness 可能中途重载，行为
      不可预测。这是实际风险不是理论风险。
    - **`build` 也不暴露**（对形态评估 §5.4 的收紧）：它是唯一会写盘的候选，
      且写入目标由调用方指定，agent 可以把 `out` 指向 `~/.claude`。排除后
      「零写入」才是绝对而非条件的不变式。
    - `aiah_diff` 调 `apply.Diff` 而非 `apply.Apply{DryRun:true}`——只读入口
      不该与写入口只差一个布尔字段。
    - 零新增依赖：stdio 上的 JSON-RPC 手写实现，二进制体积基本不变。
    - **边界锚在行为不在名单**：`TestToolCallsWriteNothing` 对注册表每个工具在
      真实 home/project 上执行并比对前后快照；变异验证证明「加写工具 + 同时改
      名单」仍然变红。
    - 与已有的「MCP 模板作为**资产**」（ADR-0004）是两回事，两份 ADR 已互相
      标注区分。

#### MCP N6 覆盖（已合入 `dev`，不改变只读边界）

基础 5 工具覆盖盘点、校验、包级 diff、安装检查和版本识别。N6 已把晚于
ADR-0005 实现的两类只读 Core 接入 MCP：

- `aiah_asset_status`：资产库中的“未纳管 / 已纳管 / 源端有更新 / 仅库内 / 阻止”；
- `aiah_migration_status`：资产库、当前安装和可选分发通道的版本对齐结果。

ADR-0005 已修订为 7 工具；`TestToolCallsWriteNothing` 已扩大到 HOME、project、
资产库、通道和包目录。Codex、Grok 已完成模型级调用；Claude Code 客户端握手
Connected，但模型请求被组织策略在工具调用前以 403 阻止，必须在策略解除后补测，
不能伪报三客户端模型调用全部通过。

`build`、资产库纳入/更新/移出、`publish/pull`、`apply/rollback` 仍不进入 MCP：
AI 负责发现、解释和提出建议，创建文件或改变机器状态继续由用户在 TUI/CLI 审阅
确认。

### 界面：TUI（取代原 Phase 3.5 Web UI）

评估 [tui-surface-assessment.md](research/tui-surface-assessment.md)、技术方案
[tui-technical-design.md](designs/tui-technical-design.md)。结论：做 **本地 TUI**
而不是本地 Web UI（不引入 TypeScript、不开监听端口、新机器/SSH 上单二进制即可用），
定位是**工作流操作台不是控制面板**——本工具的「配置」就是 manifest 文件本身，
TUI 可以编辑那个文件，但不得引入私有**业务**设置存储。已接受的 N7 方案只允许
设备本地语言、首选资产库预填和显示密度三项 UI 偏好；N7.0 已修订 ADR-0006，
后续偏好存储仍必须通过独立安全门禁。

12. **TUI Phase A：只读浏览**（inventory 树 + findings 分诊 + `/` 过滤）。
    ✅ **已实现（2026-07-26）**：source → type → asset 树、详情、增量过滤、
    findings-only、异步重扫、常驻帮助；同进程只调用 `inventory.Scan`，非 TTY /
    空或 dumb TERM 在扫描前失败。只读快照测试、Update 表驱动、golden view 与
    逐项变异验证通过。release 构建体积实增约 `0.97 MiB`。
    ✅ **真实 TTY dogfood（2026-07-26）**：当前设备扫描稳定为 15 候选 /
    0 findings；树导航、折叠、详情、`/codex`、findings-only、异步重扫、帮助内
    `q` 退出均通过。额外用 `strace` 审计一次启动 + 重扫，写权限 `openat` 与
    文件系统变更调用均为 0；非 TTY 实测 exit 1 且提示改用 `scan`。
13. **TUI Phase B：manifest 组装**（勾选候选 → 写工作区）。
    ✅ **已实现（2026-07-28）**，边界固化为
    [ADR-0006](decisions/0006-tui-as-first-interactive-surface.md)（取代 ADR-0003 §5）。
    - `aiah ui --workspace PATH` 才开写能力；**不给就保持只读**、不显示复选框、
      `w` 明确拒绝。没有默认工作区路径——猜写入目标是本项目最不该做的事。
    - 勾选后**复制资产文件进工作区** + 写 `manifest.yaml`。只写 manifest 不搬
      文件会让紧随其后的 `validate` 必然报 path 不存在，那种形态自相矛盾。
    - 已有 manifest 走 `yaml.Node` 就地编辑：注释、键序、未知字段全保留；定位不到
      `assets` / `profiles` 结构时 fail-closed。
    - 工作区文件 **create-only**；校验不过则删临时 manifest 并回滚**本次创建的**
      文件与目录，原本就存在的内容不动。
    - 属性推导 fail-closed：secret / device-scope / 不可迁移类型一律跳过并报
      finding，不用默认值蒙混。
    - 10 项变异验证全部变红；真机 PTY dogfood 走通勾选→写出，扫描的 home 内容与
      mode 均未变。
14. **TUI Phase C：diff 审阅与执行**（把 runbook 的「人工逐项确认」搬进界面，
    执行后显著展示 `backupId`）。✅ **已实现（2026-07-28）**：显式
    `--package` 启动；同进程复用 `apply.Diff` / `apply.Apply`；changes 分组审阅；
    必须完整输入 `apply` 二次确认；成功显示 `backupId` 与完整回滚命令，失败原样
    展示 Core findings。10 项变异验证和真实 PTY dogfood 均通过。
15. **TUI Phase D1：引导式本地闭环**。✅ **已实现（2026-07-28）**：
    `aiah ui` 内按 `w` 明确输入并创建/打开工作区，compose 后按 `b` 选择 profile；
    复用 `build.Build` 输出到工作区 `dist/`，成功自动进入既有 Phase C diff/apply。
    没有隐式工作区或私有设置；受管工具目录禁入，workspace 变化/重建失败会使旧包
    失效；6 项变异验证与真实 PTY 到 diff 均通过；
    publish/pull 仍走 CLI。
16. **TUI Phase D2：Doctor 与当前部署回滚**。✅ **已实现（2026-07-28）**：
    普通 `aiah ui` 按 `h` 直调 Core Doctor；仅在 Doctor 通过且存在当前 deployment
    backup 时开放 `x`，必须完整输入 `rollback`。成功后刷新 Doctor 与 inventory。
    历史 backup 仍由 CLI 显式选择；bootstrap 不扩维护入口。2026-07-29 隔离 TTY
    dogfood 完成真实 deployment → Doctor → typed rollback → CLI 对账。
17. **TUI Phase D3：版本与只读 Release 检查**。✅ **已实现（2026-07-29）**：
    `v` 页面显示 aiah version/commit/build date 与当前资产 deployment 包版本；
    打开页面不联网，只有按 `c` 才复用 `aiah update --check` 的只读 Core。
    检查不下载、不自更新，发现新版本时给出绑定精确 tag 的安装命令。隔离 TTY
    dogfood 已确认首屏不联网、按键后查询与 dev 构建不可比较状态。`v0.1.5`
    验收发现生成命令没有显式绑定安装器目标版本，列为 N2.1 P0 修复。
18. **TUI Phase E：产品体验与导航 V2**。🚧 **E1/E2/E3.1 已实现，随
    `v0.1.5` 首次发布，并用 `v0.1.6` 正式包复验（2026-07-30）；E3.2–E3.4
    已合入 `dev`，E4 待实现**：
    - 定位为“AI 编程资产管理器”，资产库可包含知识型资产，但产品不是知识库；
    - 新增任务首页，把 inventory 降为“本机 AI 资产”子页面；
    - `aiah` 在交互 TTY 默认启动首页，`aiah ui` 保持兼容，非 TTY 仍拒绝进入；
    - 主术语改为“资产库、加入资产库、资产组合、变更预览、确认应用、安装检查、
      撤销上次安装”；
    - E2 增加“未纳管 / 已纳管 / 源端有更新 / 仅在资产库”统一状态，
      `w/u/X` 分别纳入、更新、移出；更新和移出经 Core 事务与 typed confirmation；
    - 纳入、更新或移出成功后连续进入资产组合、检查/组包和变更预览；
    - 应用成功页汇总目标工具、写入/不变/跳过数量、安装恢复点和建议下一步；
    - “资产库备份 / 安装恢复点 / 跨设备分发”边界已固化，不再把 publish/pull
      称为同步；
    - 2026-07-30 隔离 TTY 候选与正式安装包均走通纳入 → 连续 profile/diff →
      typed apply → 成功摘要 → Doctor → 源端更新检测 → typed update → typed
      remove；CLI 对账确认 manifest/profile 已移出、源端保留、安装记录仍健康；
    - E3 跨设备迁移与版本对齐 TUI、E4 设置/i18n 继续按
      [产品体验方案 V2](designs/tui-product-experience-v2.md)分期，未实现入口不展示。
    - ✅ **E3.1（2026-07-30）**：首页新增只读“迁移到其他设备”；同一 Core
      汇总资产库、Doctor 当前安装与 channel versions，明确显示相同/不同/未安装/
      未发布/未选通道。`c` 只读取用户输入的已有目录，不创建、不联网、不发布、
      不 pull、不 apply；版本不同不猜测新旧。隔离 TTY 候选与正式安装包均已验证
      未选通道和同版本通道状态，仓库 fixture 无写入。
    - ✅ **E3.2 已合入 dev（2026-07-30）**：`p` 选择资产组合、复用 build 并 typed
      `publish`；`v` 列出全部发布坐标，用户明确选择版本/profile 与已有输出目录后
      pull，成功即进入既有 diff/typed `apply`。不自动取回最后发布项，不创建通道
      目录，不接管网络传输；空目录只在 typed publish 后初始化。pull 同时加固为
      完整同内容普通文件四件套幂等，残缺/不同内容/符号链接拒绝且不覆盖。相关
      Core/TUI 单测、完整门禁、两项变异验证和隔离 TTY 闭环均已通过；PR #24
      已 squash 合入 `dev@f8b3475`，合并后 CI 全绿。
    - ✅ **E3.3 已合入 dev（2026-07-30）**：迁移页按 `e` 选择资产组合，零写入显示
      device-private 排除项、secret 在当前设备的可用性、目标支持、
      adapter dropped/degraded；可导航查看全部明细。检查复用 build profile、
      inventory、adapter 与 apply secret Core，不生成安装包、不创建 `dist/`，
      不发布、不取回、不自动 apply。PR #25 已 squash 合入 `dev@e3fa372`，合并后
      CI 全绿。
    - ✅ **E3.4 已合入 dev（2026-07-30）**：用户明确取回版本后，先按 pull 返回的
      name/version/profile/SHA256 检查确切发布包和目标设备；有阻止项不能进入
      diff，检查通过仍需 Enter、diff 和 typed `apply`。双设备夹具覆盖同包幂等、
      同坐标不同内容拒绝和显式 v1→v2→v1；恶意通道夹具新增索引越界与目录软链
      拒绝，中断恢复覆盖发布树领先索引及 apply journal/自动恢复既有门禁。
      PR #26 已 squash 合入 `dev@0a7171b`，合并后 CI 9/9 全绿。

启动前置：ADR-0003 五项门槛第 3 条（跨设备分发）**已满足**（第 9 项，2026-07-28）。
Phase A/B 均已实现，边界写在 **ADR-0006**（已取代 ADR-0003 §5）。新增依赖时仍须
同步 `NOTICE` 与第三方清单；Phase A/B 至今零新增依赖，见
[技术方案 §2](designs/tui-technical-design.md)。

### 已知缺口（不阻塞，别当成新发现重复上报）

- `spec/` 只有 inventory / validation / build / manifest 四份 schema；
  **doctor、compose、publish/pull/channel 的报告都没有**。该缺口在 doctor 落地时
  就存在，不是分发链路引入的。本仓库并没有「每个 report 都要 schema」的惯例，
  补不补是选择题。
- `channel.json` 无 JSON schema，但 Go 侧用 `DisallowUnknownFields`、
  `schemaVersion`/`kind`、标准 path/archive/SHA/坐标唯一性及无软链目录链校验，
  实际约束比一份 schema 文档更严。

### 明确暂缓
- MCP add-only merge：create-only 真机跑稳后再评估，ADR-0004 门槛不降。
- 新增 target（Cursor 等）：先做 ADR-0002 阶段 A（注册表声明化，顺带消掉
  三处硬编码目录名字面量，评审 P5），再谈加端。

## 当前资产管理完成度（2026-07-30）

结论：aiah 已形成“发现 → 资产库 → 校验/组包 → 预览/应用 → 检查/撤销 →
不可变分发”的资产生命周期 MVP；TUI E1/E2/E3.1、升级提示修复和 README/SVG
门禁均已进入 `v0.1.6` 并完成正式包验收。E3.2、E3.3 已合入 `dev`，但尚未作为
Release 能力验收；E3.4 发布包绑定与双设备/失败恢复验收也已合入 `dev`。N6 已
补齐 AI 接口的统一资产状态与迁移状态并通过合并后主线 CI；E4 设置/i18n
源码候选也已通过 PR #28 合入 `dev`，当前优先缺口是正式 Release 安装包复验。

| 层面 | 已完成 | 当前边界 |
|---|---|---|
| 公开版 `v0.1.6` | CLI/Core、任务首页、统一资产状态、连续应用、Doctor/rollback、E3.1、只读 MCP、不可变通道、secret provider、修复后的升级命令 | Linux amd64 线上产物、显式 bridge 升级、正式 TUI 与幂等复装通过；旧版用户仍需显式版本 workaround |
| 当前 `main@307041e` | `v0.1.6` 已发布 tree + PR #22 发布后 pin/docs 收口 | installer 默认 pin 为 v0.1.6；tag 内 staged pin 仍按发布契约保持 v0.1.5 |
| 当前 `dev@a0ff294` | E3.2 typed publish/显式 pull；E3.3/E3.4 换机和发布包绑定；N6 统一资产状态/迁移状态 MCP；N7 双语与三项本机 UI 偏好 | PR #24–#28 已 squash 合入；五次合并后主线 CI 全绿，尚未进入公开 Release |
| 人工操作入口 | TUI 覆盖日常本机流程；CLI 保留全部高级、脚本和 CI 能力 | 写操作继续要求显式路径、diff 和 typed confirmation |
| AI 接入入口 | `v0.1.6` 提供 5 个基础只读工具；当前源码候选已增加 `asset_status` 与 `migration_status` | 7 工具零写入；Codex/Grok 模型调用通过，Claude 握手通过但模型调用被组织策略 403 阻止；不开放写操作 |
| README 视觉 | README mode、规范化主入口和视觉门禁均已进入 v0.1.6 | 本发布后 PR 只同步徽章/证明板版本；一张主流程图表达首次成功，其它流程由任务表和详细文档覆盖 |

资产管理后续可增强“资产库备份就绪与恢复验证、搜索/标签/描述、来源与许可证追踪”，
但这些不是当前发布阻塞项。aiah 仍不实现网络传输、后台双向同步或云端账户体系。

## 待决策（阻塞在产品判断，不是缺代码）

这些都不是「没人做」，是**做哪一种要先定**。散落在各报告里容易丢，统一列在这里。

| # | 决策 | 现状与影响 | 出处 |
|---|---|---|---|
| D1 | MCP create-only 的 fail-closed 是否放宽 | 已有原生配置是软链 / 0 字节 / 写成 `"args": []` 时**整单 apply 失败**。放宽会移动 ADR-0004 §3 边界。其中 `"args": []` 属比较逻辑没规范化空值，**建议无论如何单独修** | [复审 §2](reviews/2026-07-25-mcp-create-only-strict-review.md) |
| D2 | `targets` 要不要**完全字面** | 现在是「`portable` 的 skill/rules 可扩散到 Grok 且记 `Degraded`」。若要一点都不扩散，是删 `shouldIncludeAsset` 一个分支的事，代价是共享技能要显式列全 target | P7 |
| D3 | apply 要不要支持**替换软链** | 现在遇到软链目标整单失败并提示手工替换。加 opt-in 等于删用户建的东西，与「不删除未知文件」冲突 | P9 |
| D4 | 包内 manifest 要不要带 `producedBy` | 要带就得抬 `schemaVersion`（`DisallowUnknownFields` 会让旧二进制读不了新包）。同时是包格式兼容矩阵的前置 | [development.md §4](development.md) |
| D5 | 首个 tag 的版本号与时机 | ✅ 2026-07-26 已拍板并发布 `v0.1.0`；线上产物验收通过 | [release runbook](runbooks/release.md) |
| D6 | `~/.grok/skills` 的 `bundled-copy` 判定要不要做 | 三个内置拷贝会被打进包，到新机器与自带版本重复。判定方法确定（与 `bundled/skills/<name>` 比内容），只是要不要花这个工 | 真机盘点结果 |
| D7 | `~/.claude/CLAUDE.md` 怎么拆 | 通用 / Claude 专属 / 本机私有三份，**人工判断**，拆完才能进包 | 同上 |
| D8 | **repo 何时转 public** | ✅ 2026-07-28 已完成：`github.com/dff652/ai-asset-hub` 使用单提交干净公开历史，private 原历史保留在 internal 档案；`v0.1.1`、匿名验收与远端治理均已完成 | [Public readiness 评估](reviews/2026-07-27-public-readiness-assessment.md) |

## 下一步（2026-07-30 `v0.1.6` 发布后）

| 顺序 | 事项 | 估时 | 为什么在这个位置 |
|---|---|---|---|
| 1 | ✅ **TUI Phase A 真机 TTY dogfood** | 已完成 | 未发现需修复的 TUI bug |
| 2 | ✅ **拍板 D8 + 仓库身份与历史公开边界** | 已完成 | 使用 `dff652`；采用干净公开历史，设备迁移台账不进入 public export |
| 3 | ✅ **`aiah doctor` + 评审 P3** | 已完成 | 真机只读 dogfood 与变异验证通过 |
| 4 | ✅ **`install.sh`（Linux amd64）** | 已完成 | SHA256、原子替换、无 sudo、幂等与平台拒绝均有回归测试 |
| 5 | ✅ **`aiah mcp`（只读子集）** | 已完成 | 首版 5 工具零依赖实现；N6 扩为 7 工具并扩大零写入树，边界固化为 ADR-0005 |
| 6 | ✅ **TUI Phase B** | 已完成 | ADR-0006 已写；真机 PTY dogfood 通过 |
| 7 | ✅ **跨设备分发闭环**（第 9 项） | 已完成 | ADR-0007；解除了 TUI Phase C 的唯一阻塞 |
| 8 | ✅ **Secret Provider**（第 10 项） | 已完成 | 环境变量 + `pass`；解析失败整单零写入，解析值不进报告/事务元数据 |
| 9 | ✅ **TUI Phase C** | 已完成 | 二次确认、backup/rollback、Core findings 原样展示；PTY dogfood 通过 |
| 10 | ✅ **`aiah bootstrap`**（第 7 项） | 已完成 | ADR-0008；pull 前 TTY 预检，复用 Phase C typed confirmation |
| 11 | ✅ **TUI Phase D1 引导式本地闭环** | 已完成 | 显式工作区→compose→profile→build→Phase C，降低首次使用门槛 |
| 12 | ✅ **TUI Phase D2 Doctor/当前回滚** | 已完成 | Doctor gate、typed rollback、历史 backup/ bootstrap 边界保持显式 |
| 13 | ✅ **TUI Phase D3 版本/更新检查** | 已完成 | 默认离线、按键联网、CLI/TUI 共用 Core、不做隐式自更新 |
| 14 | ✅ **安装/升级与 D2/D3 隔离 dogfood** | 已完成 | `v0.1.2→v0.1.3` 真实安装器升级、候选替换、rollback 与版本检查均通过；SOP 已固化 |
| 15 | ✅ **发布 `v0.1.4`** | 已完成 | tag/Release、线上 SHA/版本/架构、`v0.1.3→v0.1.4` 升级、TUI 与幂等复装均通过 |
| 16 | ✅ **TUI 产品体验 V2 E1/E2/E3.1** | `v0.1.6` 正式安装包复验完成 | 任务首页、统一资产状态、连续应用向导和跨设备只读状态已发布；升级提示 bridge 已收口，下一步进入 E3.2 |

### 后续任务计划（从 `v0.1.6` 发布后继续）

| 顺序 | 优先级 | 任务 | 验收出口 |
|---|---|---|---|
| N0 | P0 | ✅ **严格 review 当前 E1/E2/E3.1 + 文档/视觉改动** | 已修复“读取失败误报待更新”和首页缺少自动安装状态；变异验证、完整 `check-local`、真 TTY 与 900/360 视觉检查通过；无剩余 P0/P1；[复审记录](reviews/2026-07-30-e1-e2-e3-1-strict-review.md)列出建议提交拆分，未自行 commit |
| N1 | P0 | ⚠️ **`v0.1.5` 发布与正式安装包 dogfood** | main/Release CI、线上产物、显式版本升级和正式 TUI dogfood 已通过；`update --check` 实际推荐命令复现为 no-op，Release 已标注 Known issue；准确证据见[检查点 §5](reviews/2026-07-30-v0.1.5-candidate-readiness.md#5-发布结果与已知问题) |
| N2 | P0 | ✅ **README mode：整体结构、首屏与五步使用流程** | 已进入 `main@46e6efc`；“发现 → 整理 → 准备 → 预览 → 人工确认”已固化到上手指南和项目原生 SVG；900px/360px、链接、SVG 安全、无障碍和默认分支内容检查通过 |
| N2.1 | P0 | ✅ **修复升级提示命令并收口 installer pin** | 精确 `AIAH_VERSION`、TUI 可复制换行和变异验证已进入 v0.1.6；bridge 验收通过，本发布后 PR 把默认 pin 收口到 v0.1.6 |
| N2.2 | P1 | ✅ **固化 README/SVG 视觉验收** | 四图职责、视觉 token、语义核对和 900/360 SOP 已进入 `main` 与 `check-local.sh`；v0.1.6 版本证据已复验 |
| N2.3 | P0 | ✅ **准备并验收 `v0.1.6` bridge release** | main PR/CI、annotated tag、Release、线上产物、legacy no-op、显式版本升级、正式 TUI 和幂等复装全部通过 |
| N3 | P1 | ✅ **E3.2 跨设备连续向导** | PR #24 已合入 `dev@f8b3475`；build/typed publish/versions/explicit pull/Phase C、安全输出边界和合并后 CI 已完成，待后续 Release 验收 |
| N4 | P1 | ✅ **E3.3 换机前置检查** | PR #25 已合入 `dev@e3fa372`；device-private、secret、目标与 adapter 完整只读报告、零写入门禁、变异验证、TTY 和合并后 CI 已通过 |
| N5 | P1 | ✅ **E3.4 发布包绑定、双设备与失败恢复验收** | PR #26 已合入 `dev@0a7171b`；选定 name/version/profile/SHA 包级检查和连续引导、同版本幂等、不同内容拒绝、显式旧版本恢复、发布中断恢复、索引越界和目录软链均已验收；合并后主线 CI 9/9 全绿 |
| N6 | P1 | ✅ **MCP 只读状态补齐与客户端验收** | PR #27 已 squash 合入 `dev@9eedd7b`；最终候选 push/pull_request CI 18/18、合并后 CI 9/9 全绿；7 工具零写入、Core 复用、annotations、直接协议及 Codex/Grok 模型调用通过，Claude 模型请求被组织策略 403 阻止并如实保留为外部补测 |
| N7 | P2 | 🚧 **E4 设置与 i18n** | N7.0–N7.5 源码候选已通过 PR #28 合入 `dev@a0ff294`，合并后 CI 9/9 全绿：完整双语目录、安全偏好 Core、三项设置、100/60 列核心页与五类写入确认、fake HOME/config、release-style 真 PTY 均通过；只剩正式 Release 安装包复验及通过后更新用户文档 |
| N8 | P2 | **规模化资产管理增强评估** | 由真实使用量触发；评估备份就绪/恢复验证、搜索/标签/来源追踪，不提前引入服务端或数据库事实源 |

`v0.1.6` 已从 `main@46e6efccc9ba` 发布：main 与 Release CI、线上
SHA256/版本/架构/许可、显式 `v0.1.5 → v0.1.6` bridge 升级、legacy no-op、
正式 TUI 和幂等复装均通过；Release 说明已给出旧二进制 workaround。升级命令修复
已经进入 v0.1.6，本发布后 PR 将 installer 默认 pin 和用户文档收口到 v0.1.6。
完整证据、发布与升级步骤分别见
[v0.1.6 bridge 发布与验收检查点](reviews/2026-07-30-v0.1.6-bridge-candidate-readiness.md)、
[发版 runbook](runbooks/release.md)和
[安装/升级 dogfood SOP](runbooks/install-upgrade-dogfood.md)。

排序依据：**doctor 先于安装脚本**——安装脚本扩大用户面，doctor 让扩大后的用户
能自查；反过来会先收到一批「我这边不对」而无从下手。

工程维护项 **Actions Node.js 24 迁移**已完成；Release-only action 已随正常的
public `v0.1.1` tag 实跑通过。

D8、仓库身份、历史公开边界、发布收口、安装脚本和 TUI D1/D2/D3 均已完成。
当前产品主线是 TUI 产品体验 V2：E1/E2 与 E3.1 已随 `v0.1.6` 正式包复验；
升级提示命令、tag installer、staged pin、bridge Release 和发布后 pin 收口均已
完成。E3.2 跨设备发布/查看/取回编排已通过 PR #24 合入 `dev`；E3.3 换机前置
检查已通过 PR #25 合入 `dev`。E3.4 发布包绑定、双设备/失败恢复验收已通过
PR #26 合入 `dev`，N6 MCP 状态工具也已通过 PR #27 合入 `dev@9eedd7b`；
N7 设置/i18n 已通过 PR #28 合入 `dev@a0ff294`，各次合并后主线 CI 均 9/9
全绿。E4 设置/i18n 已完成 N7.0 决策收口，以及
N7.1 首页、inventory/资产库管理、diff/apply、doctor/rollback、migration 和
version 完整双语目录。N7.2 已实现独立 `internal/preferences` Core：配置路径、
locale 和当前偏好可注入，读取损坏文件安全回退，保存使用 `0700` / `0600` 与
同目录原子替换，首选资产库复用现有 workspace 安全边界。N7.3 已把它接入
TUI：首页有偏好入口和损坏配置警告，启动按
override/保存值/locale 解析语言，预览、取消、保存失败恢复、重置和显式保存边界均
有双语 golden、零写入测试和变异验证。N7.4 已加入 `standard` / `detailed`、
`--density` 和首选资产库编辑/提示/预填；密度只改变新 diff 的可选明细默认展开，
首选路径不创建、不自动选择，显式 `--workspace` 仍优先。必要信息/确认/阻止页
等价、三项安全变异、完整门禁和隔离真实 PTY 保存/重启/预填均通过。N7.5 已补齐
100/60 列动态单栏、核心页与五类写入确认、fake HOME/config 汇总测试，以及本地
Linux amd64 release-style 候选的 60 列真实 PTY 验收；必要路径、64 位 SHA、
包/目标/版本/备份/选中数量不再因窄屏或双栏空间不足而隐藏。正式 GitHub Release
安装包复验及其通过后的 README/上手指南发布声明仍待完成；public `v0.1.6`
不含这些本地候选改动。证据见
[N7.5 源码候选验收记录](reviews/2026-07-31-n7-release-candidate-acceptance.md)。
完整发布收口清单见
[2026-07-27 Public readiness 评估](reviews/2026-07-27-public-readiness-assessment.md)。

一句话链路：修 P1/P2 → 真机 dogfood ✅ → TUI Phase A ✅ / 发版闭环 ✅ →
TUI dogfood ✅ → doctor ✅ → MCP ✅ / TUI Phase B ✅ → 跨设备分发 ✅ →
Secret Provider ✅ → TUI Phase C ✅ → bootstrap ✅。
当前再向后是 TUI D1 引导式本地闭环 ✅ → TUI D2 Doctor/当前回滚 ✅ →
TUI D3 版本/只读更新检查 ✅ → 安装升级 dogfood ✅ → `v0.1.6` bridge Release、
线上产物、显式升级、legacy no-op 与推荐升级命令修复 ✅ → E3.2 PR #24 合入
`dev` 且合并后 CI ✅ → E3.3 PR #25 合入且主线 CI ✅ → E3.4 PR #26 合入且
主线 CI ✅ → N6 PR #27 合入且主线 CI ✅ → N7.0 决策收口 ✅ →
N7.1 typed 双语首页与 inventory/资产库管理目录及 golden ✅ →
diff/apply 与二次确认目录及 golden ✅ → doctor/rollback 目录及 golden ✅ →
migration 全流程目录及 golden ✅ → version 与 N7.1 完整目录出口 ✅ →
N7.2 偏好 Core、原子保存与安全变异验证 ✅ → N7.3 设置页、语言切换、
显式保存与源码候选 PTY 重启验收 ✅ → N7.4 密度、首选资产库只预填、
必要信息矩阵与源码候选 PTY 验收 ✅ → N7.5 100/60 列、fake config 与
release-style 候选验收 ✅ → 正式 Release 安装包复验及用户文档发布声明待实施。
**首次真机 dogfood 已完成，工具已从「工程演示」变为「自用工具」**（2026-07-25）；
private `v0.1.0` 已完成流水线验收（2026-07-26）；public `v0.1.1`–`v0.1.6`
已发布（2026-07-28 至 2026-07-30），其中 v0.1.5 的推荐升级命令限制已通过
v0.1.6 bridge Release 公开并完成 workaround 验收。

## Phase 0：资产盘点与契约

- 扫描当前设备的 Claude Code、Codex 与共享 `.agents` 资产。
- 分类为 Skills、Rules、Memory、Agents、Hooks、MCP 和设备私有状态。
- 定义 manifest v1 和目录规范。
- 建立测试夹具，避免使用真实个人密钥和客户数据。
- 多端架构方向已冻结为 ADR-0002（Capability + 可插拔 Target）；实现按该
  ADR 阶段 A→F 推进，Phase 0 不要求已支持 Grok 全量盘点。

验收标准：

- 能生成只读资产清单；
- 能明确报告未知、冲突和敏感文件；
- 不修改任何现有工具目录。

## Phase 1：CLI 与构建

### Phase 1A：只读 validate（已实现）

- `aiah validate --manifest <path> [--root <path>] --output json`；
- 校验 manifest v1 schema、重复资产 ID 和依赖/冲突/profile 引用；
- 拒绝绝对路径、`..` 越界和软链接逃逸；
- 检测疑似密钥、二进制及超大文件；
- 输出确定性、脱敏的验证报告（`spec/validation.schema.json`）；
- 测试使用 `testdata/workspace-valid` 与临时目录。

Phase 1A 不写源目录，不生成资产包，也不执行脚本、hook 或 MCP。

### Phase 1B：确定性 build（已实现）

- `aiah build --manifest <path> --profile <name> --out <dir> [--root <path>]`；
- 构建前复用 validate fail-fast；
- Profile include/exclude + 依赖展开（排除的依赖视为错误）；
- 生成确定性 `.tar`、旁路 `.manifest.json` / `.lock.json` / `.sha256`；
- 报告含 targets 能力摘要（`capabilities`）；
- 疑似密钥、二进制、软链接、校验失败时不写产物。

验收标准：

- `validate` 对相同输入生成一致报告；
- 无效引用、越界路径和疑似密钥默认失败；
- 相同输入可重复生成逐字节一致的 tar 与 archiveSha256；
- 资产包可用 `tar -tf` 解压审计；
- 构建结果与报告不包含真实凭据。

## Phase 2：多 Target 部署（先 Claude/Codex，再 Grok 子集）

### Phase 2A：skill/rules 部署（已实现）

- Adapter 接口 + Claude / Codex / shared（`.agents`）实现；
- `aiah diff` / `aiah apply` / `aiah rollback`；
- 包完整性校验（lock sha256）；
- staging 写入 + 覆盖前备份 + 部署记录；
- 临时 HOME 端到端与幂等/回滚测试。

映射范围：`assets/skills/**`、`assets/rules/**` → 工具目录与共享 skill 根。

### Phase 2B：hooks/agents/mcp + grok 子集 + project（已实现）

- 扩展映射：`agent` / `hook` / `mcp`；
- Grok adapter 子集（`.grok` skills/rules/hooks/agents/mcp 文件落盘）；
- `apply --project` 安装 `scope: project` 资产；
- `assets/rules/CLAUDE.md` → 项目根 `CLAUDE.md`；`AGENTS.md` 对称；
- MCP 模板先以 sidecar 文件落盘；原生配置一次性 bootstrap 见 2C.3.2；
- manifest `targets` 改为可扩展 id 模式（支持 grok 等）。

### Phase 2B.1/2B.2：部署安全收口（已实现）

- 同路径不同内容、device scope、缺失 install root 均 fail closed；
- 目标路径规范化、根内判断和既有祖先软链接检查；
- 唯一备份 ID、apply journal、中途失败恢复及恢复失败留档；
- rollback 预检 backup ID、元数据、源文件和目标路径；
- target 必须是请求、包声明和内置支持的交集，Shared 只部署已选 target；
- tar/目录包限制成员类型、数量和大小，并验证路径、重复项与 manifest/lock/hash；
- GitHub CI 固定执行 test、race 和 vet。

### Phase 2C.1：只读 Grok Inventory Probe（已实现）

- 扫描路径收敛为最小 `scanProbes` 表（home/project 分流）；
- `SourceGrok` + home/project `.grok` 盘点；
- 排除 auth、sessions、logs、downloads、bundled、marketplace-cache 等设备状态；
- 识别 Grok skills / rules / agents / hooks / mcp / config；
- 共享 `.agents` skill 保持 `source=shared`，与 Grok skill 分开计数；
- 脱敏与软链测试；不改 apply、不做配置合并。

### Phase 2C.2：apply → scan 闭环（已实现）

- `internal/e2e`：build → apply → scan → rollback → scan；
- 覆盖 workspace-valid 与 workspace-2b（含 project、grok）；
- 断言 source/type、skill 分源计数、diff 不写盘、回滚后候选归零；
- `scripts/demo-apply-scan-loop.sh` + [runbook](runbooks/fake-home-loop.md) 假 HOME 手工/脚本演示。

### Phase 2C.3：MCP 原生配置合并（历史原型，已废弃）

- 该原型曾计划把 `assets/mcp/*.json` 合入原生配置，目标路径为：
  - Claude user：`~/.claude.json`；project：项目根 `.mcp.json`
  - Codex user：`~/.codex/config.toml`；project：`.codex/config.toml`
  - Grok user：`~/.grok/config.toml`；project：`.grok/config.toml`
- 原型会整文件重编码，并错误地把 Claude MCP 写入 `.claude/settings.json`；
  已由 ADR-0004 与 2C.3.1/2C.3.2 完全取代，现行代码不再写回已有原生配置。

### Phase 2C.4：hooks 权限与生命周期提示（已实现）

- apply 阶段对 `*/hooks/**` 做内容策略：拒绝空/二进制/原始密钥；
- 脚本 hook（`.sh` 等）必须含 `#!` shebang，安装为 `0755`；
- JSON/YAML/TOML hook 配置安装为 `0644`，并提示工具侧仍可能需 trust/注册；
- 文件名推断生命周期 hint（PreToolUse / SessionStart 等，仅信息，非事件映射）；
- 内容相同但权限漂移会触发 update 以修复 +x；
- CLI：`aiah apply --dry-run`；真机流程见 [real-home-dry-run.md](runbooks/real-home-dry-run.md)。

### Phase 2C.4.1：权限、rollback 与 MCP 冲突收口（已实现）

- 备份记录原文件 mode；原生配置默认使用受限权限，rollback 恢复原 mode；
- 避免 `rename` 成功、`chmod` 失败时当前文件遗漏自动恢复；
- MCP 已有同名 server：内容相同幂等，内容不同 fail-closed；
- MCP 容器字段类型错误 fail-closed，敏感 env key 只允许 Secret Ref；
- 补 `0600 → apply → rollback`、同名冲突、Grok MCP 与 CLI dry-run 不写盘测试；
- `go test -race ./...`、`go vet ./...` 与两套假 HOME 闭环通过。

### Phase 2C.3.1：MCP native config 安全修复（已实现，后续要求契约收口）

- 已按 ADR-0004 修正 Claude/Codex/Grok global/project 路径；
- sidecar 继续由 aiah 管理；native config 不存在才创建，已存在默认只报告不修改；
- identical MCP 语义零 stage；二次 apply 零 write、零 backup；
- 含敏感值的已有 native config 在默认流程中不进入 backup；
- MCP/hook 已改由轻量 stage policy loop 编排，hook 不修改非 hook mode；
- 已补非规范 JSON/TOML、TOML 注释、敏感配置、scope 与零写入测试。

后续严格 review 要求进一步钉死一次性 bootstrap 语义、finding 命名和 policy
边界，已进入 2C.3.2。

### Phase 2C.3.2：create-only 契约收口（已实现，待 review）

- native 缺失时只做一次性 bootstrap；创建完成后永不自动更新；
- existing identical 零写入，缺 server warning + sidecar，同名冲突整单失败；
- finding 统一为 `mcp_native_*`，用户可见文案不再称 merge；
- MCP policy 保持全部原始 staged 顺序；缺根由 plan 统一报告；
- 补一次性 bootstrap 升级、project scope 包级和 conflict 整单失败测试。

严格复审通过前，含 MCP asset 的包不得在真实 HOME/project 非 dry-run apply。
决策见 [ADR-0004](decisions/0004-native-mcp-config-ownership.md)。

### Phase 2C.5+（未实现）

- 项目规则智能三路合并/diff UI；
- hooks 跨 harness 事件模型完整映射。

验收标准（2A+2B）：

- 不依赖开发机绝对路径；
- 可从单一 tar 包向假 HOME/project 恢复 skill/rules/agent/hook/mcp 文件；
- 重复 apply 幂等；
- 回滚后恢复或删除 apply 写入的文件；
- 不支持/降级能力在 compile 报告中 `dropped` / `degraded`。

## Phase 3：跨设备

- 本地目录和移动介质。
- Git 或 Release 下载。
- WebDAV/rclone 传输。
- 设备 Profile 和 Secret Provider。
- GitHub Releases 当前只发布已原生验收的 Linux amd64；
- 其他平台补原生验收后恢复分发，再评估对应包管理器；
- CI 保留多平台交叉编译，但不把它作为运行时支持证据。

首版不做双向实时同步，只发布和拉取不可变版本。

## Phase 3.5：本地 TUI（原 Web UI 方案已取代）

Phase A/B/C/D1 已由本地 TUI 提供；普通用户运行 `aiah`，`aiah ui` 保留兼容和
高级直达参数：

- Inventory 搜索、过滤和预览；
- findings 与安全告警分诊。
- 显式工作区内的 manifest 组装；
- profile 选择、构建到工作区 `dist/` 并自动进入 diff；
- 部署 diff 分组审阅、二次确认 apply、backup/rollback 展示。

不给 `--workspace` / `--package` 时初始仍保持只读；按 `w` 明确输入并确认工作区
后才开启组装。所有写操作复用 Go Core。不再规划 localhost Web UI。

## Phase 4：受控 UI 与编辑器扩展

- manifest/Profile 编辑；
- apply 前确认、安装目标与健康状态；
- rollback 与 `backupId` 展示；
- 第三方 Skill 来源更新。
- 按真实需求评估 VS Code 扩展。

UI 使用 Go Core 的稳定接口，不复制部署逻辑。TypeScript 只用于 UI/扩展，不迁移
Core。云同步、账户和团队协作继续独立评估。

## 暂不纳入

- 云端多人协同编辑；
- RBAC 和审批流；
- Prompt 在线模型评测；
- 原生会话历史跨工具转换；
- 自动同步真实密钥；
- 自建云盘协议；
- PromptHub 数据库兼容层；
- 厂商 bundled / marketplace 缓存镜像；
- 未完成 ADR-0002 阶段 A/B 前宣称「Grok 全量迁移」。
