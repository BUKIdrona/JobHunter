package source

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"jobhunter/internal/matcher"
	"jobhunter/internal/model"
	"jobhunter/internal/textutil"
)

var goWord = regexp.MustCompile(`(?i)(^|[^a-z])go([^a-z]|$)`)

type ghCandidate struct {
	board    string
	job      ghJob
	posted   time.Time
	remote   bool
	priority int
}

func fetchGreenhouse(ctx context.Context, boards []string, minScore int, acceptAnyRemote bool, lookback time.Duration, now time.Time, maxDetails int) []model.Job {
	var candidates []ghCandidate
	for _, board := range boards {
		if ctx.Err() != nil {
			break
		}
		board = strings.TrimSpace(board)
		if board == "" {
			continue
		}
		found, err := greenhouseCandidates(ctx, board, minScore, acceptAnyRemote, lookback, now)
		if err != nil {
			log.Printf("greenhouse %s: %v", board, err)
			continue
		}
		log.Printf("greenhouse %s: %d listings worth opening", board, len(found))
		candidates = append(candidates, found...)
		sleep(ctx, 150*time.Millisecond)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].priority == candidates[j].priority {
			return candidates[i].posted.After(candidates[j].posted)
		}
		return candidates[i].priority > candidates[j].priority
	})
	if maxDetails > 0 && len(candidates) > maxDetails {
		log.Printf("greenhouse: opening %d of %d likely listings", maxDetails, len(candidates))
		candidates = candidates[:maxDetails]
	}
	var out []model.Job
	for _, cand := range candidates {
		if ctx.Err() != nil {
			break
		}
		detail, err := greenhouseDetail(ctx, cand.board, cand.job.ID)
		if err != nil {
			log.Printf("greenhouse %s job %d: %v", cand.board, cand.job.ID, err)
			sleep(ctx, 100*time.Millisecond)
			continue
		}
		plain, emails := textutil.Prepare(detail.Content)
		company := strings.TrimSpace(firstNonEmpty(detail.CompanyName, cand.job.CompanyName, companyFromToken(cand.board)))
		link := firstLink(detail.AbsoluteURL, cand.job.AbsoluteURL)
		loc := firstNonEmpty(detail.Location.Name, cand.job.Location.Name)
		out = append(out, model.Job{
			ID:          fmt.Sprintf("%s-%d", cand.board, cand.job.ID),
			Source:      SourceGreenhouse,
			Title:       strings.TrimSpace(cand.job.Title),
			Company:     company,
			Location:    strings.TrimSpace(loc),
			URL:         link,
			Description: plain,
			Emails:      emails,
			PostedAt:    cand.posted,
			Remote:      cand.remote || strings.Contains(strings.ToLower(loc), "remote"),
		})
		sleep(ctx, 80*time.Millisecond)
	}
	return out
}

func greenhouseCandidates(ctx context.Context, board string, minScore int, acceptAnyRemote bool, lookback time.Duration, now time.Time) ([]ghCandidate, error) {
	body, err := getBytes(ctx, fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs", board))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Jobs []ghJob `json:"jobs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var out []ghCandidate
	for _, job := range payload.Jobs {
		posted := parseTime(job.FirstPublished)
		if posted.IsZero() {
			posted = parseTime(job.UpdatedAt)
		}
		if !matcher.Fresh(posted, now, lookback) {
			continue
		}
		if deadline := parseTime(job.ApplicationDeadline); !deadline.IsZero() && deadline.Before(now) {
			continue
		}
		link := strings.TrimSpace(job.AbsoluteURL)
		remote := strings.Contains(strings.ToLower(job.Location.Name), "remote")
		pre := matcher.Evaluate(model.Job{
			Title:    job.Title,
			URL:      link,
			Location: job.Location.Name,
			Remote:   remote,
		}, minScore, acceptAnyRemote)
		if !worthDetail(pre.Reject) {
			continue
		}
		out = append(out, ghCandidate{
			board:    board,
			job:      job,
			posted:   posted,
			remote:   remote,
			priority: titlePriority(job.Title),
		})
	}
	return out, nil
}

func titlePriority(title string) int {
	l := strings.ToLower(title)
	score := 0
	if strings.Contains(l, "golang") || goWord.MatchString(title) {
		score += 5
	}
	if strings.Contains(l, "c++") || strings.Contains(l, "cpp") {
		score += 4
	}
	if strings.Contains(l, "backend") || strings.Contains(l, "back-end") || strings.Contains(l, "back end") {
		score += 3
	}
	if strings.Contains(l, "platform") || strings.Contains(l, "infrastructure") || strings.Contains(l, "sre") || strings.Contains(l, "kubernetes") {
		score += 2
	}
	return score
}

type ghJob struct {
	ID                  int64  `json:"id"`
	Title               string `json:"title"`
	AbsoluteURL         string `json:"absolute_url"`
	CompanyName         string `json:"company_name"`
	FirstPublished      string `json:"first_published"`
	UpdatedAt           string `json:"updated_at"`
	ApplicationDeadline string `json:"application_deadline"`
	Content             string `json:"content"`
	Location            struct {
		Name string `json:"name"`
	} `json:"location"`
}

func greenhouseDetail(ctx context.Context, board string, id int64) (ghJob, error) {
	body, err := getBytes(ctx, fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs/%d", board, id))
	if err != nil {
		return ghJob{}, err
	}
	var job ghJob
	if err := json.Unmarshal(body, &job); err != nil {
		return ghJob{}, err
	}
	return job, nil
}

func worthDetail(reject string) bool {
	switch reject {
	case "", "no Go, C/C++, or Python overlap", "fewer than two matching skills", "score below threshold":
		return true
	default:
		return false
	}
}

func fetchLever(ctx context.Context, boards []string, lookback time.Duration, now time.Time) []model.Job {
	var out []model.Job
	for _, board := range boards {
		if ctx.Err() != nil {
			break
		}
		board = strings.TrimSpace(board)
		if board == "" {
			continue
		}
		var posts []leverPost
		err := getJSON(ctx, "https://api.lever.co/v0/postings/"+board+"?mode=json", &posts)
		if err != nil {
			log.Printf("lever %s: %v", board, err)
			continue
		}
		kept := 0
		for _, post := range posts {
			posted := parseUnix(post.CreatedAt)
			if !matcher.Fresh(posted, now, lookback) {
				continue
			}
			if strings.Contains(strings.ToLower(post.Categories.Commitment), "intern") {
				continue
			}
			text := strings.Join([]string{
				post.DescriptionPlain,
				post.DescriptionBodyPlain,
				post.OpeningPlain,
				post.Description,
				post.AdditionalPlain,
			}, "\n")
			for _, list := range post.Lists {
				text += "\n" + list.Text + "\n" + list.Content
			}
			plain, emails := textutil.Prepare(text)
			loc := post.Categories.Location
			remote := strings.EqualFold(post.WorkplaceType, "remote") || strings.Contains(strings.ToLower(loc), "remote")
			out = append(out, model.Job{
				ID:          board + "-" + post.ID,
				Source:      SourceLever,
				Title:       strings.TrimSpace(post.Text),
				Company:     companyFromToken(board),
				Location:    strings.TrimSpace(loc),
				URL:         firstLink(post.ApplyURL, post.HostedURL),
				Description: plain,
				Emails:      emails,
				PostedAt:    posted,
				Remote:      remote,
			})
			kept++
		}
		log.Printf("lever %s: %d recent jobs", board, kept)
		sleep(ctx, 200*time.Millisecond)
	}
	return out
}

type leverPost struct {
	ID                   string  `json:"id"`
	Text                 string  `json:"text"`
	HostedURL            string  `json:"hostedUrl"`
	ApplyURL             string  `json:"applyUrl"`
	CreatedAt            float64 `json:"createdAt"`
	Description          string  `json:"description"`
	DescriptionPlain     string  `json:"descriptionPlain"`
	DescriptionBodyPlain string  `json:"descriptionBodyPlain"`
	OpeningPlain         string  `json:"openingPlain"`
	AdditionalPlain      string  `json:"additionalPlain"`
	WorkplaceType        string  `json:"workplaceType"`
	Categories           struct {
		Location   string `json:"location"`
		Commitment string `json:"commitment"`
	} `json:"categories"`
	Lists []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	} `json:"lists"`
}

func fetchAshby(ctx context.Context, boards []string, lookback time.Duration, now time.Time) []model.Job {
	var out []model.Job
	for _, board := range boards {
		if ctx.Err() != nil {
			break
		}
		board = strings.TrimSpace(board)
		if board == "" {
			continue
		}
		var payload struct {
			Jobs []struct {
				ID               string `json:"id"`
				Title            string `json:"title"`
				Location         string `json:"location"`
				IsListed         *bool  `json:"isListed"`
				IsRemote         bool   `json:"isRemote"`
				WorkplaceType    string `json:"workplaceType"`
				PublishedAt      string `json:"publishedAt"`
				JobURL           string `json:"jobUrl"`
				ApplyURL         string `json:"applyUrl"`
				DescriptionHTML  string `json:"descriptionHtml"`
				DescriptionPlain string `json:"descriptionPlain"`
				EmploymentType   string `json:"employmentType"`
			} `json:"jobs"`
		}
		err := getJSON(ctx, "https://api.ashbyhq.com/posting-api/job-board/"+board, &payload)
		if err != nil {
			log.Printf("ashby %s: %v", board, err)
			continue
		}
		kept := 0
		for _, job := range payload.Jobs {
			if job.IsListed != nil && !*job.IsListed {
				continue
			}
			if strings.Contains(strings.ToLower(job.EmploymentType), "intern") {
				continue
			}
			posted := parseTime(job.PublishedAt)
			if !matcher.Fresh(posted, now, lookback) {
				continue
			}
			plain, emails := textutil.Prepare(job.DescriptionPlain + "\n" + job.DescriptionHTML)
			remote := job.IsRemote || strings.EqualFold(job.WorkplaceType, "remote") || strings.Contains(strings.ToLower(job.Location), "remote")
			out = append(out, model.Job{
				ID:          board + "-" + job.ID,
				Source:      SourceAshby,
				Title:       strings.TrimSpace(job.Title),
				Company:     companyFromToken(board),
				Location:    strings.TrimSpace(job.Location),
				URL:         firstLink(job.ApplyURL, job.JobURL),
				Description: plain,
				Emails:      emails,
				PostedAt:    posted,
				Remote:      remote,
			})
			kept++
		}
		log.Printf("ashby %s: %d recent jobs", board, kept)
		sleep(ctx, 200*time.Millisecond)
	}
	return out
}
