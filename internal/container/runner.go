package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

type Runner struct {
	Runtime string
	Logger  *slog.Logger
}

func NewRunner(runtime string, logger *slog.Logger) *Runner {
	return &Runner{Runtime: runtime, Logger: logger}
}

type StartOptions struct {
	SessionID    string
	WorkspaceDir string
	Image        string
	Command      string
	Env          map[string]string
}

type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	done   chan error
	once   sync.Once
}

func (r *Runner) Start(ctx context.Context, options StartOptions) (*Process, error) {
	absWorkspaceDir, err := filepath.Abs(options.WorkspaceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	args := []string{
		"run",
		"--rm",
		"-i",
		"--name",
		"relayshell-" + options.SessionID,
		"-v",
		fmt.Sprintf("%s:/workspace", absWorkspaceDir),
		"-w",
		"/workspace",
	}

	for _, envVar := range sortedEnvFlags(options.Env) {
		args = append(args, "-e", envVar)
	}

	args = append(args,
		options.Image,
		"sh",
		"-lc",
		options.Command,
	)

	cmd := exec.CommandContext(ctx, r.Runtime, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("container stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("container stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("container stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start container process: %w", err)
	}

	process := &Process{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		done:   make(chan error, 1),
	}

	go func() {
		process.done <- cmd.Wait()
		close(process.done)
	}()

	return process, nil
}

func sortedEnvFlags(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return result
}

func (p *Process) WriteInput(input string) error {
	if p.stdin == nil {
		return errors.New("stdin is not available")
	}
	_, err := io.WriteString(p.stdin, input+"\r\n")
	return err
}

func (p *Process) Stdout() io.Reader {
	return p.stdout
}

func (p *Process) Stderr() io.Reader {
	return p.stderr
}

func (p *Process) Done() <-chan error {
	return p.done
}

func (p *Process) Stop() error {
	var stopErr error
	p.once.Do(func() {
		if p.cmd == nil || p.cmd.Process == nil {
			return
		}

		_ = p.cmd.Process.Signal(syscall.SIGTERM)

		select {
		case <-time.After(5 * time.Second):
			_ = p.cmd.Process.Kill()
			if err := <-p.done; err != nil && !errors.Is(err, context.Canceled) {
				stopErr = err
			}
		case err := <-p.done:
			if err != nil && !errors.Is(err, context.Canceled) {
				stopErr = err
			}
		}
	})
	return stopErr
}
