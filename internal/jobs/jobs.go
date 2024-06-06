package jobs

import (
	"fmt"

	"time"

	"github.com/go-co-op/gocron/v2"
)

type job struct {
	duration time.Duration
	job      gocron.Job
	task     func()
	active   bool
}

type Scheduler struct {
	core gocron.Scheduler
	jobs []*job
}

func (s *Scheduler) DeactiveJob(id int) error {
	if id == 0 || id > len(s.jobs) {
		return fmt.Errorf("invalid id: %d", id)
	}

	job := s.jobs[id-1]
	s.core.RemoveJob(job.job.ID())
	job.active = false

	return nil
}

func (s *Scheduler) ActiveJob(id int) error {
	if id == 0 || id > len(s.jobs) {
		return fmt.Errorf("invalid id: %d", id)
	}

	job := s.jobs[id-1]
	if job.active {
		return fmt.Errorf("job id %d is already active", id)
	}

	j, _ := s.core.NewJob(gocron.DurationJob(job.duration), gocron.NewTask(job.task))

	job.job = j
	job.active = true

	return nil
}

func ScheduleJobs() *Scheduler {
	s, _ := gocron.NewScheduler()

	jobs := []*job{
		{2 * time.Second, nil, subdomainEnumerate, false},
	}

	scheduler := &Scheduler{s, jobs}

	for id := range scheduler.jobs {
		scheduler.ActiveJob(id + 1)
	}

	s.Start()

	return scheduler
}

func subdomainEnumerate() {
	fmt.Println("test")
}
