package collector

import (
	"maps"
	"net/url"
	"slices"
	"strings"
)

// trackingParams — параметры запроса, не влияющие на идентичность статьи.
var trackingParams = []string{
	"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
	"fbclid", "gclid", "yclid", "ref", "_ga",
}

// NormalizeURL приводит URL к каноническому виду для дедупликации:
//   - lowercase scheme + host
//   - убирает фрагмент (#...)
//   - удаляет tracking-параметры
//   - query-параметры сортируются автоматически через url.Values.Encode
//
// При ошибке парсинга возвращает исходный URL.
func NormalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

	q := u.Query()
	maps.DeleteFunc(q, func(k string, _ []string) bool {
		return slices.Contains(trackingParams, k)
	})
	u.RawQuery = q.Encode()

	return u.String()
}
