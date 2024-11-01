package sl

import (
	"fmt"
	"log/slog"
)

func Err(err error) slog.Attr {
	return slog.String("error", err.Error())
}

// Secret returns a string with the first 5 characters of the input string
// used to hide sensitive information in logs
func Secret(key, value string) slog.Attr {
	r := "***"
	if len(value) > 5 {
		r = fmt.Sprintf("%s***", value[0:5])
	}
	if value == "" {
		r = "?"
	}
	return slog.String(key, r)
}

func Module(mod string) slog.Attr {
	return slog.String("mod", mod)
}
