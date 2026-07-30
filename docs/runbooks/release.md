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
RELEASE_VERSION=0.1.5
VERSION="$RELEASE_VERSION" ./scripts/release-build.sh
./scripts/check-release-checksums.sh
file "dist/release/aiah_${RELEASE_VERSION}_linux_amd64"
```

必须看到：

- 一个 `aiah_<version>_linux_amd64` 二进制，且没有其他平台二进制；
- `LICENSE`、`NOTICE`、`THIRD_PARTY_LICENSES.txt`、
  `THIRD_PARTY_DEPENDENCIES.md` 与 `SHA256SUMS`；
- `sha256sum -c` 全部成功；
- 脚本末尾的 self check 打印出正确版本号（脚本自己会断言，不匹配直接退出 1）。

CI 的跨平台目标只证明**可构建**，不代表在那些平台上验证过行为
（[ADR-0003 §4](../decisions/0003-cli-first-go-core-and-product-surfaces.md)）。
任何新平台必须先补原生行为验收，再加入安装器和 Release 输出。

## 3. 打 tag 并发布

先把 `dev` 通过 PR 提升到 `main`，并等 `main` 上同一 commit 的 CI 全绿，再创建
和推送 tag。tag push 会直接触发 Release，不能拿 Release job 代替主分支门禁。

```bash
gh run list --branch main --limit 1
gh run watch <run-id> --exit-status
RELEASE_TAG=v0.1.5
git tag -a "$RELEASE_TAG" -m "aiah $RELEASE_TAG"
git rev-parse "${RELEASE_TAG}^{}"    # 必须等于刚通过 CI 的 main commit
git push origin "$RELEASE_TAG"       # 这一步触发 Release
```

## 4. 发布后验收

```bash
RELEASE_TAG=v0.1.5
RELEASE_VERSION="${RELEASE_TAG#v}"
gh release view "$RELEASE_TAG"
# 下载并校验（换成实际平台）
RELEASE_CHECK_DIR="$(mktemp -d)"
gh release download "$RELEASE_TAG" \
  -p 'aiah_*_linux_amd64' -p 'LICENSE' -p 'NOTICE' \
  -p 'THIRD_PARTY_*' -p 'SHA256SUMS' -D "$RELEASE_CHECK_DIR"
cd "$RELEASE_CHECK_DIR"
sha256sum -c SHA256SUMS --ignore-missing
chmod +x "aiah_${RELEASE_VERSION}_linux_amd64"
"./aiah_${RELEASE_VERSION}_linux_amd64" version
```

`version` 输出的版本号必须与 tag 一致（去掉 `v`）。不一致说明 ldflags 注入或
workflow 的 `VERSION` 处理坏了，**先撤下 Release 再排查**。

随后按[安装、升级与 TUI dogfood SOP](install-upgrade-dogfood.md)用上一公开版本
真实升级到本版本，并增加升级提示契约门禁。用旧版已安装二进制取得 JSON，不执行
任意返回字符串；先逐字核对，再用相同固定参数在隔离目录执行：

```bash
OLD_AIAH=/path/to/previous/aiah
RELEASE_TAG=v0.1.5
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
当前源码已修复命令生成并把 `main` 默认 pin 收口到 v0.1.5；这些改动仍须通过
PR/CI，并在下一版本用旧公开二进制重新执行本节门禁。不要把本地测试通过写成
`v0.1.5` 已发布二进制被追溯修复。下一版本是一次性 bridge release；再下一版本
才是修复后推荐命令的首次完整 Release → Release 证明。

## 5. 出问题怎么退

| 情况 | 处理 |
|---|---|
| Release 产物有问题，还没人下载 | 删 Release + 删 tag（`git push origin :refs/tags/vX.Y.Z`），修完重发同号 |
| 已经有人下载 | **不要复用版本号**。发 `vX.Y.Z+1`，并在旧 Release 说明里标注问题 |
| 只是发布说明写错 | 直接编辑 Release 说明，不动产物 |

## 6. 已知缺口

- **无签名**。目前只有 SHA256 校验和，能防传输损坏，不能防仓库被攻破。
  用户量起来后再评估 cosign / minisign。
- **无包格式兼容矩阵**。旧资产包被新 `aiah` 读到什么程度还没有定论；
  `pkgload` 用 `DisallowUnknownFields`，所以**给包内 manifest 加字段是破坏性变更**，
  必须连 `schemaVersion` 一起抬。这条同时是 ADR-0003「启动 UI 门槛」第 2 条的前置。
- **只分发 Linux amd64**。macOS、Windows 和 arm64 尚无原生端到端行为验收；
  `v0.1.1` 中这些平台的文件是历史交叉编译产物，不代表当前支持。
- **包管理器渠道未接入**。当前已有 Release 与可审查的 Linux amd64
  `install.sh`，尚未接包管理器。
