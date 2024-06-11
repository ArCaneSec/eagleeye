package jobs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
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
				fmt.Printf("[~] Current domain: %s\n", domain)

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

	t.log("[~] Resolving new assets finished, Inserting...")

	resolvedSubs := strings.Split(strings.TrimSpace(op), "\n")

	if len(resolvedSubs) == 1 && resolvedSubs[0] == op {
		t.log("[~] Didn't find any A record for new subs.")
		return
	}

	_, err = t.db.Collection("subdomains").UpdateMany(ctx,
		bson.D{{"subdomain", bson.D{{"$in", resolvedSubs}}}},
		bson.D{{"$set", bson.D{{"dns", true}}}},
	)
	if err != nil {
		t.notify.ErrNotif("[!] Error while updating subdomains", err)
		return
	}

	t.notify.NewDnsNotif(resolvedSubs)
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
