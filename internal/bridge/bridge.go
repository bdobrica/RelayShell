package bridge

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bdobrica/RelayShell/internal/container"
)

type MatrixSender interface {
	SendText(ctx context.Context, roomID, body string) error
	SetTyping(ctx context.Context, roomID string, typing bool, timeout time.Duration) error
}

type Bridge struct {
	logger    *slog.Logger
	sender    MatrixSender
	roomID    string
	proc      *container.Process
	batchIdle time.Duration
	cancel    context.CancelFunc

	typingMu       sync.Mutex
	typingRefCount int
	typingLastSent time.Time
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
	typingActive := false

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
	defer func() {
		if typingActive {
			b.endTyping(ctx)
		}
	}()

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
			if !typingActive {
				typingActive = true
				b.beginTyping(ctx)
			} else {
				b.renewTyping(ctx)
			}
			resetTimer()
		case <-timer.C:
			timerActive = false
			if err := flushBatch(); err != nil {
				b.logger.Error("bridge send output failed", "error", err)
				return
			}
			if typingActive {
				typingActive = false
				b.endTyping(ctx)
			}
		case err := <-errCh:
			if err != io.EOF {
				b.logger.Error("bridge reader failed", "error", err)
			}
			if err := flushBatch(); err != nil {
				b.logger.Error("bridge flush output failed", "error", err)
			}
			if typingActive {
				typingActive = false
				b.endTyping(ctx)
			}
			return
		}
	}
}

func (b *Bridge) beginTyping(ctx context.Context) {
	now := time.Now()
	shouldSend := false
	b.typingMu.Lock()
	b.typingRefCount++
	if b.typingRefCount == 1 {
		b.typingLastSent = now
		shouldSend = true
	}
	b.typingMu.Unlock()

	if shouldSend {
		if err := b.sender.SetTyping(ctx, b.roomID, true, b.typingTimeout()); err != nil {
			b.logger.Debug("bridge typing start failed", "error", err)
		}
	}
}

func (b *Bridge) renewTyping(ctx context.Context) {
	timeout := b.typingTimeout()
	renewAfter := timeout / 2
	if renewAfter < time.Second {
		renewAfter = time.Second
	}

	now := time.Now()
	shouldSend := false
	b.typingMu.Lock()
	if b.typingRefCount > 0 && now.Sub(b.typingLastSent) >= renewAfter {
		b.typingLastSent = now
		shouldSend = true
	}
	b.typingMu.Unlock()

	if shouldSend {
		if err := b.sender.SetTyping(ctx, b.roomID, true, timeout); err != nil {
			b.logger.Debug("bridge typing renew failed", "error", err)
		}
	}
}

func (b *Bridge) endTyping(ctx context.Context) {
	shouldSend := false
	b.typingMu.Lock()
	if b.typingRefCount > 0 {
		b.typingRefCount--
	}
	if b.typingRefCount == 0 {
		b.typingLastSent = time.Time{}
		shouldSend = true
	}
	b.typingMu.Unlock()

	if shouldSend {
		if err := b.sender.SetTyping(ctx, b.roomID, false, 0); err != nil {
			b.logger.Debug("bridge typing stop failed", "error", err)
		}
	}
}

func (b *Bridge) typingTimeout() time.Duration {
	timeout := b.batchIdle + time.Second
	if timeout < 5*time.Second {
		timeout = 5 * time.Second
	}
	return timeout
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
