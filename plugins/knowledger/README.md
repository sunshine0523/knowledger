# Knowledger Plugin for Codex and Claude Code

This plugin connects Codex or Claude Code to Knowledger through the existing `knowledger mcp` command and adds skill instructions for better retrieval and capture timing.

The plugin is intentionally thin. Knowledger's Go binary remains the source of truth for storage, indexing, search, and MCP tool behavior.

## What It Provides

- Plugin manifests for Codex and Claude Code.
- An MCP server configuration named `knowledger` that runs `knowledger mcp`.
- A `knowledger` skill that tells Claude when to search saved knowledge and when to propose durable capture.
- A lifecycle hook that syncs Git-backed knowledge when a session starts.

The retrieval skill is opt-in. Prefix a prompt with `kb` (for example,
`kb find the project's API conventions`) to trigger `knowledger:knowledger`.
Other prompts do not trigger the retrieval skill unless the user explicitly
requests Knowledger or knowledge-base retrieval.

## Prerequisites

Install or build the `knowledger` CLI before using the plugin. The first plugin version expects `knowledger` to be available on `PATH`.

Build from this repository:

```bash
go build -o knowledger ./cmd/knowledger
```

Install the CLI:

```bash
go install github.com/kindbrave/knowledger/cmd/knowledger@latest
```

Confirm the MCP server starts:

```bash
knowledger mcp
```

## Install Locally

Install into Codex with the Knowledger CLI:

```bash
knowledger install --codex
```

Verify the installation, then start a new Codex thread:

```bash
codex plugin list
```

Install into Claude Code:

```bash
knowledger install --claude
```

From a checkout of this repository, load the plugin for a single Claude Code session:

```bash
claude --plugin-dir ./plugins/knowledger
```

Validate the plugin structure:

```bash
claude plugin validate ./plugins/knowledger
```

Use strict validation in CI or release checks:

```bash
claude plugin validate --strict ./plugins/knowledger
```

## MCP Configuration

The plugin declares this MCP server (the key matches the binary name, `knowledger`):

```json
{
  "knowledger": {
    "command": "knowledger",
    "args": ["mcp"]
  }
}
```

If `knowledger` is not on `PATH`, install it or start Claude Code from a shell where the binary is available.

## Expected MCP Tools

The MCP server exposes one typed knowledge-base channel plus management tools.
Use `type: specification` when creating a rules/conventions KB. Load those KBs
with `list_knowledge_bases`, `list_knowledge_items`, and `get_knowledge_item`.

Knowledge retrieval and management:

- `search_knowledge`
- `get_knowledge_item`
- `list_knowledge_items`
- `add_knowledge_item`
- `delete_knowledge_item`
- `list_knowledge_bases`
- `create_knowledge_base`
- `delete_knowledge_base`
- `index_knowledge`

Git-backed knowledge and specs:

- `git_knowledge_add`
- `git_knowledge_pull`
- `git_knowledge_list`

## Safety

Claude may search Knowledger automatically when the skill trigger is strong. Writing knowledge should happen only when the user explicitly asks for capture or clearly confirms it.

Do not store secrets, credentials, private tokens, one-off task state, or information that is already derivable from the repository.

## Troubleshooting

If Codex or Claude Code cannot start the MCP server, check:

1. `command -v knowledger` prints an executable path.
2. `knowledger mcp` starts without errors in the same shell.
3. At least one knowledge base is configured or the default local storage can be initialized.
4. The plugin validates with `claude plugin validate ./plugins/knowledger`.

If the MCP server starts but retrieval is poor, verify the target knowledge base contains relevant items and try a lexical search through the Knowledger CLI first.
