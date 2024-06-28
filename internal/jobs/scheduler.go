package jobs

import (
	"EagleEye/internal/notifs"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

type job struct {
	duration  time.Duration
	cronJob   gocron.Job
	task      Task
	active    bool
	cDuration time.Duration
	killer    context.CancelFunc
	isRunning bool
	subTasks  []Task
}

func (j *job) runTask() {
	j.isRunning = true
	defer func() {
		j.isRunning = false
	}()

	ctx, cancel := context.WithTimeout(context.Background(), j.cDuration)
	j.killer = cancel

	if j.subTasks != nil {
		j.task.Start(ctx, true)
		for _, task := range j.subTasks {
			task.Start(ctx, true)
		}
		return
	}
	j.task.Start(ctx, false)
}

func execute(ctx context.Context, command string, args ...string) (string, error) {
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
	wg   *sync.WaitGroup
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

func (s *Scheduler) Shutdown() error {
	for _, job := range s.jobs {
		if !job.active {
			continue
		}

		err := s.core.RemoveJob(job.cronJob.ID())
		if err != nil {
			return fmt.Errorf("error while shutting scheduler down: %w", err)
		}
		job.active = false
	}
	return nil
}

func ScheduleJobs(db *mongo.Database, wg *sync.WaitGroup) *Scheduler {
	s, _ := gocron.NewScheduler(gocron.WithLimitConcurrentJobs(1, gocron.LimitModeWait))
	notifier := notifs.NewNotif(os.Getenv("DISCORD_WEBHOOK"))
	deps := &Dependencies{
		db:     db,
		notify: notifier,
		wg:     wg,
	}

	jobs := []*job{
		subdomainEnumerationJob(deps),
		dnsResolveAllJob(deps),
		httpDiscoveryAllJob(deps),
	}

	scheduler := &Scheduler{s, jobs, wg}

	s.Start()
	for id := range scheduler.jobs {
		scheduler.ActiveJob(id + 1)
		time.Sleep(5 * time.Millisecond)
	}

	return scheduler
}

func dnsResolveAllJob(d *Dependencies) *job {
	return &job{
		duration: 1 * time.Hour,
		task: &DnsResolveAll{
			&DnsResolve{
				Dependencies: d,
				scriptPath:   "/home/arcane/automation/resolve.sh",
			},
		},
		cDuration: 2 * time.Hour,
	}
}

func subdomainEnumerationJob(d *Dependencies) *job {
	return &job{
		duration: 1 * time.Hour,
		task: &SubdomainEnumeration{
			Dependencies: d,
			scriptPath:   "/home/arcane/automation/enumeration.sh",
		},
		cDuration: 2 * time.Hour,
		subTasks: []Task{
			&DnsResolve{
				Dependencies: d,
				scriptPath:   "/home/arcane/automation/resolve.sh",
			},
			&HttpDiscovery{
				Dependencies: d,
				scriptPath:   "/home/arcane/automation/discovery.sh",
			},
		},
	}
}

func httpDiscoveryAllJob(d *Dependencies) *job {
	return &job{
		duration: 1 * time.Hour,
		task: &HttpDiscoveryAll{
			HttpDiscovery: &HttpDiscovery{
				Dependencies: d,
				scriptPath:   "/home/arcane/automation/discovery.sh",
			},
		},
		cDuration: 2 * time.Hour,
	}
}
