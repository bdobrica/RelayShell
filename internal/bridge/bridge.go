package bridge

import (
	"context"
	"io"
	"log/slog"
	"regexp"
	"strings"

	"github.com/bdobrica/RelayShell/internal/container"
)

type MatrixSender interface {
	SendText(ctx context.Context, roomID, body string) error
}

var (
	ansiOSCRegexp    = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	ansiCSIRegexp    = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansiSingleRegexp = regexp.MustCompile(`\x1b[@-_]`)
)

type Bridge struct {
	logger *slog.Logger
	sender MatrixSender
	roomID string
	proc   *container.Process
	cancel context.CancelFunc
}

func New(logger *slog.Logger, sender MatrixSender, roomID string, proc *container.Process) *Bridge {
	return &Bridge{
		logger: logger,
		sender: sender,
		roomID: roomID,
		proc:   proc,
	}
}

func (b *Bridge) Start(ctx context.Context) {
	bridgeCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel

	go b.pumpOutput(bridgeCtx, b.proc.Stdout(), "")
	go b.pumpOutput(bridgeCtx, b.proc.Stderr(), "[stderr] ")
}

func (b *Bridge) ForwardInput(text string) error {
	return b.proc.WriteInput(text)
}

func (b *Bridge) Stop() error {
	if b.cancel != nil {
		b.cancel()
	}
	return b.proc.Stop()
}

func (b *Bridge) pumpOutput(ctx context.Context, reader io.Reader, prefix string) {
	buffer := make([]byte, 4096)
	pending := ""

	flushPending := func(force bool) error {
		if pending == "" {
			return nil
		}

		for {
			newlineIndex := strings.IndexByte(pending, '\n')
			if newlineIndex < 0 {
				if force {
					chunk := sanitizeTerminalOutput(pending)
					pending = ""
					if chunk != "" {
						if err := b.sender.SendText(ctx, b.roomID, prefix+chunk); err != nil {
							return err
						}
					}
				}
				return nil
			}

			line := sanitizeTerminalOutput(pending[:newlineIndex])
			pending = pending[newlineIndex+1:]
			if line == "" {
				continue
			}
			if err := b.sender.SendText(ctx, b.roomID, prefix+line); err != nil {
				return err
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := reader.Read(buffer)
		if n > 0 {
			pending += string(buffer[:n])
			if err := flushPending(true); err != nil {
				b.logger.Error("bridge send output failed", "error", err)
				return
			}
		}

		if err != nil {
			if err != io.EOF {
				b.logger.Error("bridge reader failed", "error", err)
			}
			if err := flushPending(true); err != nil {
				b.logger.Error("bridge flush output failed", "error", err)
			}
			return
		}
	}
}

func sanitizeTerminalOutput(input string) string {
	output := strings.ReplaceAll(input, "\r", "")
	output = ansiOSCRegexp.ReplaceAllString(output, "")
	output = ansiCSIRegexp.ReplaceAllString(output, "")
	output = ansiSingleRegexp.ReplaceAllString(output, "")
	return strings.TrimSpace(output)
}
