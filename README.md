# cc-feishu-link (`ccfl`)

Claude Code 的飞书托管会话（Feishu-managed session）Go 语言核心。

为 `claude --feishu` 这种用法提供后端支持：会话启动时自动建立一个飞书 session 文件夹，
第一条 prompt 后生成临时标题并重命名，之后把**低频 checkpoint** 写入飞书、把**高频原始事件**
留在本地缓冲区，并通过策略拦截危险操作与测试削弱。

> Go core 不做模型推理，不替代 Claude。它只负责：会话生命周期、飞书 API、hook 事件缓冲、
> 同步策略、风险策略、本地状态、（后续）MCP 工具。

---

## 当前实现范围（MVP，Phase 1–4）

已完成：

- ✅ 本地核心：配置、会话状态、事件缓冲（`events.jsonl`）、脏状态、标题 slug、checkpoint 渲染、`status`
- ✅ Hook 解析与策略：`session-start` / `user-prompt-submit` / `pre-tool-use` / `post-tool-use` / `stop`
  （另含最小实现的 `pre-compact` / `post-compact` / `session-end`）、风险匹配、测试完整性匹配、validation 命令识别、脱敏
- ✅ Mock 飞书后端：把 markdown 文件写到本地 `.mock-feishu/`，多进程共享 manifest
- ✅ Real 飞书后端：鉴权（tenant_access_token）、创建文件夹、创建 docx、重命名（尽力而为）、追加内容

- ✅ v2 文档集（驾驶舱/契约/纪要/验证决策/交接/记忆）+ `ccfl mcp` MCP server（8 个文档撰写工具）

尚未实现（留给 V2，见文末）：artifact 上传、审批卡片、多维表格 registry、context graph、常驻 daemon 等。

---

## 安装与构建

需要 Go 1.26+。

```bash
git clone <repo> cc-feishu-link
cd cc-feishu-link
go build -o ccfl ./cmd/ccfl
# 放到 PATH 中，例如：
mv ccfl /usr/local/bin/ccfl
```

跑测试：

```bash
go test ./...
```

---

## 快速开始

```bash
# 1. 生成配置
ccfl init                       # 写入 ~/.cc-feishu-link/config.yaml

# 2. 本地干跑（不调用任何外部 API）
ccfl hook session-start      --dry-run < internal/hooks/fixtures/session_start.json
ccfl hook user-prompt-submit --dry-run < internal/hooks/fixtures/first_prompt.json
ccfl status

# 3. 用 mock 后端跑完整流程（把"飞书文档"落到本地 .mock-feishu/）
export CCFL_BACKEND=mock
ccfl hook session-start      < internal/hooks/fixtures/session_start.json
ccfl hook user-prompt-submit < internal/hooks/fixtures/first_prompt.json
ccfl hook pre-tool-use       < internal/hooks/fixtures/pre_rm.json        # 输出 deny
ccfl hook pre-tool-use       < internal/hooks/fixtures/pre_test_edit.json # 输出 ask
ccfl hook post-tool-use      < internal/hooks/fixtures/post_gotest.json   # 触发同步
ccfl hook stop               < internal/hooks/fixtures/session_start.json
ccfl status
```

---

## CLI 命令

最小命令集（本版本已实现）：

| 命令 | 说明 |
|------|------|
| `ccfl init` | 生成默认 `config.yaml` |
| `ccfl status [--session ID]` | 查看会话状态（默认最近一个） |
| `ccfl sync [--session ID] [--force] [--dry-run]` | 手动同步到飞书 |
| `ccfl hook session-start` | SessionStart：创建文件夹与 6 个文档 |
| `ccfl hook user-prompt-submit` | UserPromptSubmit：首条 prompt 生成标题并重命名 |
| `ccfl hook pre-tool-use` | PreToolUse：风险策略拦截 |
| `ccfl hook post-tool-use` | PostToolUse：写本地缓冲、按策略同步 |
| `ccfl hook stop` | Stop：生成 checkpoint 与 handoff |

附带实现（最小）：`ccfl hook pre-compact` / `post-compact` / `session-end`。

所有 hook 子命令都支持 `--dry-run`：只在本地生成状态、打印将要进行的飞书操作，不调用 API。

---

## Claude Code 集成

把 `examples/settings.json` 的内容合并进你的 `.claude/settings.json`（确保 `ccfl` 在 PATH 中）。
hook 输入由 Claude Code 通过 stdin 传入 JSON，`ccfl` 在 stdout 返回 hook 决策（如 `permissionDecision`）。

---

## 后端选择（dry / mock / real）

后端按以下顺序确定：

1. 任意 hook/sync 命令带 `--dry-run` → **dry**：完全不碰外部，打印意图。
2. 环境变量 `CCFL_BACKEND=mock|real` → 强制指定。
3. 否则：配置里填了 `app_id` + `app_secret` → **real**，否则 → **mock**（默认安全，不会误调真实 API）。

| 后端 | 行为 |
|------|------|
| `dry` | 不写飞书、不写 mock，只在本地存 session 状态并打印日志 |
| `mock` | 把文档写到 `CCFL_MOCK_DIR`（默认 `~/.cc-feishu-link/.mock-feishu/`） |
| `real` | 调用飞书开放平台 API |

---

## 同步策略：本地优先，飞书只存 checkpoint

高频事件（每次工具调用）只进本地 `events.jsonl`；飞书只在以下情况被更新：

- **立即同步事件**：`first_prompt_title_generated`、`plan_created`、`validation_completed`、
  `compact_completed`、`blocked`、`human_approval_required`、`stop`、`session_end`
- **累积阈值**：脏事件数 ≥ `min_dirty_events`（默认 5）
- **时间阈值**：距上次同步 ≥ `max_unsynced_minutes`（默认 30 分钟）
- **手动**：`ccfl sync`

同步决策是确定性的（`internal/sync/policy.go`）。每次同步在日志里都会打印原因（“no hidden magic”）。

---

## 风险策略（PreToolUse）

确定性规则（`internal/policy`）：

**直接 block（Bash）**：`rm`、`git push`、`deploy`、`sudo`、`curl|sh` / `wget|sh`。

**需人工确认（require approval）**：

- 编辑 `tests/**`、`**/test_*.py`、`*_test.go`、`*.spec.*`、`golden/`、`testdata/`、
  `expected/`、`benchmark/`、`eval/`、含 `threshold` 的文件
- CI 配置（`.github/workflows/`、`.gitlab-ci.yml`、`.circleci/` 等）
- lockfile（`go.sum`、`package-lock.json`、`yarn.lock`、`Cargo.lock` 等）
- Bash：`chmod` / `chown` / `git reset --hard` / `git clean`

block → 返回 `permissionDecision: deny`；approval → 返回 `permissionDecision: ask`，并把决策记录到
`04_VALIDATION_AND_DECISIONS`。

**validation 命令识别**：`pytest`、`go test`、`cargo test`、`npm/pnpm/yarn test`、`make test`、
`coverage`、`jest`、`vitest` 等会被标记为验证事件并触发一次同步。

---

## 安全与隐私

- 不上传 `.env`、`*.pem`、`*.key`、含 secret/credential 的文件（`IsSensitivePath`）
- 事件摘要在落盘前做关键字脱敏（`token`/`secret`/`password`/`api_key`/`authorization`/`bearer`/`cookie`/`private_key`/`app_secret`）
- `app_id` / `app_secret` 只在本地配置，不写入任何飞书文档
- 原始 raw logs 默认不上传飞书
- 本地 session 文件以 `0600` 写入，目录 `0700`
- `--dry-run` 不调用任何外部 API

---

## 本地状态目录

默认 `~/.cc-feishu-link/`（可用 `CCFL_HOME` 覆盖）：

```
~/.cc-feishu-link/
  config.yaml
  .mock-feishu/                 # mock 后端输出
  sessions/
    <local_session_id>/
      session.json              # SessionState
      events.jsonl              # 高频事件缓冲
      raw/
        hook_payloads/
        tool_outputs/
```

---

## 飞书会话文件夹结构（v2 — 人读驾驶舱，非事件日志）

新会话使用 v2 文档集。这些文档是**人读的项目面**（驾驶舱 + 任务契约 + 交接笔记），
不是 agent 事件日志；原始事件流只留在本地 `events.jsonl`，不上传飞书。

```
S-YYYYMMDD-HHMM__<slug>/        # 首条 prompt 前为 "CC Session - Untitled - <timestamp>"
  00_COCKPIT                    # 驾驶舱：目标·状态灯·此刻·阻塞·下一步·链接（每次同步全量替换）
  01_TASK_CONTRACT              # 任务契约：目标/范围/验收标准(勾选)/约束/风险（按需替换）
  02_RECAP                      # 进展纪要：Claude 撰写的叙事（替换）
  03_VALIDATION_AND_DECISIONS   # 验证 & 决策（追加）
  04_HANDOFF                    # 交接笔记（替换）
  05_MEMORY                     # 记忆：跨 compact 存活的关键事实（PreCompact 自动沉淀 + 手动追加）
```

> 旧的事件日志式文档（`00_SESSION_INDEX` … `05_HANDOFF`）已移除；所有会话用 v2。
> 创建时 `doc_layout` 标记为 `v2`。

### 谁写这些文档：MCP 工具 → 状态 → ccfl 渲染

ccfl 只填机器可知的骨架（元数据、状态、自动记录的 block/approval、PreCompact 记忆沉淀）。
叙事内容由 Claude 通过 `ccfl mcp` 暴露的 MCP 工具写入结构化字段，ccfl 据此确定性渲染。

注册（项目根 `.mcp.json`，见 `examples/.mcp.json`）：

```json
{ "mcpServers": { "ccfl": { "type": "stdio", "command": "ccfl", "args": ["mcp"] } } }
```

工具（对 Claude 显示为 `mcp__ccfl__*`，均可选 `session_id`，省略则用最新会话）：

| 工具 | 作用 |
|---|---|
| `feishu_get_status` | 读当前目标/阶段/健康/验收标准/文档链接 |
| `feishu_set_contract` | 设任务契约（目标/范围/验收标准/约束/风险）→ 01 + 00 |
| `feishu_update_cockpit` | 更新驾驶舱（summary/next_step/blocker/health/phase）→ 00 |
| `feishu_append_decision` | 追加有意义的决策+理由 → 03 |
| `feishu_append_validation` | 追加验证结果（人读）→ 03 |
| `feishu_update_recap` | 替换进展纪要叙事 → 02 + 00 |
| `feishu_append_memory` | 追加跨 compact 关键事实 → 05 |
| `feishu_update_handoff` | 更新交接笔记 → 04 |

具体何时调用见 `plugin/skills/feishu-session/SKILL.md`。

---

## 目录结构

```
cmd/ccfl/            # CLI 入口与子命令分发
internal/app/        # 配置、依赖装配、各 hook 处理器、同步/状态命令
internal/session/    # SessionState、生命周期、标题 slug
internal/hooks/      # hook stdin 解析 / stdout 输出
internal/policy/     # 风险匹配、测试完整性、validation 识别、脱敏
internal/sync/       # 事件缓冲、脏状态、同步策略、v2 文档渲染
internal/feishu/     # Client 接口、Mock 实现、Real 实现、鉴权
internal/mcp/        # stdio JSON-RPC MCP server + 文档撰写工具
internal/templates/  # v2 文档模板
plugin/              # Claude Code 插件（hooks + MCP）与 feishu-session skill
examples/            # settings.json、.mcp.json、config.example.yaml、hook 输入样例
```

> 说明：计划文档里把各 hook 处理器放在 `internal/hooks/` 下；实现时为避免 import 循环，
> 把可运行的处理器集中到了 `internal/app/`，`internal/hooks/` 只保留纯粹的输入解析与输出格式化。

---

## 已知局限（real 后端）

- 文档内容以**纯文本段落块**写入 docx，不渲染 markdown 富文本。
- `UpdateDoc` 当前实现为**追加**而非整体替换（docx 整体替换需删除所有子块，留待 V2）。
- 文件夹重命名走 drive PATCH，部分租户不支持时**降级为不报错**，以免阻断会话。

---

## 非目标（V2 再做）

完整 context graph、原始 transcript 全量上传、复杂审批卡片、多维表格 session registry、
subagent branch graph、rewind/fork 图谱、常驻后台 daemon、全局记忆晋升、企业级 marketplace 分发。

后续将补齐的命令：`ccfl artifact upload`、`ccfl approval request`。（`ccfl mcp` 已实现）

---

## 实现原则

1. 本地优先，飞书做 checkpoint。
2. 飞书存摘要，不存原始事件流。
3. dry-run 与 mock 必须先于 real 可用。
4. 风险策略必须确定性。
5. 测试完整性规则在工具执行前强制生效。
6. 模板必须人类可读。
7. 本地状态可恢复、可检视。
8. 不搞隐式魔法：每次同步都有明确原因。
