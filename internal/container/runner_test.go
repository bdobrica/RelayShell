package container
package container

import (
	"strings"
	"testing"
)

func TestBuildRunArgs_Phase7OptionsAndEnvKeysOnly(t *testing.T) {
	t.Parallel()

	args := buildRunArgs("/tmp/workspace", StartOptions{
		SessionID:   "sess-123",
		Image:       "relayshell-codex:latest",
		Command:     "codex --no-alt-screen",
		Env:         map[string]string{"OPENAI_API_KEY": "secret-value", "GH_TOKEN": "token-value"},
		RunAsUser:   "1000:1000",
		CPULimit:    "1.5",
		MemoryLimit: "2g",
		Network:     "none",
	})

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--user 1000:1000") {
		t.Fatalf("args missing user flag: %q", joined)
	}
	if !strings.Contains(joined, "--cpus 1.5") {
		t.Fatalf("args missing cpu flag: %q", joined)
	}
	if !strings.Contains(joined, "--memory 2g") {
		t.Fatalf("args missing memory flag: %q", joined)
	}
	if !strings.Contains(joined, "--network none") {
		t.Fatalf("args missing network flag: %q", joined)
	}

	if strings.Contains(joined, "secret-value") || strings.Contains(joined, "token-value") {
		t.Fatalf("args should not include secret values: %q", joined)
	}
	if !strings.Contains(joined, "-e OPENAI_API_KEY") || !strings.Contains(joined, "-e GH_TOKEN") {
		t.Fatalf("args missing env key flags: %q", joined)
	}
}

func TestMergedCommandEnv_IncludesProvidedValues(t *testing.T) {
	t.Parallel()

	env := mergedCommandEnv(map[string]string{"OPENAI_API_KEY": "secret"})
	found := false
	for _, item := range env {
		if item == "OPENAI_API_KEY=secret" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("mergedCommandEnv() did not include provided key/value")
	}
}
