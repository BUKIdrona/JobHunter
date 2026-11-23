package textutil

import "testing"

func TestPrepareExtractsMailtoAndObfuscated(t *testing.T) {
	raw := `<p>Write to <a href="mailto:talent@example.com">talent</a> or hr [at] acme.com [dot] in</p>`
	// obfuscation pattern expects [at] and [dot]
	raw = `<div>Contact &lt;a href=&quot;mailto:talent@example.com&quot;&gt;us&lt;/a&gt; or hr [at] acme [dot] com</div>`
	plain, emails := Prepare(raw)
	if plain == "" {
		t.Fatal("expected plain text")
	}
	joined := stringsJoin(emails)
	if !contains(emails, "talent@example.com") {
		t.Fatalf("mailto missing in %s", joined)
	}
	if !contains(emails, "hr@acme.com") {
		t.Fatalf("obfuscated missing in %s", joined)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func stringsJoin(list []string) string {
	out := ""
	for i, s := range list {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}
