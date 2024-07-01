package jobs

import (
	"context"
	"fmt"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (r *RunNewTemplates) Start(ctx context.Context, isSubTask bool) {
	r.wg.Add(1)
	defer r.wg.Done()

	log.Println("[*] RunNewTemplates started...")
	var err error

	templatesPath, err := r.fetchConfig(ctx)
	if err != nil {
		r.notify.ErrNotif(err)
		return
	}

	hosts, err := r.fetchAssets(ctx)
	if err != nil {
		r.notify.ErrNotif(err)
		return
	}

	output, err := r.runCommand(ctx, templatesPath, hosts)
	if err != nil {
		r.notify.ErrNotif(err)
		return
	}

	results, err := checkResults(output)
	if err != nil {
		if _, ok := err.(ErrNoResult); ok {
			log.Println("[*] RunNewTemplates finished.")
			return
		}

		r.notify.ErrNotif(err)
		return
	}

	r.wg.Add(1)
	go r.notify.NucleiResultsNotif(results)
	log.Println("[*] RunNewTemplates finished.")
}

func (r *RunNewTemplates) fetchConfig(ctx context.Context) (string, error) {
	opts := options.FindOne().SetProjection(bson.M{"_id": 0, "nucleiUpdatePath": 1})

	var res bson.M
	err := r.db.Collection("config").FindOne(ctx, bson.M{}, opts).Decode(&res)
	if err != nil {
		return "", fmt.Errorf("[!] Error while fetching config from db: %w", err)
	}

	return res["nucleiUpdatePath"].(string), nil
}

func (r *RunNewTemplates) fetchAssets(ctx context.Context) ([]string, error) {
	opts := options.Find().SetProjection(bson.M{"_id": 0, "host": 1})

	cursor, err := r.db.Collection("http-services").Find(ctx, bson.M{"isActive": true}, opts)
	if err != nil {
		return nil, fmt.Errorf("[!] Error fetching assets from db: %w", err)
	}

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("[!] Error deserializing hosts from db: %w", err)
	}

	hosts := make([]string, 0, len(results))
	for _, hostMap := range results {
		hosts = append(hosts, hostMap["host"].(string))
	}

	return hosts, nil
}

func (r *RunNewTemplates) runCommand(ctx context.Context, tmplPath string, hosts []string) (string, error) {
	tempFile, err := os.CreateTemp("/tmp/", "hosts")
	if err != nil {
		return "", fmt.Errorf("[!] Error creating temp file: %w", err)
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	for _, host := range hosts {
		tempFile.WriteString(fmt.Sprintf("%s\n", host))
	}

	results, err := execute(ctx, r.scriptPath, tmplPath, tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("[!] Error while executing new tempaltes script: %w", err)
	}

	return results, nil
}
