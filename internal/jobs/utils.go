package jobs

import (
	m "EagleEye/internal/models"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func fetchTargets(ctx context.Context, db *mongo.Database) ([]m.Target, error) {
	var targets []m.Target

	cursor, _ := db.Collection("targets").Find(ctx, bson.D{{}})

	if err := cursor.All(ctx, &targets); err != nil {
		return []m.Target{}, err
	}

	return targets, nil
}

func fetchSubs(ctx context.Context, db *mongo.Database) ([]m.Subdomain, error) {
	var subs []m.Subdomain

	cursor, _ := db.Collection("subdomains").Find(ctx, bson.D{{}})
	if err := cursor.All(ctx, &subs); err != nil {
		return []m.Subdomain{}, err
	}

	return subs, nil
}

func fetchNewSubs(ctx context.Context, db *mongo.Database) ([]m.Subdomain, error) {
	var subs []m.Subdomain
	values := bson.D{{"subdomain", 1}, {"_id", 0}}

	cursor, _ := db.Collection("subdomains").Find(
		ctx,
		bson.D{{"dns", nil}}, options.Find().SetProjection(values))

	if err := cursor.All(ctx, &subs); err != nil {
		return []m.Subdomain{}, err
	}

	return subs, nil
}

func fetchNewSubsWithIP(ctx context.Context, db *mongo.Database) ([]m.Subdomain, error) {
	var subs []m.Subdomain

	cursor, _ := db.Collection("subdomains").Find(
		ctx,
		bson.D{{"$and",
			bson.A{
				bson.D{{"dns", bson.D{{"$ne", nil}}}},
				bson.D{{"dns.created",
					bson.D{{"$gt", primitive.NewDateTimeFromTime(
						time.Now().Add(-24 * time.Hour),
					)}}}},
			}}},
	)

	if err := cursor.All(ctx, &subs); err != nil {
		return []m.Subdomain{}, err
	}

	return subs, nil
}

func (t *task) insertSubs(ctx context.Context, wg *sync.WaitGroup, op string, target m.Target, domain string) {
	defer wg.Done()

	var subdomains []interface{}
	cursor := t.db.Collection("subdomains")
	now := time.Now()

	subs := strings.Split(op, "\n")

	for _, sub := range subs {
		sub = strings.TrimSpace(sub)
		if sub == "" {
			continue
		}
		subdomains = append(subdomains, m.Subdomain{Target: target.ID, Subdomain: sub, Created: now})
	}

	breakAfterFirstFail := options.InsertMany().SetOrdered(false)

	val, err := t.db.Collection("subdomains").InsertMany(ctx, subdomains, breakAfterFirstFail)
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		t.notify.ErrNotif("[!] Error while inserting subdomains to database", err)
		return
	}

	t.log(
		fmt.Sprintf("[+] Found %d new subdomains for %s.",
			len(val.InsertedIDs),
			target.Name),
	)

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

		t.notify.NewAssetNotif(target.Name, domain, allSubs)
	}
}
