package alertworker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/FischukSergey/otus-ms/internal/models"
)

type mockReader struct {
	mu          sync.Mutex
	messages    []kafka.Message
	commits     int
	commitErr   error
	currentRead int
}

func (m *mockReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	m.mu.Lock()
	if m.currentRead < len(m.messages) {
		msg := m.messages[m.currentRead]
		m.currentRead++
		m.mu.Unlock()
		return msg, nil
	}
	m.mu.Unlock()

	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (m *mockReader) CommitMessages(_ context.Context, _ ...kafka.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commits++
	return m.commitErr
}

func (m *mockReader) Close() error { return nil }

func (m *mockReader) commitCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commits
}

type mockWriter struct{}

func (m *mockWriter) WriteMessages(context.Context, ...kafka.Message) error { return nil }
func (m *mockWriter) Close() error                                          { return nil }

type mockDelivery struct {
	reserveErr error
}

func (m *mockDelivery) ReserveAlertDelivery(context.Context, models.NewsAlertEvent) (bool, string, error) {
	if m.reserveErr != nil {
		return false, "", m.reserveErr
	}
	return true, "ready", nil
}

func (m *mockDelivery) FinalizeAlertDelivery(context.Context, string, string, string, *time.Time) error {
	return nil
}

type mockSender struct {
	err error
}

func (m *mockSender) Send(context.Context, models.NewsAlertEvent) error { return m.err }

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRun_CommitsOffsetAfterSuccessfulHandling(t *testing.T) {
	t.Parallel()

	event := models.NewsAlertEvent{
		EventID:  "event-1",
		RuleID:   "rule-1",
		UserUUID: "user-1",
		NewsID:   "news-1",
		Keyword:  "go",
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	reader := &mockReader{
		messages: []kafka.Message{
			{Partition: 0, Offset: 1, Value: body},
		},
	}
	svc := &Service{
		reader:    reader,
		dltWriter: &mockWriter{},
		delivery:  &mockDelivery{},
		sender:    &mockSender{},
		logger:    newLogger(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if reader.commitCount() >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not stop after cancellation")
	}

	if reader.commitCount() != 1 {
		t.Fatalf("expected 1 commit on successful handling, got %d", reader.commitCount())
	}
}

func TestRun_DoesNotCommitOffsetOnRetryableHandlingError(t *testing.T) {
	t.Parallel()

	event := models.NewsAlertEvent{
		EventID:  "event-2",
		RuleID:   "rule-2",
		UserUUID: "user-2",
		NewsID:   "news-2",
		Keyword:  "kafka",
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	reader := &mockReader{
		messages: []kafka.Message{
			{Partition: 0, Offset: 2, Value: body},
		},
	}
	svc := &Service{
		reader:    reader,
		dltWriter: &mockWriter{},
		delivery:  &mockDelivery{reserveErr: errors.New("grpc unavailable")},
		sender:    &mockSender{},
		logger:    newLogger(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not stop after cancellation")
	}

	if reader.commitCount() != 0 {
		t.Fatalf("expected no commits on retryable handling error, got %d", reader.commitCount())
	}
}

func TestCommitWithRetry_RetriesThreeTimesOnCommitError(t *testing.T) {
	t.Parallel()

	reader := &mockReader{commitErr: errors.New("commit failed")}
	svc := &Service{
		reader:    reader,
		dltWriter: &mockWriter{},
		logger:    newLogger(),
	}

	err := svc.commitWithRetry(context.Background(), kafka.Message{Partition: 0, Offset: 3})
	if err == nil {
		t.Fatal("expected commitWithRetry to return error")
	}
	if reader.commitCount() != 3 {
		t.Fatalf("expected 3 commit attempts, got %d", reader.commitCount())
	}
}
