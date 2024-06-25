package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (h *HttpDiscovery) Start(ctx context.Context) {
	startRegularTask(ctx, h, h.Dependencies.wg)
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
		return "", fmt.Errorf("[!] Error service discovering subdomains: %w", err)
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
	now := time.Now()
	updates := make([]mongo.WriteModel, 0, len(t.hosts))
	var newHttpServices []string

	for _, host := range results {
		hostWithPort := extractHost(host)
		httpObj := t.httpMap[hostWithPort]

		if httpObj.Created == nil {
			updates = append(updates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": httpObj.ID}).
				SetUpdate(bson.M{"$set": bson.M{"isActive": true, "created": now, "updated": now}}))
			newHttpServices = append(newHttpServices, host)
		} else {
			if !httpObj.IsActive {
				newHttpServices = append(newHttpServices, host)
			}
			updates = append(updates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": httpObj.ID}).
				SetUpdate(bson.M{"$set": bson.M{"isActive": true, "updated": now}}))
		}
		delete(t.httpMap, hostWithPort)
	}

	for _, notResolvedhost := range t.httpMap {
		if notResolvedhost.Created == nil {
			updates = append(updates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": notResolvedhost.ID}).
				SetUpdate(bson.M{"$set": bson.M{"isActive": false, "created": now, "updated": now}}))
		} else {
			updates = append(updates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": notResolvedhost.ID}).
				SetUpdate(bson.M{"$set": bson.M{"isActive": false, "updated": now}}))
		}
	}

	_, err := t.db.Collection("http-services").BulkWrite(ctx, updates)
	if err != nil {
		// t.notify.ErrNotif(fmt.Errorf("[!] Error while updating http field for new assets: %w", err))
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
