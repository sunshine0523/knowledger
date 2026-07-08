package knowledger_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitSyncHookPassesScopeToPullAndIndex(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	logPath := filepath.Join(tmp, "calls.log")
	fakeKnowledger := filepath.Join(binDir, "knowledger")
	script := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$KNOWLEDGER_CALL_LOG"
case "$1" in
  kb-git-knowledge-list)
    printf '[{"scope":"global","id":"docs"},{"scope":"project","id":"proj"}]\n'
    ;;
  kb-git-knowledge-pull)
    printf 'Updating %s\n' "$5"
    ;;
  index)
    printf '{"results":[]}\n'
    ;;
esac
`
	if err := os.WriteFile(fakeKnowledger, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake knowledger: %v", err)
	}

	cmd := exec.Command("bash", filepath.Join("hooks", "git-sync"))
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"KNOWLEDGER_CALL_LOG="+logPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git-sync failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read calls log: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"kb-git-knowledge-pull --scope global --id docs",
		"index --scope global --kb docs --quiet",
		"kb-git-knowledge-pull --scope project --id proj",
		"index --scope project --kb proj --quiet",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing call %q\ncalls:\n%s\nhook output:\n%s", want, got, out)
		}
	}
}
