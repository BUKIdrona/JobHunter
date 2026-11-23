package source

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"jobhunter/internal/config"
	"jobhunter/internal/model"
	"jobhunter/internal/textutil"
)

const (
	SourceIndeed = "Indeed"
	SourceNaukri = "Naukri"
)

const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func fetchIndeed(ctx context.Context, queries []config.IndeedQuery, lookback time.Duration) []model.Job {
	var out []model.Job
	days := clampDays(lookback, 14)
	for _, query := range queries {
		if ctx.Err() != nil {
			break
		}
		query.Query = strings.TrimSpace(query.Query)
		query.Location = strings.TrimSpace(query.Location)
		if query.Query == "" {
			continue
		}
		if query.Location == "" {
			query.Location = "India"
		}
		values := url.Values{}
		values.Set("q", query.Query)
		values.Set("l", query.Location)
		values.Set("fromage", strconv.Itoa(days))
		values.Set("sort", "date")
		endpoint := "https://in.indeed.com/rss?" + values.Encode()
		body, err := getBytesHeaders(ctx, endpoint, map[string]string{
			"User-Agent": browserUA,
			"Accept":     "application/rss+xml, application/xml, text/xml, */*",
		})
		if err != nil {
			log.Printf("indeed %s in %s: %v", query.Query, query.Location, err)
			continue
		}
		jobs, err := parseIndeedRSS(body, query.Location)
		if err != nil {
			log.Printf("indeed %s in %s: %v", query.Query, query.Location, err)
			continue
		}
		log.Printf("indeed %s in %s: %d jobs", query.Query, query.Location, len(jobs))
		out = append(out, jobs...)
		sleep(ctx, 300*time.Millisecond)
	}
	return out
}

func parseIndeedRSS(body []byte, location string) ([]model.Job, error) {
	if !bytes.Contains(bytes.ToLower(body), []byte("<rss")) && !bytes.Contains(bytes.ToLower(body), []byte("<item")) {
		return nil, fmt.Errorf("indeed did not return a job feed")
	}
	var feed struct {
		Channel struct {
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Description string `xml:"description"`
				PubDate     string `xml:"pubDate"`
				Source      string `xml:"source"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	var out []model.Job
	for _, item := range feed.Channel.Items {
		link := strings.TrimSpace(item.Link)
		title := strings.TrimSpace(item.Title)
		if link == "" || title == "" {
			continue
		}
		plain, emails := textutil.Prepare(item.Description)
		company := strings.TrimSpace(item.Source)
		if company == "" {
			company = companyFromIndeedTitle(title)
		}
		out = append(out, model.Job{
			ID:          indeedID(link),
			Source:      SourceIndeed,
			Title:       title,
			Company:     company,
			Location:    location,
			URL:         link,
			Description: plain,
			Emails:      emails,
			PostedAt:    parseTime(item.PubDate),
			Remote:      strings.Contains(strings.ToLower(location+" "+plain), "remote"),
		})
	}
	return out, nil
}

func fetchNaukri(ctx context.Context, queries []config.NaukriQuery, lookback time.Duration) []model.Job {
	var out []model.Job
	days := clampDays(lookback, 30)
	detailsLeft := 20
	for _, query := range queries {
		if ctx.Err() != nil {
			break
		}
		query.Keyword = strings.TrimSpace(query.Keyword)
		if query.Keyword == "" {
			continue
		}
		if query.Experience <= 0 {
			query.Experience = 1
		}
		values := url.Values{}
		values.Set("noOfResults", "20")
		values.Set("urlType", "search_by_keyword")
		values.Set("searchType", "adv")
		values.Set("keyword", query.Keyword)
		values.Set("k", query.Keyword)
		values.Set("pageNo", "1")
		values.Set("experience", strconv.Itoa(query.Experience))
		values.Set("jobAge", strconv.Itoa(days))
		endpoint := "https://www.naukri.com/jobapi/v3/search?" + values.Encode()
		body, err := getBytesHeaders(ctx, endpoint, naukriHeaders())
		if err != nil {
			log.Printf("naukri %s: %v", query.Keyword, err)
			continue
		}
		jobs, err := parseNaukri(body)
		if err != nil {
			log.Printf("naukri %s: %v", query.Keyword, err)
			continue
		}
		for i := range jobs {
			if jobs[i].ID == "" || len(jobs[i].Emails) > 0 || detailsLeft <= 0 || ctx.Err() != nil {
				continue
			}
			detail, err := fetchNaukriDetail(ctx, jobs[i].ID)
			detailsLeft--
			if err != nil {
				log.Printf("naukri job %s: %v", jobs[i].ID, err)
				sleep(ctx, 150*time.Millisecond)
				continue
			}
			if detail.Description != "" {
				jobs[i].Description = detail.Description
			}
			if len(detail.Emails) > 0 {
				jobs[i].Emails = detail.Emails
			}
			if detail.URL != "" {
				jobs[i].URL = detail.URL
			}
			sleep(ctx, 150*time.Millisecond)
		}
		log.Printf("naukri %s: %d jobs", query.Keyword, len(jobs))
		out = append(out, jobs...)
		sleep(ctx, 300*time.Millisecond)
	}
	return out
}

func fetchNaukriDetail(ctx context.Context, id string) (model.Job, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, " ") {
		return model.Job{}, fmt.Errorf("missing job id")
	}
	var last error
	for _, endpoint := range []string{
		"https://www.naukri.com/jobapi/v4/job/" + url.PathEscape(id),
		"https://www.naukri.com/jobapi/v3/job/" + url.PathEscape(id),
	} {
		body, err := getBytesHeaders(ctx, endpoint, naukriHeaders())
		if err != nil {
			last = err
			continue
		}
		jobs, err := parseNaukri(body)
		if err != nil {
			last = err
			continue
		}
		if len(jobs) == 0 {
			last = fmt.Errorf("empty job detail")
			continue
		}
		return jobs[0], nil
	}
	if last == nil {
		last = fmt.Errorf("no detail")
	}
	return model.Job{}, last
}

func parseNaukri(body []byte) ([]model.Job, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] == '<' {
		return nil, fmt.Errorf("naukri did not return a job feed")
	}
	var wrap struct {
		JobDetails json.RawMessage `json:"jobDetails"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(wrap.JobDetails)) == 0 {
		return nil, fmt.Errorf("naukri response had no jobs")
	}
	var records []naukriRecord
	payload := bytes.TrimSpace(wrap.JobDetails)
	if payload[0] == '[' {
		if err := json.Unmarshal(payload, &records); err != nil {
			return nil, err
		}
	} else {
		var one naukriRecord
		if err := json.Unmarshal(payload, &one); err != nil {
			return nil, err
		}
		records = []naukriRecord{one}
	}
	var out []model.Job
	for _, record := range records {
		job := record.toJob()
		if job.Title == "" || job.URL == "" {
			continue
		}
		out = append(out, job)
	}
	return out, nil
}

type naukriRecord struct {
	JobID          flexID `json:"jobId"`
	Title          string `json:"title"`
	CompanyName    string `json:"companyName"`
	JobDescription string `json:"jobDescription"`
	Description    string `json:"description"`
	JdURL          string `json:"jdURL"`
	StaticURL      string `json:"staticUrl"`
	CreatedDate    any    `json:"createdDate"`
	Email          string `json:"email"`
	RecruiterEmail string `json:"recruiterEmail"`
	ContactEmail   string `json:"contactEmail"`
	CompanyDetail  struct {
		Name string `json:"name"`
	} `json:"companyDetail"`
	Placeholders []struct {
		Type  string `json:"type"`
		Label string `json:"label"`
	} `json:"placeholders"`
}

func (record naukriRecord) toJob() model.Job {
	rawText := strings.Join([]string{record.JobDescription, record.Description, record.Email, record.RecruiterEmail, record.ContactEmail}, "\n")
	plain, emails := textutil.Prepare(rawText)
	for _, extra := range []string{record.Email, record.RecruiterEmail, record.ContactEmail} {
		extra = strings.ToLower(strings.TrimSpace(extra))
		if strings.Contains(extra, "@") {
			emails = append(emails, extra)
		}
	}
	location := naukriLocation(record.Placeholders)
	company := firstNonEmpty(record.CompanyName, record.CompanyDetail.Name)
	return model.Job{
		ID:          string(record.JobID),
		Source:      SourceNaukri,
		Title:       strings.TrimSpace(record.Title),
		Company:     company,
		Location:    location,
		URL:         naukriLink(record.StaticURL, record.JdURL, string(record.JobID)),
		Description: plain,
		Emails:      emails,
		PostedAt:    naukriTime(record.CreatedDate),
		Remote:      strings.Contains(strings.ToLower(location), "remote") || strings.Contains(strings.ToLower(location), "work from home"),
	}
}

func naukriHeaders() map[string]string {
	return map[string]string{
		"User-Agent": browserUA,
		"Accept":     "application/json",
		"appid":      "109",
		"systemid":   "Naukri",
		"clientid":   "d3skt0p",
		"Referer":    "https://www.naukri.com/",
	}
}

func naukriLocation(placeholders []struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}) string {
	var parts []string
	for _, place := range placeholders {
		if strings.EqualFold(place.Type, "location") && strings.TrimSpace(place.Label) != "" {
			parts = append(parts, strings.TrimSpace(place.Label))
		}
	}
	return strings.Join(parts, ", ")
}

func naukriLink(staticURL, jdURL, id string) string {
	for _, link := range []string{staticURL, jdURL} {
		link = strings.TrimSpace(link)
		switch {
		case strings.HasPrefix(link, "http://"), strings.HasPrefix(link, "https://"):
			return link
		case strings.HasPrefix(link, "/"):
			return "https://www.naukri.com" + link
		case link != "":
			return "https://www.naukri.com/" + link
		}
	}
	if id != "" {
		return "https://www.naukri.com/job-listings-" + id
	}
	return ""
}

func naukriTime(value any) time.Time {
	switch typed := value.(type) {
	case float64:
		return parseUnix(typed)
	case string:
		return parseTime(typed)
	case json.Number:
		n, err := typed.Float64()
		if err != nil {
			return time.Time{}
		}
		return parseUnix(n)
	default:
		return time.Time{}
	}
}

func indeedID(link string) string {
	parsed, err := url.Parse(link)
	if err == nil {
		if jk := parsed.Query().Get("jk"); jk != "" {
			return jk
		}
	}
	return link
}

func companyFromIndeedTitle(title string) string {
	if parts := strings.Split(title, " - "); len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return ""
}

func clampDays(lookback time.Duration, max int) int {
	days := int(lookback.Hours() / 24)
	if days < 1 {
		return 1
	}
	if days > max {
		return max
	}
	return days
}
