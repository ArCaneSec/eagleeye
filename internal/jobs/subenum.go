package jobs

import (
	m "EagleEye/internal/models"
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *SubdomainEnumeration) Start(ctx context.Context, isSubTask bool) {
	s.wg.Add(1)
	defer s.wg.Done()

	name := reflect.TypeOf(s).Elem().Name()
	log.Printf("[*] %s started...\n", name)

	if err := s.fetchAssets(ctx); err != nil {
		s.notify.ErrNotif(err)
	}

	var output string
	var err error

	for _, target := range s.targets {

		for _, domain := range target.Scope {
			select {
			case <-ctx.Done():
				s.notify.ErrNotif(
					fmt.Errorf("[!] Context deadline exceeds in subdomain enumeration job"),
				)
				return

			default:
				output, err = s.runCommand(ctx, domain)
				if err != nil {
					s.notify.ErrNotif(err)
					return
				}

				checkNinsert := func() {
					subs, err := s.checkResults(output, target.ID)
					if err != nil {
						if _, ok := err.(ErrNoResult); ok {
							return
						}
						s.notify.ErrNotif(err)
						return
					}

					s.insertDB(ctx, subs, target, domain)
				}

				if !isSubTask {
					s.wg.Add(1)
					go func() {
						defer s.wg.Done()
						checkNinsert()
					}()
				} else {
					checkNinsert()
				}
			}
		}
	}
	log.Printf("[#] %s finished.\n", name)
}

func (t *SubdomainEnumeration) fetchAssets(ctx context.Context) error {
	cursor, _ := t.db.Collection("targets").Find(ctx, bson.D{{}})

	if err := cursor.All(ctx, &t.targets); err != nil {
		return fmt.Errorf("[!] Error while fetching targets: %w", err)
	}
	return nil
}

func (t *SubdomainEnumeration) runCommand(ctx context.Context, domain string) (string, error) {

	log.Printf("[~] Current domain: %s\n", domain)

	op, err := execute(ctx, t.scriptPath, domain)

	if err != nil {
		return "", fmt.Errorf("[!] Error while enumerating subdomains: %w", err)
	}

	return op, err
}

func (t *SubdomainEnumeration) checkResults(output string, id primitive.ObjectID) ([]interface{}, error) {
	subdomains := make([]interface{}, 0, 100)
	now := time.Now()

	subs := strings.Split(strings.TrimSpace(output), "\n")
	if len(subs) == 0 {
		return nil, ErrNoResult{}
	}

	for _, sub := range subs {
		subdomains = append(subdomains, m.Subdomain{Target: id, Subdomain: sub, Created: now})
	}

	return subdomains, nil
}

func (t *SubdomainEnumeration) insertDB(ctx context.Context, subs []interface{}, target m.Target, domain string) {
	cursor := t.db.Collection("subdomains")
	breakAfterFirstFail := options.InsertMany().SetOrdered(false)

	val, err := t.db.Collection("subdomains").InsertMany(ctx, subs, breakAfterFirstFail)
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		t.notify.ErrNotif(fmt.Errorf("[!] Error while inserting subdomains to database: %w", err))
		return
	}

	if len(val.InsertedIDs) != 0 {
		log.Printf("[+] Found %d new subdomains for %s.\n", len(val.InsertedIDs), target.Name)

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
