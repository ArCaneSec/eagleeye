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

func (t *SubdomainEnumeration) run(ctx context.Context) {
	err := t.fetchAssets(ctx)
	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	for _, target := range t.targets {
		for _, domain := range target.Scope {
			select {
			case <-ctx.Done():
				t.notify.ErrNotif(
					fmt.Errorf("[!] Context deadline exceeds in subdomain enumeration job"),
				)
				return

			default:
				log.Printf("[~] Current domain: %s\n", domain)

				op, err := execute(ctx, t.scriptPath, domain)

				if err != nil {
					t.notify.ErrNotif(fmt.Errorf("[!] Error while enumerating subdomains: %w", err))
					continue
				}

				t.wg.Add(1)
				go func() {
					defer t.wg.Done()
					t.insertSubs(ctx, op, target, domain)
				}()
			}
		}
	}
}

func (t *DnsResolve) run(ctx context.Context) {
	err := t.fetchAssets(ctx)

	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	tempFile, subsMap, err := tempFileNSubsMap(t.subdomains)

	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	defer os.Remove(tempFile)

	op, err := execute(ctx, t.scriptPath, tempFile)
	if err != nil {
		t.notify.ErrNotif(fmt.Errorf("[!] Error while resolving all subdomains: %w", err))
		return
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		now := time.Now()
		resolvedSubs := strings.Split(strings.TrimSpace(op), "\n")

		if len(resolvedSubs) == 1 && resolvedSubs[0] == "" {
			log.Println("[~] Didn't find any dns records.")
			return
		}

		updates := make([]mongo.WriteModel, 0, len(t.subdomains))
		https := make([]interface{}, 0, len(resolvedSubs)*2)
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
			createEmptyHttps(&https, *subObj)
			delete(subsMap, resolvedSub)
		}

		for _, notResolvedSub := range subsMap {
			if notResolvedSub.Dns == nil {
				updates = append(
					updates,
					mongo.NewUpdateOneModel().
						SetFilter(bson.M{"_id": notResolvedSub.ID}).
						SetUpdate(bson.D{{"$set",
							bson.D{{"dns",
								&m.Dns{IsActive: false, Created: now, Updated: now}}},
						}},
						))
			} else {
				updates = append(updates, mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": notResolvedSub.ID}).
					SetUpdate(bson.M{"$set": bson.M{"dns.isActive": false, "dns.updated": now}}))
			}
		}

		_, err = t.db.Collection("subdomains").BulkWrite(ctx, updates)
		if err != nil {
			t.notify.ErrNotif(fmt.Errorf(
				"[!] Error updating all subdomains with new resolved subs: %w", err,
			))
			return
		}

		// keep inserting if you've found already existed http doc.
		httpOpts := options.InsertMany().SetOrdered(false)

		_, err = t.db.Collection("http-services").InsertMany(ctx, https, httpOpts)
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			t.notify.ErrNotif(fmt.Errorf(
				"[!] Error while creating empty http objects for resolved subs: %w", err,
			))
		}

		if len(newResolvedSubs) == 0 {
			return
		}

		log.Printf("[+] Found %d new dns records.\n", len(newResolvedSubs))
		t.notify.NewDnsNotif(newResolvedSubs)
	}()
}

func (t *HttpDiscovery) run(ctx context.Context, assets []asset) {

	err := t.fetchAssets(ctx)
	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	tempFile, httpMap, err := tempFileNServicesMap(t.hosts)

	if err != nil {
		t.notify.ErrNotif(err)
		return
	}

	defer os.Remove(tempFile)

	op, err := execute(ctx,
		"/home/arcane/automation/discovery.sh",
		tempFile,
	)

	if err != nil {
		t.notify.ErrNotif(fmt.Errorf("[!] Error service discovering subdomains: %w", err))
		return
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		resolvedHosts := strings.Split(strings.TrimSpace(op), "\n")
		if len(resolvedHosts) == 1 && resolvedHosts[0] == "" {
			log.Println("[~] Didn't find any new http services.")
			return
		}

		now := time.Now()
		updates := make([]mongo.WriteModel, 0, len(t.hosts))
		var newHttpServices []string

		for _, host := range resolvedHosts {
			hostWithPort := extractHost(host)
			httpObj := httpMap[hostWithPort]

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
			delete(httpMap, hostWithPort)
		}

		for _, notResolvedhost := range httpMap {
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

		_, err = t.db.Collection("http-services").BulkWrite(ctx, updates)
		if err != nil {
			t.notify.ErrNotif(fmt.Errorf("[!] Error while updating http field for new assets: %w", err))
			return
		}
		if len(newHttpServices) == 0 {
			return
		}
		log.Printf("[+] Found %d new http services.\n", len(newHttpServices))
		t.notify.NewHttpNotif(newHttpServices)
	}()
}