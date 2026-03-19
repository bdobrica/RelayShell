package bridge

import (
	"bufio"
	"context"
	"io"
	"log/slog"

	"github.com/bdobrica/RelayShell/internal/container"
)

type MatrixSender interface {
	SendText(ctx context.Context, roomID, body string) error
}

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
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if err := b.sender.SendText(ctx, b.roomID, prefix+line); err != nil {
			b.logger.Error("bridge send output failed", "error", err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		b.logger.Error("bridge scanner failed", "error", err)
	}
}
