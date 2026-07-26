package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type mockQueue struct {
	items []string
	idx   int
}

func (m *mockQueue) pop(ctx context.Context) (string, error) {
	if m.idx >= len(m.items) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	v := m.items[m.idx]
	m.idx++
	return v, nil
}

func TestWorker_ProcessesSubmissions(t *testing.T) {
	q := &mockQueue{items: []string{"sub-001", "sub-002"}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	w := &Worker{queue: q, logger: logger}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	w.Run(ctx)

	if q.idx != 2 {
		t.Errorf("expected 2 items processed, got %d", q.idx)
	}
}

func TestWorker_StopsOnContextCancel(t *testing.T) {
	q := &mockQueue{items: []string{}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	w := &Worker{queue: q, logger: logger}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after context cancel")
	}
}
