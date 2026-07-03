package knowledger

import "embed"

//go:embed README.md .mcp.json .claude-plugin/plugin.json .claude-plugin/marketplace.json skills/knowledger/SKILL.md skills/git-knowledge/SKILL.md skills/update-knowledger/SKILL.md skills/kb-code-review/SKILL.md hooks/hooks.json hooks/precheck hooks/git-sync hooks/code-review-precheck hooks/code-review-stop hooks/edit-tracker
var Bundle embed.FS
