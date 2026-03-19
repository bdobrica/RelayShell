package sessions

import (
	"fmt"
	"strings"
)

type CommandName string

const (
	CommandStart   CommandName = "start"
	CommandRestart CommandName = "restart"
	CommandExit    CommandName = "exit"
	CommandCommit  CommandName = "commit"
	CommandStatus  CommandName = "status"
)

type Command struct {
	Name   CommandName
	Repo   string
	Branch string
	Agent  string
}

// ParseCommand parses supported slash commands with strict argument validation.
func ParseCommand(input string) (Command, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Command{}, fmt.Errorf("empty command")
	}
	if !strings.HasPrefix(trimmed, "/") {
		return Command{}, fmt.Errorf("commands must start with '/'")
	}

	parts := strings.Fields(trimmed)
	name := strings.TrimPrefix(parts[0], "/")

	switch CommandName(name) {
	case CommandRestart, CommandExit, CommandCommit, CommandStatus:
		if len(parts) != 1 {
			return Command{}, fmt.Errorf("/%s does not accept arguments", name)
		}
		return Command{Name: CommandName(name)}, nil
	case CommandStart:
		if len(parts) != 4 {
			return Command{}, fmt.Errorf("/start requires exactly repo=<repo> branch=<branch> agent=<agent>")
		}

		args := map[string]string{}
		for _, kv := range parts[1:] {
			split := strings.SplitN(kv, "=", 2)
			if len(split) != 2 || split[0] == "" || split[1] == "" {
				return Command{}, fmt.Errorf("invalid argument %q", kv)
			}
			if _, exists := args[split[0]]; exists {
				return Command{}, fmt.Errorf("duplicate argument %q", split[0])
			}
			args[split[0]] = split[1]
		}

		repo, okRepo := args["repo"]
		branch, okBranch := args["branch"]
		agent, okAgent := args["agent"]
		if !okRepo || !okBranch || !okAgent || len(args) != 3 {
			return Command{}, fmt.Errorf("/start requires exactly repo=<repo> branch=<branch> agent=<agent>")
		}

		return Command{Name: CommandStart, Repo: repo, Branch: branch, Agent: agent}, nil
	default:
		return Command{}, fmt.Errorf("unsupported command %q", parts[0])
	}
}
