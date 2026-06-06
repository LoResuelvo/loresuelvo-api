package file

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func buildObjectKey(createdOn time.Time, purpose, fileID, originalName string) string {
	ext := strings.ToLower(filepath.Ext(originalName))
	return fmt.Sprintf("files/%d/%02d/%s/%s%s", createdOn.Year(), createdOn.Month(), purpose, fileID, ext)
}
