# 工具安装、升级与 TUI dogfood SOP

- 适用：验证 `aiah` 工具自身的 Linux amd64 安装、Release 间升级，以及发布候选的
  TUI 日常维护能力。
- 不适用：把用户资产部署到真实 HOME；该流程见
  [真机 dry-run runbook](real-home-dry-run.md)。
- 默认边界：所有写入都在 `mktemp` 目录，不覆盖 `~/.local/bin/aiah`，不把
  `HOME` 环境变量改指测试目录。
- 最近一次实跑：2026-08-01，public `v0.1.8 → v0.1.9` 严格升级命令、真实升级、
  同版本幂等复装、init 正反路径、裸/显式 `aiah` 真 TTY 首页和 MCP 只读零写入
  回归均通过；完整偏好生命周期与双设备应用/撤销闭环最近在 v0.1.7 完整执行。
- 已知问题：同次验收确认 `v0.1.4` / `v0.1.5` 的 `update --check` 推荐命令缺少
  `AIAH_VERSION`，执行后可能仍停留在旧 pin。Release 说明已提供显式版本命令；
  `v0.1.6` 验收已在第二个隔离目录执行 legacy 命令，确认版本与 SHA256 均保持
  `v0.1.5` 且无 stage 残留；主目录显式升级成功。
- `v0.1.6` 已从 `main@46e6efccc9ba` 发布，Release workflow、线上 SHA256、静态
  ELF、版本/commit、许可材料和正式 TTY 均通过；完整证据见
  [bridge 检查点](../reviews/2026-07-30-v0.1.6-bridge-candidate-readiness.md)。
- `v0.1.7` 已从 `main@b6779193c3ac` 发布，首次完成修复后推荐命令的严格
  Release → Release 证明；正式包证据见
  [v0.1.7 发布与正式验收记录](../reviews/2026-08-01-v0.1.7-release-acceptance.md)。
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

以下示例验证当前 `v0.1.6 → v0.1.7`。两个版本都必须已存在于
GitHub Releases：

```bash
FROM_VERSION=0.1.6
TO_VERSION=0.1.7

AIAH_VERSION=$FROM_VERSION AIAH_INSTALL_DIR=$DOGFOOD_INSTALL sh scripts/install.sh
"$DOGFOOD_INSTALL/aiah" version --output json
before_sha=$(sha256sum "$DOGFOOD_INSTALL/aiah" | awk '{print $1}')

upgrade_command=$("$DOGFOOD_INSTALL/aiah" update --check --output json |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["upgradeCommand"])')
expected="curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v$TO_VERSION/scripts/install.sh | AIAH_VERSION=$TO_VERSION sh"
legacy_expected="curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/v$TO_VERSION/scripts/install.sh | sh"
case "$FROM_VERSION" in
  0.1.4 | 0.1.5)
    test "$upgrade_command" = "$legacy_expected"
    printf 'known legacy upgrade command: %s\n' "$upgrade_command"

    LEGACY_INSTALL=$DOGFOOD_ROOT/legacy/bin
    mkdir -p "$LEGACY_INSTALL"
    install -m 0755 "$DOGFOOD_INSTALL/aiah" "$LEGACY_INSTALL/aiah"
    legacy_before_sha=$(sha256sum "$LEGACY_INSTALL/aiah" | awk '{print $1}')
    curl -fsSL \
      "https://raw.githubusercontent.com/dff652/ai-asset-hub/v$TO_VERSION/scripts/install.sh" |
      env -u AIAH_VERSION AIAH_INSTALL_DIR="$LEGACY_INSTALL" sh
    legacy_after_version=$("$LEGACY_INSTALL/aiah" version --output json |
      python3 -c 'import json,sys; print(json.load(sys.stdin)["version"])')
    test "$legacy_after_version" = "$FROM_VERSION"
    test "$legacy_before_sha" = \
      "$(sha256sum "$LEGACY_INSTALL/aiah" | awk '{print $1}')"
    test -z "$(find "$LEGACY_INSTALL" -maxdepth 1 \
      -name '.aiah.install.*' -print -quit)"
    ;;
  *)
    test "$upgrade_command" = "$expected"
    ;;
esac

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
返回值直接使用 `eval`；先断言命令等于“修复后模板”或明确列出的 legacy 模板。
legacy 分支在第二个隔离目录执行固定 URL、显式清除 `AIAH_VERSION` 并确认版本和
SHA256 均保持不变；主升级目录再显式传入目标版本，确认确实到达 `TO_VERSION`。

`v0.1.5` 首次执行这条新增门禁时发现：实际命令没有 `AIAH_VERSION=0.1.5`，会让
v0.1.4 保持原版本。因此该版本只能记为“显式版本升级通过，推荐命令失败且已公开
告知”，不能记为安装入口完整闭环。

`v0.1.4` / `v0.1.5` 二进制不可追溯修改。由 `v0.1.5` 升级到首个包含修复的版本时，
仍会命中上面的 legacy 分支：Release 必须公开显式版本 workaround，并把实际旧命令
执行后的 no-op 作为已知失败证据。只有从首个修复版升级到再下一版时，才能首次要求
`upgrade_command == expected` 并宣布推荐命令端到端闭环。

验收记录至少保留：

- 升级前后 `aiah version --output json`；
- 两个 SHA256；
- 最终 mode；
- 安装器输出；
- staging 文件检查结果；
- `update --check` 实际命令文本；
- bridge legacy 目录执行前后的版本与 SHA256；
- 显式版本升级后的版本与 SHA256。

## 3. 发布前 dev 候选 dogfood

候选版本必须带开发标记，避免被误认为已发布 Release：

```bash
mkdir -p "$DOGFOOD_ROOT/candidate"
VERSION=0.1.8-dev.1 OUT="$DOGFOOD_ROOT/candidate/aiah" ./scripts/build.sh
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

启动真实 TTY。普通入口必须使用裸 `aiah`；`ui` 仅在需要显式测试目录参数时保留：

```bash
"$AIAH" ui --home "$DOGFOOD_HOME" --project "$DOGFOOD_PROJECT"
```

另开一个真实 TTY，验证无参数入口：

```bash
"$AIAH"
```

裸入口会只读扫描当前用户 HOME；本项只验证任务首页和 `q` 退出，不进入任何写操作。
隔离写流程继续使用上一条显式 `ui --home/--project` 命令。两者必须进入同一任务
首页。自动 PTY 脚本应等待首屏实际出现后再发送按键，不能用固定的过早按键时序代替
初始化判断。

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
