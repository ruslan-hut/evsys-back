package aireport

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxTemplateLength = 200

var (
	reAddress   = regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}(:\d+)?(->\d{1,3}(\.\d{1,3}){3}(:\d+)?)?:?$`)
	reEmail     = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	reUserId    = regexp.MustCompile(`^user_[0-9a-f]+`)
	reTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?Z?)?`)
	reDuration  = regexp.MustCompile(`^(\d+h)?(\d+m)?(\d+(\.\d+)?(s|ms))$|^\d+h\d+m$`)
	reNumber    = regexp.MustCompile(`^(#?)\d+([.,]\d+)?$`)
	reHexId     = regexp.MustCompile(`^([0-9a-f]{8,}|[0-9A-F]{8,})(\.\.\.)?$`)
	// reAssignment is a key=value token; the value is reduced on its own
	reAssignment = regexp.MustCompile(`^([A-Za-z_]+[=:])(.+)$`)
)

// identifyingKeys name values that point at a person or a card, whatever
// they look like.
var identifyingKeys = map[string]bool{
	"idtag": true, "id_tag": true, "tag": true, "user": true, "username": true, "name": true,
	"email": true, "card": true, "phone": true, "userid": true, "user_id": true,
}

// templateOf reduces a log text to its shape, so that records differing only
// in numbers, addresses, times or user ids count as one event. Vendor error
// codes such as CO.01.112 survive: only whole numeric tokens are replaced.
// Email addresses and user ids are removed - they never reach the model.
func templateOf(text string) string {
	text = reEmail.ReplaceAllString(text, "<email>")
	fields := strings.Fields(text)
	for i, f := range fields {
		lead, core, tail := splitPunctuation(f)
		key := ""
		if m := reAssignment.FindStringSubmatch(core); m != nil && !reAddress.MatchString(core) && !reTimestamp.MatchString(core) {
			key, core = m[1], m[2]
		}
		switch {
		case key != "" && identifyingKeys[strings.ToLower(key[:len(key)-1])]:
			core = "<id>"
		case reAddress.MatchString(core):
			core = "<addr>"
		case reUserId.MatchString(core):
			core = "user_*"
		case reTimestamp.MatchString(core):
			core = "<time>"
		case reDuration.MatchString(core):
			core = "<dur>"
		case reNumber.MatchString(core):
			core = reNumber.ReplaceAllString(core, "${1}N")
		case reHexId.MatchString(core):
			core = "<id>"
		}
		fields[i] = lead + key + core + tail
	}
	return truncate(strings.Join(fields, " "), maxTemplateLength)
}

// truncate cuts s to at most n bytes on a rune boundary.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

// splitPunctuation separates the brackets and quotes around a token, and the
// punctuation after it, from the token itself.
func splitPunctuation(token string) (string, string, string) {
	start := 0
	for start < len(token) && strings.ContainsRune("(\"'[{", rune(token[start])) {
		start++
	}
	end := len(token)
	for end > start && strings.ContainsRune(":;,)]}\"'", rune(token[end-1])) {
		end--
	}
	return token[:start], token[start:end], token[end:]
}
