# 假 HOME 端到端闭环（不碰真实家目录）

在临时目录跑通：

```text
validate → build → diff → apply → scan → rollback → scan
```

## 一键脚本

在仓库根目录：

```bash
# 默认 workspace-valid（skill + rules → claude/codex/shared）
./scripts/demo-apply-scan-loop.sh

# Phase 2B fixture（含 agent/hook/mcp/grok + project CLAUDE.md）
./scripts/demo-apply-scan-loop.sh workspace-2b

# 保留临时目录便于手工查看
KEEP_WORKDIR=1 ./scripts/demo-apply-scan-loop.sh
```

脚本会：

1. 若缺少 `build/aiah` 则自动 `go build`
2. 在 `mktemp` 目录创建 `fake-home` / `fake-project` / `dist`
3. 跑完整闭环，并在 rollback 后断言 inventory `candidateAssets == 0`
4. **不会读写真实 `$HOME`**

## 手工命令（等价步骤）

```bash
go build -o build/aiah ./cmd/aiah

WORKDIR=$(mktemp -d)
HOME_FAKE=$WORKDIR/fake-home
PROJ_FAKE=$WORKDIR/fake-project
DIST=$WORKDIR/dist
mkdir -p "$HOME_FAKE" "$PROJ_FAKE" "$DIST"

./build/aiah validate --manifest testdata/workspace-valid/manifest.yaml --output json
./build/aiah build \
  --manifest testdata/workspace-valid/manifest.yaml \
  --profile personal \
  --out "$DIST" \
  --output json

# 将输出中的 package.archive 拼到 $DIST/
PKG=$DIST/fixture-personal-2026.07.1.tar

./build/aiah diff --package "$PKG" --home "$HOME_FAKE" --targets claude,codex --output json
./build/aiah apply --package "$PKG" --home "$HOME_FAKE" --targets claude,codex --output json
./build/aiah scan --home "$HOME_FAKE" --output json | head
./build/aiah rollback --home "$HOME_FAKE" --output json
./build/aiah scan --home "$HOME_FAKE" --output json

rm -rf "$WORKDIR"
```

## 自动化测试

Go 集成测试（CI 也会跑）：

```bash
go test ./internal/e2e/ -v
```

## 注意

- 本 runbook 只验证 **fixture 包 → 假 HOME**，不是生产机迁移手册。
- 真实 `~` 上操作见 [real-home-dry-run.md](real-home-dry-run.md)（默认只 dry-run）。
- workspace-2b 会验证 create-only native config、sidecar、权限和 rollback
  闭环；不能把假 HOME 结果直接外推到真实 HOME。
- Script hooks 安装为可执行（`0755`）且要求 shebang。
