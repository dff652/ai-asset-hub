# Public 发布 runbook

- 适用：把当前 private 开发仓库整理为干净历史的 public 仓库；
- 已决策：canonical module path 使用 `github.com/dff652/ai-asset-hub`；
- 已决策：设备迁移台账、会话交接和原 Git 作者信息不进入 public history；
- 保留动作：push、仓库改名/创建、tag、Release、分支保护和 visibility 均由所有者
  执行。

本 runbook 不改写现有 private 仓库历史。它保留原仓库作为内部档案，再从通过门禁的
单个提交导出干净工作树，建立新的 public history。这样无需 force-push，也不会让
旧 tag、作者邮箱或已删除文档随 visibility 一起公开。

## 1. 准备并冻结候选提交

在 private 开发仓库：

```bash
git status --short --branch
git rev-list --count origin/dev..dev
./scripts/check-local.sh
git rev-parse HEAD
```

必须满足：

- 工作树干净；
- `go list -m` 输出 `github.com/dff652/ai-asset-hub`；
- `rg 'github\.com/ilabel/ai-asset-hub'` 无结果；
- 完整本地门禁全绿；
- 候选 commit 已由所有者 push 到 private `dev`，且同一 commit 的远端 CI 全绿。

## 2. 导出 public 工作树

`.gitattributes` 用 `export-ignore` 排除 `docs/handoffs` 和 `docs/migrations`。
`git archive` 只导出已提交的 `HEAD`，因此必须先完成第 1 节。

```bash
PUBLIC_TREE="$(mktemp -d)"
git archive --format=tar HEAD | tar -xf - -C "$PUBLIC_TREE"

test ! -e "$PUBLIC_TREE/docs/handoffs"
test ! -e "$PUBLIC_TREE/docs/migrations"
test -f "$PUBLIC_TREE/LICENSE"
test -f "$PUBLIC_TREE/NOTICE"
test -f "$PUBLIC_TREE/SECURITY.md"
test -f "$PUBLIC_TREE/THIRD_PARTY_LICENSES.txt"
```

在导出目录复验：

```bash
cd "$PUBLIC_TREE"
test "$(go list -m)" = "github.com/dff652/ai-asset-hub"
! rg 'github\.com/ilabel/ai-asset-hub'
! rg '/home/[[:alnum:]_.-]+/' README.md docs
go test ./...
go vet ./...
./scripts/check-gofmt.sh
VERSION=0.1.1 ./scripts/release-build.sh
./scripts/check-release-checksums.sh
```

若任一命令失败，回 private `dev` 修复并重新导出；不要直接修改临时导出目录后发布，
否则 private 事实源与 public 内容会分叉。

## 3. 建立干净 Git 历史

在导出目录建立单提交历史。提交身份使用 GitHub noreply 地址，不使用工作邮箱：

```bash
cd "$PUBLIC_TREE"
git init -b main
git config user.name dff652
git config user.email 100176208+dff652@users.noreply.github.com
git add -A
git commit -m "feat: publish AI Asset Hub technical preview"
git show --stat --oneline HEAD
```

此时检查：

- `git log` 只有一个 public 根提交；
- 没有 private 仓库的 remote、branch 或 tag；
- `docs/handoffs`、`docs/migrations` 不在 `git ls-files`；
- `v0.1.0` 属 private 历史，不复制到新仓库；首个 public 版本使用 `v0.1.1`。

## 4. GitHub 切换

为避免对现有 private 仓库做历史重写：

1. 所有者先把当前 `dff652/ai-asset-hub` 改名为
   `dff652/ai-asset-hub-internal`，保持 private；
2. 原开发 checkout 的 `origin` 同步改到 internal 新地址；
3. 创建新的 `dff652/ai-asset-hub`，初始保持 private；
4. 只从第 3 节的干净导出仓库 push `main`，再从同一根提交创建 `dev`；绝不从
   原 checkout push `--mirror`；
5. 等新仓库 `main` / `dev` CI 全绿；
6. 配置 `main` / `dev` 分支保护、必需 CI、GitHub private vulnerability
   reporting、description 和 topics。

先以 private staging 验证，可以在改变 visibility 前发现构建、链接或权限错误。

## 5. 首个 public Release

新仓库 `main` / `dev` 指向同一已通过 CI 的干净根提交后，按
[发版 runbook](release.md)本地预演并发布 `v0.1.1`。该版本必须包含
`aiah doctor`、六平台二进制、`SHA256SUMS`、项目许可和第三方许可材料。

发布后在仍为 private 的 staging 仓库验证：

```bash
gh release view v0.1.1 --repo dff652/ai-asset-hub
```

下载产物，校验 SHA256，并在 Linux amd64 运行：

```bash
./aiah_0.1.1_linux_amd64 version
./aiah_0.1.1_linux_amd64 doctor --help
```

版本必须是 `0.1.1`，且 `doctor` 命令存在。

## 6. 切换 public 与匿名验收

所有前置门槛通过后，所有者再修改 visibility。切换后使用未登录环境验证：

```bash
PUBLIC_CHECK="$(mktemp -d)"
git clone https://github.com/dff652/ai-asset-hub.git "$PUBLIC_CHECK/repo"
curl -fL \
  https://github.com/dff652/ai-asset-hub/releases/download/v0.1.1/SHA256SUMS \
  -o "$PUBLIC_CHECK/SHA256SUMS"
curl -fL \
  https://github.com/dff652/ai-asset-hub/releases/download/v0.1.1/aiah_0.1.1_linux_amd64 \
  -o "$PUBLIC_CHECK/aiah"
```

再完成：

- SHA256 校验通过；
- `aiah version` 输出 `0.1.1`；
- `aiah doctor --help` 可用；
- README、LICENSE、NOTICE、SECURITY 和 Release 链接均可匿名访问；
- GitHub Security 页面能发起 private vulnerability report；
- `main` 保护规则和必需 CI 生效。

## 7. Public 后的事实源

匿名验收通过后，新的 public 仓库成为代码事实源。后续开发必须从 public 仓库的
全新 clone 开始，不能继续从含 private 历史的旧 checkout 向 public push。

原 `ai-asset-hub-internal` 冻结为只读档案，用于保留旧开发历史和设备迁移台账；
不得把 internal remote、旧 tag 或完整 refs 镜像到 public 仓库。新的设备级记录应
放入独立 private 笔记仓库，不再提交到 public 代码仓库。

随后再开发 `install.sh` / `install.ps1`，并接 Homebrew 和 Scoop。这些渠道必须
下载固定版本、验证 `SHA256SUMS`、原子安装且默认不使用 sudo。
