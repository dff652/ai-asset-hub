# MCP 客户端接入与只读验收

本 SOP 验证 aiah 作为本地 stdio MCP server 被 Claude Code、Codex、Grok 发现和调用，
并把四件事分开记录：

1. server 直接协议是否正常；
2. 客户端是否完成 initialize / tools/list 握手；
3. 模型是否实际调用指定工具；
4. fake HOME、project、资产库、通道和包目录是否零写入。

`Connected` 不能替代模型调用；账户或组织策略在模型请求前拒绝，也不能归因成 aiah
协议失败。

## 1. 边界与准备

只在 disposable 目录中验收，不把真实 HOME、真实资产库或 secret 放进 prompt：

```bash
AIAH_MCP_REPO="$(pwd)"
AIAH_MCP_TEST_ROOT="$(mktemp -d /tmp/aiah-mcp-acceptance.XXXXXX)"
go build -trimpath -o "$AIAH_MCP_TEST_ROOT/aiah" ./cmd/aiah
cp -a "$AIAH_MCP_REPO/testdata/home-basic" "$AIAH_MCP_TEST_ROOT/home"
cp -a "$AIAH_MCP_REPO/testdata/workspace-valid" "$AIAH_MCP_TEST_ROOT/workspace"
```

这些变量不替换 shell 的 `HOME`、`CODEX_HOME` 或 `GROK_HOME`。客户端临时配置只指向
`$AIAH_MCP_TEST_ROOT/aiah mcp`。

## 2. 直接协议基线

依次发送 initialize、initialized notification、tools/list 和两个状态工具调用。
客户端可请求更新协议，server 返回自己支持的修订：

```bash
{
  jq -cn \
    '{jsonrpc:"2.0",id:1,method:"initialize",params:{
      protocolVersion:"2025-11-25",capabilities:{},
      clientInfo:{name:"aiah-acceptance",version:"1"}}}'
  jq -cn '{jsonrpc:"2.0",method:"notifications/initialized"}'
  jq -cn '{jsonrpc:"2.0",id:2,method:"tools/list"}'
  jq -cn \
    --arg workspace "$AIAH_MCP_TEST_ROOT/workspace" \
    --arg home "$AIAH_MCP_TEST_ROOT/home" \
    '{jsonrpc:"2.0",id:3,method:"tools/call",params:{
      name:"aiah_asset_status",
      arguments:{workspace:$workspace,home:$home}}}'
  jq -cn \
    --arg workspace "$AIAH_MCP_TEST_ROOT/workspace" \
    --arg home "$AIAH_MCP_TEST_ROOT/home" \
    '{jsonrpc:"2.0",id:4,method:"tools/call",params:{
      name:"aiah_migration_status",
      arguments:{workspace:$workspace,home:$home}}}'
} | "$AIAH_MCP_TEST_ROOT/aiah" mcp
```

通过标准：

- protocol 为 server 支持的正式修订；
- tools/list 恰好列出当前 ADR-0005 的工具数（public `v0.1.9` 为 7；含 N10.3 的
  源码/`dev` 为 8，含 `aiah_migration_readiness`）；
- 每个工具带只读 annotations，schema 禁止额外参数；
- 两次调用分别返回 `asset-catalog`、`migration-status`，无 `isError=true`。

## 3. Claude Code

日常用户配置：

```bash
claude mcp add aiah -- aiah mcp
claude mcp get aiah
```

候选验收优先用 `--mcp-config` + `--strict-mcp-config` 指向 disposable binary，并禁用
其它工具。若组织策略、订阅或 API key 在模型请求前拒绝：

- `mcp get` 显示 `Connected`：只记“客户端握手通过”；
- 没有产生 MCP tool call：模型级调用记 `blocked`，保留原始状态码与原因；
- 不通过切换真实 HOME、复制 token 或放宽权限来绕过。

## 4. Codex

用一次性配置、临时会话和只读 sandbox，不修改用户的 `config.toml`：

```bash
codex exec \
  --ignore-user-config \
  --ephemeral \
  --sandbox read-only \
  --cd "$AIAH_MCP_TEST_ROOT" \
  --skip-git-repo-check \
  -c "mcp_servers.aiah.command=\"$AIAH_MCP_TEST_ROOT/aiah\"" \
  -c 'mcp_servers.aiah.args=["mcp"]' \
  "只调用 aiah_asset_status 一次；workspace=$AIAH_MCP_TEST_ROOT/workspace，home=$AIAH_MCP_TEST_ROOT/home；返回 kind 和 ok。"
```

记录 MCP tool-call 事件中的 server、tool、arguments、结果 kind/ok；不要只保存模型
最后一句话。

## 5. Grok

在 disposable 目录写 project scope 配置：

```bash
(
  cd "$AIAH_MCP_TEST_ROOT"
  grok mcp add --scope project aiah -- "$AIAH_MCP_TEST_ROOT/aiah" mcp
  GROK_FOLDER_TRUST=0 grok mcp doctor aiah --json
)
```

`GROK_FOLDER_TRUST=0` 只允许用于本 SOP 自己创建、内容已知的临时目录；不要在下载的
仓库或未知目录关闭 folder trust。Doctor 必须显示 command found、server started、
handshake OK 和 7 tools discovered。

模型级调用使用 headless 模式，移除 shell/web/edit/agent 工具，只允许 aiah 的指定
MCP 工具。记录最终 kind/ok 和会话 id；模型调用会产生正常的模型用量与临时会话记录。

## 6. 零写入对账

至少比较 fake HOME 和资产库；包含通道/包时一并比较：

```bash
diff -r --no-dereference \
  "$AIAH_MCP_REPO/testdata/home-basic" \
  "$AIAH_MCP_TEST_ROOT/home"
diff -r --no-dereference \
  "$AIAH_MCP_REPO/testdata/workspace-valid" \
  "$AIAH_MCP_TEST_ROOT/workspace"
```

还必须运行 `go test ./internal/mcp`；其中 `TestToolCallsWriteNothing` 对注册表中的每个
工具比较 HOME、project、资产库、通道和包目录的正文与 mode。验收完成后先确认
`AIAH_MCP_TEST_ROOT` 确实位于 `/tmp/aiah-mcp-acceptance.*`，再用系统回收机制清理。

## 7. 2026-07-30 N6 `dev` 验收记录

| 客户端 | 版本 | 握手 | 模型级工具调用 |
|---|---|---|---|
| Claude Code | 2.1.198 | `Connected` | blocked：组织策略在调用前返回 403，未伪报成功 |
| Codex CLI | 0.145.0 | 7 工具可用 | `aiah_asset_status` → `asset-catalog`, `ok=true` |
| Grok | 0.2.114 | protocol `2025-06-18`，7 tools，healthy | `aiah_migration_status` → `migration-status`, `ok=true` |

全部调用后 fake HOME 与资产库和原始 fixture 一致。该矩阵证明两个客户端的模型级
调用和三个客户端的协议兼容；它不证明 Claude 模型调用，后者须在组织策略解除后补测。
