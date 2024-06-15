package jobs

import (
	m "EagleEye/internal/models"
	"context"
	"fmt"
	"regexp"
	"strconv"

	"os"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)


func (t *task) checkErr(err error){
	if err != nil {
		t.notify.ErrNotif("", err)
	}
}

func (t *task) subdomainEnumerate(ctx context.Context) {
	targets, err := fetchTargets(ctx, t.db)
	if err != nil {
		t.notify.ErrNotif("", err)
		return
	}

	var wg sync.WaitGroup

	for _, target := range targets {

		for _, domain := range target.Scope {
			select {
			case <-ctx.Done():
				t.notify.ErrNotif("",
					fmt.Errorf("[!] Context deadline exceeds in subdomain enumeration job"),
				)
				return

			default:
				t.log(fmt.Sprintf("[~] Current domain: %s", domain))

				op, err := t.execute(ctx, "subfinder", "-d", domain, "-all", "-silent")

				if err != nil {
					t.notify.ErrNotif(op, err)
					continue
				}

				wg.Add(1)
				go t.insertSubs(ctx, &wg, op, target, domain)
			}
		}
	}
	wg.Wait()
}

func (t *task) resolveNewSubs(ctx context.Context) {
	newSubs, err := fetchNewSubs(ctx, t.db)
	if len(newSubs) == 0 {
		t.log("[~] Didn't find any new sub without A record.")
		return
	}

	t.checkErr(err)

	tempFile, subsMap, err := WriteToTempFile(newSubs)
	t.checkErr(err)
	defer os.Remove(tempFile)


	op, err := t.execute(ctx,
		"/home/arcane/automation/resolve.sh",
		tempFile,
	)

	if err != nil {
		t.notify.ErrNotif("[!] Error while resolving new subdomains", err)
		return
	}

	resolvedSubs := strings.Split(strings.TrimSpace(op), "\n")

	if len(resolvedSubs) == 1 && resolvedSubs[0] == op {
		t.log("[~] Didn't find any A record for new subs.")
		return
	} else {
		t.log(fmt.Sprintf("[~] Found %d new A records.", len(resolvedSubs)))
	}

	now := time.Now()

	var updates []mongo.WriteModel
	for _, resolvedSub := range resolvedSubs {
		subObj := subsMap[resolvedSub]
		updates = append(updates,
			mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": subObj.ID}).
				SetUpdate(bson.M{"$set": bson.M{"dns": &m.Dns{IsActive: true, Created: now, Updated: now}}}),
		)

		delete(subsMap, resolvedSub)
	}

	for _, notResolvedSub := range subsMap {
		updates = append(updates,
			mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": notResolvedSub.ID}).
				SetUpdate(bson.M{"$set": bson.M{"dns": &m.Dns{IsActive: false, Created: now, Updated: now}}}),
		)
	}

	opts := options.BulkWrite().SetOrdered(true)
	_, err = t.db.Collection("subdomains").BulkWrite(ctx, updates, opts)

	if err != nil {
		t.notify.ErrNotif("[!] Error while updating subdomains", err)
		return
	}

	t.notify.NewDnsNotif(resolvedSubs)
}

func (t *task) httpDiscovery(ctx context.Context) {
	subsWithIP, err := fetchNewSubsWithIP(ctx, t.db)
	t.checkErr(err)

	tempFile, subsMap, err := WriteToTempFile(subsWithIP)
	t.checkErr(err)
	defer os.Remove(tempFile)

	op, err := t.execute(ctx,
		"/home/arcane/automation/discovery.sh",
		tempFile,
	)

	if err != nil {
		t.notify.ErrNotif("[!] Error service discovering new subdomains", err)
		return
	}

	hosts := strings.Split(strings.TrimSpace(op), "\n")
	if len(hosts) == 1 && hosts[0] == "" {
		return
	}

	t.log(fmt.Sprintf("[+] Found %d new http services.", len(hosts)))

	rhPattern := regexp.MustCompile(`(?:^https?:\/\/)(.*\..*$)`)

	now := time.Now()
	var updates []mongo.WriteModel

	for _, host := range hosts {
		var port int

		switch {
		case strings.HasPrefix(host, "http:"):
			port = 80
		case strings.HasPrefix(host, "https:"):
			port = 443
		default:
			port, err = strconv.Atoi(strings.Split(host, ":")[1])
			if err != nil {
				t.notify.ErrNotif(fmt.Sprintf("[!] Couldn't find port of %s", host), err)
				return
			}
		}
		rawHost := rhPattern.FindStringSubmatch(host)[1]
		subObj := subsMap[rawHost]

		updates = append(
			updates,
			mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": subObj.ID}).
				SetUpdate(bson.D{{"$set",
					bson.D{{"http", []m.Http{{
						IsActive: true,
						Port:     port,
						Created:  now,
						Updated:  now,
					}},
					}},
				}},
				),
		)
	}
	opts := options.BulkWrite().SetOrdered(true)
	_, err = t.db.Collection("subdomains").BulkWrite(ctx, updates, opts)

	if err != nil {
		t.notify.ErrNotif("[!] Error while updating http field for new assets.", err)
		return
	}

	t.notify.NewHttpNotif(hosts)
}

func (t *task) dnsResolveAll(ctx context.Context) {
	subs, err := fetchAllSubs(ctx, t.db)
	t.checkErr(err)

	tempFile, subsMap, err := WriteToTempFile(subs)
	t.checkErr(err)

	defer os.Remove(tempFile)

	op, err := t.execute(ctx, "/home/arcane/automation/resolve.sh", tempFile)
	if err != nil {
		t.notify.ErrNotif("[!] Error while resolving all subdomains", err)
		return
	}

	now := time.Now()
	resolvedSubs := strings.Split(strings.TrimSpace(op), "\n")

	var updates []mongo.WriteModel
	var newResolvedSubs []string

	for _, resolvedSub := range resolvedSubs {
		subObj := subsMap[resolvedSub]

		if subObj.Dns == nil {
			updates = append(
				updates,
				mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": subObj.ID}).
					SetUpdate(bson.D{{"$set",
						bson.D{{"dns",
							&m.Dns{IsActive: true, Created: now, Updated: now}}},
					}},
					))
			newResolvedSubs = append(newResolvedSubs, resolvedSub)
		} else {
			if !subObj.Dns.IsActive {
				newResolvedSubs = append(newResolvedSubs, resolvedSub)
			}
			updates = append(
				updates,
				mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": subObj.ID}).
					SetUpdate(bson.D{{"$set",
						bson.M{"dns.isActive": true, "dns.updated": now},
					}},
					))
		}

		delete(subsMap, resolvedSub)
	}

	for _, notResolvedSub := range subsMap {
		updates = append(updates, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": notResolvedSub.ID}).
			SetUpdate(bson.M{"$set": bson.M{"dns.isActive": false, "dns.updated": now}}))
	}

	opts := options.BulkWrite().SetOrdered(true)
	_, err = t.db.Collection("subdomains").BulkWrite(ctx, updates, opts)
	if err != nil {
		t.notify.ErrNotif("[!] Error updating all subdomains with new resolved subs", err)
		return
	}

	if len(newResolvedSubs) == 0 {
		return
	}
	t.notify.NewDnsNotif(newResolvedSubs)
}


