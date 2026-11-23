package source

import (
	"context"
	"sync"
	"time"

	"jobhunter/internal/config"
	"jobhunter/internal/model"
)

// Collect pulls every configured feed. One failing source does not stop the others.
// Company career boards are listed first so a duplicate keeps the company apply link.
func Collect(ctx context.Context, cfg *config.Config, now time.Time) []model.Job {
	type batch struct {
		name string
		jobs []model.Job
	}
	var (
		mu      sync.Mutex
		batches []batch
		wg      sync.WaitGroup
	)
	run := func(name string, fn func() []model.Job) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jobs := fn()
			mu.Lock()
			batches = append(batches, batch{name: name, jobs: jobs})
			mu.Unlock()
		}()
	}

	run("greenhouse", func() []model.Job {
		return fetchGreenhouse(ctx, cfg.Greenhouse, cfg.MinScore, cfg.AcceptAnyRemote, cfg.Lookback, now, cfg.MaxDetailFetches)
	})
	run("lever", func() []model.Job {
		return fetchLever(ctx, cfg.Lever, cfg.Lookback, now)
	})
	run("ashby", func() []model.Job {
		return fetchAshby(ctx, cfg.Ashby, cfg.Lookback, now)
	})
	run("indeed", func() []model.Job {
		return fetchIndeed(ctx, cfg.Indeed, cfg.Lookback)
	})
	run("naukri", func() []model.Job {
		return fetchNaukri(ctx, cfg.Naukri, cfg.Lookback)
	})
	run("remotive", func() []model.Job {
		return fetchRemotive(ctx, cfg.RemotiveQueries)
	})
	run("jobicy", func() []model.Job {
		return fetchJobicy(ctx, cfg.JobicyTags)
	})
	run("remoteok", func() []model.Job {
		return fetchRemoteOK(ctx)
	})
	run("arbeitnow", func() []model.Job {
		return fetchArbeitnow(ctx)
	})
	wg.Wait()

	order := []string{"greenhouse", "lever", "ashby", "indeed", "naukri", "remotive", "jobicy", "remoteok", "arbeitnow"}
	byName := map[string][]model.Job{}
	for _, b := range batches {
		byName[b.name] = b.jobs
	}
	var all []model.Job
	for _, name := range order {
		all = append(all, byName[name]...)
	}
	return all
}
