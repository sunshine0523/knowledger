---
name: knowledger
description: Retrieve durable project or global knowledge and specification knowledge bases before answering or changing code.
version: 3.1.0
triggers:
  - "knowledger"
  - "knowledge base"
  - "知识库"
  - "查知识库"
  - "记一下"
  - "保存到知识库"
---

# Knowledger

Knowledger exposes one knowledge-base model. Each KB has `type: knowledge` or
`type: specification`; the latter contains enforceable rules and conventions.

## Retrieval before work

For coding, design, debugging, or questions affected by project context:

1. Call `list_knowledge_bases`.
2. For every enabled `type: specification` KB, call `list_knowledge_items` with
   enough pagination to cover all items, then call `get_knowledge_item` for each
   item that may apply. Specification KBs are loaded exhaustively; never use
   relevance search as a substitute.
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
capture, or add information. Confirm the target KB when it is ambiguous.

Never store secrets, credentials, temporary task state, or facts derivable from
the repository and git history.
