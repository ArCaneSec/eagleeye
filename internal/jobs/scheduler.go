package jobs

import (
	"EagleEye/internal/notifs"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

type taskFunc func(context.Context)

type job struct {
	name      string
	duration  time.Duration
	cronJob   gocron.Job
	task      taskFunc
	active    bool
	cDuration time.Duration
	killer    context.CancelFunc
	isRunning bool
	// subTask   taskFunc
}

func (j *job) runTask() {
	ctx, cancel := context.WithTimeout(context.Background(), j.cDuration)
	defer cancel()

	j.killer = cancel

	log.Printf("[*] %s started...\n", j.name)

	j.isRunning = true
	j.task(ctx)
	j.isRunning = false

	log.Printf("[#] %s finished.\n", j.name)

	// if j.subTask != nil {
	// 	j.subTask(ctx)
	// }
}

type task struct {
	db     *mongo.Database
	notify notifs.Notify
}

func (t *task) log(msg string) {
	log.Println(msg)
}

func (t *task) execute(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	op, err := cmd.CombinedOutput()

	if err != nil {
		return string(op), err
	}

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

	s.core.RemoveJob(job.cronJob.ID())
	job.active = false

	if job.isRunning {
		job.killer()
	}

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

	j, _ := s.core.NewJob(
		gocron.DurationJob(job.duration),
		gocron.NewTask(job.runTask),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)

	job.cronJob = j
	job.active = true

	return nil
}

func ScheduleJobs(db *mongo.Database) *Scheduler {
	s, _ := gocron.NewScheduler(gocron.WithLimitConcurrentJobs(3, gocron.LimitModeWait))
	t := &task{db, notifs.NewNotif(os.Getenv("DISCORD_WEBHOOK"))}

	jobs := []*job{
		// {name: "Subdomain Enumeration",duration: 6 * time.Hour, task: t.subdomainEnumerate, cDuration: 1 * time.Hour},
		{name: "Resolve New Subs", duration: 6 * time.Hour, task: t.resolveNewSubs, cDuration: 1 * time.Hour},
	}

	scheduler := &Scheduler{s, jobs}

	s.Start()
	for id := range scheduler.jobs {
		scheduler.ActiveJob(id + 1)
		time.Sleep(5 * time.Millisecond)
	}

	return scheduler
}
