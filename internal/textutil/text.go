package textutil

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]+>`)
	wsRe     = regexp.MustCompile(`\s+`)
	mailtoRe = regexp.MustCompile(`(?i)mailto:([^?"'\s>]+)`)
	emailRe  = regexp.MustCompile(`(?i)[a-z0-9][a-z0-9._%+\-]{0,63}@[a-z0-9][a-z0-9.\-]{0,250}\.[a-z]{2,24}`)
	obfusRe  = regexp.MustCompile(`(?i)([a-z0-9][a-z0-9._%+\-]{0,63})\s*(?:\[at\]|\(at\)|\{at\})\s*([a-z0-9][a-z0-9.\-]{0,250})\s*(?:\[dot\]|\(dot\)|\{dot\})\s*([a-z]{2,24})`)
)

// Prepare turns HTML from a job board into plain text and any contact emails.
func Prepare(raw string) (string, []string) {
	unescaped := unescape(raw)
	var found []string
	found = append(found, mailtoEmails(unescaped)...)
	found = append(found, findEmails(unescaped)...)
	plain := toPlain(unescaped)
	found = append(found, findEmails(plain)...)
	found = append(found, obfuscated(unescaped)...)
	found = append(found, obfuscated(plain)...)
	return plain, uniqueEmails(found)
}

func unescape(s string) string {
	for i := 0; i < 3; i++ {
		next := html.UnescapeString(s)
		if next == s {
			break
		}
		s = next
	}
	return s
}

func toPlain(s string) string {
	s = scriptRe.ReplaceAllString(s, " ")
	s = styleRe.ReplaceAllString(s, " ")
	s = tagRe.ReplaceAllString(s, " ")
	s = unescape(s)
	s = wsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func mailtoEmails(s string) []string {
	var out []string
	for _, m := range mailtoRe.FindAllStringSubmatch(s, -1) {
		addr, err := url.QueryUnescape(m[1])
		if err != nil {
			addr = m[1]
		}
		out = append(out, addr)
	}
	return out
}

func findEmails(s string) []string {
	return emailRe.FindAllString(s, -1)
}

func obfuscated(s string) []string {
	var out []string
	for _, m := range obfusRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1]+"@"+m[2]+"."+m[3])
	}
	return out
}

func uniqueEmails(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, e := range in {
		e = strings.ToLower(strings.Trim(e, ".,;:<>()[]\"'"))
		if !strings.Contains(e, "@") || len(e) > 80 {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}
