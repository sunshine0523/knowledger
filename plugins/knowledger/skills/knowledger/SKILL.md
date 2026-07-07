---
name: knowledger
description: MUST invoke BEFORE answering any question ("how do I", "what is", "how does X work", "what's our convention"), writing code, writing a design/proposal/plan, debugging, or making a technical recommendation — even if the answer seems obvious. Search Knowledger first; project decisions and conventions often live here and nowhere else. MUST also invoke for capture when the user says save/remember/capture/记一下/保存到知识库. Skip ONLY for pure greetings/acknowledgements, trivial single-line typo fixes, and pure git/file operations with no design decisions involved.
version: 3.0.0
triggers:
  # English — recall / lookup
  - "remember"
  - "do you remember"
  - "recall"
  - "we discussed"
  - "we talked about"
  - "last time"
  - "previously"
  - "as I mentioned"
  - "mentioned before"
  - "did we save"
  - "is there a note"
  - "what did we decide"
  - "from our last session"
  # English — capture
  - "save this"
  - "save that"
  - "capture this"
  - "note this"
  - "store this"
  - "remember this"
  - "add to knowledge"
  - "save to knowledger"
  # English — meta
  - "knowledger"
  - "knowledge base"
  - "knowledge bases"
  # 中文 — 回忆 / 查询
  - "记得"
  - "还记得"
  - "记不记得"
  - "我们之前"
  - "上次"
  - "之前提过"
  - "之前提到"
  - "之前说过"
  - "之前讨论"
  - "之前的决定"
  - "查一下知识库"
  - "查知识库"
  - "知识库里"
  - "有没有记录"
  - "有没有保存"
  # 中文 — 捕获
  - "记一下"
  - "记下来"
  - "记下"
  - "保存到知识库"
  - "存到知识库"
  - "添加到知识库"
  - "存一下知识库"
  - "归档到知识库"
  # 中文 — meta
  - "知识库"
---

# Knowledger

Knowledger 通过 MCP 暴露两条独立通道：

- **规范（Specs / rules）** — 通过 `list_spec_rules` **全量**强制加载的可执行约定。
- **知识（Knowledge / KBs）** — 通过 `search_knowledge` + `get_knowledge_item` 按相关性检索的笔记/决策/参考。

跳过任何一条，都意味着用项目专属规则未读或项目决策未获取的状态作答。

## The Rule

**在回答、写代码、设计、或给出技术建议之前**，派发一个 subagent **同时**执行两条通道：

1. `list_spec_rules` — 不做过滤地拉取**所有**可执行规则，注入主上下文。
2. `search_knowledge` — 按任务类型构造 1..N 个查询，检索相关知识条目。

主 agent 必须等待 subagent 返回 `{rules, knowledge}` 结构体后才能继续任何其它工作。

**升级检查**：本 skill 需要 knowledger v0.2.x+（提供 `list_spec_rules`）。Session 首次调用时，若 `list_spec_rules` 工具不存在，告知用户运行 `knowledger update` 升级，然后停止——不静默回退旧协议。

## Red Flags

These thoughts mean STOP — you are rationalizing:

| Thought | Reality |
|---------|---------|
| "I know this from training" | Generic knowledge ≠ project knowledge. Scan KBs first. |
| "The repo will tell me" | Conventions and decisions are often NOT in the repo. Scan. |
| "Simple coding task" | Simple tasks have project-specific conventions. Scan. |
| "Quick question" | Quick questions have saved answers. Scan. |
| "I'll search if I need to later" | You won't. Scan BEFORE answering. |
| "No obvious KB topic" | Weak signal is not zero signal. Scan. |
| "I already know the answer" | The KB may contradict or refine it. Scan. |
| "The user didn't mention KB" | Users never say "check the KB" — that's your job. |
| "This is just a clarification" | Clarifications shape implementation. Scan first. |

## Task Classification

在派发 subagent 前，将当前请求分类为以下三类之一。**分类只影响 `search_knowledge` 的查询数量和措辞；规范永远全量加载，不受分类影响。** 主 agent 必须将分类结果和具体查询列表传给 subagent。

| 任务类型 | 信号词 | `search_knowledge` 策略 |
|---|---|---|
| **Daily Q&A** | "how do I"、"what is"、"记得"、"上次"、一般问答 | 1 个查询——用户问题浓缩为一次检索 |
| **Technical design** | "design"、"propose"、"架构"、"方案"、"怎么设计" | **N 个查询**——按子话题拆分：架构决策/历史方案/领域约束各一次 |
| **Coding task** | "implement"、"fix"、"refactor"、"写代码"、"实现" | **N 个查询**——按涉及模块/文件/库各查一次；宁多勿缺 |

## Subagent Retrieval Protocol

**使命**：全量拉取规则 + 检索相关知识，将两者返回主 agent。禁止回答用户、修改代码、调用其它 skill、或继续用户任务。

**主 agent 传入**：
- `task_type`：`"daily_qa"` | `"design"` | `"coding"`
- `search_queries`：字符串数组（主 agent 根据分类构造）

**步骤**：
1. 调用 `list_spec_rules`（不加 scope 过滤）。原样捕获 `specifications` 和 `rule_sets`。
2. 对每个 `search_queries` 中的查询，调用 `search_knowledge`（`search_mode=auto`，`limit=10`），合并命中结果。
3. 对每个唯一命中（score ≥ 0.3 或 title/tag 明显相关），调用 `get_knowledge_item` 拉完整正文。
4. 若命中集合为空 **且** `task_type` 为 `"design"` 或 `"coding"`：回退到 `list_knowledge_bases` + `list_knowledge_items`，按 title/tag 筛选可能相关的条目，再 `get_knowledge_item`。
5. **禁止**用 Read/grep/cat 直接读 KB 文件。**禁止**调用 `run_lint`。**禁止**调用任何 skill。

**返回格式**（纯 JSON，无 prose，无 markdown fence）：

```json
{
  "rules": {
    "specifications": [...],
    "rule_sets": [...]
  },
  "knowledge": [
    {
      "kb_id": "...",
      "item_id": "...",
      "scope": "project|global",
      "title": "...",
      "tags": [...],
      "content": "...",
      "why_relevant": "一行说明与当前任务的关联"
    }
  ]
}
```

### Subagent Boundaries — Hard Rules

The subagent has ONE job: **retrieve relevant knowledge and return it to the main agent.** It must NOT:

- Answer the user's question.
- Write, modify, or review code.
- Make design recommendations or technical decisions.
- Invoke other skills, tools, or agents (e.g. superpowers, frontend, git-master).
- Continue the user's task in any form.

The subagent returns knowledge ONLY. The main agent resumes the user's task after retrieval completes.

### Never Read KB Files Directly

**NEVER read knowledge-base files directly with Read, grep, cat, or any file tool.** Knowledger backs its KBs in SQLite, text directories, Chroma vectors, and registry files — none of these are stable surfaces for direct reads. The only correct way to read knowledge is through the knowledger MCP tools:

- `list_knowledge_bases`
- `list_knowledge_items`
- `search_knowledge`
- `get_knowledge_item`

Direct file reads bypass indexing, miss semantic matches, can return stale or partial content, and may corrupt concurrent writes. This applies to the main agent AND to every dispatched subagent.

## 禁止通过 `search_knowledge` 检索规范

规范走 spec 通道。用 `search_knowledge` 找规范，会漏掉单条查询打分不高但确实适用的规则——这正是 `list_spec_rules` 全量加载的原因。对称地，不要把知识 KB 塞给 `list_spec_rules`——知识是检索/浏览的，不是强制执行的。

## Inject and Apply

**规则（rules）**：视为强制性项目约束。与用户意图或仓库状态冲突时，必须 surface 冲突——不静默丢弃。

**知识（knowledge）**：视为参考上下文。与仓库/用户指令冲突时，surface 冲突。标注来源 KB 和条目 ID。

## Retrieval First, Then Continue Other Work

**Knowledge retrieval MUST complete BEFORE the main agent starts any other task.** Do not begin coding, designing, answering, or delegating implementation work until the retrieval subagent has returned and the retrieved rules and knowledge are injected into context.

However — **retrieval completion is NOT task completion.** Do not forget the remaining steps of the user's request. After retrieval, the main agent must still carry out whatever work the user asked for, and must still invoke other skills/tools/agents when the situation calls for it.

Example flow when the user also has the `superpowers` skill installed:

```
1. User asks for a coding task.
2. Main agent classifies: "coding task" → coding-convention KB category.
3. Main agent dispatches retrieval subagent with task type + search queries.
4. WAIT for subagent to return retrieved knowledge. Do not start coding yet.
5. Inject retrieved knowledge into context.
6. NOW begin the actual work — if the task matches a superpowers trigger (e.g. "implement a feature"), invoke superpowers at this point, with the retrieved conventions already in context so superpowers-guided code follows them.
```

The ordering is: **classify → dispatch retrieval subagent → wait → inject rules and knowledge → continue the user's real task (including calling other skills like superpowers, frontend, debugging, etc. as needed).**

Never skip step 4. Never skip step 6.

## Capture Durable Knowledge

Perform capture when the user provides:
- A project decision, convention, or reusable note.
- A stable external reference and why it matters.
- Explicit capture intent: "remember this", "save this", "记一下", "保存到知识库".

Before `add_knowledge_item`, confirm the target KB. If unclear, call `list_knowledge_bases` and ask.

## Never Capture

- Secrets, credentials, private tokens, API keys.
- One-off task state, temp logs, stack traces, command output.
- Anything already derivable from the repo or git history.

## Skip Only For

- Pure greetings or acknowledgements with zero task content.
- The immediately preceding assistant message already ran the full KB scan for the same topic.
- The user explicitly says "skip knowledger" / "不用查知识库".

Do not narrate the scan to the user — dispatch the subagent silently, then answer.
