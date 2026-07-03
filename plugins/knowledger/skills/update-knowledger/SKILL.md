---
name: update-knowledger
description: Use when the user says "更新knowledger", "update knowledger", "升级knowledger", "刷新knowledger", or wants to update and reinstall the knowledger CLI and Claude Code plugin.
version: 2.0.0
triggers:
  - "更新knowledger"
  - "升级knowledger"
  - "刷新knowledger"
  - "update knowledger"
  - "update the knowledger"
  - "rebuild knowledger"
  - "reinstall knowledger"
  - "更新claude code插件"
  - "reinstall plugin"
---

# Update Knowledger

Updates the knowledger binary and reinstalls the Claude Code plugin in one command.

## Workflow

```
knowledger update
    ├── Fetches latest release from GitHub
    ├── Downloads binary for current OS/arch
    ├── Atomically replaces current binary
    └── Reinstalls Claude Code plugin automatically
```

### Step 1: Run update

```bash
knowledger update
```

This single command handles everything: binary self-update + plugin reinstall.

### Step 2: Verify

```bash
knowledger version
```

Report the version and confirm the update succeeded.

## Flags

| Flag | Description |
|------|-------------|
| `--check` | Check for updates without installing |
| `--skip-plugin` | Skip Claude Code plugin reinstall after binary update |

## Error Handling

- If GitHub is unreachable → check network connectivity
- If binary lacks write permission → run with `sudo knowledger update`
- If plugin reinstall fails after binary update → run `knowledger install --claude` manually
