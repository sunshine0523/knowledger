# pi-knowledger

A thin pi package that exposes the local-first Knowledger CLI as native pi tools and adds retrieval/capture guidance as an Agent Skill.

The package does not duplicate storage or indexing. The Go `knowledger` binary remains the source of truth.

## Prerequisite

Install Knowledger and verify it is available:

```bash
knowledger version
```

To use a non-default binary or config file:

```bash
export KNOWLEDGER_BIN=/absolute/path/to/knowledger
export KNOWLEDGER_CONFIG=/absolute/path/to/knowledger.yaml
```

## Install

From this repository:

```bash
pi install ./plugins/pi-knowledger
```

Install for only the current project:

```bash
pi install -l ./plugins/pi-knowledger
```

Try it without changing settings:

```bash
pi -e ./plugins/pi-knowledger
```

After publishing to npm:

```bash
pi install npm:@kindbrave/pi-knowledger
```

## Tools

- `search_knowledge`
- `get_knowledge_item`
- `list_knowledge_items`
- `list_knowledge_bases`
- `add_knowledge_item`
- `delete_knowledge_item`
- `index_knowledge`

Use `/knowledger-status` in pi to check the configured executable.

The extension runs Knowledger in pi's current working directory, so Knowledger's project/global scope auto-detection continues to work. Every scoped tool also accepts an explicit `project` or `global` scope.

## Package

Inspect the npm tarball before publishing:

```bash
cd plugins/pi-knowledger
npm pack --dry-run
```
