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

先把 `scripts/install.sh` 和 README 的默认安装版本同步为
本次版本；安装器默认版本必须指向一个实际存在的 Release，不能提前指向尚未发布的
tag。运行安装器测试：

```bash
./scripts/test-install.sh
```

```bash
VERSION=0.1.1 ./scripts/release-build.sh
./scripts/check-release-checksums.sh
cd dist/release && file aiah_0.1.1_* | cut -c1-80
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

先推 `dev` 并等同一 commit 的 CI 全绿，再创建和推送 tag。tag push 会直接触发
Release，不能拿 Release job 代替 dev 门禁。

```bash
git push origin dev
gh run list --branch dev --limit 1
gh run watch <run-id> --exit-status
git tag -a v0.1.1 -m "aiah v0.1.1"
git rev-parse v0.1.1^{}      # 必须等于刚通过 CI 的 dev commit
git push origin v0.1.1        # 这一步触发 Release
```

## 4. 发布后验收

```bash
gh release view v0.1.1
# 下载并校验（换成实际平台）
gh release download v0.1.1 \
  -p 'aiah_*_linux_amd64' -p 'LICENSE' -p 'NOTICE' \
  -p 'THIRD_PARTY_*' -p 'SHA256SUMS' -D /tmp/rel-check
cd /tmp/rel-check && sha256sum -c SHA256SUMS --ignore-missing
chmod +x aiah_*_linux_amd64 && ./aiah_*_linux_amd64 version
```

`version` 输出的版本号必须与 tag 一致（去掉 `v`）。不一致说明 ldflags 注入或
workflow 的 `VERSION` 处理坏了，**先撤下 Release 再排查**。

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
