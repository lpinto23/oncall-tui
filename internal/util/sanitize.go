package util

import "strings"

var sanitizeReplacer = strings.NewReplacer(
	"/", "-",
	"\\", "-",
	" ", "_",
	":", "-",
)

func Sanitize(s string) string {
	return sanitizeReplacer.Replace(s)
}
