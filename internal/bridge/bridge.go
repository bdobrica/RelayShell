package bridge

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/bdobrica/RelayShell/internal/container"
)

type MatrixSender interface {
	SendText(ctx context.Context, roomID, body string) error
}

type Bridge struct {
	logger    *slog.Logger
	sender    MatrixSender
	roomID    string
	proc      *container.Process
	batchIdle time.Duration
	cancel    context.CancelFunc
}

const defaultOutputBatchIdle = 300 * time.Millisecond

func New(logger *slog.Logger, sender MatrixSender, roomID string, proc *container.Process, batchIdle time.Duration) *Bridge {
	if batchIdle <= 0 {
		batchIdle = defaultOutputBatchIdle
	}

	return &Bridge{
		logger:    logger,
		sender:    sender,
		roomID:    roomID,
		proc:      proc,
		batchIdle: batchIdle,
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
	chunkCh := make(chan string, 32)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)

		buffer := make([]byte, 4096)
		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				select {
				case chunkCh <- string(buffer[:n]):
				case <-ctx.Done():
					return
				}
			}

			if err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				}
				return
			}
		}
	}()

	var batch strings.Builder
	timer := time.NewTimer(b.batchIdle)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timerActive := false

	resetTimer := func() {
		if timerActive {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
		timer.Reset(b.batchIdle)
		timerActive = true
	}

	flushBatch := func() error {
		if batch.Len() == 0 {
			return nil
		}

		renderedLines := renderBatchToLines(batch.String())
		batch.Reset()

		for _, line := range renderedLines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if err := b.sender.SendText(ctx, b.roomID, prefix+line); err != nil {
				return err
			}
		}

		return nil
	}

	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := flushBatch(); err != nil {
				b.logger.Error("bridge flush output on cancel failed", "error", err)
			}
			return
		case chunk, ok := <-chunkCh:
			if !ok {
				chunkCh = nil
				continue
			}
			batch.WriteString(chunk)
			resetTimer()
		case <-timer.C:
			timerActive = false
			if err := flushBatch(); err != nil {
				b.logger.Error("bridge send output failed", "error", err)
				return
			}
		case err := <-errCh:
			if err != io.EOF {
				b.logger.Error("bridge reader failed", "error", err)
			}
			if err := flushBatch(); err != nil {
				b.logger.Error("bridge flush output failed", "error", err)
			}
			return
		}
	}
}

func sanitizeTerminalOutput(input string) string {
	output := normalizeSpaceHeavyLines(input)

	// Collapse excessive blank lines produced by redraw-heavy outputs.
	for strings.Contains(output, "\n\n\n") {
		output = strings.ReplaceAll(output, "\n\n\n", "\n\n")
	}
	return output
}

func normalizeSpaceHeavyLines(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, " ")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			lines[i] = line
			continue
		}

		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
		spaceCount := strings.Count(line, " ")
		if leadingSpaces > 24 || spaceCount*3 >= len(line) {
			line = strings.Join(strings.Fields(line), " ")
		}

		lines[i] = line
	}

	return strings.Join(lines, "\n")
}
