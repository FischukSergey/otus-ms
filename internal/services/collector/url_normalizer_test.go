package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/FischukSergey/otus-ms/internal/services/collector"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "уже нормализованный URL",
			input: "https://example.com/article/123",
			want:  "https://example.com/article/123",
		},
		{
			name:  "uppercase scheme и host приводятся к lowercase",
			input: "HTTPS://EXAMPLE.COM/article",
			want:  "https://example.com/article",
		},
		{
			name:  "убирает фрагмент",
			input: "https://example.com/article#section1",
			want:  "https://example.com/article",
		},
		{
			name:  "убирает utm-параметры, оставляет остальные",
			input: "https://example.com/article?utm_source=rss&utm_medium=feed&id=42",
			want:  "https://example.com/article?id=42",
		},
		{
			name:  "убирает fbclid и gclid, оставляет остальные",
			input: "https://example.com/news?fbclid=abc&gclid=xyz&title=hello",
			want:  "https://example.com/news?title=hello",
		},
		{
			name:  "только tracking-параметры — query становится пустым",
			input: "https://example.com/article?utm_source=rss&utm_campaign=test",
			want:  "https://example.com/article",
		},
		{
			name:  "URL без параметров не изменяется",
			input: "https://example.com/news/2024/some-article",
			want:  "https://example.com/news/2024/some-article",
		},
		{
			name:  "фрагмент + tracking-параметры убираются вместе",
			input: "https://example.com/article?utm_source=rss&id=99#comments",
			want:  "https://example.com/article?id=99",
		},
		{
			name:  "невалидный URL возвращается без изменений",
			input: "not a url ://??",
			want:  "not a url ://??",
		},
		{
			name:  "пустой URL возвращается без изменений",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collector.NormalizeURL(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
