package model

import "time"

// Job is one posting gathered from a public board or job feed.
type Job struct {
	ID          string
	Source      string
	Title       string
	Company     string
	Location    string
	URL         string
	Description string
	Emails      []string
	Salary      string
	PostedAt    time.Time
	Remote      bool
}

// Match is a posting that fits the resume profile.
type Match struct {
	Job    Job
	Score  int
	Skills []string
}
