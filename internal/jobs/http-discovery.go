package jobs

import (
	m "github.com/ArCaneSec/eagleeye/pkg/models"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (h *HttpDiscovery) Start(ctx context.Context, isSubTask bool) {
	startRegularTask(ctx, h, h.Dependencies.wg, isSubTask)
}

func (h *HttpDiscoveryAll) Start(ctx context.Context, isSubTask bool) {
	startRegularTask(ctx, h, h.Dependencies.wg, isSubTask)
}

func (h *HttpDiscovery) fetchAssets(ctx context.Context) error {
	cursor, _ := h.db.Collection("http-services").Find(
		ctx,
		bson.M{"created": nil},
	)

	if err := cursor.All(ctx, &h.hosts); err != nil {
		return fmt.Errorf("[!] Error while fetching new services: %w", err)
	}

	return nil
}

func (t *HttpDiscoveryAll) fetchAssets(ctx context.Context) error {
	cursor, _ := t.db.Collection("http-services").Find(ctx, bson.M{})
	if err := cursor.All(ctx, &t.hosts); err != nil {
		return fmt.Errorf("[!] Error while fetching http services: %w", err)
	}

	return nil
}

func (h *HttpDiscovery) runCommand(ctx context.Context) (string, error) {
	tempFile, httpMap, err := tempFileNServicesMap(h.hosts)

	if err != nil {
		return "", err
	}
	h.httpMap = httpMap

	defer os.Remove(tempFile)

	op, err := execute(ctx,
		"/home/arcane/automation/discovery.sh",
		tempFile,
	)

	if err != nil {
		return "", fmt.Errorf("[!] Error service discovering subdomains: %w", op)
	}

	return op, nil
}

func (t *HttpDiscovery) checkResults(output string) ([]string, error) {
	resolvedHosts := strings.Split(strings.TrimSpace(output), "\n")
	if len(resolvedHosts) == 1 && resolvedHosts[0] == "" {
		log.Println("[~] Didn't find any new http services.")
		return nil, ErrNoResult{}
	}

	return resolvedHosts, nil
}

func (t *HttpDiscovery) insertDB(ctx context.Context, results []string) error {

	var (
		now             = time.Now()
		updates         = make([]mongo.WriteModel, 0, len(t.hosts))
		newHttpServices = make([]string, 0, len(t.hosts)/2)
		url             string
		hostWithPort    string
		httpObj         *m.HttpService
		ok              bool
	)

	for _, host := range results {
		url, hostWithPort = extractHostNUrl(host)
		httpObj, ok = t.httpMap[url]

		if !ok {
			httpObj = t.httpMap[hostWithPort]
		}

		if httpObj.Created == nil {
			updates = append(updates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": httpObj.ID}).
				SetUpdate(bson.M{"$set": bson.M{"host": url, "isActive": true, "created": now, "updated": now}}))
			newHttpServices = append(newHttpServices, host)

			// When http service is created for the first time, host value is schemeless, check dns resolve job.
			delete(t.httpMap, hostWithPort)
		} else {
			if !httpObj.IsActive {
				newHttpServices = append(newHttpServices, host)
			}
			updates = append(updates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": httpObj.ID}).
				SetUpdate(bson.M{"$set": bson.M{"isActive": true, "updated": now}}))

			delete(t.httpMap, url)
		}
	}

	for _, notResolvedhost := range t.httpMap {
			updates = append(updates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": notResolvedhost.ID}).
				SetUpdate(bson.M{"$set": bson.M{"isActive": false, "updated": now}}))
	}

	_, err := t.db.Collection("http-services").BulkWrite(ctx, updates)
	if err != nil {
		return fmt.Errorf("[!] Error while updating http field for new assets: %w", err)
	}

	if len(newHttpServices) != 0 {
		log.Printf("[+] Found %d new http services.\n", len(newHttpServices))
		t.notify.NewHttpNotif(newHttpServices)
	}

	return nil
}

func (h *HttpDiscovery) ErrNotif(err error) {
	h.notify.ErrNotif(err)
}
