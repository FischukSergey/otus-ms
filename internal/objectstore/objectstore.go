// Package objectstore предоставляет абстракцию объектного хранилища.
package objectstore

import (
	"context"
	"path"
	"strings"
	"time"
)

// Store определяет минимальный интерфейс объектного хранилища.
type Store interface {
	// PutText сохраняет текстовый объект по ключу и возвращает сохранённый ключ.
	PutText(ctx context.Context, key string, content string) (string, error)
}

// BuildNewsTextKey строит ключ S3-объекта для текстового артефакта новости.
// Формат: <prefix>/<yyyy>/<mm>/<dd>/<newsID>.txt.
func BuildNewsTextKey(prefix, newsID string, t time.Time) string {
	trimmed := strings.Trim(prefix, "/")
	datePath := t.UTC().Format("2006/01/02")
	fileName := newsID + ".txt"

	if trimmed == "" {
		return path.Join(datePath, fileName)
	}
	return path.Join(trimmed, datePath, fileName)
}
