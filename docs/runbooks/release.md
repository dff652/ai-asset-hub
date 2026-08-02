# 发版 runbook

- 适用：发布 `aiah` 二进制本身。发布用户资产包走
  [真机 dry-run runbook](real-home-dry-run.md)，两条链路不要混
  （见 [development.md §0](../development.md)）。
- 版本口径：SemVer，tag 形如 `v0.1.0`。tag 里带 `v`，二进制里的版本号不带。

## 0. 触发方式

**push tag 即发布**。`.github/workflows/release.yml` 监听 `v*`，跑测试与 gofmt
门禁后调用 `scripts/release-build.sh`，把 Linux amd64 二进制、许可材料与
`SHA256SUMS` 附到 GitHub Release。其他目标只保留在 CI 交叉编译矩阵中，不进入
Release。

发布逻辑刻意放在脚本里而不是 workflow YAML 里：**YAML 没法本地跑，脚本可以**。
改发布流程时先在本地把脚本跑通，再动 workflow。

## 1. Pre-release 检查（本机，全绿才继续）

```bash
go test ./... && go test -race ./... && go vet ./...
./scripts/check-gofmt.sh              # 与 CI 同一份脚本
golangci-lint run ./...               # 与 CI 同版本 v1.62.2
./scripts/demo-apply-scan-loop.sh     # 闭环，不碰真实 $HOME
```

再加一条本仓库特有的：**真机只读复验**。这个工具的输出会指导用户改自己的 home，
发版前至少确认扫描没有回归。

```bash
./scripts/build.sh
./build/aiah scan --output json | python3 -c "
import json,sys,collections
r=json.load(sys.stdin)
print(r['producedBy'], r['summary']['candidateAssets'], '候选资产')
print(collections.Counter(f['code'] for f in r['findings']).most_common())"
```

与上次发版的数字对不上时，先查清楚是资产真变了还是分类规则回归了，再发。
如果换了设备，没有同机历史数字可比，按
[开发环境搭建 SOP §4](development-environment.md) 先建立该设备的只读基线；
不要拿另一台设备的绝对数量直接判回归。

### 本设备只读基线

| 日期 | 候选资产 | findings | 说明 |
|---|---|---|---|
| 2026-07-26 | 15 | 0 | TUI Phase A dogfood 时的记录 |
| 2026-08-01 | 29 | `symlinked_asset`×3、`suspected_secret`×1 | 见下 |

2026-08-01 的差值经**同机差分**确认不是分类回归：用改动前的树（`3d1078c`）与改动后
分别构建扫描，两者**逐字相同**（29 候选、同样 4 条 findings），且
`internal/inventory/` 自 technical preview 发布以来未被改动。

四条 findings 都是文档记录过的预期类别，不是新问题：

- `suspected_secret` on `home/.claude.json` —— 设备本地 native MCP config 必须含真值，
  [资产模型 §7](../asset-model.md) 明确规定 inventory 将其作为 `suspected_secret` 排除；
- `symlinked_asset` ×3 —— 同一个软链 skill 在 `.agents` / `.claude` / `.codex` 三个根下，
  正是 4b 补 warning 时要让它「不再静默消失」的那一组。

增长来自 codex（12）/ grok（9）/ claude（5）/ shared（3）的日常使用。下次发版拿
**2026-08-01 这一行**做比较基准，不要再用 15。

## 2. 本地预演发布产物

安装器默认版本必须始终指向一个实际存在且已验收的 Release，不能提前指向尚未发布
的 tag。因此发布新版本时先保留上一个已验收版本；新 Release 通过 §4 验收后，再用
独立 PR 同步 `scripts/install.sh`、README 和上手指南的默认版本。

这个 staged-pin 规则要求升级提示**另外显式传入目标版本**。只把安装脚本 URL 绑定
到新 tag 不够，因为该 tag 内脚本的默认 pin 仍可能指向上一版。`v0.1.5` 正式验收
已证明这会让推荐升级命令停留在旧版；后续版本必须同时满足：

- `upgradeCommand` 含精确 tag URL 和 `AIAH_VERSION=<目标版本>`；
- 安装器默认 pin 仍保持上一已验收版本，直到发布后门禁全部通过；
- §4 核对并执行程序实际给出的升级契约，而不是只测试人工补齐版本的命令。

运行安装器测试：

```bash
./scripts/test-install.sh
```

```bash
RELEASE_VERSION=0.1.11
VERSION="$RELEASE_VERSION" ./scripts/release-build.sh
./scripts/check-release-checksums.sh "$ROOT/dist/release"   # or pass OUT
ls -1 dist/release/
file "dist/release/aiah_${RELEASE_VERSION}_linux_amd64"
```

必须看到（平台清单以 `scripts/_release_platforms.sh` 为准，当前为）：

- 三个二进制：`aiah_<version>_linux_amd64`、
  `aiah_<version>_darwin_arm64`、`aiah_<version>_darwin_amd64`；
- **没有** `linux_arm64` / Windows 等未验收产物；
- `LICENSE`、`NOTICE`、`THIRD_PARTY_LICENSES.txt`、
  `THIRD_PARTY_DEPENDENCIES.md` 与 `SHA256SUMS`；
- `check-release-checksums.sh` 通过（校验**产物集形状**，不只是在场文件完好）；
- 本机若属于发布平台，脚本末尾 self check 打印正确版本号。

档位：`linux/amd64` 与 `darwin/arm64` 为完整支持（各 OS 真机全套）；
`darwin/amd64` 为交叉编译 + 冒烟。CI build-matrix 里未列入发布清单的目标只证明
**可构建**，不等于写入语义已验收
（[ADR-0003 §4](../decisions/0003-cli-first-go-core-and-product-surfaces.md)）。
任何新平台必须先补原生行为验收，再加入 `_release_platforms.sh`、安装器和
Release 输出。

## 3. 打 tag 并发布

先把 `dev` 通过 PR 提升到 `main`，并等 `main` 上同一 commit 的 CI 全绿，再创建
和推送 tag。tag push 会直接触发 Release，不能拿 Release job 代替主分支门禁。
Release 页面里的 `targetCommitish=main` 只是元数据，不能证明 tag 来自 main；
`v0.1.8` 曾在 main 仍停留于上一版时从 dev 打 tag，必须由 SHA 门禁阻止再次发生。

```bash
git fetch --prune origin
RELEASE_TAG=v0.1.7
RELEASE_COMMIT="$(git rev-parse origin/main)"
MAIN_RUN_ID="$(gh run list --branch main --workflow ci.yml --limit 1 \
  --json databaseId --jq '.[0].databaseId')"
gh run watch "$MAIN_RUN_ID" --exit-status
test "$(gh run view "$MAIN_RUN_ID" --json headSha,conclusion \
  --jq '.headSha + " " + .conclusion')" = "$RELEASE_COMMIT success"
git tag -a "$RELEASE_TAG" -m "aiah $RELEASE_TAG" "$RELEASE_COMMIT"
test "$(git rev-parse "${RELEASE_TAG}^{}")" = "$RELEASE_COMMIT"
git branch -r --contains "${RELEASE_TAG}^{}" | grep -q 'origin/main'
git push origin "$RELEASE_TAG"       # 这一步触发 Release
```

在 `v0.1.8` 之后的第一个修复版中，上一 tag 与 main 可能因历史 squash 仍显示
diverged；Release notes 不得直接把 GitHub 自动生成的 Full Changelog 当准确范围，
应以 tag 间文件树 diff 和已合入 PR 清单人工整理。连续两个 tag 都重新从 main 发布后，
再恢复祖先式 compare 链接。

## 4. 发布后验收

```bash
RELEASE_TAG=v0.1.11
RELEASE_VERSION="${RELEASE_TAG#v}"
gh release view "$RELEASE_TAG"
RELEASE_CHECK_DIR="$(mktemp -d)"
gh release download "$RELEASE_TAG" \
  -p 'aiah_*' -p 'LICENSE' -p 'NOTICE' \
  -p 'THIRD_PARTY_*' -p 'SHA256SUMS' -D "$RELEASE_CHECK_DIR"
cd "$RELEASE_CHECK_DIR"
# Prefer the repo verifier when available (shape + checksums):
#   ./scripts/check-release-checksums.sh "$RELEASE_CHECK_DIR"
sha256sum -c SHA256SUMS --ignore-missing
# Host platform binary (example: Linux amd64 CI host):
chmod +x "aiah_${RELEASE_VERSION}_linux_amd64"
"./aiah_${RELEASE_VERSION}_linux_amd64" version
# Expect three binaries named from scripts/_release_platforms.sh, not only linux.
```

`version` 输出的版本号必须与 tag 一致（去掉 `v`）。不一致说明 ldflags 注入或
workflow 的 `VERSION` 处理坏了，**先撤下 Release 再排查**。

随后按[安装、升级与 TUI dogfood SOP](install-upgrade-dogfood.md)用上一公开版本
真实升级到本版本，并增加升级提示契约门禁。用旧版已安装二进制取得 JSON，不执行
任意返回字符串；先逐字核对，再用相同固定参数在隔离目录执行：

```bash
OLD_AIAH=/path/to/previous/aiah
RELEASE_TAG=v0.1.11
RELEASE_VERSION="${RELEASE_TAG#v}"
UPGRADE_JSON="$("$OLD_AIAH" update --check --output json)"
UPGRADE_COMMAND="$(printf '%s' "$UPGRADE_JSON" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["upgradeCommand"])')"
EXPECTED_COMMAND="curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/${RELEASE_TAG}/scripts/install.sh | AIAH_VERSION=${RELEASE_VERSION} sh"
LEGACY_COMMAND="curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/${RELEASE_TAG}/scripts/install.sh | sh"
OLD_VERSION="$("$OLD_AIAH" version --output json |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["version"])')"
case "$OLD_VERSION" in
  0.1.4 | 0.1.5)
    test "$UPGRADE_COMMAND" = "$LEGACY_COMMAND"
    LEGACY_ROOT="$(mktemp -d)"
    mkdir -p "$LEGACY_ROOT/bin"
    install -m 0755 "$OLD_AIAH" "$LEGACY_ROOT/bin/aiah"
    curl -fsSL \
      "https://raw.githubusercontent.com/dff652/ai-asset-hub/${RELEASE_TAG}/scripts/install.sh" |
      env -u AIAH_VERSION AIAH_INSTALL_DIR="$LEGACY_ROOT/bin" sh
    test "$("$LEGACY_ROOT/bin/aiah" version --output json |
      python3 -c 'import json,sys; print(json.load(sys.stdin)["version"])')" = "$OLD_VERSION"
    ;;
  *)
    test "$UPGRADE_COMMAND" = "$EXPECTED_COMMAND"
    ;;
esac

UPGRADE_ROOT="$(mktemp -d)"
curl -fsSL \
  "https://raw.githubusercontent.com/dff652/ai-asset-hub/${RELEASE_TAG}/scripts/install.sh" |
  AIAH_VERSION="$RELEASE_VERSION" AIAH_INSTALL_DIR="$UPGRADE_ROOT/bin" sh
"$UPGRADE_ROOT/bin/aiah" version
```

必须再核对安装前后版本、commit、SHA256、mode、stage 残留和同版本复装。上一公开
版本不是 `v0.1.4` / `v0.1.5` 时，`UPGRADE_COMMAND` 与修复后模板不相等就必须阻断
发布；不得以手工命令成功替代该门禁。

上一公开版本是 `v0.1.4` / `v0.1.5` 时属于不可追溯修复的桥接例外：还要在第二个
隔离目录执行其 legacy 命令并确认仍停留旧版，把失败证据和显式版本 workaround
写入新 Release 说明。该 Release 只能写“显式版本升级通过，legacy 推荐命令仍失败”。
从首个包含修复的 Release 升级到再下一版时，例外结束，必须首次走严格相等门禁。

只有 Release 下载、升级提示契约、真实升级、TUI 和幂等复装全部通过，才另开 PR
把 `scripts/install.sh` 的默认 pin 与用户文档同步到新版本。

`v0.1.5` 是已知例外：产物、显式版本升级和 TUI 已通过，但上述命令相等性门禁失败；
Release 说明已公开显式 `AIAH_VERSION=0.1.5` 的 workaround。不要重写 tag 或产物，
修复已经通过 PR #20 进入 `v0.1.6`。该 bridge Release 已通过 main/Release CI、
线上产物复验、legacy no-op、显式版本升级、正式 TUI 和幂等复装；Release 说明也已
公开边界。不要把结果写成 `v0.1.5` 已发布二进制被追溯修复。`v0.1.6` 仍是一次性
bridge release；再下一版本才是修复后推荐命令的首次完整 Release → Release 证明。
完整证据见
[v0.1.6 bridge 检查点](../reviews/2026-07-30-v0.1.6-bridge-candidate-readiness.md)。

`v0.1.7` 已完成这条“再下一版本”门禁：`v0.1.6` 实际生成的命令与修复后模板
逐字相等，随后真实升级、版本/commit/SHA256/mode、无 stage 残留和幂等复装均
通过。正式 TUI 偏好、MCP 零写入和双设备业务闭环也已完成；证据见
[v0.1.7 发布与正式验收](../reviews/2026-08-01-v0.1.7-release-acceptance.md)。

## 5. 发布后把文件树同步回 `dev`

本仓库的 Release PR 会 squash 到 `main`，而 `dev` 也受线性历史保护：GitHub
不允许 merge-commit PR，分支保护也拒绝直接推送 merge commit。因此发布后的
`main` 不能通过普通 merge 变成 `dev` 的祖先；同步目标是**文件树相同**，不是伪造
祖先关系。

> ⚠️ **这个操作会删除 `dev` 独有的内容，这是它的性质而非意外。** 门禁是「文件树
> 与 `main` 完全一致」，因此发布切出 `main` 之后、同步之前落到 `dev` 的任何改动，
> 都会被这次同步抹掉。**先做下面第 0 步再动手。**
>
> 实例（2026-08-02）：`v0.1.10` 的同步提交 `bb6204d` 从 `docs/roadmap.md` 删掉
> 44 行，其中包括 PR #45 刚合入 `dev` 的「下一阶段方向」章节与待决策 D9，同时
> 回退了 `docs/decisions/0007-*.md` 与 `docs/README.md` 的编辑。新增文件本身没被
> 删，于是留下一份**没有任何文件引用的孤儿文档**——比整份删掉更难被发现。

安全流程：

0. **先列出 `dev` 独有的内容，并逐项决定去留**：

   ```bash
   git fetch --prune origin
   git diff --stat origin/main origin/dev      # dev 与 main 的全部差异
   git log --oneline origin/main..origin/dev   # main 里没有的提交
   ```

   对每一项明确回答「这次同步之后它应该还在吗」。答案是「应该在」的，**必须先
   带进 `main`**（或在同步 PR 里一并重新施加），不能指望它自己活下来。
   只有确认差异全部是「本就该被 main 覆盖」的内容，才继续第 1 步。

1. `git fetch --prune origin`，记录 `origin/dev`、`origin/main` 和 ahead/behind；
2. 从 `origin/dev` 建专用同步分支，并保留本地
   `backup/dev-before-<version>-sync-<date>`；
3. 只把已验收 `origin/main` 的文件树带入同步提交；冲突必须按文件清单审阅；
4. 提交前用 `git diff --quiet origin/main <sync-head>` 证明最终文件树完全一致，
   再运行 `./scripts/check-local.sh` 和 `git diff --check`；
5. 建 PR 到 `dev`，等待 push 与 pull_request 两轮 CI；
6. 按仓库允许的 squash 方式合入；再次 fetch，确认
   `git diff --quiet origin/dev origin/main`。

`v0.1.6` 的实例是 PR #23：尝试普通 merge 和非强制快进均被仓库策略明确拒绝，
远端未发生部分写入；最终 `dev@3b48566` 与 `main@307041e` 文件树完全一致。后续
报告必须如实写“tree 已同步但 main 不是 dev 祖先”，不能把 squash 说成历史合并。

## 6. 出问题怎么退

| 情况 | 处理 |
|---|---|
| Release 产物有问题，还没人下载 | 删 Release + 删 tag（`git push origin :refs/tags/vX.Y.Z`），修完重发同号 |
| 已经有人下载 | **不要复用版本号**。发 `vX.Y.Z+1`，并在旧 Release 说明里标注问题 |
| 只是发布说明写错 | 直接编辑 Release 说明，不动产物 |

## 7. 已知缺口

- **无签名**。目前只有 SHA256 校验和，能防传输损坏，不能防仓库被攻破。
  用户量起来后再评估 cosign / minisign。
- **无包格式兼容矩阵**。旧资产包被新 `aiah` 读到什么程度还没有定论；
  `pkgload` 用 `DisallowUnknownFields`，所以**给包内 manifest 加字段是破坏性变更**，
  必须连 `schemaVersion` 一起抬。这条同时是 ADR-0003「启动 UI 门槛」第 2 条的前置。
- **只分发 Linux amd64**。macOS、Windows 和 arm64 尚无原生端到端行为验收；
  `v0.1.1` 中这些平台的文件是历史交叉编译产物，不代表当前支持。
- **包管理器渠道未接入**。当前已有 Release 与可审查的 Linux amd64
  `install.sh`，尚未接包管理器。
