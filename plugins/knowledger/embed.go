package knowledger

import "embed"

//go:embed README.md .mcp.json .claude-plugin/plugin.json .claude-plugin/marketplace.json .codex-plugin/plugin.json skills/knowledger/SKILL.md skills/git-knowledge/SKILL.md skills/update-knowledger/SKILL.md skills/kb-code-review/SKILL.md skills/create-knowledge-base/SKILL.md hooks/hooks.json hooks/precheck hooks/git-sync hooks/code-review-precheck
var Bundle embed.FS
