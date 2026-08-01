# 工程流程：开发 / 测试 / 构建 / 部署 / 发布

- 更新时间：2026-08-01
- 定位：长期有效的工程约束与流程现状。排期与**待决策清单 D1–D8** 看
  [MVP 路线图](roadmap.md)。

## 0. 先分清两条「部署」

这两条经常被混为一谈，判断任何「部署 SOP 是否就绪」之前先站队：

| | 部署**用户资产** | 发布**工具自身** |
|---|---|---|
| 做什么 | 把资产包装进 `~/.claude` / `~/.codex` / `~/.grok` | 把 `aiah` 二进制交付给用户 |
| 链路 | `build → diff/dry-run → apply → rollback` | tag → Linux amd64 二进制与许可材料 → 校验和 → Release |
| 状态 | **已固化，且 2026-07-25 真机验证** | public `v0.1.9` 已从受保护 main 发布；线上资产、严格升级、init 正反路径、真 TTY 与 MCP 回归通过，完整偏好/双设备闭环仍引用 v0.1.7，见 §5 |
| SOP | [真机 dry-run runbook](runbooks/real-home-dry-run.md)、[假 HOME 闭环](runbooks/fake-home-loop.md) | [发版 runbook](runbooks/release.md)、[安装/升级 dogfood](runbooks/install-upgrade-dogfood.md) |

## 1. 开发

- 根 `CLAUDE.md` 是 Claude Code / Codex 共用的项目说明单一事实源；Codex 依赖
  `project_doc_fallback_filenames = ["CLAUDE.md"]`。不要再生成同级 `AGENTS.md`。
- 分支：日常只在 `dev`，大里程碑再合 `main`。
- 新设备或切换设备先按
  [开发环境搭建 SOP](runbooks/development-environment.md) bootstrap，再运行只读
  `./scripts/dev-doctor.sh`；机器用户名、绝对 HOME 路径不属于项目约束。
- 决策进 ADR（`docs/decisions/`），一份 ADR 锁一个真有分歧的决定；评审记录是
  时点快照，修复后在原文标注结果，**不回改结论**。
- 文档分类看 [docs/README.md](README.md)；新文档先想清楚属于哪一类再落笔。
- Core 与 CLI 只用 Go（[ADR-0003](decisions/0003-cli-first-go-core-and-product-surfaces.md)）。
  界面不得复制业务规则：分类、路径安全、adapter 映射、备份、回滚只有一份实现。

## 2. 测试纪律

### 2.1 变异验证是硬要求

新增安全或行为测试后，**必须把被测防线删掉或把 bug 放回去，确认测试变红**。
这条不是洁癖，是本项目已经栽过两次的地方：

- `classify_test.go` 自己抄了一份分类逻辑、根本没调用生产代码，bug 原样存在也全绿；
- 第一版 tar 锚点的恶意成员不在 lock 里，被更外层的「`assets/` 成员必须在 lock 中」
  挡掉，把 typeflag / 成员数 / 成员大小三道防线逐个删掉，测试依然全绿。

推论：**恶意输入的测试必须内部自洽**（manifest/lock 与成员对齐），否则测的是
外层检查不是目标防线；**边界值测试该造真数据就造**（64MiB 成员那条伪造 header
只会被当截断包拒绝，什么也锚不住）。

### 2.2 本地必跑

```bash
./scripts/check-local.sh
```

该脚本先跑只读环境诊断及其 fake PATH 回归测试，再依次运行：

```bash
go test ./... && go test -race ./... && go vet ./...
./scripts/check-gofmt.sh            # 与 CI 同一份脚本
golangci-lint run ./...             # 与 CI 同版本 v1.62.2
./scripts/demo-apply-scan-loop.sh   # 闭环，不碰真实 $HOME
```

CI 跑的就是这几条里的同名脚本，不是另抄一份 inline 步骤——门禁必须能本地跑，
这也是 §2.3 的直接推论。

MCP surface 变化还必须：

1. 更新 ADR-0005 的工具清单和只读边界；
2. 让 `TestToolCallsWriteNothing` 覆盖新增工具可达的所有目录；
3. 临时放回写入或错误 Core 路由，确认安全/行为测试会变红后恢复；
4. 用实际 Claude Code、Codex、Grok 客户端做握手；模型级调用被账户策略阻止时，
   必须标为 blocked，不能拿 `Connected` 伪装为调用成功。

可重复命令和结果记录格式见
[MCP 客户端接入 runbook](runbooks/mcp-client-acceptance.md)。

### 2.3 不许把没本机验证过的东西推进 CI

`b274ce1` 是在「以为本机没有 go 工具链」的情况下直接 push 的，两个测试缺陷
（编译失败 + 假测试）一直到下一个会话才发现。后来又发现交接文档把上一台机器的
绝对安装路径当成了跨设备事实。现在两类问题都由环境 SOP + `dev-doctor.sh` 在本地
提前阻断。

## 3. CI 门禁（`.github/workflows/ci.yml`）

| 任务 | 内容 |
|---|---|
| `go` | `go test` / `go test -race` / `go vet` / `check-gofmt.sh` / `demo-apply-scan-loop.sh` |
| `lint` | golangci-lint v1.62.2；用当前仓库 Go 直接从源码安装并执行，不经过 Node.js action wrapper，避免上游预编译版的构建 Go 低于目标版本 |
| `install-linux` | Linux amd64 安装器的校验、幂等、原子替换与平台拒绝回归 |
| `build-matrix` | linux / darwin / windows × amd64 / arm64，只交叉编译 |

`build-matrix` **只证明可构建，不等于该平台语义已验证**
（[ADR-0003 §4](decisions/0003-cli-first-go-core-and-product-surfaces.md)）：
Windows 的 `chmod`、shebang、配置根语义都要单独的行为验收，不能用「编译通过」
代替。当前安装和 Release 只支持 Linux amd64。

## 4. 构建与版本

```bash
go build -o build/aiah ./cmd/aiah   # 版本 = dev，未发布二进制的诚实答案
./scripts/build.sh                  # 注入 git 版本 / commit / 时间
VERSION=0.1.8-dev.1 ./scripts/build.sh  # 未发布候选必须带开发标记
aiah version [--output json]
aiah update --check [--output text|json]  # 只读查 Release，不安装
```

版本戳规则只有一份：`scripts/_stamp.sh`，由 `build.sh` 与 `release-build.sh`
共同 source（发布构建额外传 `EXTRA_LDFLAGS="-s -w"`）。**不要在别处再算一遍**
VERSION/COMMIT/DATE——两份戳规则正是本地构建与发布构建开始分叉的方式。
闭环脚本也走 `build.sh`，所以 CI 里跑的是带版本戳的二进制。

**每个 JSON 报告和每条部署记录都带 `producedBy`**（形如
`aiah 0.1.0+d9dd3b263a6f`）。理由：这个工具会写别人的 home 目录，而 adapter 映射
和分类规则一直在演进，安装记录必须能回答「是哪个二进制按哪套规则装的」。

**包内 `manifest.json` 不带该字段**：`pkgload` 用 `DisallowUnknownFields` 解码，
加字段会让旧二进制读不了新包。那是格式变更，必须连 `schemaVersion` 一起抬。

## 5. 发布

**流水线已完成 public 实跑**：private `v0.1.0` 已于 2026-07-26 发布并验收；
干净 public history 没有复制该 tag。首个 public 版本 `v0.1.1` 已于
2026-07-28 从单提交干净历史发布并完成匿名验收。

```bash
VERSION=0.1.7 ./scripts/release-build.sh   # 本地预演：Linux amd64 + 许可材料 + 校验和 + 自检
git tag -a v0.1.7 -m "aiah v0.1.7" && git push origin v0.1.7   # 触发 Release
```

`.github/workflows/release.yml` 监听 `v*` tag，跑测试与 gofmt 门禁后调用同一个
`scripts/release-build.sh`。**发布逻辑放脚本不放 YAML**：YAML 没法本地跑，脚本
可以（§2.3）。完整流程、验收与回退见
[发版 runbook](runbooks/release.md)。

public `v0.1.1` Release workflow 全绿；当时下载线上六平台交叉编译二进制与许可材料后，
`SHA256SUMS` 所列文件全部通过，匿名 Linux amd64 自检为
`aiah 0.1.1, commit ce1ba00dc56d`，且 `doctor --help` 可用。这证明发布链路已经
端到端闭环，不改变「跨平台构建不等于跨平台语义验证」的边界。后续发布范围已
收口为完成原生验收的 Linux amd64。

public `v0.1.2` 于 2026-07-28 从 `main@942ab508dd18` 发布：Release workflow
全绿，只上传一个 Linux amd64 二进制及许可/校验材料；线上 SHA256、版本号、
commit 与 `doctor --help` 均通过验收。

public `v0.1.3` 于 2026-07-28 从 `main@ebb08dcd20ca` 发布：Release workflow
全绿，只上传一个 Linux amd64 二进制及许可/校验材料；重新下载后的 SHA256、
ELF x86-64 架构和 `aiah 0.1.3, commit ebb08dcd20ca` 均通过验收。该版本首次包含
TUI Phase D1 引导式本地闭环。

public `v0.1.4` 包含 TUI D2 Doctor/当前回滚与 D3 版本/显式更新检查。2026-07-29，
[PR #12](https://github.com/dff652/ai-asset-hub/pull/12) 已合入
`main@014e3268e305`；合并后的 main CI
[run 30437294951](https://github.com/dff652/ai-asset-hub/actions/runs/30437294951)
共 9 个 job 全绿，本地同树的 Linux amd64 release-build、SHA256 与版本自检也已
通过。annotated tag `v0.1.4` 精确指向该 commit，Release
[run 30440495126](https://github.com/dff652/ai-asset-hub/actions/runs/30440495126)
全绿；线上只发布一个 Linux amd64 二进制及许可/校验材料，重新下载后的 SHA256、
静态 ELF x86-64 架构、`aiah 0.1.4, commit 014e3268e305` 与显式更新检查均通过。

发布前已在隔离目录完成真实 `v0.1.2 → v0.1.3` 安装器升级，以及
`dev@2780840` 的候选替换、TUI Doctor/typed rollback、版本页和按键后 Release
检查 dogfood。发布后又完成真实 `v0.1.3 → v0.1.4` 安装器升级、同版本幂等复装、
TUI D2 rollback、D3 `current/latest 0.1.4` 与 CLI 对账；因此安装器默认 pin 已
更新为 `v0.1.4`。可重复步骤见
[安装、升级与 TUI dogfood SOP](runbooks/install-upgrade-dogfood.md)。

public `v0.1.5` 于 2026-07-30 从 `main@8ca70f0cde4b` 发布：
[main CI 30515168352](https://github.com/dff652/ai-asset-hub/actions/runs/30515168352)
与 [Release workflow 30515370094](https://github.com/dff652/ai-asset-hub/actions/runs/30515370094)
全绿；线上 Linux amd64 产物的 SHA256、静态 ELF x86-64 架构、版本、commit 和
许可材料均通过下载复验。在隔离目录显式设置 `AIAH_VERSION=0.1.5` 后，真实
`v0.1.4 → v0.1.5` 升级、同版本幂等复装和正式安装包 TUI E1/E2/E3.1 闭环均通过。

本次验收同时发现 P1：`v0.1.4` / `v0.1.5` 的 `aiah update --check` 生成命令没有
显式传入 `AIAH_VERSION`，而 `v0.1.5` tag 内安装器默认 pin 仍为 `v0.1.4`；执行
推荐命令会停留在旧版。Release 说明已公开 workaround，tag 与产物不重写。下一版本
必须把“执行程序实际输出的升级命令”加入发布门禁。后续 N2.1 已通过
[PR #20](https://github.com/dff652/ai-asset-hub/pull/20) 合入 `dev@e7d813ec00e3`：

- 让 `upgradeCommand` 同时绑定 tag URL 和 `AIAH_VERSION`；
- 增加精确字符串测试并完成删除防线会变红的变异验证；
- 同步 TUI 窄屏可复制命令的四行展示；
- 把 dev/候选 installer 默认 pin 更新为已验收的 `v0.1.5`；
- 把 README SVG 静态规范检查纳入 `check-local.sh`。

PR 的两组检查和合并后的
[dev CI 30518125897](https://github.com/dff652/ai-asset-hub/actions/runs/30518125897)
均全绿。该 tree 随 PR #21 进入 `main@46e6efccc9ba`；合并后
[main CI 30521118041](https://github.com/dff652/ai-asset-hub/actions/runs/30521118041)
的 9 个 job 全绿。annotated tag `v0.1.6` 精确指向该 commit，
[Release workflow 30521763546](https://github.com/dff652/ai-asset-hub/actions/runs/30521763546)
完成测试、vet、gofmt、构建和发布。

线上六项资产的 SHA256 全部通过；Linux amd64 二进制为 stripped 静态 ELF，
报告 `aiah 0.1.6, commit 46e6efccc9ba`，SHA256 为
`2535ed343e6e398f88456f9280a1f51717b3f2a7adb1ff6f3e23f789456aef70`。隔离
`v0.1.5 → v0.1.6` bridge 验收确认：legacy 命令为无残留 no-op，显式
`AIAH_VERSION=0.1.6` 升级成功，同版本复装幂等；正式安装包的裸 `aiah`、Doctor、
版本检查、typed rollback 与 CLI 对账均通过。Release 说明已公开 workaround。

由于 `v0.1.5` 自身不能追溯修复，`v0.1.6` 是一次性 bridge release；当时的
源码安装器默认 pin 已在发布后收口到验收完成的 `v0.1.6`。随后
`v0.1.6 → v0.1.7` 首次证明推荐命令端到端闭环。默认 pin 随后收口到 `v0.1.8`；
该版本修复安装器运行时加载校验函数的问题，同时新增 `aiah init` 并发布 N8.1。
发布后审计发现 init 受管目录边界与 tag 来源 main 两项缺口。`v0.1.9` 已从受保护
`main@3523d75` 发布，关闭两个缺口，并通过线上资产、严格 v0.1.8 → v0.1.9 升级、
幂等复装、init 正反路径、真 TTY 首页和 MCP 只读回归；当前默认 pin 因而收口到
`v0.1.9`。证据见
[v0.1.9 发布与正式验收](reviews/2026-08-01-v0.1.9-release-acceptance.md)、
[v0.1.8 发布后审计](reviews/2026-08-01-v0.1.8-post-release-audit.md)、
[v0.1.6 bridge 检查点](reviews/2026-07-30-v0.1.6-bridge-candidate-readiness.md)和
[v0.1.5 检查点 §5](reviews/2026-07-30-v0.1.5-candidate-readiness.md#5-发布结果与已知问题)。

发布后 [PR #22](https://github.com/dff652/ai-asset-hub/pull/22) 已把安装器默认 pin、
README 与发布证据收口到 `main@307041ec7c33`，最终提交的 push / pull_request
两轮 CI 共 18 个 job 全绿。随后 [PR #23](https://github.com/dff652/ai-asset-hub/pull/23)
把该文件树同步回 `dev@3b4856629cb5`；两端 `git diff --quiet` 为 0。仓库策略禁止
merge commit，普通 PR merge 和直接非强制快进均被服务器拒绝且没有远端部分写入，
所以 PR #23 按 squash 合入：**tree 相同，但 main 不是 dev 的祖先**。可重复流程见
[发版 runbook §5](runbooks/release.md#5-发布后把文件树同步回-dev)。

E3.2 已通过 [PR #24](https://github.com/dff652/ai-asset-hub/pull/24) 合入 `dev`，
实现 TUI typed publish、显式 versions/pull 和 pull→diff/typed apply 连续向导，
并修复 pull 输出覆盖缺口。E3.3 已通过
[PR #25](https://github.com/dff652/ai-asset-hub/pull/25) 合入 `dev`，增加当前
资产库/profile 的零写入换机检查。两次合并后 CI 均通过，但都不属于 `v0.1.6`
Release 能力。

E3.4 已通过 [PR #26](https://github.com/dff652/ai-asset-hub/pull/26) 合入
`dev@0a7171b`，把 pull 与 diff 之间补成“绑定 name/version/profile/SHA256 的
取回版本检查 → 用户 Enter → diff”，并补双设备、旧版本显式恢复、中断恢复和
恶意通道夹具；合并后主线 CI 9/9 全绿。该时点 E3.2–E3.4 尚未进入公开 Release，
后续发布结果见下段。

public `v0.1.7` 于 2026-07-31 从 `main@b6779193c3ac` 发布。main CI
[30603359981](https://github.com/dff652/ai-asset-hub/actions/runs/30603359981) 与
Release workflow
[30604175819](https://github.com/dff652/ai-asset-hub/actions/runs/30604175819) 均通过；
annotated tag 精确指向同一 commit。线上六项资产与 SHA256、静态 stripped ELF、
版本/commit 和许可材料通过。隔离 `v0.1.6 → v0.1.7` 首次证明旧版实际生成的
升级命令逐字等于带精确 tag 和 `AIAH_VERSION=0.1.7` 的模板；真实升级、`0755`、
无 stage 残留和同版本幂等复装全部通过。

正式包还完成裸 `aiah` PTY、三项偏好保存/重启/临时覆盖/损坏回退、7 工具 MCP
直接协议零写入，以及 build/apply/doctor/publish/pull/第二设备 apply/rollback
闭环。源包与取回包 SHA256 一致。完整证据见
[v0.1.7 发布与正式验收记录](reviews/2026-08-01-v0.1.7-release-acceptance.md)。
E3.2–E3.4、N6 和 N7 因此从“源码候选”转为 public `v0.1.7` 已验收能力。

首次发布时 GitHub 把 `actions/checkout@v4`、`actions/setup-go@v5` 与
`softprops/action-gh-release@v2` 的 Node.js 20 action 强制运行在 Node.js 24，
并发出弃用告警。dev 已把它们升级为明确声明 `node24` 的 v7 / v7 / v3。
`golangci-lint-action@v6` 同样仍用 Node.js 20，但其 v7+ 要求 golangci-lint v2，
所以不能为 runtime 告警顺带改变 linter 大版本；CI 改为用 Go 1.26.5 直接
`go install` 固定的 v1.62.2，与设备 bootstrap 同口径。上述迁移在推送前仍须跑完
本机门禁，不能把“只改 CI”当成跳过 §2.3 的理由。

迁移提交 `2bf0edd` 的 dev CI 8 个 job 全绿，annotations 总数为 0，Node.js 20
告警已消失。checkout、setup-go 与直接安装 lint 已在 CI 实跑；
`action-gh-release@v3` 也已随正常的 public `v0.1.1` Release 完成 runner 实跑。

仍缺：产物签名、包格式兼容矩阵（`DisallowUnknownFields` 让给包内 manifest 加字段
成为破坏性变更）、Homebrew/Scoop 等安装渠道。

分发顺序由 ADR-0003 定：GitHub Releases 多平台二进制 → Homebrew →
Scoop/winget → 可审查安装脚本 → `go install` 仅供开发者。**不加 npm launcher**。

## 6. 部署用户资产（已固化）

长期流程不因复审通过而放松：

```text
假 HOME 闭环 → aiah apply --dry-run 看 diff → 人工逐项确认 → apply → 记 backupId
```

- 含 MCP asset 的包已解除「仅 dry-run」限制
  （[2026-07-25 复审](reviews/2026-07-25-mcp-create-only-strict-review.md)），
  但上面四步一步不能省；
- `apply` 幂等：重复 apply 应 `written=0`；
- 任何一次真 apply 都会产生 `backupId`，回滚命令必须当场记下来；
- 首次真机执行已完成 apply、幂等复验与 rollback 验收；设备级明细不进入 public
  文档。
