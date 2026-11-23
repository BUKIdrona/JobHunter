package matcher

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"jobhunter/internal/model"
	"jobhunter/internal/profile"
)

// Result is the scoring decision for one posting.
type Result struct {
	OK     bool
	Score  int
	Skills []string
	Reject string
}

var (
	engineeringTitle = regexp.MustCompile(`(?i)\b(software|backend|back-end|back end|full[- ]?stack|golang|sre|devops|platform|infrastructure|systems|site reliability|developer|sde|programmer|engineer|cpp)\b|\bc\+\+|\bgo\b`)
	hardExclude      = regexp.MustCompile(`(?i)\b(intern|internship|sales|marketing|recruiter|recruiting|talent acquisition|account executive|accountant|illustrator|designer|animator|content|copywriter|customer support|customer success|business development|product manager|product owner|audit|compliance|procurement|relationship manager|video editor|social media|seo|nurse|physician|teacher|counsel|legal|unpaid|volunteer|go-to-market|go to market|solutions engineer|solutions engineering|solution engineer|field engineer)\b`)
	leadership       = regexp.MustCompile(`(?i)\b(staff|principal|distinguished)\s+(software\s+|backend\s+|frontend\s+)?(engineer|developer)|\b(director|vice president|head of)\b|\bvp\b|\bengineering manager\b|\b(software|solution|systems)\s+architect\b|\bmanager\b`)
	yearsRe          = regexp.MustCompile(`(?i)(\d+)\s*\+?\s*years`)
	javaRe           = regexp.MustCompile(`(?i)\bjava\b`)
	otherLangRe      = regexp.MustCompile(`(?i)\b(node\.?js|javascript|typescript|php|ruby|scala|\.net|c#|kotlin|swift|android|ios)\b`)
)

// Evaluate scores a posting against the resume. Location and seniority filters
// run before the skill score so unrelated roles never reach the mailbox.
func Evaluate(job model.Job, minScore int, acceptAnyRemote bool) Result {
	title := strings.TrimSpace(job.Title)
	if title == "" || strings.TrimSpace(job.URL) == "" {
		return Result{Reject: "missing title or link"}
	}
	if hardExclude.MatchString(title) {
		return Result{Reject: "title is outside software engineering"}
	}
	if leadership.MatchString(title) {
		return Result{Reject: "title is a management or staff-plus role"}
	}
	if !engineeringTitle.MatchString(title) {
		return Result{Reject: "title is not an engineering role"}
	}
	if !LocationOK(job.Location, title, job.Remote, acceptAnyRemote) {
		return Result{Reject: "location is outside India and unrestricted remote"}
	}

	text := title + "\n" + job.Description
	rejectYears, yearPenalty := yearsAdjustment(text)
	if rejectYears {
		return Result{Reject: "asks for 6 or more years"}
	}

	var skills []string
	score := 0
	for _, sk := range profile.Skills() {
		if sk.Name == "Go" {
			if mentionsGo(text) {
				skills = append(skills, sk.Name)
				score += sk.Weight
			}
			continue
		}
		for _, p := range sk.Patterns {
			if p.MatchString(text) {
				skills = append(skills, sk.Name)
				score += sk.Weight
				break
			}
		}
	}

	score += titleBonus(title)
	score -= yearPenalty

	hasGo := containsSkill(skills, "Go")
	hasCPP := containsSkill(skills, "C++") || containsSkill(skills, "C")
	hasPy := containsSkill(skills, "Python")
	core := hasGo || hasCPP || hasPy
	if !core {
		return Result{Score: score, Skills: skills, Reject: "no Go, C/C++, or Python overlap"}
	}
	if !hasGo && !hasCPP && !hasPy && (javaRe.MatchString(text) || otherLangRe.MatchString(text)) {
		score -= 4
	}

	if len(skills) < 2 {
		return Result{Score: score, Skills: skills, Reject: "fewer than two matching skills"}
	}
	if score < minScore {
		return Result{Score: score, Skills: skills, Reject: "score below threshold"}
	}
	return Result{OK: true, Score: score, Skills: skills}
}

// LocationOK keeps India roles, worldwide remote roles, and remote roles that do not
// name another country. A remote flag alone does not keep an office in another country.
func LocationOK(location, title string, remote, acceptAnyRemote bool) bool {
	loc := strings.ToLower(strings.TrimSpace(location))
	if acceptAnyRemote && (remote || strings.Contains(loc, "remote") || loc == "") {
		return true
	}
	if indiaOrOpen(loc) {
		return true
	}
	if loc == "" {
		return remote && !restricted(title)
	}
	generic := strings.Contains(loc, "remote") || strings.Contains(loc, "work from home") || strings.Contains(loc, "wfh")
	if generic && !restricted(loc) && !restricted(title) {
		return true
	}
	return false
}

// Fresh reports whether a posting falls inside the lookback window.
func Fresh(posted, now time.Time, lookback time.Duration) bool {
	if posted.IsZero() {
		return false
	}
	if lookback <= 0 {
		return true
	}
	return !posted.Before(now.Add(-lookback))
}

func titleBonus(title string) int {
	bonus := 0
	l := strings.ToLower(title)
	if mentionsGo(title) || strings.Contains(l, "golang") {
		bonus += 8
	}
	if strings.Contains(l, "c++") || strings.Contains(l, "cpp") {
		bonus += 5
	}
	if strings.Contains(l, "backend") || strings.Contains(l, "back-end") || strings.Contains(l, "back end") {
		bonus += 4
	}
	if strings.Contains(l, "platform") || strings.Contains(l, "infrastructure") || strings.Contains(l, "systems") || strings.Contains(l, "sre") || strings.Contains(l, "devops") {
		bonus += 3
	}
	if strings.Contains(l, "software") || strings.Contains(l, "developer") || strings.Contains(l, "sde") {
		bonus += 2
	}
	if regexp.MustCompile(`(?i)\bsenior\b`).MatchString(title) {
		bonus -= 3
	}
	if regexp.MustCompile(`(?i)\blead\b`).MatchString(title) {
		bonus -= 2
	}
	return bonus
}

func mentionsGo(text string) bool {
	if regexp.MustCompile(`(?i)\bgolang\b|\bgo\s*lang\b|\bgo1\.[0-9]+|\bgofiber\b`).MatchString(text) {
		return true
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bgo\b.{0,24}\b(developer|engineer|backend|programmer|fiber|gin|echo|grpc|gorm|goroutine)`),
		regexp.MustCompile(`(?i)\b(developer|engineer|backend|programmer|fiber|gin|echo|grpc|gorm)\b.{0,24}\bgo\b`),
		regexp.MustCompile(`(?i)(experience|proficient|proficiency|expertise|skilled|programming|written|coding|develop(?:ed|ing)?|using|with|in)\s+(?:in\s+)?(?:the\s+)?go\b`),
		regexp.MustCompile(`(?i)\b(java|python|rust|c\+\+|kotlin|node|php|scala)\b\s*(?:/|,|and)\s*\bgo\b`),
		regexp.MustCompile(`(?i)\bgo\b\s*(?:/|,|and)\s*\b(java|python|rust|c\+\+|kotlin|node|php|scala)\b`),
	}
	for _, p := range patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

var openLoc = regexp.MustCompile(`(?i)\b(india|bharat|gurugram|gurgaon|delhi|noida|ghaziabad|faridabad|bangalore|bengaluru|hyderabad|pune|mumbai|bombay|chennai|kolkata|ahmedabad|jaipur|chandigarh|indore|kochi|lucknow|ncr|worldwide|anywhere|global|apac|asia)\b`)

var restrictedLoc = regexp.MustCompile(`(?i)\b(united states|united kingdom|north america|south america|latin america|latam|europe|european|emea|canada|germany|france|netherlands|australia|singapore|london|england|new york|san francisco|seattle|austin|boston|chicago|california|switzerland|ireland|belgium|austria|spain|italy|poland|portugal|sweden|norway|denmark|finland|brazil|mexico|japan|korea|israel|uae|dubai|dublin|zurich|amsterdam|berlin|paris|munich|tokyo|sydney|usa|u\.s\.a\.?|u\.s\.|uk|us|eu)\b`)

func indiaOrOpen(loc string) bool {
	return openLoc.MatchString(loc)
}

func restricted(loc string) bool {
	return restrictedLoc.MatchString(loc)
}

// yearsAdjustment rejects 6+ year requirements and lowers the score for 4-5 year ones.
// Company-age phrases such as "founded 10 years ago" are ignored.
func yearsAdjustment(text string) (reject bool, penalty int) {
	for _, idx := range yearsRe.FindAllStringSubmatchIndex(text, -1) {
		start := idx[0]
		windowStart := start - 60
		if windowStart < 0 {
			windowStart = 0
		}
		window := strings.ToLower(text[windowStart:idx[1]])
		if strings.Contains(window, "founded") || strings.Contains(window, "our company") || strings.Contains(window, "company has") || strings.Contains(window, "company is") {
			continue
		}
		n, err := strconv.Atoi(text[idx[2]:idx[3]])
		if err != nil {
			continue
		}
		phrase := strings.ToLower(text[idx[0]:idx[1]])
		requirement := strings.Contains(phrase, "+") ||
			strings.Contains(window, "experience") ||
			strings.Contains(window, "minimum") ||
			strings.Contains(window, "at least") ||
			strings.Contains(window, "required") ||
			strings.Contains(window, "qualification")
		if !requirement {
			continue
		}
		switch {
		case n >= 6:
			return true, 0
		case n >= 5 && penalty < 4:
			penalty = 4
		case n >= 4 && penalty < 2:
			penalty = 2
		}
	}
	return false, penalty
}

func containsSkill(skills []string, name string) bool {
	for _, s := range skills {
		if s == name {
			return true
		}
	}
	return false
}

// Dedupe drops repeated postings, keeping the first copy. Company boards should
// be passed in before aggregator feeds so the apply link is the company site.
func Dedupe(jobs []model.Job) []model.Job {
	seenURL := map[string]struct{}{}
	seenPair := map[string]struct{}{}
	out := make([]model.Job, 0, len(jobs))
	for _, job := range jobs {
		urlKey := strings.ToLower(strings.TrimSpace(job.URL))
		pair := norm(job.Company) + "|" + norm(job.Title)
		if urlKey != "" {
			if _, ok := seenURL[urlKey]; ok {
				continue
			}
		}
		if _, ok := seenPair[pair]; ok {
			continue
		}
		if urlKey != "" {
			seenURL[urlKey] = struct{}{}
		}
		seenPair[pair] = struct{}{}
		out = append(out, job)
	}
	return out
}

func norm(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9+]+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
