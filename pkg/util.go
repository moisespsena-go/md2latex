package pkg

import (
	"path"
	"strings"
)

func FormatFileName(fmt, name string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(
				fmt, "%D%", path.Dir(name)),
			"%B%", strings.TrimSuffix(path.Base(name), ".md")),
		"%BE%", path.Base(name))
}
