package devimage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectStack(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  Stack
	}{
		{name: "go", setup: touch("go.mod"), want: StackGo},
		{name: "python-requirements", setup: touch("requirements.txt"), want: StackPython},
		{name: "python-pyproject", setup: touch("pyproject.toml"), want: StackPython},
		{name: "node", setup: touch("package.json"), want: StackNode},
		{name: "mixed", setup: multiTouch("go.mod", "package.json"), want: StackMixed},
		{name: "unknown", setup: func(t *testing.T, dir string) {}, want: StackUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			got, err := DetectStack(dir)
			if err != nil {
				t.Fatalf("DetectStack() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("DetectStack() = %q, want %q", got, tt.want)
			}
		})
	}
}

func touch(name string) func(t *testing.T, dir string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func multiTouch(names ...string) func(t *testing.T, dir string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		for _, name := range names {
			touch(name)(t, dir)
		}
	}
}
