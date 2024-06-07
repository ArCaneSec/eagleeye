package jobs

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type job struct {
	duration time.Duration
	job      gocron.Job
	task     func()
	active   bool
}

func notif(data string) {
	fmt.Println(data)
}

func errNotif(err error, val string) {
	fmt.Println(err, val)
}

func execute(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	err := cmd.Run()

	if err != nil {
		// stderr, _ := cmd.StderrPipe()
		// op, _ := io.ReadAll(stderr)
		return "", err
	}

	stdout, _ := cmd.StdoutPipe()
	op, _ := io.ReadAll(stdout)

	return string(op), nil
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
	if !job.active {
		return fmt.Errorf("job id %d is already inactive", id)
	}

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

	j, _ := s.core.NewJob(gocron.DurationJob(job.duration), gocron.NewTask(job.task), gocron.WithStartAt(gocron.WithStartImmediately()))

	job.job = j
	job.active = true

	return nil
}

func ScheduleJobs() *Scheduler {
	s, _ := gocron.NewScheduler(gocron.WithLimitConcurrentJobs(1, gocron.LimitModeWait))

	jobs := []*job{
		{5 * time.Second, nil, subdomainEnumerate, false},
		// {5 * time.Second, nil, test, false},
	}

	scheduler := &Scheduler{s, jobs}

	for id := range scheduler.jobs {
		scheduler.ActiveJob(id + 1)
	}
	s.Start()

	return scheduler
}

func subdomainEnumerate() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	op, err := execute(ctx, "subfinder", "-s", "-d", "memoryleaks.ir")

	if err != nil {
		errNotif(err, op)
		return
	}

	notif(op)
}

func test() {
	fmt.Println("test job")
	time.Sleep(3 * time.Second)
}
