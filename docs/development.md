# 工程流程：开发 / 测试 / 构建 / 部署 / 发布

- 更新时间：2026-07-29
- 定位：长期有效的工程约束与流程现状。排期与**待决策清单 D1–D8** 看
  [MVP 路线图](roadmap.md)。

## 0. 先分清两条「部署」

这两条经常被混为一谈，判断任何「部署 SOP 是否就绪」之前先站队：

| | 部署**用户资产** | 发布**工具自身** |
|---|---|---|
| 做什么 | 把资产包装进 `~/.claude` / `~/.codex` / `~/.grok` | 把 `aiah` 二进制交付给用户 |
| 链路 | `build → diff/dry-run → apply → rollback` | tag → 多平台二进制 → 校验和 → Release |
| 状态 | **已固化，且 2026-07-25 真机验证** | public `v0.1.3` 已发布并完成下载验收，见 §5 |
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
VERSION=0.1.4-dev.1 ./scripts/build.sh  # 未发布候选必须带开发标记
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
VERSION=0.1.4 ./scripts/release-build.sh   # 本地预演：Linux amd64 + 许可材料 + 校验和 + 自检
git tag -a v0.1.4 -m "aiah v0.1.4" && git push origin v0.1.4   # 触发 Release
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

`v0.1.4` 正在准备中：计划包含 TUI D2 Doctor/当前回滚与 D3 版本/显式更新检查。
2026-07-29 已在隔离目录完成真实 `v0.1.2 → v0.1.3` 安装器升级，以及
`dev@2780840` 的候选替换、TUI Doctor/typed rollback、版本页和按键后 Release
检查 dogfood。可重复步骤见
[安装、升级与 TUI dogfood SOP](runbooks/install-upgrade-dogfood.md)。

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
