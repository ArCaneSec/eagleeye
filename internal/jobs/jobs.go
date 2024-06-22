package jobs

import (
	m "EagleEye/internal/models"
	"context"
	"fmt"
	"regexp"

	"os"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (t *task) subdomainEnumerate(ctx context.Context, wg *sync.WaitGroup) {
	targets, err := fetchTargets(ctx, t.db)
	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	for _, target := range targets {

		for _, domain := range target.Scope {
			select {
			case <-ctx.Done():
				t.notify.ErrNotif(
					fmt.Errorf("[!] Context deadline exceeds in subdomain enumeration job"),
				)
				return

			default:
				t.log(fmt.Sprintf("[~] Current domain: %s", domain))

				op, err := t.execute(ctx, "subfinder", "-d", domain, "-all", "-silent")

				if err != nil {
					t.notify.ErrNotif(fmt.Errorf("[!] Error while enumerating subdomains: %w", err))
					continue
				}

				wg.Add(1)
				go func() {
					defer wg.Done()
					t.insertSubs(ctx, op, target, domain)
				}()
			}
		}
	}
}

func (t *task) resolveNewSubs(ctx context.Context, wg *sync.WaitGroup) {
	newSubs, err := fetchNewSubs(ctx, t.db)
	if len(newSubs) == 0 {
		t.log("[~] Didn't find any new sub without A record.")
		return
	}

	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	tempFile, subsMap, err := tempFileNMap(newSubs)

	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	defer os.Remove(tempFile)

	op, err := t.execute(ctx,
		"/home/arcane/automation/resolve.sh",
		tempFile,
	)

	if err != nil {
		t.notify.ErrNotif(fmt.Errorf("[!] Error while resolving new subdomains: %w", err))
		return
	}

	wg.Add(1)
	go func() {
		resolvedSubs := strings.Split(strings.TrimSpace(op), "\n")

		if len(resolvedSubs) == 1 && resolvedSubs[0] == op {
			t.log("[~] Didn't find any A record for new subs.")
			return
		} else {
			t.log(fmt.Sprintf("[~] Found %d new A records.", len(resolvedSubs)))
		}

		now := time.Now()

		updates := make([]mongo.WriteModel, 0, len(newSubs))
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
			t.notify.ErrNotif(fmt.Errorf("[!] Error while updating subdomains: %w", err))
			return
		}

		t.notify.NewDnsNotif(resolvedSubs)
	}()
}

func (t *task) httpDiscovery(ctx context.Context, wg *sync.WaitGroup) {
	subsWithIP, err := fetchNewSubsWithIP(ctx, t.db)
	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	tempFile, subsMap, err := tempFileNMap(subsWithIP)

	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	defer os.Remove(tempFile)

	op, err := t.execute(ctx,
		"/home/arcane/automation/discovery.sh",
		tempFile,
	)

	if err != nil {
		t.notify.ErrNotif(fmt.Errorf("[!] Error service discovering new subdomains: %w", err))
		return
	}

	wg.Add(1)
	go func() {
		resolvedHosts := strings.Split(strings.TrimSpace(op), "\n")
		if len(resolvedHosts) == 1 && resolvedHosts[0] == "" {
			return
		}

		t.log(fmt.Sprintf("[+] Found %d new http services.", len(resolvedHosts)))

		rhPattern := regexp.MustCompile(`(?:^https?:\/\/)(.*\..*$)`)

		now := time.Now()
		updates := make([]mongo.WriteModel, 0, len(subsWithIP))

		for _, host := range resolvedHosts {
			port, err := getPort(host)
			if err != nil {
				t.notify.ErrNotif(err)
				return
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

			delete(subsMap, host)
		}
		for _, notResolvedHost := range subsMap {
			updates = append(
				updates,
				mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": notResolvedHost.ID}).
					SetUpdate(bson.D{{"$set",
						bson.D{{"http", []m.Http{{
							IsActive: false,
							Port:     0,
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
			t.notify.ErrNotif(fmt.Errorf("[!] Error while updating http field for new assets: %w", err))
			return
		}

		t.notify.NewHttpNotif(resolvedHosts)
	}()
}

func (t *task) dnsResolveAll(ctx context.Context, wg *sync.WaitGroup) {
	subs, err := fetchAllSubs(ctx, t.db)

	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	tempFile, subsMap, err := tempFileNMap(subs)

	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	defer os.Remove(tempFile)

	op, err := t.execute(ctx, "/home/arcane/automation/resolve.sh", tempFile)
	if err != nil {
		t.notify.ErrNotif(fmt.Errorf("[!] Error while resolving all subdomains: %w", err))
		return
	}

	wg.Add(1)
	go func() {
		now := time.Now()
		resolvedSubs := strings.Split(strings.TrimSpace(op), "\n")

		updates := make([]mongo.WriteModel, 0, len(subs))
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
			t.notify.ErrNotif(fmt.Errorf("[!] Error updating all subdomains with new resolved subs: %w", err))
			return
		}

		if len(newResolvedSubs) == 0 {
			return
		}
		t.notify.NewDnsNotif(newResolvedSubs)
	}()
}

func (t *task) httpDiscoveryAll(ctx context.Context, wg *sync.WaitGroup) {
	subs, err := fetchAllSubs(ctx, t.db)
	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	tempFile, _, err := tempFileNMap(subs)
	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	defer os.Remove(tempFile)

	op, err := t.execute(
		ctx,
		"/home/arcane/automation/discovery.sh",
		tempFile,
	)
	if err != nil {
		t.notify.ErrNotif(fmt.Errorf("[!] Error while http discovering all subdomains: %w", err))
		return
	}

	wg.Add(1)
	go func() {
		hosts := strings.Split(strings.TrimSpace(op), "\n")

		if len(hosts) == 1 && hosts[0] == "" {
			t.log("[~] No new http services has been found on all assets.")
			return
		} else {
			t.log(fmt.Sprint("[~] Found %d new http services.", len(hosts)))
		}

		// now := time.Now()
		// updates := make([]mongo.WriteModel, 0, len(subs))
	}()

}

func (t *task) testJob(ctx context.Context, wg *sync.WaitGroup) {
	for i := range 10 {
		fmt.Println(i)
		time.Sleep(time.Second)
	}
}
