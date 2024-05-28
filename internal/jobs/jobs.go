package jobs

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type job struct {
	id       int
	duration time.Duration
	job      gocron.Job
	task     func()
	active   bool
}

type schedular struct {
	core gocron.Scheduler
	jobs []job
}

func (s *schedular) DeactiveJob(id int) error {
	if id == 0 || id > len(s.jobs) {
		return fmt.Errorf("invalid id: %d", id)
	}

	job := s.jobs[id-1]
	s.core.RemoveJob(job.job.ID())
	job.active = false

	return nil
}

func (s *schedular) ActiveJob(id int) error {
	if id == 0 || id > len(s.jobs) {
		return fmt.Errorf("invalid id: %d", id)
	}

	job := s.jobs[id-1]

	j, _ := s.core.NewJob(gocron.DurationJob(job.duration), gocron.NewTask(job.task))

	job.job = j
	job.active = true

	return nil
}

func ScheduleJobs() *schedular {
	s, _ := gocron.NewScheduler()

	jobs := []job{
		{1, time.Minute, nil, SubdomainEnumerate, true},
	}

	schedular := &schedular{s, jobs}

	for id, _ := range schedular.jobs {
		schedular.ActiveJob(id + 1)
	}

	s.Start()

	return schedular
}

func SubdomainEnumerate() {
	fmt.Println("test")
}
