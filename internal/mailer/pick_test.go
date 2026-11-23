package mailer

import (
	"strings"
	"testing"

	"jobhunter/internal/config"
	"jobhunter/internal/model"
)

func TestPickRecruiter(t *testing.T) {
	emails := []string{
		"noreply@acme.com",
		"privacy@acme.com",
		"jobs@greenhouse.io",
		"alex@acme.com",
		"talent@acme.com",
		"deepakkumarmandal01092002@gmail.com",
	}
	got := PickRecruiter(emails, "deepakkumarmandal01092002@gmail.com")
	if got != "talent@acme.com" {
		t.Fatalf("got %q", got)
	}

	if PickRecruiter([]string{"privacy@acme.com", "noreply@acme.com"}, "me@gmail.com") != "" {
		t.Fatal("expected no recruiter address")
	}
}

func TestApplicationCarriesRoleAndResume(t *testing.T) {
	user := config.User{
		Name:     "Deepak Kumar Mandal",
		Email:    "deepakkumarmandal01092002@gmail.com",
		Phone:    "+91 9667009851",
		LinkedIn: "https://www.linkedin.com/in/deepak-kumar-mandal-583850252/",
		GitHub:   "https://github.com/BUKIdrona",
		LeetCode: "https://leetcode.com/u/Deepakthegreat/",
	}
	job := model.Job{Title: "Backend Engineer", Company: "Groww", URL: "https://example.com/jobs/groww"}
	msg := Application(user, job, []string{"Go", "Kubernetes"})
	msg.To = "talent@groww.in"
	msg.AttachName = "Deepak_Kumar_Mandal_Resume.pdf"
	msg.Attach = []byte("%PDF-1.4 resume")
	if !strings.Contains(msg.Text, "Backend Engineer") || !strings.Contains(msg.Text, "Groww") || !strings.Contains(msg.Text, "Go and Kubernetes") {
		t.Fatalf("letter = %s", msg.Text)
	}
	raw := string(buildMIME(user.Name, user.Email, msg))
	if !strings.Contains(raw, "talent@groww.in") || !strings.Contains(raw, "Deepak_Kumar_Mandal_Resume.pdf") {
		t.Fatal("message is missing the hiring address or the resume attachment")
	}
}

func TestDigestIncludesLinkAndDescription(t *testing.T) {
	msg := DigestMessage("deepakkumarmandal01092002@gmail.com", Digest{
		Name: "Deepak",
		Apply: []Item{{
			Title:       "Golang Engineer",
			Company:     "Canonical",
			URL:         "https://jobicy.com/jobs/149538-golang-engineer-2",
			Skills:      "Go, C++, Python",
			Score:       29,
			Source:      "Jobicy",
			Description: "Build container tooling in Go and C++.",
		}},
	})
	if !strings.Contains(msg.Text, "https://jobicy.com/jobs/149538-golang-engineer-2") {
		t.Fatal("text missing apply link")
	}
	if !strings.Contains(msg.Text, "Build container tooling in Go and C++.") {
		t.Fatal("text missing description")
	}
	if !strings.Contains(msg.HTML, "Build container tooling in Go") {
		t.Fatalf("html missing description:\n%s", msg.HTML)
	}
}
