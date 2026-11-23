package profile

import "regexp"

// Skill is one resume skill the matcher looks for in a posting.
// Weights come from Deepak Kumar Mandal's resume: Go/Fiber and C++ are the
// core of the current SDE work, with Python, SQL, Docker, Kubernetes, Linux,
// and network programming close behind.
type Skill struct {
	Name     string
	Weight   int
	Patterns []*regexp.Regexp
}

// Skills is the resume skill set used for scoring.
func Skills() []Skill {
	return []Skill{
		{Name: "Go", Weight: 6},
		compiled("C++", 5, `c\+\+`, `\bcpp\b`, `c/c\+\+`),
		compiled("C", 2, `\bc/c\+\+\b`, `\bc language\b`),
		compiled("Python", 3, `\bpython\b`),
		compiled("Fiber", 4, `\bgofiber\b`, `golang/fiber`, `go fiber`, `fiber framework`),
		compiled("Kubernetes", 3, `\bkubernetes\b`, `\bk8s\b`),
		compiled("Docker", 3, `\bdocker\b`),
		compiled("SQL", 2, `\bsql\b`, `\bpostgres(?:ql)?\b`, `\bmysql\b`, `\bmongodb\b`),
		compiled("Linux", 2, `\blinux\b`),
		compiled("Microservices", 3, `\bmicro-?services?\b`),
		compiled("Networking", 3, `\btcp/?ip\b`, `\btcp\b`, `\budp\b`, `\bsctp\b`, `\bgrpc\b`, `\bsockets?\b`),
		compiled("REST APIs", 2, `\brest(?:ful)? api\b`, `\brestful\b`),
		compiled("Distributed Systems", 3, `distributed systems`),
		compiled("Concurrency", 2, `\bconcurrent\b`, `\bconcurrency\b`, `\bmulti-?thread`, `\bgoroutines?\b`),
		compiled("Django", 2, `\bdjango\b`),
	}
}

func compiled(name string, weight int, patterns ...string) Skill {
	sk := Skill{Name: name, Weight: weight}
	for _, p := range patterns {
		sk.Patterns = append(sk.Patterns, regexp.MustCompile(`(?i)`+p))
	}
	return sk
}
