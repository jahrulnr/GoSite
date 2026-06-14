package app

import (
	"context"
	"log"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	cronsvc "github.com/jahrulnr/gosite/internal/service/cron"
)

// runCronScheduler dispatches cron jobs on interval boundaries.
func runCronScheduler(ctx context.Context, repo *sqlite.CronJobRepository, jobs *sqlite.JobRepository, enqueue func(int64)) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	prev := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cronJobs, err := repo.List(ctx)
			if err != nil {
				log.Printf("cron scheduler: list: %v", err)
				prev = now
				continue
			}
			for _, job := range cronJobs {
				if !cronsvc.ShouldRun(prev, now, job.RunEvery) {
					continue
				}
				run, err := jobs.Create(ctx, sqlite.JobRun{
					JobType: "cron",
					Name:    job.Name,
					Status:  sqlite.JobStatusPending,
					Output:  job.Payload,
				})
				if err != nil {
					log.Printf("cron scheduler: create job: %v", err)
					continue
				}
				enqueue(run.ID)
				if err := repo.TouchExecutedAt(ctx, job.ID); err != nil {
					log.Printf("cron scheduler: touch executed_at: %v", err)
				}
			}
			prev = now
		}
	}
}
