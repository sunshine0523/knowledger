# Claude Code Knowledger Plugin

This plugin connects Claude Code to Knowledger through the existing `knowledger mcp` command and adds skill instructions for better retrieval and capture timing.

The plugin is intentionally thin. Knowledger's Go binary remains the source of truth for storage, indexing, search, and MCP tool behavior.

## What It Provides

- A Claude Code plugin manifest at `.claude-plugin/plugin.json`.
- An MCP server configuration named `knowledger` that runs `knowledger mcp`.
- A `knowledger` skill that tells Claude when to search saved knowledge and when to propose durable capture.

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

The MCP server exposes two retrieval channels plus management tools:

Spec rules:

- `list_spec_rules`
- `list_specifications`
- `run_lint`
- `add_specification`
- `delete_specification`
- `add_rule_to_spec`

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
- `spec_git_add`
- `spec_git_pull`
- `spec_git_list`

## Safety

Claude may search Knowledger automatically when the skill trigger is strong. Writing knowledge should happen only when the user explicitly asks for capture or clearly confirms it.

Do not store secrets, credentials, private tokens, one-off task state, or information that is already derivable from the repository.

## Troubleshooting

If Claude Code cannot start the MCP server, check:

1. `command -v knowledger` prints an executable path.
2. `knowledger mcp` starts without errors in the same shell.
3. At least one knowledge base is configured or the default local storage can be initialized.
4. The plugin validates with `claude plugin validate ./plugins/knowledger`.

If the MCP server starts but retrieval is poor, verify the target knowledge base contains relevant items and try a lexical search through the Knowledger CLI first.
