package matcher

import (
	"testing"
	"time"

	"jobhunter/internal/model"
)

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name string
		job  model.Job
		ok   bool
	}{
		{
			name: "golang backend in bengaluru",
			ok:   true,
			job: model.Job{
				Title:       "Backend Engineer (Golang)",
				URL:         "https://example.com/jobs/1",
				Location:    "Bengaluru, India",
				Description: "Build microservices in Golang. Docker, Kubernetes, and PostgreSQL. 3+ years.",
			},
		},
		{
			name: "c++ systems",
			ok:   true,
			job: model.Job{
				Title:       "Systems Software Engineer, C++",
				URL:         "https://example.com/jobs/2",
				Location:    "Gurugram, India",
				Description: "C++ on Linux. Sockets, TCP, UDP, and multithreaded services.",
			},
		},
		{
			name: "python backend",
			ok:   true,
			job: model.Job{
				Title:       "Software Engineer",
				URL:         "https://example.com/jobs/3",
				Location:    "Remote",
				Remote:      true,
				Description: "Python and Django, PostgreSQL, Linux, REST API.",
			},
		},
		{
			name: "finance content",
			job: model.Job{
				Title:       "Finance Content Creator",
				URL:         "https://example.com/jobs/4",
				Location:    "Bengaluru, India",
				Description: "Write about Golang and Kubernetes.",
			},
		},
		{
			name: "staff engineer",
			job: model.Job{
				Title:       "Staff Software Engineer, Go",
				URL:         "https://example.com/jobs/5",
				Location:    "India",
				Description: "Golang, Kubernetes, Docker, PostgreSQL.",
			},
		},
		{
			name: "us only remote",
			job: model.Job{
				Title:       "Backend Engineer",
				URL:         "https://example.com/jobs/6",
				Location:    "Remote - US",
				Remote:      true,
				Description: "Golang, Docker, Kubernetes, PostgreSQL.",
			},
		},
		{
			name: "office outside india",
			job: model.Job{
				Title:       "Backend Engineer",
				URL:         "https://example.com/jobs/6b",
				Location:    "Zug",
				Remote:      true,
				Description: "Golang, Docker, PostgreSQL, microservices, and Python.",
			},
		},
		{
			name: "eu only remote",
			job: model.Job{
				Title:       "Backend Developer, remote from EU",
				URL:         "https://example.com/jobs/6c",
				Location:    "Remote job",
				Remote:      true,
				Description: "Golang, Docker, Kubernetes, PostgreSQL.",
			},
		},
		{
			name: "java only",
			job: model.Job{
				Title:       "Backend Engineer",
				URL:         "https://example.com/jobs/7",
				Location:    "Pune, India",
				Description: "Java, Spring Boot, microservices, Docker, SQL.",
			},
		},
		{
			name: "too many years",
			job: model.Job{
				Title:       "Backend Engineer, Golang",
				URL:         "https://example.com/jobs/8",
				Location:    "Hyderabad, India",
				Description: "Golang, Docker, Kubernetes. Minimum 8 years of experience.",
			},
		},
		{
			name: "intern",
			job: model.Job{
				Title:       "Software Engineer Intern",
				URL:         "https://example.com/jobs/9",
				Location:    "India",
				Description: "Golang and Docker.",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(tc.job, 10, false)
			if got.OK != tc.ok {
				t.Fatalf("ok=%v reject=%q score=%d skills=%v", got.OK, got.Reject, got.Score, got.Skills)
			}
		})
	}
}

func TestFresh(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	if !Fresh(now.Add(-2*time.Hour), now, 48*time.Hour) {
		t.Fatal("expected recent job to be fresh")
	}
	if Fresh(now.Add(-10*24*time.Hour), now, 48*time.Hour) {
		t.Fatal("expected old job to be stale")
	}
	if Fresh(time.Time{}, now, time.Hour) {
		t.Fatal("expected zero time to be stale")
	}
}
