// Package processor содержит конвейер обработки новостей news-processor.
package processor

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/FischukSergey/otus-ms/internal/models"
)

var (
	reTags    = regexp.MustCompile(`<[^>]+>`)
	reSpaces  = regexp.MustCompile(`\s+`)
	reEntHTML = regexp.MustCompile(`&[a-zA-Z]{2,6};|&#[0-9]{1,5};`)
)

// StripHTML удаляет HTML-теги, HTML-сущности и нормализует пробелы.
func StripHTML(s string) string {
	s = reTags.ReplaceAllString(s, " ")
	s = reEntHTML.ReplaceAllString(s, " ")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// ExtractSummary формирует краткое резюме из первых maxSent предложений.
// Приоритет источника: content → description → title.
func ExtractSummary(content, description, title string, maxSent int) string {
	source := content
	if source == "" {
		source = description
	}
	if source == "" {
		return title
	}

	sentences := splitSentences(source)
	if len(sentences) == 0 {
		// Если не удалось разбить — возвращаем первые 500 символов как есть.
		if len(source) > 500 {
			return source[:500] + "…"
		}
		return source
	}

	end := min(maxSent, len(sentences))
	return strings.Join(sentences[:end], " ")
}

// splitSentences разбивает текст на предложения по точке, восклицательному и вопросительному знакам.
func splitSentences(text string) []string {
	var sentences []string
	var cur strings.Builder

	runes := []rune(text)
	for i, r := range runes {
		cur.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			next := i + 1
			// Конец строки или за знаком следует пробел / перенос.
			if next >= len(runes) || runes[next] == ' ' || runes[next] == '\n' || runes[next] == '\r' {
				s := strings.TrimSpace(cur.String())
				if len([]rune(s)) > 10 {
					sentences = append(sentences, s)
				}
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		if s := strings.TrimSpace(cur.String()); s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

// categoryKeywords — наборы ключевых слов для каждой категории (RU + EN).
//
// TODO: перенести словари в main-service как настройку системы (таблица category_keywords).
// news-processor должен загружать их через gRPC при старте и обновлять по расписанию.
// Это позволит менять категории и ключевые слова без перекомпиляции сервиса.
var categoryKeywords = map[string][]string{
	"tech": {
		"software", "hardware", "artificial intelligence", "machine learning", "cloud",
		"cybersecurity", "programming", "developer", "smartphone", "processor", "database",
		"код", "программ", "искусственный интеллект", "технолог", "процессор",
		"смартфон", "приложени", "кибербезопасност", "разработ",
	},
	"politics": {
		"government", "president", "minister", "election", "parliament", "senate",
		"congress", "political", "policy", "diplomatic",
		"правительство", "президент", "министр", "выборы", "парламент",
		"политик", "закон", "дипломат",
	},
	"economy": {
		"economy", "market", "stock", "gdp", "inflation", "trade", "bank",
		"financial", "investment", "budget", "currency",
		"экономик", "рынок", "банк", "инфляц", "бюджет",
		"инвестиц", "финанс", "торговл", "валют",
	},
	"sports": {
		"football", "basketball", "tennis", "olympic", "championship", "tournament",
		"athlete", "coach", "match",
		"футбол", "баскетбол", "хоккей", "спорт", "чемпионат",
		"турнир", "олимпи", "тренер",
	},
	"science": {
		"research", "study", "scientist", "discovery", "experiment", "physics",
		"biology", "chemistry", "space", "climate",
		"наук", "исследован", "учёный", "открыт", "эксперимент",
		"физик", "биолог", "химия", "космос", "климат",
	},
}

// DetectCategory определяет категорию новости по ключевым словам.
// Возвращает категорию с наибольшим числом совпадений или "other".
func DetectCategory(text string) string {
	lower := strings.ToLower(text)
	best, bestCount := "other", 0
	for cat, keywords := range categoryKeywords {
		count := 0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			best = cat
		}
	}
	return best
}

// FetchPageContent загружает страницу по URL и возвращает очищенный от HTML текст.
// Ограничение — 512 KB тела. При любой ошибке возвращает пустую строку (не критично).
func FetchPageContent(ctx context.Context, rawURL string, timeout time.Duration) string {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; OtusNewsBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return ""
	}

	return StripHTML(string(body))
}

// truncate обрезает строку до maxRunes символов (по рунам, не байтам).
// Если строка длиннее — добавляет «…» в конце.
func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes-1]) + "…"
}

// Process запускает полный конвейер обработки одной сырой новости.
// При fetchContent=true и пустом Content загружает страницу по URL.
func Process(ctx context.Context, raw *models.RawNews, fetchContent bool, fetchTimeout time.Duration) *models.ProcessedNews {
	title := truncate(StripHTML(raw.Title), 500)
	description := StripHTML(raw.Description)
	content := StripHTML(raw.Content)

	if content == "" && fetchContent && raw.URL != "" {
		content = FetchPageContent(ctx, raw.URL, fetchTimeout)
	}

	fullText := content
	if fullText == "" {
		fullText = description
	}

	summary := ExtractSummary(content, description, title, 3)
	category := truncate(DetectCategory(title+" "+fullText), 50)

	return &models.ProcessedNews{
		ID:          raw.ID,
		SourceID:    raw.SourceID,
		Title:       title,
		Summary:     summary,
		URL:         raw.URL,
		Category:    category,
		Tags:        []string{},
		PublishedAt: raw.PublishedAt,
		ProcessedAt: time.Now().UTC(),
	}
}
