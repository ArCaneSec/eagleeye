package jobs

import (
	m "EagleEye/internal/models"
	"EagleEye/internal/notifs"
	"context"
	"log"
	"reflect"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
)

type ErrNoResult struct{}

func (err ErrNoResult) Error() string {
	return ""
}

type Task interface {
	Start(context.Context, bool)
}

type regularTask interface {
	fetchAssets(context.Context) error
	runCommand(context.Context) (string, error)
	checkResults(string) ([]string, error)
	insertDB(context.Context, []string) error
	ErrNotif(error)
}

func startRegularTask(ctx context.Context, t regularTask, wg *sync.WaitGroup, isSubTask bool) {
	wg.Add(1)
	defer wg.Done()

	name := reflect.TypeOf(t).Elem().Name()
	log.Printf("[*] %s started...\n", name)

	if err := t.fetchAssets(ctx); err != nil {
		t.ErrNotif(err)
		return
	}

	output, err := t.runCommand(ctx)
	if err != nil {
		t.ErrNotif(err)
		return
	}

	checkNinsert := func() {
		results, err := t.checkResults(output)
		if err != nil {
			if _, ok := err.(ErrNoResult); ok {
				log.Printf("[#] %s finished successfully.\n", name)
				return
			}
			t.ErrNotif(err)
			return
		}

		if err := t.insertDB(ctx, results); err != nil {
			t.ErrNotif(err)
			return
		}
		log.Printf("[#] %s finished successfully.\n", name)
	}

	if !isSubTask {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checkNinsert()
		}()
		return
	}
	checkNinsert()
}

type Dependencies struct {
	db     *mongo.Database
	notify notifs.Notify
	wg     *sync.WaitGroup
}

type SubdomainEnumeration struct {
	*Dependencies
	scriptPath string
	targets    []m.Target
}

type DnsResolve struct {
	*Dependencies
	scriptPath string
	subdomains []m.Subdomain
	subsMap    map[string]*m.Subdomain
}

type DnsResolveAll struct {
	*DnsResolve
}

type HttpDiscovery struct {
	*Dependencies
	scriptPath string
	hosts      []m.HttpService
	httpMap    map[string]*m.HttpService
}

type HttpDiscoveryAll struct {
	*HttpDiscovery
}

type UpdateNuclei struct {
	*Dependencies
	scriptPath string
	configFile string `bson:"scriptsConfigFile"`
}

type RunNewTemplates struct {
	*Dependencies
	scriptPath string
}