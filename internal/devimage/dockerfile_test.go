package devimage

import (
	"strings"
	"testing"
)

func TestRenderDockerfileIncludesBaseImageAndPackageManagerFallback(t *testing.T) {
	out, err := RenderDockerfile(StackGo)
	if err != nil {
		t.Fatalf("RenderDockerfile() error = %v", err)
	}

	mustContain := []string{
		"# syntax=docker/dockerfile:1.7",
		"FROM relayshell-codex:latest",
		"Common tools for all session images",
		"Go stack tools",
		"--mount=type=cache,id=relayshell-apk-cache,target=/var/cache/apk",
		"--mount=type=cache,id=relayshell-apt-cache,target=/var/cache/apt,sharing=locked",
		"--mount=type=cache,id=relayshell-apt-lists,target=/var/lib/apt/lists,sharing=locked",
		"command -v apk",
		"command -v apt-get",
		"go",
		"python3",
		"make",
	}

	for _, token := range mustContain {
		if !strings.Contains(out, token) {
			t.Fatalf("RenderDockerfile() missing token %q", token)
		}
	}
}

func TestRenderDockerfile_LanguageBlocks(t *testing.T) {
	out, err := RenderDockerfile(StackPython)
	if err != nil {
		t.Fatalf("RenderDockerfile() error = %v", err)
	}

	if !strings.Contains(out, "Python stack tools") {
		t.Fatal("expected Python stack block in rendered template")
	}
	if strings.Contains(out, "Go stack tools") {
		t.Fatal("did not expect Go stack block for python-only stack")
	}
	if strings.Contains(out, "Node.js stack tools") {
		t.Fatal("did not expect Node.js stack block for python-only stack")
	}
}
