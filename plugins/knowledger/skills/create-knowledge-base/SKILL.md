---
name: create-knowledge-base
description: Use when the user asks to create, initialize, or register a Knowledger knowledge base, including a specification knowledge base.
version: 1.1.0
triggers:
  - "创建知识库"
  - "新建知识库"
  - "初始化知识库"
  - "创建规范库"
  - "创建规则库"
  - "create knowledge base"
  - "create specification"
---

# Create Knowledge Bases

Knowledger has one knowledge-base model. Set `type` to `knowledge` for durable
notes and references, or `specification` for rules and conventions. Do not use
separate specification tools or storage files.

Prefer the MCP tools; use the CLI when MCP is unavailable. Do not edit
`.knowledger/registry.json`, `knowledger.yaml`, or other storage files directly.

## Workflow

1. Call `list_knowledge_bases` first when the ID or target may already exist.
2. Call `create_knowledge_base` with:
   - `id`
   - `type`: `knowledge` or `specification` (defaults to `knowledge`)
   - `store_type`: `text` or `sqlite`
   - explicit `scope` when the user supplied one
3. For a project-scoped KB, pass `semantic_enabled: false` unless the user
   explicitly requests semantic indexing, and do not call `index_knowledge`
   automatically.
4. For a Git-backed KB, use `git_knowledge_add` or
   `knowledger kb-git-knowledge-add`; pass `type: specification` when the
   repository contains rules.
5. Verify the returned `id`, `scope`, `type`, store type, and indexing state.

MCP example:

```json
{
  "scope": "project",
  "id": "project-rules",
  "type": "specification",
  "name": "Project Rules",
  "store_type": "sqlite",
  "semantic_enabled": false
}
```

CLI example:

```sh
knowledger --scope project kb-create \
  --id project-rules \
  --name "Project Rules" \
  --type specification \
  --store-type sqlite \
  --semantic-enabled=false
```

