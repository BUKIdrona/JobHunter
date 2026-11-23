package mailer

import "strings"

// PickRecruiter chooses one hiring address from addresses found on a posting.
// Privacy, noreply, and job-board boilerplate addresses are ignored.
func PickRecruiter(emails []string, self string) string {
	self = strings.ToLower(strings.TrimSpace(self))
	best := ""
	bestScore := 0
	for _, raw := range emails {
		addr := strings.ToLower(strings.TrimSpace(raw))
		if addr == "" || addr == self || !strings.Contains(addr, "@") {
			continue
		}
		local, domain, _ := strings.Cut(addr, "@")
		if blockedDomain(domain) || blockedLocal(local) {
			continue
		}
		score := 1
		switch {
		case containsAny(local, "hr", "career", "talent", "recruit", "hiring", "jobs", "people", "apply"):
			score = 8
		case containsAny(local, "hello", "contact", "info", "careers"):
			score = 5
		}
		if containsAny(local, "privacy", "legal", "security", "abuse", "support", "unsubscribe", "newsletter", "noreply", "no-reply", "donotreply", "billing", "accounts", "payroll") {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = addr
		}
	}
	return best
}

func blockedDomain(domain string) bool {
	blocked := []string{
		"example.com", "email.com", "test.com", "domain.com", "sentry.io",
		"wixpress.com", "schema.org", "w3.org", "googleapis.com",
		"googleusercontent.com", "gravatar.com", "linkedin.com", "greenhouse.io",
		"lever.co", "ashbyhq.com", "remotive.com", "jobicy.com", "remoteok.com",
		"arbeitnow.com", "naukri.com", "infoedge.com", "indeed.com", "indeedemail.com",
		"gmail.com", "yahoo.com", "outlook.com", "hotmail.com",
		"icloud.com",
	}
	for _, b := range blocked {
		if domain == b || strings.HasSuffix(domain, "."+b) {
			return true
		}
	}
	if strings.HasSuffix(domain, ".png") || strings.HasSuffix(domain, ".jpg") || strings.HasSuffix(domain, ".jpeg") || strings.HasSuffix(domain, ".gif") || strings.HasSuffix(domain, ".svg") || strings.HasSuffix(domain, ".webp") {
		return true
	}
	return false
}

func blockedLocal(local string) bool {
	switch local {
	case "noreply", "no-reply", "donotreply", "do-not-reply", "notifications", "notification", "bounce", "mailer-daemon":
		return true
	default:
		return false
	}
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
