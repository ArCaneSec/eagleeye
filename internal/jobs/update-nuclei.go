package jobs

import (
	"context"
	"fmt"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (u *UpdateNuclei) Start(ctx context.Context, isSubTask bool) {
	u.wg.Add(1)
	defer u.wg.Done()
	log.Println("[*] UpdateNuclei started...")

	if err := u.fetchConfig(ctx); err != nil {
		u.notify.ErrNotif(err)
		return
	}

	output, err := u.runCommand(ctx)
	if err != nil {
		u.notify.ErrNotif(err)
		return
	}

	res, err := u.checkResults(output)
	if err != nil {
		if _, ok := err.(ErrNoResult); ok {
			log.Println("[#] UpdateNuclei finished.")
			return
		}

		u.notify.ErrNotif(err)
		return
	}
	log.Printf("[+] Found %s new templates.\n", res)
	log.Println("[#] UpdateNuclei finished.")
}

func (u *UpdateNuclei) fetchConfig(ctx context.Context) error {
	opts := options.FindOne().SetProjection(bson.M{"_id": 0, "scriptsConfigFile": 1})

	v := bson.M{}
	err := u.db.Collection("config").FindOne(ctx, bson.M{}, opts).Decode(&v)
	if err != nil {
		return fmt.Errorf("[!] Error fetching config file: %w", err)
	}

	value, ok := v["scriptsConfigFile"].(string)
	if !ok {
		return fmt.Errorf("[!] Error while deserializing data from db, invalid map key.")
	}

	u.configFile = value
	return nil
}

func (u *UpdateNuclei) runCommand(ctx context.Context) (string, error) {
	results, err := execute(ctx, u.scriptPath, u.configFile)

	if err != nil {
		return "", fmt.Errorf("[!] Error while executing update nuclei command: %w, %s", err, results)
	}

	return results, nil
}

func (u *UpdateNuclei) checkResults(output string) (string, error) {
	tmplCounts := strings.TrimSpace(output)
	if tmplCounts == "0" {
		return "", ErrNoResult{}
	}

	return tmplCounts, nil
}
