package sessions
package sessions

import "testing"

func TestParseCommand_DiffWithoutPath(t *testing.T) {
	t.Parallel()

	cmd, err := ParseCommand("/diff")
	if err != nil {
		t.Fatalf("ParseCommand() error = %v", err)
	}
	if cmd.Name != CommandDiff {
		t.Fatalf("Name = %q, want %q", cmd.Name, CommandDiff)
	}
	if cmd.Path != "" {
		t.Fatalf("Path = %q, want empty", cmd.Path)
	}
}

func TestParseCommand_DiffWithPath(t *testing.T) {
	t.Parallel()

	cmd, err := ParseCommand("/diff internal/gitops/workspace.go")
	if err != nil {
		t.Fatalf("ParseCommand() error = %v", err)
	}
	if cmd.Name != CommandDiff {
		t.Fatalf("Name = %q, want %q", cmd.Name, CommandDiff)
	}
	if cmd.Path != "internal/gitops/workspace.go" {
		t.Fatalf("Path = %q, want internal/gitops/workspace.go", cmd.Path)
	}
}

func TestParseCommand_TreeAndPush(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"/tree", "/push"} {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			cmd, err := ParseCommand(input)
			if err != nil {
				t.Fatalf("ParseCommand(%q) error = %v", input, err)
			}
			if cmd.Name != CommandName(input[1:]) {
				t.Fatalf("Name = %q, want %q", cmd.Name, input[1:])
			}
		})
	}
}
