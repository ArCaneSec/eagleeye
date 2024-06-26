package jobs

import (
	m "EagleEye/internal/models"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (d *DnsResolve) Start(ctx context.Context, isSubTask bool) {
	startRegularTask(ctx, d, d.Dependencies.wg, isSubTask)
}

func (d *DnsResolveAll) Start(ctx context.Context, isSubTask bool) {
	startRegularTask(ctx, d, d.Dependencies.wg, isSubTask)
}

func (d *DnsResolve) fetchAssets(ctx context.Context) error {
	cursor, _ := d.db.Collection("subdomains").Find(
		ctx,
		bson.D{{"dns", nil}})

	if err := cursor.All(ctx, &d.subdomains); err != nil {
		return fmt.Errorf("[!] Error while fetching new subs: %w", err)
	}
	
	return nil
}

func (d *DnsResolveAll) fetchAssets(ctx context.Context) error {
	cursor, _ := d.db.Collection("subdomains").Find(ctx, bson.D{{}})
	if err := cursor.All(ctx, &d.subdomains); err != nil {
		return fmt.Errorf("[!] Error while fetching targets: %w", err)
	}

	return nil
}

func (d *DnsResolve) runCommand(ctx context.Context) (string, error) {
	tempFile, subsMap, err := tempFileNSubsMap(d.subdomains)

	if err != nil {
		return "", err
	}

	d.subsMap = subsMap

	defer os.Remove(tempFile)

	op, err := execute(ctx, d.scriptPath, tempFile)
	if err != nil {
		return "", fmt.Errorf("[!] Error while resolving all subdomains: %w", err)
	}

	return op, nil
}

func (d *DnsResolve) checkResults(result string) ([]string, error) {
	resolvedSubs := strings.Split(strings.TrimSpace(result), "\n")
	if len(resolvedSubs) == 1 && resolvedSubs[0] == "" {
		log.Println("[~] Didn't find any dns records.")
		return nil, ErrNoResult{}
	}

	return resolvedSubs, nil
}

func (d *DnsResolve) insertDB(ctx context.Context, subs []string) error {
	now := time.Now()
	updates := make([]mongo.WriteModel, 0, len(d.subdomains))
	https := make([]interface{}, 0, len(subs)*2)
	newResolvedSubs := make([]string, 0, len(subs))

	for _, resolvedSub := range subs {
		subObj := d.subsMap[resolvedSub]

		if subObj.Dns == nil {
			updates = append(
				updates,
				mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": subObj.ID}).
					SetUpdate(bson.D{{"$set", bson.D{{"dns", &m.Dns{IsActive: true, Created: now, Updated: now}}}}}))
			newResolvedSubs = append(newResolvedSubs, resolvedSub)
		} else {
			if !subObj.Dns.IsActive {
				newResolvedSubs = append(newResolvedSubs, resolvedSub)
			}
			updates = append(
				updates,
				mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": subObj.ID}).
					SetUpdate(bson.D{{"$set", bson.M{"dns.isActive": true, "dns.updated": now}}}))
		}
		createEmptyHttps(&https, *subObj)
		delete(d.subsMap, resolvedSub)
	}

	for _, notResolvedSub := range d.subsMap {
		if notResolvedSub.Dns == nil {
			updates = append(
				updates,
				mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": notResolvedSub.ID}).
					SetUpdate(bson.D{{"$set", bson.D{{"dns", &m.Dns{IsActive: false, Created: now, Updated: now}}}}}))
		} else {
			updates = append(updates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": notResolvedSub.ID}).
				SetUpdate(bson.M{"$set": bson.M{"dns.isActive": false, "dns.updated": now}}))
		}
	}

	// ses, _ := t.db.Client().StartSession()
	// ses.StartTransaction()
	_, err := d.db.Collection("subdomains").BulkWrite(ctx, updates)
	if err != nil {
		return fmt.Errorf(
			"[!] Error updating all subdomains with new resolved subs: %w", err,
		)
	}

	// keep inserting if you've found already existed http doc.
	httpOpts := options.InsertMany().SetOrdered(false)

	_, err = d.db.Collection("http-services").InsertMany(ctx, https, httpOpts)
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return fmt.Errorf(
			"[!] Error while creating empty http objects for resolved subs: %w", err,
		)
	}
	// ses.CommitTransaction(ctx)

	if len(newResolvedSubs) != 0 {
		log.Printf("[+] Found %d new dns records.\n", len(newResolvedSubs))
		d.notify.NewDnsNotif(newResolvedSubs)
	}
	return nil
}

func (d *DnsResolve) ErrNotif(err error) {
	d.notify.ErrNotif(err)
}
