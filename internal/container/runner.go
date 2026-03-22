package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty/v2"
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
	RunAsUser    string
	CPULimit     string
	MemoryLimit  string
	Network      string
}

type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	ptmx   *os.File
	done   chan error
	once   sync.Once

	stateMu sync.RWMutex
	exited  bool
	exitErr error
}

var (
	ErrProcessExited = errors.New("process exited")
	ErrBrokenPipe    = errors.New("broken pipe")
)

func (r *Runner) Start(ctx context.Context, options StartOptions) (*Process, error) {
	absWorkspaceDir, err := filepath.Abs(options.WorkspaceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	args := buildRunArgs(absWorkspaceDir, options)

	proc, err := r.startWithPTY(ctx, args, options.Env)
	if err == nil {
		return proc, nil
	}

	r.Logger.Warn("PTY startup failed, falling back to pipe mode", "error", err)

	pipeArgs := make([]string, len(args))
	copy(pipeArgs, args)
	pipeArgs = removeArg(pipeArgs, "-t")

	return r.startWithPipes(ctx, pipeArgs, options.Env)
}

func buildRunArgs(absWorkspaceDir string, options StartOptions) []string {
	args := []string{
		"run",
		"--rm",
		"-i",
		"-t",
		"--name",
		"relayshell-" + options.SessionID,
		"-v",
		fmt.Sprintf("%s:/workspace", absWorkspaceDir),
		"-w",
		"/workspace",
	}

	if trimmed := strings.TrimSpace(options.RunAsUser); trimmed != "" {
		args = append(args, "--user", trimmed)
	}
	if trimmed := strings.TrimSpace(options.CPULimit); trimmed != "" {
		args = append(args, "--cpus", trimmed)
	}
	if trimmed := strings.TrimSpace(options.MemoryLimit); trimmed != "" {
		args = append(args, "--memory", trimmed)
	}
	if trimmed := strings.TrimSpace(options.Network); trimmed != "" {
		args = append(args, "--network", trimmed)
	}

	for _, envVar := range sortedEnvKeys(options.Env) {
		args = append(args, "-e", envVar)
	}

	args = append(args,
		options.Image,
		"sh",
		"-lc",
		options.Command,
	)

	return args
}

func (r *Runner) startWithPTY(ctx context.Context, args []string, env map[string]string) (*Process, error) {
	cmd := exec.CommandContext(ctx, r.Runtime, args...)
	cmd.Env = mergedCommandEnv(env)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start PTY process: %w", err)
	}

	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120})

	process := &Process{
		cmd:    cmd,
		stdin:  ptmx,
		stdout: ptmx,
		stderr: io.NopCloser(strings.NewReader("")),
		ptmx:   ptmx,
		done:   make(chan error, 1),
	}

	go func() {
		err := cmd.Wait()
		process.setExit(err)
		process.done <- err
		close(process.done)
	}()

	return process, nil
}

func (r *Runner) startWithPipes(ctx context.Context, args []string, env map[string]string) (*Process, error) {
	cmd := exec.CommandContext(ctx, r.Runtime, args...)
	cmd.Env = mergedCommandEnv(env)

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
		err := cmd.Wait()
		process.setExit(err)
		process.done <- err
		close(process.done)
	}()

	return process, nil
}

func removeArg(args []string, target string) []string {
	result := make([]string, 0, len(args))
	removed := false
	for _, arg := range args {
		if !removed && arg == target {
			removed = true
			continue
		}
		result = append(result, arg)
	}
	return result
}

func sortedEnvKeys(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

func mergedCommandEnv(values map[string]string) []string {
	merged := append([]string{}, os.Environ()...)
	for _, key := range sortedEnvKeys(values) {
		merged = append(merged, key+"="+values[key])
	}
	return merged
}

func (p *Process) WriteInput(input string) error {
	if p.stdin == nil {
		return errors.New("stdin is not available")
	}
	if exited, err := p.exitStatus(); exited {
		if err == nil {
			return ErrProcessExited
		}
		return fmt.Errorf("%w: %v", ErrProcessExited, err)
	}
	if input != "" {
		if _, err := io.WriteString(p.stdin, input); err != nil {
			if isBrokenPipeError(err) {
				return fmt.Errorf("%w: %v", ErrBrokenPipe, err)
			}
			return err
		}
	}
	_, err := io.WriteString(p.stdin, "\r")
	if err != nil && isBrokenPipeError(err) {
		return fmt.Errorf("%w: %v", ErrBrokenPipe, err)
	}
	return err
}

func (p *Process) WriteRaw(input string) error {
	if p.stdin == nil {
		return errors.New("stdin is not available")
	}
	if exited, err := p.exitStatus(); exited {
		if err == nil {
			return ErrProcessExited
		}
		return fmt.Errorf("%w: %v", ErrProcessExited, err)
	}
	_, err := io.WriteString(p.stdin, input)
	if err != nil && isBrokenPipeError(err) {
		return fmt.Errorf("%w: %v", ErrBrokenPipe, err)
	}
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

func (p *Process) setExit(err error) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	p.exited = true
	p.exitErr = err
}

func (p *Process) exitStatus() (bool, error) {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.exited, p.exitErr
}

func isBrokenPipeError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, os.ErrClosed) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "broken pipe")
}

func (p *Process) Stop() error {
	var stopErr error
	p.once.Do(func() {
		if p.ptmx != nil {
			defer p.ptmx.Close()
		}

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
