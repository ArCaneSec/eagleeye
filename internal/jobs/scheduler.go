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

type taskDetails struct {
	name     string
	taskFunc func(context.Context)
}

type job struct {
	duration  time.Duration
	cronJob   gocron.Job
	task      taskDetails
	active    bool
	cDuration time.Duration
	killer    context.CancelFunc
	isRunning bool
	subTasks  []taskDetails
}

func (j *job) runTask() {
	ctx, cancel := context.WithTimeout(context.Background(), j.cDuration)
	defer cancel()

	j.killer = cancel

	log.Printf("[*] %s started...\n", j.task.name)

	j.isRunning = true
	j.task.taskFunc(ctx)

	log.Printf("[#] %s finished.\n", j.task.name)

	if len(j.subTasks) != 0 {
		for _, task := range j.subTasks {
			log.Printf("[#-*] %s started...\n", task.name)

			task.taskFunc(ctx)
			log.Printf("[#-#] %s finished.\n", task.name)

		}
	}
	j.isRunning = false
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
		{
			duration:  6 * time.Hour,
			task:      taskDetails{"Subdomain Enumeration", t.subdomainEnumerate},
			cDuration: 1 * time.Hour,
			subTasks: []taskDetails{
				{"Resolve New Subs", t.resolveNewSubs},
				{"Service Discovery (news)", t.httpDiscovery}},
		},
		// {
		// 	duration:  6 * time.Hour,
		// 	task:      taskDetails{"Resolve New Subs", t.resolveNewSubs},
		// 	cDuration: 1 * time.Hour,
		// },
		// {
		// 	duration: 6 * time.Hour,
		// 	task: taskDetails{
		// 		name:     "Service Discovery (news)",
		// 		taskFunc: t.httpDiscovery,
		// 	},
		// 	cDuration: 1 * time.Hour,
		// },
		{
			duration:  48 * time.Hour,
			task:      taskDetails{"Dns Resolve", t.dnsResolveAll},
			cDuration: 1 * time.Hour,
			subTasks: []taskDetails{},
		},
	}

	scheduler := &Scheduler{s, jobs}

	s.Start()
	for id := range scheduler.jobs {
		scheduler.ActiveJob(id + 1)
		time.Sleep(5 * time.Millisecond)
	}

	return scheduler
}
