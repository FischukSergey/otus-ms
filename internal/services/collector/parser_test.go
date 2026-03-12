package collector_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FischukSergey/otus-ms/internal/services/collector"
)

const validRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>Test RSS feed</description>
    <item>
      <title>First Article</title>
      <link>https://example.com/1</link>
      <description>Description of first article</description>
      <pubDate>Mon, 01 Jan 2024 10:00:00 +0000</pubDate>
      <author>John Doe</author>
    </item>
    <item>
      <title>Second Article</title>
      <link>https://example.com/2</link>
      <description>Description of second article</description>
      <pubDate>Tue, 02 Jan 2024 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`

func newTestParser(t *testing.T) *collector.Parser {
	t.Helper()
	return collector.NewParser(5*time.Second, newTestLogger(t))
}

func TestParser_ParseFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = fmt.Fprint(w, validRSS)
	}))
	defer srv.Close()

	p := newTestParser(t)
	news, err := p.ParseFeed("source_1", srv.URL)

	require.NoError(t, err)
	require.Len(t, news, 2)

	assert.Equal(t, "source_1", news[0].SourceID)
	assert.Equal(t, "First Article", news[0].Title)
	assert.Equal(t, "https://example.com/1", news[0].URL)
	assert.Equal(t, "John Doe", news[0].Author)
	assert.NotEmpty(t, news[0].ID, "ID должен быть UUID")
	assert.False(t, news[0].CollectedAt.IsZero(), "CollectedAt должен быть заполнен")
	assert.False(t, news[0].PublishedAt.IsZero(), "PublishedAt должен быть заполнен")

	assert.Equal(t, "source_1", news[1].SourceID)
	assert.Equal(t, "Second Article", news[1].Title)
	assert.Empty(t, news[1].Author, "Автор не задан — должен быть пустым")
}

func TestParser_ParseFeed_InvalidURL(t *testing.T) {
	p := newTestParser(t)
	news, err := p.ParseFeed("source_1", "http://127.0.0.1:1")

	require.Error(t, err)
	assert.Nil(t, news)
}

func TestParser_ParseFeed_InvalidXML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "this is not xml at all")
	}))
	defer srv.Close()

	p := newTestParser(t)
	news, err := p.ParseFeed("source_1", srv.URL)

	require.Error(t, err)
	assert.Nil(t, news)
}

func TestParser_ParseFeedWithRetry_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = fmt.Fprint(w, validRSS)
	}))
	defer srv.Close()

	p := collector.NewParser(5*time.Second, newTestLogger(t))
	news, err := p.ParseFeedWithRetry("source_1", srv.URL, 3)

	require.NoError(t, err)
	assert.Len(t, news, 2)
	assert.Equal(t, 3, attempts, "должно быть ровно 3 попытки")
}

func TestParser_ParseFeedWithRetry_AllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := collector.NewParser(5*time.Second, newTestLogger(t))
	news, err := p.ParseFeedWithRetry("source_1", srv.URL, 2)

	require.Error(t, err)
	assert.Nil(t, news)
	assert.Contains(t, err.Error(), "all 2 attempts failed")
}

func TestParser_ParseFeed_ExtractContent(t *testing.T) {
	const rssWithContent = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>Test</title>
    <item>
      <title>Article with content</title>
      <link>https://example.com/1</link>
      <description>Short description</description>
      <content:encoded>Full article text here</content:encoded>
    </item>
  </channel>
</rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, rssWithContent)
	}))
	defer srv.Close()

	p := newTestParser(t)
	news, err := p.ParseFeed("source_1", srv.URL)

	require.NoError(t, err)
	require.Len(t, news, 1)
	assert.Equal(t, "Full article text here", news[0].Content, "Content должен браться из content:encoded")
}
