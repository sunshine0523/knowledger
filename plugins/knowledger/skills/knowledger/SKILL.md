---
name: knowledger
description: Retrieve durable project or global knowledge and specification knowledge bases before answering or changing code. Use before work affected by project context. All retrieval must run in one dedicated subagent; the root agent must never call retrieval tools.
---

# Knowledger

Knowledger exposes one knowledge-base model. Each KB has `type: knowledge` or
`type: specification`; the latter contains enforceable rules and conventions.

## Retrieval before work

For coding, design, debugging, or questions affected by project context, the
root agent must use this delegation contract:

1. Spawn exactly one dedicated retrieval subagent with the host's subagent tool
   (`spawn_agent` in Codex or `Agent` in Claude Code).
2. Tell the subagent that it is already the delegated retrieval worker, must
   execute the workflow below directly, must not delegate again, and must not
   edit files or continue the user's task.
3. Wait for the subagent to return a concise summary of applicable
   specifications and knowledge items before continuing.

The root agent MUST NOT call `list_knowledge_bases`, `list_knowledge_items`,
`search_knowledge`, or `get_knowledge_item`. All retrieval tool calls must run
inside the dedicated subagent. If the host cannot create subagents, report that
retrieval is blocked; never fall back to running those tools in the root agent.

Inside the dedicated retrieval subagent:

1. Call `list_knowledge_bases`.
2. For every enabled `type: specification` KB, call `list_knowledge_items` with
   enough pagination to cover all items, then call `get_knowledge_item` for
   every item. Specification KBs are loaded exhaustively; never use relevance
   search as a substitute.
3. Search enabled `type: knowledge` KBs with `search_knowledge`, then fetch
   applicable hits with `get_knowledge_item`.

Treat specification items as mandatory project constraints. Treat knowledge
items as contextual references, and surface conflicts with the user request or
repository state.

Do not read SQLite, text, registry, or vector files directly. Do not invoke
removed specification tools such as `list_spec_rules`, `add_specification`, or
`run_lint`; they no longer exist.

## Capture

Use `add_knowledge_item` only when the user explicitly asks to save, remember,
capture, or add information. When the target KB is ambiguous, use the same
dedicated retrieval-subagent boundary to list choices, then confirm the target.

Never store secrets, credentials, temporary task state, or facts derivable from
the repository and git history.
