package utils

import (
	"mime"
	"path/filepath"
	"strings"
)

// DetectContentTypeFromName returns a best-effort MIME type for a file name or path.
// It guarantees a correct type for common image formats, especially SVG.
func DetectContentTypeFromName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))

	// Explicit overrides for extensions we care about
	switch ext {
	case ".svg":
		return "image/svg+xml"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	}

	if ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}

	// Safe default
	return "application/octet-stream"
}

