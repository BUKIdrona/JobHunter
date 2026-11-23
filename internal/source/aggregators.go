package source

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"strings"

	"jobhunter/internal/model"
	"jobhunter/internal/textutil"
)

const (
	SourceRemotive   = "Remotive"
	SourceJobicy     = "Jobicy"
	SourceRemoteOK   = "Remote OK"
	SourceArbeitnow  = "Arbeitnow"
	SourceGreenhouse = "Greenhouse"
	SourceLever      = "Lever"
	SourceAshby      = "Ashby"
)

func fetchRemotive(ctx context.Context, queries []string) []model.Job {
	var out []model.Job
	for _, q := range queries {
		if ctx.Err() != nil {
			break
		}
		endpoint := "https://remotive.com/api/remote-jobs?search=" + url.QueryEscape(q)
		var payload struct {
			Jobs []struct {
				ID          flexID `json:"id"`
				URL         string `json:"url"`
				Title       string `json:"title"`
				Company     string `json:"company_name"`
				JobType     string `json:"job_type"`
				Location    string `json:"candidate_required_location"`
				Publication string `json:"publication_date"`
				Salary      string `json:"salary"`
				Description string `json:"description"`
			} `json:"jobs"`
		}
		if err := getJSON(ctx, endpoint, &payload); err != nil {
			log.Printf("remotive %q: %v", q, err)
			continue
		}
		for _, job := range payload.Jobs {
			plain, emails := textutil.Prepare(job.Description)
			out = append(out, model.Job{
				ID:          string(job.ID),
				Source:      SourceRemotive,
				Title:       strings.TrimSpace(job.Title),
				Company:     strings.TrimSpace(job.Company),
				Location:    strings.TrimSpace(job.Location),
				URL:         strings.TrimSpace(job.URL),
				Description: plain,
				Emails:      emails,
				Salary:      strings.TrimSpace(job.Salary),
				PostedAt:    parseTime(job.Publication),
				Remote:      true,
			})
		}
		log.Printf("remotive %q: %d jobs", q, len(payload.Jobs))
	}
	return out
}

func fetchJobicy(ctx context.Context, tags []string) []model.Job {
	var out []model.Job
	for _, tag := range tags {
		if ctx.Err() != nil {
			break
		}
		endpoint := "https://jobicy.com/api/v2/remote-jobs?count=50&tag=" + url.QueryEscape(tag)
		var payload struct {
			Jobs []struct {
				ID          flexID  `json:"id"`
				URL         string  `json:"url"`
				Title       string  `json:"jobTitle"`
				Company     string  `json:"companyName"`
				Geo         string  `json:"jobGeo"`
				Excerpt     string  `json:"jobExcerpt"`
				Description string  `json:"jobDescription"`
				PubDate     string  `json:"pubDate"`
				SalaryMin   float64 `json:"salaryMin"`
				SalaryMax   float64 `json:"salaryMax"`
				Currency    string  `json:"salaryCurrency"`
			} `json:"jobs"`
		}
		if err := getJSON(ctx, endpoint, &payload); err != nil {
			log.Printf("jobicy %q: %v", tag, err)
			continue
		}
		for _, job := range payload.Jobs {
			plain, emails := textutil.Prepare(job.Description + " " + job.Excerpt)
			out = append(out, model.Job{
				ID:          string(job.ID),
				Source:      SourceJobicy,
				Title:       strings.TrimSpace(job.Title),
				Company:     strings.TrimSpace(job.Company),
				Location:    strings.TrimSpace(job.Geo),
				URL:         strings.TrimSpace(job.URL),
				Description: plain,
				Emails:      emails,
				Salary:      strings.TrimSpace(salaryRange(job.SalaryMin, job.SalaryMax, job.Currency)),
				PostedAt:    parseTime(job.PubDate),
				Remote:      true,
			})
		}
		log.Printf("jobicy %q: %d jobs", tag, len(payload.Jobs))
	}
	return out
}

func fetchRemoteOK(ctx context.Context) []model.Job {
	body, err := getBytes(ctx, "https://remoteok.com/api")
	if err != nil {
		log.Printf("remote ok: %v", err)
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		log.Printf("remote ok: %v", err)
		return nil
	}
	var out []model.Job
	for _, item := range raw {
		var job struct {
			ID          flexID   `json:"id"`
			Position    string   `json:"position"`
			Company     string   `json:"company"`
			Location    string   `json:"location"`
			URL         string   `json:"url"`
			ApplyURL    string   `json:"apply_url"`
			Date        string   `json:"date"`
			Epoch       float64  `json:"epoch"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
			SalaryMin   float64  `json:"salary_min"`
			SalaryMax   float64  `json:"salary_max"`
		}
		if err := json.Unmarshal(item, &job); err != nil || strings.TrimSpace(job.Position) == "" {
			continue
		}
		posted := parseTime(job.Date)
		if posted.IsZero() {
			posted = parseUnix(job.Epoch)
		}
		link := firstLink(job.ApplyURL, job.URL)
		plain, emails := textutil.Prepare(job.Description + " " + strings.Join(job.Tags, " "))
		out = append(out, model.Job{
			ID:          string(job.ID),
			Source:      SourceRemoteOK,
			Title:       strings.TrimSpace(job.Position),
			Company:     strings.TrimSpace(job.Company),
			Location:    strings.TrimSpace(job.Location),
			URL:         link,
			Description: plain,
			Emails:      emails,
			Salary:      strings.TrimSpace(salaryRange(job.SalaryMin, job.SalaryMax, "USD")),
			PostedAt:    posted,
			Remote:      true,
		})
	}
	log.Printf("remote ok: %d jobs", len(out))
	return out
}

func fetchArbeitnow(ctx context.Context) []model.Job {
	var payload struct {
		Data []struct {
			Slug        string   `json:"slug"`
			Company     string   `json:"company_name"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Remote      bool     `json:"remote"`
			URL         string   `json:"url"`
			Tags        []string `json:"tags"`
			Location    string   `json:"location"`
			CreatedAt   float64  `json:"created_at"`
		} `json:"data"`
	}
	if err := getJSON(ctx, "https://www.arbeitnow.com/api/job-board-api", &payload); err != nil {
		log.Printf("arbeitnow: %v", err)
		return nil
	}
	var out []model.Job
	for _, job := range payload.Data {
		plain, emails := textutil.Prepare(job.Description + " " + strings.Join(job.Tags, " "))
		out = append(out, model.Job{
			ID:          job.Slug,
			Source:      SourceArbeitnow,
			Title:       strings.TrimSpace(job.Title),
			Company:     strings.TrimSpace(job.Company),
			Location:    strings.TrimSpace(job.Location),
			URL:         strings.TrimSpace(job.URL),
			Description: plain,
			Emails:      emails,
			PostedAt:    parseUnix(job.CreatedAt),
			Remote:      job.Remote,
		})
	}
	log.Printf("arbeitnow: %d jobs", len(out))
	return out
}

func firstLink(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
			return v
		}
	}
	return ""
}
