# 产品形态与分发边界评估

- 日期：2026-07-26
- 状态：§5（MCP server）已于 2026-07-28 落地并冻结为
  [ADR-0005](../decisions/0005-read-only-mcp-server-surface.md)，实现时对 `build`
  做了收紧（见 §5.4 标注）。§2–§4 仍是**评估结论，未冻结为 ADR**
- 触发：所有者提问「应该做成服务端 docker 还是保持当前形态」「内网多用户统一
  管理是否合理」「安装能不能像 claude 那样 curl」「要不要做 MCP 让 AI 工具接入」
- 关联：[ADR-0003](../decisions/0003-cli-first-go-core-and-product-surfaces.md)
  （CLI-first / 否决云端管理台）、[ADR-0004](../decisions/0004-native-mcp-config-ownership.md)
  （MCP 原生配置所有权）

本文把四组相互关联的形态问题一次收口，避免它们散落在会话里反复重问。

## 1. 结论速查

| 问题 | 结论 |
|---|---|
| 做成服务端 docker？ | **不做。** 执行面天然在宿主机，容器化零收益且重新暴露已验证的文件语义 |
| 内网多用户统一管理？ | **合理，但只覆盖管理/分发面。** 第一版用内网 Git + CI，不自研服务端 |
| docker 有没有正确用途？ | **有，且只有一个：CI 门禁镜像**（跑 `validate`/`build`，不 apply、不挂 HOME） |
| curl 一行安装？ | **该做**，方向 ADR-0003 §3 已批。D8 已决定转 public；安装脚本排在首个 public Release 匿名验收之后 |
| 做 MCP server？ | **做，但只暴露只读子集**，`apply`/`rollback`（落地时另加 `build`）不进 MCP。已实现，见 ADR-0005 |

## 2. 为什么不做服务端 docker

### 2.1 执行面天然在宿主机

aiah 的核心动作是写 `~/.claude` / `~/.codex` / `~/.grok`：保持文件 mode
（`600 → apply → backup → rollback` 往返测试）、判软链逃逸、原子 rename、
备份与回滚。容器化必须挂载 HOME，随即撞上：

- uid/gid 映射与属主漂移；
- mode 位在部分挂载形态下失真；
- symlink 语义与逃逸判定基准变化；
- 跨 mount 的 rename 不再原子；
- SELinux / AppArmor label。

这恰好是本项目投入测试最多的一层。容器化等于把已验证的语义压到一层不确定性
下面，**换不到任何功能**。

已有容器化项目中的 `ContainerPath` / `HostPath` / `to_host_path()` 整套复杂度
就是「容器进程要操作宿主文件」的成品代价，不应在本项目重演。

### 2.2 bootstrap 鸡生蛋

产品价值场景是「换新设备一键恢复」。若新设备必须先装 docker、拉镜像、配
daemon 才能恢复 skill 文件，就比 `curl` 一个 ~6 MB 静态二进制差一个数量级。
而离线机器、客户现场、Windows 上未必有 docker——这些正是目标场景。

### 2.3 存储面不需要自研服务端

包是不可变 tar + `SHA256SUMS`。Git、网盘、S3、OCI registry 都能存。自建 server
换来的是账户、鉴权、TLS、备份、升级与隐私责任，是纯负债。

### 2.4 docker 唯一正确的用途：CI 门禁

可选镜像的职责只有一个——在资产仓库的 PR 检查里跑 `aiah validate` / `build`。
**它不 apply、不挂 HOME**，只跑纯函数子集。成本低、边界干净，与 §2.1 不冲突。

## 3. 内网多用户统一管理

### 3.1 拆两个面，结论相反

| 面 | 内容 | 服务端 |
|---|---|---|
| 管理 / 分发面 | 谁维护哪些资产、审阅、版本、发给谁 | ✅ 合理，是真需求 |
| 执行面 | 往每个人 HOME 写文件、备份、回滚 | ❌ 只能在各人机器上 |

「统一管理」的实质是前者。后者不可能服务端化——除非在每台开发机跑常驻特权
agent，那就从工具变成 MDM（见 §3.3）。

### 3.2 第一版不自研服务端

把需求拆细，看谁已经能提供：

| 需求 | 现成方案 | 要自研吗 |
|---|---|---|
| 资产集中存放 + 版本化 | 内网 Git | 否 |
| 谁能改什么 | Git 权限 + PR review | 否 |
| 改了要校验 | CI 跑 `aiah validate` | 否 |
| 分发包 | Release artifact / 共享目录 / OCI registry | 否 |
| 装到各人机器 | `aiah diff` + `aiah apply` | 已有 |
| **谁装了哪个版本（审计）** | — | **是，唯一缺口** |
| **非工程师维护者的编辑界面** | — | 是，若确有此类用户 |

一个内网 Git 仓 + CI 覆盖「统一管理」的绝大部分，零自研。真正需要服务端的
只有最后两行，且这两行目前都未被验证存在。

### 3.3 红线：订阅式，不是推送式

**明确反对**「服务端下发 + 客户端 daemon 自动 apply」：

- 需要在每台开发机跑常驻特权进程并存放凭据；
- 一个 bug 批量污染所有人的 HOME，而那是同事的私人开发环境；
- 与现有 backup / rollback 的单机语义冲突（回滚了下次又被推回来）；
- 开发者 dotfiles 是私人领地，强推必然引发绕过。

正确模型是**发布 / 订阅**：服务端只发布版本，用户主动 `diff` → 人审 →
`apply`，永远保留 `rollback`。aiah 现有语义（create-only、不删未知文件、
冲突 fail-closed）本来就是照这个模型设计的，不要用服务端把它推翻。

### 3.4 分阶段路径

| 阶段 | 内容 | 触发条件 |
|---|---|---|
| 0 | 内网 `ai-assets` Git 仓 + CI 跑 `validate`/`build`，产物挂 Release / 共享目录 | 现在就能做 |
| 1 | profile 分层：`team-common` / `role-backend` / `role-frontend`；个人包不进公共仓 | 3+ 人在用 |
| 2 | 薄 receiver 收 apply 报告（已有 `producedBy` 与部署记录 JSON） | 出现审计需求 |
| 3 | Web 编辑面 | 出现非工程师维护者 |

阶段 2 才第一次出现服务端，且是**只读看板，不下发指令**。

### 3.5 团队化会抬高密钥门槛

单人时 `sensitivity: sensitive` + `${ENV:...}` 引用是自律；多人共享内网仓时，
任何人一次疏忽把真 token 写进 `assets/`，就是全团队泄露且 Git 历史留痕。
上团队前应先补 CI 门禁：`aiah validate` 报 `suspected_secret` 即失败。

## 4. 安装分发（curl 一行）

### 4.1 方向已批，前置已齐

ADR-0003 §3 的分发顺序第 4 项即「可审查的安装脚本」。private `v0.1.0` 已证明
前置能力：六平台**裸二进制**（`aiah_<version>_<goos>_<goarch>`，Windows 带 `.exe`）
+ `SHA256SUMS` + `scripts/check-release-checksums.sh`。裸二进制无需解压，
安装脚本比 tar.gz 形态更简单。

### 4.2 Public 前置与已决策路径

`raw.githubusercontent.com` 与 Release assets 对 private repo 都需要 token；
Homebrew / Scoop 同样要求 public。分发产物侧已有 `LICENSE` / `NOTICE` /
`THIRD_PARTY_LICENSES.txt` / Apache-2.0 徽章，但 2026-07-27 复核确认公开仓库治理
尚未全部收口：canonical module path、历史隐私、默认分支、Release/README 一致性
和漏洞报告入口仍须处理。所有者已于 2026-07-27 拍板 D8：转 public、使用
`dff652` module path、采用干净公开历史；完整门槛见
[Public readiness 评估](../reviews/2026-07-27-public-readiness-assessment.md)。

若未来撤销 public 决策，备选仍是内网自托管 `install.sh` 与二进制，与 §3.4
阶段 0 合流。

### 4.3 脚本设计约束

```bash
curl -fsSL https://<host>/install.sh | sh
AIAH_VERSION=0.1.0 AIAH_INSTALL_DIR=~/.local/bin sh -
```

必须守住：

1. **必须校验 `SHA256SUMS`**。一个以「校验和 + 可回滚 + 可审计」为卖点的工具，
   其安装脚本无校验执行远程代码是自相矛盾。流程：下 `SHA256SUMS` → 下二进制
   → 比对 → 才安装。
2. **原子安装，绝不先删旧的**：下载到 `mktemp -d` → 校验通过 → `chmod +x`
   → `mv` 就位。先删后装会在中断时留下 stub，产生「命令存在但不可用」的
   反复排障（其它 CLI 工具的实战教训）。
3. **默认装 `~/.local/bin`，不用 sudo，不写 `/usr/local`**，不改用户 profile；
   PATH 缺失只打印提示。
4. **平台探测 `uname -s` / `-m` 映射到六个 target 之一**，不在矩阵内明确报错，
   不猜测。
5. **幂等**：已装同版本零动作，与 `apply` 的 `written=0` 同语义。

Windows 单独 `install.ps1`，不塞进 sh。脚本落 `scripts/install.sh`，校验逻辑
复用 `scripts/_sha256.sh`，不写第二套。

文档中 `curl | sh` 是便捷路径，「下载 → 阅读 → 执行」的两步式仍是推荐路径。

## 5. MCP server（让 AI 工具调用 aiah）

### 5.1 先分清两个 MCP

| | 含义 | 状态 |
|---|---|---|
| 已有 | aiah 把 MCP 模板当**资产**管理（`type: mcp`，create-only 写原生配置） | ADR-0004，已实现 |
| 本节 | aiah 自己**当 MCP server**，被 Claude / Codex 调用 | ADR-0005，2026-07-28 已实现 |

### 5.2 值得做

- ADR-0003 §1 已预留：「CLI 是长期稳定的产品内核、自动化接口和 **Agent 接口**」；
- 所有命令已有确定性 JSON 输出，MCP server 是几乎零业务逻辑的薄包装；
- 场景真实：「这台机器哪些 skill 还没进包」「这个包装上去会改哪些文件」，
  这类问题让 agent 读 JSON 比人读高效。

### 5.3 但存在递归污染风险

`aiah apply` 写 `~/.claude`，而 Claude Code 自身在读 `~/.claude`。让 agent 经
MCP 调用 `apply`，等于让它改自己的运行时配置——harness 可能中途重载，行为不可
预测；一个坏 prompt 即可改掉用户 dotfiles。这是实际风险，不是理论风险。

### 5.4 边界：只读子集

| 工具 | 暴露 | 理由 |
|---|---|---|
| `scan` | ✅ | 只读 |
| `validate` | ✅ | 只读 |
| `diff` / `apply --dry-run` | ✅ | 只读，且价值最高（回答「会改什么」） |
| `build` | ⚠️ **落地时改为不暴露** | 见本节末标注 |
| `doctor` | ✅ | 只读（实现后） |
| `apply` | ❌ | 写 HOME，可能改 agent 自身配置 |
| `rollback` | ❌ | 同上 |

该边界与既有 Ops/Debug MCP「默认只读、写操作双开关」采用同一判据，
不是临时取舍。**实现时应把此边界固化为 ADR**，避免后续被逐个工具地放宽。

> **2026-07-28 实现时的收紧（不回改上表结论，只标注结果）**：`build` 最终**未**
> 暴露。上表判它可暴露的理由是「只写 `--out`，不碰 HOME」，但那个 `out` 由**调用方
> 指定**——agent 完全可以把它指向 `~/.claude`。排除后，这个 surface 的不变式从
> 「只写你指定的地方」变成绝对的「**零写入**」，既好测试也好向用户承诺。理由与
> 恢复条件见 [ADR-0005 §3](../decisions/0005-read-only-mcp-server-surface.md)。

### 5.5 实现建议：先不引依赖

MCP stdio server 本质是 stdin/stdout 上的 JSON-RPC。现有命令已产确定性 JSON，
一个 `aiah mcp` 子命令约 200–300 行手写即可。理由：Bubble Tea 一族刚付出
`+0.97 MiB` 体积与一轮 `NOTICE` / 第三方清单同步成本，为薄协议层再引一族依赖
不划算。协议稳定后再评估换库。

## 6. 与 ADR-0003 的关系

本文不推翻 ADR-0003，是对它的细化：

- §2 强化了 ADR-0003「先建设云端管理台——不采用」的理由，从「过早引入复杂度」
  补充为「执行面在技术上就不可服务端化」；
- §3 给出 ADR-0003 未覆盖的团队场景答案（用现成 Git 基建，不自研）；
- §4 落实 ADR-0003 §3 分发顺序的第 4 项；
- §5 落实 ADR-0003 §1 的「Agent 接口」承诺，并为其补上安全边界。
