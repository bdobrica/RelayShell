package devimage

import (
	_ "embed"
)

//go:embed templates/Dockerfile.dev.tmpl
var dockerfileTemplateSource string

const defaultDerivedBaseImage = "relayshell-codex:latest"

func RenderDockerfile() string {
	return dockerfileTemplateSource
}

func buildArgsForStack(stack Stack) []string {
	args := []string{
		"--build-arg", "BASE_IMAGE=" + defaultDerivedBaseImage,
		"--build-arg", "ENABLE_GO=0",
		"--build-arg", "ENABLE_PYTHON=0",
		"--build-arg", "ENABLE_NODEJS=0",
	}

	switch stack {
	case StackGo:
		args = enableBuildArg(args, "ENABLE_GO")
	case StackPython:
		args = enableBuildArg(args, "ENABLE_PYTHON")
	case StackNode:
		args = enableBuildArg(args, "ENABLE_NODEJS")
	case StackMixed:
		args = enableBuildArg(args, "ENABLE_GO")
		args = enableBuildArg(args, "ENABLE_PYTHON")
		args = enableBuildArg(args, "ENABLE_NODEJS")
	case StackUnknown:
		fallthrough
	default:
		return args
	}

	return args
}

func enableBuildArg(args []string, name string) []string {
	for i := 0; i < len(args)-1; i += 2 {
		if args[i] == "--build-arg" && args[i+1] == name+"=0" {
			args[i+1] = name + "=1"
			return args
		}
	}
	return append(args, "--build-arg", name+"=1")
}
