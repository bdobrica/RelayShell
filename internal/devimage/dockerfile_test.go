package devimage

import (
	"strings"
	"testing"
)

func TestRenderDockerfileIncludesBaseImageAndPackageManagerFallback(t *testing.T) {
	out := RenderDockerfile()

	mustContain := []string{
		"# syntax=docker/dockerfile:1.7",
		"ARG BASE_IMAGE=relayshell-codex:latest",
		"FROM ${BASE_IMAGE}",
		"ARG ENABLE_GO=0",
		"ARG ENABLE_PYTHON=0",
		"ARG ENABLE_NODEJS=0",
		"Common tools for all session images",
		"Go stack tools",
		"if [ \"${ENABLE_GO}\" = \"1\" ]; then",
		"if [ \"${ENABLE_PYTHON}\" = \"1\" ]; then",
		"if [ \"${ENABLE_NODEJS}\" = \"1\" ]; then",
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

func TestBuildArgsForStack(t *testing.T) {
	tests := []struct {
		name       string
		stack      Stack
		expectedOn []string
	}{
		{
			name:       "go",
			stack:      StackGo,
			expectedOn: []string{"ENABLE_GO=1"},
		},
		{
			name:       "python",
			stack:      StackPython,
			expectedOn: []string{"ENABLE_PYTHON=1"},
		},
		{
			name:       "node",
			stack:      StackNode,
			expectedOn: []string{"ENABLE_NODEJS=1"},
		},
		{
			name:       "mixed",
			stack:      StackMixed,
			expectedOn: []string{"ENABLE_GO=1", "ENABLE_PYTHON=1", "ENABLE_NODEJS=1"},
		},
		{
			name:       "unknown",
			stack:      StackUnknown,
			expectedOn: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := strings.Join(buildArgsForStack(tt.stack), " ")
			if !strings.Contains(args, "BASE_IMAGE="+defaultDerivedBaseImage) {
				t.Fatal("expected BASE_IMAGE build arg")
			}

			for _, arg := range tt.expectedOn {
				if !strings.Contains(args, arg) {
					t.Fatalf("expected %q to be enabled, got args: %s", arg, args)
				}
			}

			for _, arg := range []string{"ENABLE_GO=0", "ENABLE_PYTHON=0", "ENABLE_NODEJS=0"} {
				if strings.Contains(args, strings.Replace(arg, "=0", "=1", 1)) {
					continue
				}
				if !strings.Contains(args, arg) {
					t.Fatalf("expected explicit default build arg %q, got args: %s", arg, args)
				}
			}
		})
	}
}
