package services

import (
	"context"
	"log"
	"sync"
	"time"
)

// JobInterval is how often registered jobs run (research.md: every minute).
const JobInterval = time.Minute

// Job is a named unit of periodic work.
type Job struct {
	Name string
	Run  func(ctx context.Context) error
}

// JobRunner runs registered jobs on a ticker with logging and panic recovery: a failing or
// panicking job never stops the loop or the other jobs.
type JobRunner struct {
	mu       sync.Mutex
	jobs     []Job
	interval time.Duration
}

// NewJobRunner creates a runner that ticks every JobInterval.
func NewJobRunner() *JobRunner { return &JobRunner{interval: JobInterval} }

// Register adds a job.
func (r *JobRunner) Register(name string, run func(ctx context.Context) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, Job{Name: name, Run: run})
}

// RunOnce runs every job once, in registration order.
func (r *JobRunner) RunOnce(ctx context.Context) {
	r.mu.Lock()
	jobs := append([]Job(nil), r.jobs...)
	r.mu.Unlock()
	for _, j := range jobs {
		runJob(ctx, j)
	}
}

// Run blocks, running the jobs immediately and then every interval, until ctx is cancelled.
func (r *JobRunner) Run(ctx context.Context) {
	r.RunOnce(ctx)
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.RunOnce(ctx)
		}
	}
}

func runJob(ctx context.Context, j Job) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("job %s panicked: %v", j.Name, p)
		}
	}()
	if err := j.Run(ctx); err != nil {
		log.Printf("job %s failed: %v", j.Name, err)
	}
}
