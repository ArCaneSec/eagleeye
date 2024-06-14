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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (t *task) subdomainEnumerate(ctx context.Context) {
	targets, err := fetchTargets(ctx, t.db)
	if err != nil {
		t.notify.ErrNotif("[!] Error while fetching targets", err)
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
		t.log("cyka blyat")
		return
	}

	if err != nil {
		t.notify.ErrNotif("[!] Error while fetching new subs", err)
		return
	}

	tempFile, err := os.CreateTemp("/tmp/", "new-subs")
	if err != nil {
		t.notify.ErrNotif("[!] Error creating temp file", err)
		return
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	for _, sub := range newSubs {
		tempFile.WriteString(fmt.Sprintf("%s\n", sub.Subdomain))
	}

	op, err := t.execute(ctx,
		"/home/arcane/automation/resolve.sh",
		tempFile.Name(),
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
	_, err = t.db.Collection("subdomains").UpdateMany(ctx,
		bson.D{{"subdomain", bson.D{{"$in", resolvedSubs}}}},
		bson.D{{"$set", bson.D{{"dns",
			m.Dns{IsActive: true, Created: now, Updated: now}}}}},
	)
	if err != nil {
		t.notify.ErrNotif("[!] Error while updating subdomains", err)
		return
	}

	t.notify.NewDnsNotif(resolvedSubs)
}

func (t *task) httpDiscovery(ctx context.Context) {
	subsWithIP, err := fetchNewSubsWithIP(ctx, t.db)
	if err != nil {
		t.notify.ErrNotif("[!] Error while fetching new subs with ip", err)
		return
	}

	tempFile, err := os.CreateTemp("/tmp/", "new-subs")
	if err != nil {
		t.notify.ErrNotif("[!] Error creating temp file", err)
		return
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	subsMap := make(map[string]m.Subdomain, len(subsWithIP))
	allIDS := make([]primitive.ObjectID, len(subsWithIP))

	for _, sub := range subsWithIP {
		tempFile.WriteString(fmt.Sprintf("%s\n", sub.Subdomain))
		subsMap[sub.Subdomain] = sub
		allIDS = append(allIDS, sub.ID)
	}

	op, err := t.execute(ctx,
		"/home/arcane/automation/discovery.sh",
		tempFile.Name(),
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

	rhPattern := regexp.MustCompile("(?:^https?:\\/\\/)(.*\\..*$)")

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

// func (t *task) dnsResolveAll(ctx context.Context) {
// 	subs, err := fetchSubs(ctx, t.db)
// 	if err != nil {
// 		t.notify.ErrNotif("[!] Error while fetching targets", err)
// 	}

// 	for _, sub := range subs {
// 		select {
// 		case <-ctx.Done():
// 			t.notify.ErrNotif("", fmt.Errorf("[!] Context deadline exceeds in subdomain enumeration job"))
// 			return

// 		default:
// 			t.execute(ctx, "puredns")
// 		}
// 	}

// }

// func (t *task) httpDiscoveryAll(ctx context.Context){
// httpFound := false
// for _, http := range subObj.Http {

// 	if port == http.Port {
// 		httpFound = true
// 		if !http.IsActive {
// 			http.IsActive = true
// 			http.Updated = now
// 		}
// 		break
// 	}

// 	if port != http.Port {
// 		http.Updated = now
// 		http.IsActive = false
// 	}

// 	subObj.Http = append(subObj.Http, m.Http{
// 		IsActive: true,
// 		Port:     port,
// 		Created:  now,
// 		Updated:  now,
// 	})
// }}
