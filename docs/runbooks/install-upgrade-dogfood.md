# 工具安装、升级与 TUI dogfood SOP

- 适用：验证 `aiah` 工具自身的 Linux amd64 安装、Release 间升级，以及发布候选的
  TUI 日常维护能力。
- 不适用：把用户资产部署到真实 HOME；该流程见
  [真机 dry-run runbook](real-home-dry-run.md)。
- 默认边界：所有写入都在 `mktemp` 目录，不覆盖 `~/.local/bin/aiah`，不把
  `HOME` 环境变量改指测试目录。
- 最近一次实跑：2026-07-30，public `v0.1.4 → v0.1.5` 在显式设置
  `AIAH_VERSION=0.1.5` 时升级通过；同版本幂等复装、裸 `aiah`、统一资产状态、
  typed apply/update/remove/rollback、E3.1 和退出后 CLI 对账均通过。
- 已知问题：同次验收确认 `v0.1.4` / `v0.1.5` 的 `update --check` 推荐命令缺少
  `AIAH_VERSION`，执行后可能仍停留在旧 pin。Release 说明已提供显式版本命令；
  下一版本必须修复并把“执行实际输出命令”加入发布门禁。
- E2 候选实跑：2026-07-30，隔离 `0.1.5-dev.e2` 二进制完成统一资产状态、纳入、
  连续 profile/diff、typed apply、成功摘要、Doctor、typed update/remove 与 CLI
  对账。该记录只证明 dev 候选，不代表未发布 Release 的安装器升级已通过。
- N1 候选收口：2026-07-30，`dev@8552bef` 的 `0.1.5-dev.n1` 隔离安装完成裸
  `aiah`、纳入/更新/移出、连续 apply、Doctor/rollback、E3.1 通道对齐和重复候选
  替换；本地 `0.1.5` Release 产物也通过 SHA256/ELF/版本自检。完整证据与仍需门槛见
  [v0.1.5 候选就绪检查点](../reviews/2026-07-30-v0.1.5-candidate-readiness.md)。
  后续正式发布结果与已知问题见同一检查点 §5。

## 0. 四种验证不要混

| 验证 | 能证明什么 | 不能证明什么 |
|---|---|---|
| `scripts/test-install.sh` | 无网络夹具下的校验失败保旧、幂等、原子替换、平台拒绝 | 线上 Release 与真实下载可用 |
| Release → Release 升级 | 用户重跑安装命令能从旧版本升级到新版本 | 尚未发布的候选可安装 |
| dev 候选替换 | 当前 commit 的真实二进制能在安装位置运行和完成 TUI dogfood | 新 Release 的 installer URL 已可用 |
| 发布后下载验收 | tag、Release 资产、SHA256、版本戳和安装入口完整闭环 | 其它 OS/arch 的原生语义 |

**尚未发布的版本不能伪装成安装器升级已通过。** tag/Release 存在前，只能做本地
release-build 与候选二进制 dogfood；真实旧版 → 新版安装必须在 Release 发布后补跑，
通过后才能把安装器默认 pin 改到新版本。

## 1. 建立隔离环境

在仓库根目录：

```bash
DOGFOOD_ROOT=$(mktemp -d /tmp/aiah-upgrade.XXXXXX)
DOGFOOD_INSTALL=$DOGFOOD_ROOT/install/bin
DOGFOOD_HOME=$DOGFOOD_ROOT/home
DOGFOOD_PROJECT=$DOGFOOD_ROOT/project
DOGFOOD_DIST=$DOGFOOD_ROOT/dist
mkdir -p "$DOGFOOD_INSTALL" "$DOGFOOD_HOME" "$DOGFOOD_PROJECT" "$DOGFOOD_DIST"
```

先确认没有误用真实安装路径：

```bash
printf '%s\n' "$DOGFOOD_ROOT" "$DOGFOOD_INSTALL" "$DOGFOOD_HOME"
test "$DOGFOOD_INSTALL" != "$HOME/.local/bin"
```

## 2. Release → Release 真实升级（发布后必跑）

以下示例验证 `v0.1.4 → v0.1.5`。两个版本都必须已存在于 GitHub Releases：

```bash
FROM_VERSION=0.1.4
TO_VERSION=0.1.5

AIAH_VERSION=$FROM_VERSION AIAH_INSTALL_DIR=$DOGFOOD_INSTALL sh scripts/install.sh
"$DOGFOOD_INSTALL/aiah" version --output json
before_sha=$(sha256sum "$DOGFOOD_INSTALL/aiah" | awk '{print $1}')

upgrade_command=$("$DOGFOOD_INSTALL/aiah" update --check --output json |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["upgradeCommand"])')
expected="curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v$TO_VERSION/scripts/install.sh | AIAH_VERSION=$TO_VERSION sh"
test "$upgrade_command" = "$expected"

AIAH_VERSION=$TO_VERSION AIAH_INSTALL_DIR=$DOGFOOD_INSTALL sh scripts/install.sh
"$DOGFOOD_INSTALL/aiah" version --output json
after_sha=$(sha256sum "$DOGFOOD_INSTALL/aiah" | awk '{print $1}')

test "$before_sha" != "$after_sha"
test "$(stat -c '%a' "$DOGFOOD_INSTALL/aiah")" = 755
test -z "$(find "$DOGFOOD_INSTALL" -maxdepth 1 -name '.aiah.install.*' -print -quit)"
```

再执行一次同版本安装，必须是幂等 no-op，摘要不变：

```bash
AIAH_VERSION=$TO_VERSION AIAH_INSTALL_DIR=$DOGFOOD_INSTALL sh scripts/install.sh
test "$after_sha" = "$(sha256sum "$DOGFOOD_INSTALL/aiah" | awk '{print $1}')"
```

必须在安装 `TO_VERSION` **之前**从旧版本读取并核对程序实际生成的升级命令；升级后
再检查只会得到 `current`，不能证明旧用户看到的命令正确。不要对未经验证的网络
返回值直接使用 `eval`；先断言命令完全等于项目定义的安全模板，再在另一个隔离目录
执行模板中的固定 URL 和显式版本，确认确实到达 `TO_VERSION`。

`v0.1.5` 首次执行这条新增门禁时发现：实际命令没有 `AIAH_VERSION=0.1.5`，会让
v0.1.4 保持原版本。因此该版本只能记为“显式版本升级通过，推荐命令失败且已公开
告知”，不能记为安装入口完整闭环。

验收记录至少保留：

- 升级前后 `aiah version --output json`；
- 两个 SHA256；
- 最终 mode；
- 安装器输出；
- staging 文件检查结果。
- `update --check` 实际命令文本与执行后的版本。

## 3. 发布前 dev 候选 dogfood

候选版本必须带开发标记，避免被误认为已发布 Release：

```bash
mkdir -p "$DOGFOOD_ROOT/candidate"
VERSION=0.1.6-dev.1 OUT="$DOGFOOD_ROOT/candidate/aiah" ./scripts/build.sh
"$DOGFOOD_ROOT/candidate/aiah" version --output json
```

如需模拟安装位置上的原子替换，用同目录 stage + rename；这只验证候选二进制，
**不替代第 2 节的真实安装器升级**：

```bash
candidate_stage=$(mktemp "$DOGFOOD_INSTALL/.aiah.candidate.XXXXXX")
cp "$DOGFOOD_ROOT/candidate/aiah" "$candidate_stage"
chmod 0755 "$candidate_stage"
"$candidate_stage" version --output json >/dev/null
mv "$candidate_stage" "$DOGFOOD_INSTALL/aiah"
cmp "$DOGFOOD_ROOT/candidate/aiah" "$DOGFOOD_INSTALL/aiah"
```

## 4. TUI D2：Doctor 与当前部署回滚

先用无密钥 fixture 建包并部署到隔离 HOME：

```bash
AIAH=$DOGFOOD_INSTALL/aiah
"$AIAH" build \
  --manifest testdata/workspace-valid/manifest.yaml \
  --profile personal \
  --out "$DOGFOOD_DIST" \
  --output json >"$DOGFOOD_ROOT/build.json"

PACKAGE=$DOGFOOD_DIST/$(python3 -c \
  'import json,sys; print(json.load(open(sys.argv[1]))["package"]["archive"])' \
  "$DOGFOOD_ROOT/build.json")

"$AIAH" apply \
  --package "$PACKAGE" \
  --home "$DOGFOOD_HOME" \
  --project "$DOGFOOD_PROJECT" \
  --targets claude,codex \
  --output json >"$DOGFOOD_ROOT/apply.json"
```

启动真实 TTY：

```bash
"$AIAH" ui --home "$DOGFOOD_HOME" --project "$DOGFOOD_PROJECT"
```

人工验收：

1. 按 `h`；Doctor 显示 healthy、当前 package/version、`unchanged > 0` 和 backup。
2. 按 `x`；仅出现确认页，不应立即写入。
3. 完整输入 `rollback` 并回车。
4. 界面显示 rollback 完成，deployment 变为 no，inventory 刷新为空。

退出后用 CLI 对账：

```bash
"$AIAH" scan --home "$DOGFOOD_HOME" --project "$DOGFOOD_PROJECT" --output json
"$AIAH" doctor --home "$DOGFOOD_HOME" --project "$DOGFOOD_PROJECT" --output json
```

期望：`candidateAssets == 0`、`deployment == null`、backup 仍可审计，fixture 的
目标文件已删除。

## 5. TUI D3：版本与显式更新检查

在同一个 TUI 中：

1. 按 `v`；核对 version、12 位 commit、build date 和当前资产 deployment。
2. 首屏必须显示 `not checked`；仅打开版本页不能联网。
3. 按 `c` 后才显示 checking，再显示 latest Release 或可见错误。
4. 稳定旧版本应显示 update available 和可复制的精确 tag 安装命令。
5. dev/预发布构建应显示 comparison unavailable，不伪装成稳定版本。

首次引入 D3 的版本没有“带 D3 的上一公开版”可供第 4 条实测，可以用显式旧版本戳
构建仅验证呈现；从下一版本起必须直接安装上一公开版本完成真实检查。

CLI 同源对账：

```bash
"$AIAH" update --check
"$AIAH" update --check --output json
```

## 6. 清理

先核对目标确实是本 SOP 创建的 `/tmp/aiah-upgrade.*`，再删除：

```bash
case "$DOGFOOD_ROOT" in
  /tmp/aiah-upgrade.*) find "$DOGFOOD_ROOT" -depth -delete ;;
  *) echo "refuse unexpected cleanup target: $DOGFOOD_ROOT" >&2; exit 1 ;;
esac
test ! -e "$DOGFOOD_ROOT"
```

最后确认：

```bash
git status --short
command -v aiah || true
```

如果测试前用户 PATH 中没有 `aiah`，测试后也不应凭空出现；本 SOP 不负责安装到真实
`~/.local/bin`。
