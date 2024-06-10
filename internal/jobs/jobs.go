package jobs

import (
	m "EagleEye/internal/models"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type taskFunc func(context.Context)

type job struct {
	duration  time.Duration
	cronJob   gocron.Job
	task      taskFunc
	active    bool
	cDuration time.Duration
	killer    context.CancelFunc
	isRunning bool
}

func (j *job) runTask() {
	ctx, cancel := context.WithTimeout(context.Background(), j.cDuration)
	defer cancel()

	j.killer = cancel
	j.isRunning = true
	j.task(ctx)
	j.isRunning = false
}

type task struct {
	db      *mongo.Database
	webhook string
}

func (t *task) newAssetNotif(target string, domain string, asset []string) {
	d := NewInfo(t.webhook)

	// var l []string
	// for i := 0; i < 100; i++ {
	// 	l = append(l, fmt.Sprint(i))

	// }
	// strAssets := strings.Join(l, "\n")
	strAssets := strings.Join(asset, "\n")

	d.SendMessage("Subdomain Enumeration", fmt.Sprintf("%d new subdomains found for %s", len(asset), target), fmt.Sprintf("domain: %s", domain), strAssets)
}

func (t *task) errNotif(detail string, err error) {
	fmt.Println(detail)
	fmt.Println(err)
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

	j, _ := s.core.NewJob(gocron.DurationJob(job.duration), gocron.NewTask(job.runTask), gocron.WithStartAt(gocron.WithStartImmediately()))

	job.cronJob = j
	job.active = true

	return nil
}

func ScheduleJobs(db *mongo.Database) *Scheduler {
	s, _ := gocron.NewScheduler(gocron.WithLimitConcurrentJobs(3, gocron.LimitModeWait))
	t := task{db, os.Getenv("DISCORD_WEBHOOK")}

	jobs := []*job{
		{duration: 6 * time.Hour, task: t.subdomainEnumerate, cDuration: 1 * time.Hour},
	}

	scheduler := &Scheduler{s, jobs}

	s.Start()
	for id := range scheduler.jobs {
		scheduler.ActiveJob(id + 1)
		time.Sleep(5 * time.Millisecond)
	}

	return scheduler
}

func (t *task) subdomainEnumerate(ctx context.Context) {
	var targets []m.Target

	val, _ := t.db.Collection("targets").Find(ctx, bson.D{{}})

	if err := val.All(ctx, &targets); err != nil {
		fmt.Println(err)
		return
	}
	now := time.Now()

	cursor := t.db.Collection("subdomains")

	for _, target := range targets {

		for _, domain := range target.Scope {
			select {
			case <-ctx.Done():
				t.errNotif("", fmt.Errorf("[!] Context deadline exceeds in subdomain enumeration job"))
				return

			default:
				fmt.Printf("[~] Current domain: %s\n", domain)

				op, err := t.execute(ctx, "subfinder", "-d", domain, "-all", "-silent")

				if err != nil {
					t.errNotif(op, err)
					continue
				}

				var subdomains []interface{}

				subs := strings.Split(op, "\n")

				for _, sub := range subs {
					sub = strings.TrimSpace(sub)
					if sub == "" {
						continue
					}
					fmt.Println("found:", sub)
					subdomains = append(subdomains, m.Subdomain{Target: target.ID, Subdomain: sub, Created: now})
				}

				breakAfterFirstFail := options.InsertMany().SetOrdered(false)

				val, err := cursor.InsertMany(ctx, subdomains, breakAfterFirstFail)
				if err != nil && !mongo.IsDuplicateKeyError(err) {
					t.errNotif("[!] Error while inserting subdomains to database", err)
					return
				}

				fmt.Printf("[+] Found %d new subdomains.\n", len(val.InsertedIDs))

				if len(val.InsertedIDs) != 0 {
					filter := bson.D{{"_id", bson.D{{"$in", val.InsertedIDs}}}}
					values := options.Find().SetProjection(bson.D{{"subdomain", 1}})

					newSubsRecords, _ := cursor.Find(context.TODO(), filter, values)
					var newSubsObjs []m.Subdomain

					newSubsRecords.All(context.TODO(), &newSubsObjs)

					var allSubs []string
					for _, subObj := range newSubsObjs {
						allSubs = append(allSubs, subObj.Subdomain)
					}

					t.newAssetNotif(target.Name, domain, allSubs)
				}
			}
		}
	}
	fmt.Println("[#] Subdomain enumeration job finished.")
}
