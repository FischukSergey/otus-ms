//nolint:testpackage // тестируем неэкспортируемые части конвейера обработки.
package processor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/FischukSergey/otus-ms/internal/models"
)

type mockReader struct {
	commitCalls int
	commitErr   error
}

func (m *mockReader) FetchMessage(context.Context) (kafka.Message, error) {
	return kafka.Message{}, nil
}

func (m *mockReader) CommitMessages(_ context.Context, _ ...kafka.Message) error {
	m.commitCalls++
	return m.commitErr
}

func (m *mockReader) Close() error {
	return nil
}

func (m *mockReader) Stats() kafka.ReaderStats {
	return kafka.ReaderStats{}
}

type mockArtifactStore struct {
	err error
}

func (m *mockArtifactStore) PutText(_ context.Context, _ string, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "key", nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWorkerLoop_CommitsAfterSuccessfulProcessing(t *testing.T) {
	t.Parallel()

	reader := &mockReader{}
	svc := &Service{
		reader:        reader,
		artifactStore: &mockArtifactStore{},
		logger:        newTestLogger(),
	}

	tasks := make(chan rawTask, 1)
	results := make(chan *models.ProcessedNews, 1)

	tasks <- rawTask{
		msg: kafka.Message{Partition: 1, Offset: 10},
		raw: &models.RawNews{
			ID:          "news-1",
			SourceID:    "source-1",
			Title:       "title",
			URL:         "https://example.org/1",
			PublishedAt: time.Now().UTC(),
		},
	}
	close(tasks)

	svc.workerLoop(context.Background(), tasks, results)

	if reader.commitCalls != 1 {
		t.Fatalf("expected 1 commit call, got %d", reader.commitCalls)
	}
	if len(results) != 1 {
		t.Fatalf("expected one processed result, got %d", len(results))
	}
}

func TestWorkerLoop_DoesNotCommitWhenProcessingFails(t *testing.T) {
	t.Parallel()

	reader := &mockReader{}
	svc := &Service{
		reader:        reader,
		artifactStore: &mockArtifactStore{err: errors.New("s3 unavailable")},
		logger:        newTestLogger(),
	}

	tasks := make(chan rawTask, 1)
	results := make(chan *models.ProcessedNews, 1)

	tasks <- rawTask{
		msg: kafka.Message{Partition: 1, Offset: 11},
		raw: &models.RawNews{
			ID:          "news-2",
			SourceID:    "source-1",
			Title:       "title",
			Content:     "content requiring artifact upload",
			URL:         "https://example.org/2",
			PublishedAt: time.Now().UTC(),
		},
	}
	close(tasks)

	svc.workerLoop(context.Background(), tasks, results)

	if reader.commitCalls != 0 {
		t.Fatalf("expected no commits on processing failure, got %d", reader.commitCalls)
	}
	if len(results) != 0 {
		t.Fatalf("expected no processed results on processing failure, got %d", len(results))
	}
}

func TestCommitWithRetry_RetriesThreeTimesOnCommitError(t *testing.T) {
	t.Parallel()

	reader := &mockReader{commitErr: errors.New("commit failed")}
	svc := &Service{
		reader: reader,
		logger: newTestLogger(),
	}

	err := svc.commitWithRetry(context.Background(), kafka.Message{Partition: 1, Offset: 12})
	if err == nil {
		t.Fatal("expected commitWithRetry to return error")
	}
	if reader.commitCalls != 3 {
		t.Fatalf("expected 3 commit attempts, got %d", reader.commitCalls)
	}
}
