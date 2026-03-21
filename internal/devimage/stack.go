package devimage

import (
	"os"
	"path/filepath"
	"strings"
)

type Stack string

const (
	StackUnknown Stack = "unknown"
	StackGo      Stack = "go"
	StackPython  Stack = "python"
	StackNode    Stack = "node"
	StackMixed   Stack = "mixed"
)

func DetectStack(workspaceDir string) (Stack, error) {
	trimmed := strings.TrimSpace(workspaceDir)
	if trimmed == "" {
		return StackUnknown, nil
	}

	hasGo, err := exists(filepath.Join(trimmed, "go.mod"))
	if err != nil {
		return StackUnknown, err
	}
	hasNode, err := exists(filepath.Join(trimmed, "package.json"))
	if err != nil {
		return StackUnknown, err
	}
	hasPython, err := hasPythonSignals(trimmed)
	if err != nil {
		return StackUnknown, err
	}

	count := 0
	if hasGo {
		count++
	}
	if hasNode {
		count++
	}
	if hasPython {
		count++
	}

	switch {
	case count == 0:
		return StackUnknown, nil
	case count > 1:
		return StackMixed, nil
	case hasGo:
		return StackGo, nil
	case hasPython:
		return StackPython, nil
	case hasNode:
		return StackNode, nil
	default:
		return StackUnknown, nil
	}
}

func hasPythonSignals(workspaceDir string) (bool, error) {
	markers := []string{
		"pyproject.toml",
		"setup.py",
		"setup.cfg",
		"Pipfile",
		"requirements.txt",
	}
	for _, marker := range markers {
		ok, err := exists(filepath.Join(workspaceDir, marker))
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	matches, err := filepath.Glob(filepath.Join(workspaceDir, "requirements*.txt"))
	if err != nil {
		return false, err
	}
	if len(matches) > 0 {
		return true, nil
	}

	return false, nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
