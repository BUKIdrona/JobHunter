package source

import "testing"

func TestParseIndeedRSS(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
<item>
<title><![CDATA[Backend Engineer - Golang]]></title>
<link>https://in.indeed.com/viewjob?jk=abc123</link>
<source>Groww</source>
<pubDate>Mon, 02 Nov 2026 09:00:00 GMT</pubDate>
<description><![CDATA[<p>Golang, Docker and Kubernetes in Gurugram. Write to talent@groww.example</p>]]></description>
</item>
</channel></rss>`)
	jobs, err := parseIndeedRSS(body, "Gurugram")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs=%d", len(jobs))
	}
	job := jobs[0]
	if job.Company != "Groww" || job.ID != "abc123" || job.Location != "Gurugram" {
		t.Fatalf("%+v", job)
	}
	if job.PostedAt.IsZero() {
		t.Fatal("expected a posted date")
	}
	if !containsEmail(job.Emails, "talent@groww.example") {
		t.Fatalf("emails=%v", job.Emails)
	}
	if _, err := parseIndeedRSS([]byte("<html>blocked</html>"), "India"); err == nil {
		t.Fatal("expected a block page to fail")
	}
}

func TestParseNaukri(t *testing.T) {
	body := []byte(`{
		"jobDetails": [{
			"jobId": "9911",
			"title": "Golang Developer",
			"companyName": "PhonePe",
			"jdURL": "/job-listings-golang-developer-phonepe-9911",
			"createdDate": 1793000000000,
			"placeholders": [{"type": "location", "label": "Bengaluru"}],
			"jobDescription": "Build services in Golang, Docker and Kubernetes. Send your resume to hr@phonepe.example"
		}]
	}`)
	jobs, err := parseNaukri(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs=%d", len(jobs))
	}
	job := jobs[0]
	if job.Company != "PhonePe" || job.Location != "Bengaluru" {
		t.Fatalf("%+v", job)
	}
	if job.URL != "https://www.naukri.com/job-listings-golang-developer-phonepe-9911" {
		t.Fatalf("url=%s", job.URL)
	}
	if !containsEmail(job.Emails, "hr@phonepe.example") {
		t.Fatalf("emails=%v", job.Emails)
	}
	if _, err := parseNaukri([]byte("<html>blocked</html>")); err == nil {
		t.Fatal("expected a block page to fail")
	}
}

func containsEmail(emails []string, want string) bool {
	for _, email := range emails {
		if email == want {
			return true
		}
	}
	return false
}
