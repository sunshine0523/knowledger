package knowledger_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func runPrecheck(t *testing.T, input string) string {
	t.Helper()

	cmd := exec.Command("bash", "hooks/precheck")
	cmd.Stdin = strings.NewReader(input)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("precheck failed: %v\nstderr:\n%s", err, stderr.String())
	}
	return stdout.String()
}

func TestPrecheckHookOnlyInjectsKnowledgerForKBPrefixedPrompts(t *testing.T) {
	for _, tc := range []struct {
		name       string
		prompt     string
		shouldFire bool
	}{
		{name: "kb prefix", prompt: "kb 查找项目约定", shouldFire: true},
		{name: "ordinary prompt", prompt: "查找项目约定", shouldFire: false},
		{name: "prefix is not first", prompt: "please kb 查找", shouldFire: false},
		{name: "uppercase prefix", prompt: "KB 查找", shouldFire: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]string{"prompt": tc.prompt})
			if err != nil {
				t.Fatalf("marshal prompt: %v", err)
			}
			out := runPrecheck(t, string(payload))
			fired := strings.Contains(out, "knowledger:knowledger")
			if fired != tc.shouldFire {
				t.Fatalf("prompt %q fired=%v, want %v; output=%s", tc.prompt, fired, tc.shouldFire, out)
			}
		})
	}
}
