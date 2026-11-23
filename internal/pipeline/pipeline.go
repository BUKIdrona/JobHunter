package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"jobhunter/internal/config"
	"jobhunter/internal/mailer"
	"jobhunter/internal/matcher"
	"jobhunter/internal/model"
	"jobhunter/internal/source"
	"jobhunter/internal/store"
)

// Run collects new postings, emails hiring contacts when a posting lists one,
// and sends the remaining roles to the candidate.
func Run(ctx context.Context, cfg *config.Config) error {
	st, err := store.Open(cfg.StatePath)
	if err != nil {
		return err
	}
	if cfg.DryRun {
		log.Printf("dry-run: messages will be written to %s; nothing is sent and state is not updated", cfg.OutboxDir)
	}

	now := time.Now()
	fetchCtx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	jobs := source.Collect(fetchCtx, cfg, now)
	cancel()

	unique := matcher.Dedupe(jobs)
	fresh := 0
	var matches []model.Match
	for _, job := range unique {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !matcher.Fresh(job.PostedAt, now, cfg.Lookback) {
			continue
		}
		fresh++
		result := matcher.Evaluate(job, cfg.MinScore, cfg.AcceptAnyRemote)
		if !result.OK {
			continue
		}
		if rec, ok := st.Get(jobKey(job)); ok && (rec.DigestSent || rec.HRStatus == store.HRSent) {
			continue
		}
		matches = append(matches, model.Match{Job: job, Score: result.Score, Skills: result.Skills})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Job.PostedAt.After(matches[j].Job.PostedAt)
		}
		return matches[i].Score > matches[j].Score
	})
	log.Printf("fetched %d postings, %d unique, %d inside the lookback, %d new matches", len(jobs), len(unique), fresh, len(matches))
	if len(matches) > cfg.MaxDigest {
		matches = matches[:cfg.MaxDigest]
	}

	sender := mailer.Sender{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
		FromName: cfg.User.Name,
		DryRun:   cfg.DryRun,
		Outbox:   cfg.OutboxDir,
	}

	resume, resumeErr := os.ReadFile(cfg.User.Resume)
	if resumeErr != nil {
		log.Printf("resume %s: %v", cfg.User.Resume, resumeErr)
	}

	budget := cfg.MaxHRPerRun
	if left := cfg.MaxHRPerDay - st.HRSentToday(now); left < budget {
		budget = left
	}
	if budget < 0 {
		budget = 0
	}

	var sent []mailer.Item
	var apply []mailer.Item
	var mark []store.Record
	queued := 0

	for _, rec := range st.PendingDigest() {
		sent = append(sent, itemFromRecord(rec))
		rec.DigestSent = true
		mark = append(mark, rec)
	}

	for _, match := range matches {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("match %d [%s] %s @ %s — %s", match.Score, strings.Join(match.Skills, ", "), match.Job.Title, match.Job.Company, match.Job.URL)
		rec := recordFrom(match)
		if prev, ok := st.Get(rec.Key); ok {
			rec.FirstSeen = prev.FirstSeen
			rec.HRStatus = prev.HRStatus
			rec.HREmail = prev.HREmail
			rec.HRSentAt = prev.HRSentAt
			rec.DigestSent = prev.DigestSent
		}
		if rec.HRStatus == store.HRSent {
			continue
		}

		email := mailer.PickRecruiter(match.Job.Emails, cfg.User.Email)
		if email != "" && rec.HRStatus != store.HRFailed {
			if resumeErr != nil {
				apply = append(apply, toItem(match, email, "Resume file was missing, so the hiring contact was not emailed."))
				rec.HREmail = email
				rec.HRStatus = store.HRFailed
				rec.DigestSent = true
				mark = append(mark, rec)
				continue
			}
			if budget <= 0 {
				queued++
				continue
			}
			msg := mailer.Application(cfg.User, match.Job, match.Skills)
			msg.To = email
			msg.Attach = resume
			msg.AttachName = "Deepak_Kumar_Mandal_Resume.pdf"
			if err := sender.Send(msg); err != nil {
				log.Printf("hiring email %s for %s: %v", email, match.Job.Title, err)
				apply = append(apply, toItem(match, email, "Could not send to the hiring contact. Use the link to apply."))
				rec.HREmail = email
				rec.HRStatus = store.HRFailed
				rec.DigestSent = true
				mark = append(mark, rec)
				continue
			}
			log.Printf("emailed %s about %s at %s", email, match.Job.Title, match.Job.Company)
			rec.HREmail = email
			rec.HRStatus = store.HRSent
			rec.HRSentAt = now
			rec.DigestSent = false
			if err := remember(cfg, st, rec); err != nil {
				return err
			}
			sent = append(sent, toItem(match, email, ""))
			budget--
			rec.DigestSent = true
			mark = append(mark, rec)
			continue
		}

		apply = append(apply, toItem(match, "", ""))
		if rec.HRStatus == "" {
			rec.HRStatus = store.HRNone
		}
		rec.DigestSent = true
		mark = append(mark, rec)
	}

	if queued > 0 {
		log.Printf("%d hiring emails are queued for a later run because the send cap was reached", queued)
	}
	if len(sent)+len(apply) == 0 {
		log.Printf("no new mail to send")
		return nil
	}

	digest := mailer.DigestMessage(cfg.User.Email, mailer.Digest{
		Name:  firstName(cfg.User.Name),
		Sent:  sent,
		Apply: apply,
	})
	if err := sender.Send(digest); err != nil {
		return fmt.Errorf("digest to %s: %w", cfg.User.Email, err)
	}
	if cfg.DryRun {
		log.Printf("not sent to %s. Add SMTP_PASSWORD to .env to deliver %d hiring emails and %d apply links", cfg.User.Email, len(sent), len(apply))
		return nil
	}
	log.Printf("mailed %s (%d hiring emails, %d apply links)", cfg.User.Email, len(sent), len(apply))
	for _, rec := range mark {
		if err := st.Put(rec); err != nil {
			return err
		}
	}
	return nil
}

func remember(cfg *config.Config, st *store.Store, rec store.Record) error {
	if cfg.DryRun {
		return nil
	}
	return st.Put(rec)
}

func recordFrom(match model.Match) store.Record {
	return store.Record{
		Key:      jobKey(match.Job),
		Title:    match.Job.Title,
		Company:  match.Job.Company,
		URL:      match.Job.URL,
		Location: match.Job.Location,
		Source:   match.Job.Source,
		Skills:   match.Skills,
		Score:    match.Score,
	}
}

func jobKey(job model.Job) string {
	if job.ID != "" {
		return job.Source + ":" + job.ID
	}
	return strings.ToLower(job.Source + "|" + job.Company + "|" + job.Title + "|" + job.URL)
}

func toItem(match model.Match, email, note string) mailer.Item {
	posted := ""
	if !match.Job.PostedAt.IsZero() {
		posted = match.Job.PostedAt.Local().Format("02 Jan 2006")
	}
	return mailer.Item{
		Title:       match.Job.Title,
		Company:     match.Job.Company,
		Location:    match.Job.Location,
		URL:         match.Job.URL,
		Source:      match.Job.Source,
		Salary:      match.Job.Salary,
		Email:       email,
		Posted:      posted,
		Note:        note,
		Score:       match.Score,
		Skills:      strings.Join(match.Skills, ", "),
		Description: excerpt(match.Job.Description, 1200),
	}
}

func excerpt(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	chars := []rune(s)
	if len(chars) <= n {
		return s
	}
	cut := string(chars[:n])
	if i := strings.LastIndex(cut, " "); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "..."
}

func itemFromRecord(rec store.Record) mailer.Item {
	return mailer.Item{
		Title:    rec.Title,
		Company:  rec.Company,
		Location: rec.Location,
		URL:      rec.URL,
		Source:   rec.Source,
		Email:    rec.HREmail,
		Score:    rec.Score,
		Skills:   strings.Join(rec.Skills, ", "),
		Note:     "Hiring email was already sent.",
	}
}

func firstName(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	if len(fields) == 0 {
		return "there"
	}
	return fields[0]
}
