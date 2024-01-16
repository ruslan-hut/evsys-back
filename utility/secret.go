package utility

import "fmt"

// Secret returns a string with the first 5 characters of the input string
// used to hide sensitive information in logs
func Secret(some string) string {
	if len(some) > 5 {
		return fmt.Sprintf("%s***", some[0:5])
	}
	if some == "" {
		return "?"
	}
	return "***"
}
