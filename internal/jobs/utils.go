package jobs

import (
	m "EagleEye/internal/models"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (t *SubdomainEnumeration) fetchAssets(ctx context.Context) error {
	cursor, _ := t.db.Collection("targets").Find(ctx, bson.D{{}})

	if err := cursor.All(ctx, &t.targets); err != nil {
		return fmt.Errorf("[!] Error while fetching targets: %w", err)
	}
	return nil
}

func (t *DnsResolve) fetchAssets(ctx context.Context) error {
	// values := bson.D{{"subdomain", 1}, {"_id", 0}}

	cursor, _ := t.db.Collection("subdomains").Find(
		ctx,
		bson.D{{"dns", nil}}) //, options.Find().SetProjection(values)

	if err := cursor.All(ctx, &t.subdomains); err != nil {
		return fmt.Errorf("[!] Error while fetching new subs: %w", err)
	}

	return nil
}

func (t *HttpDiscovery) fetchAssets(ctx context.Context) error {
	cursor, _ := t.db.Collection("http-services").Find(
		ctx,
		bson.M{"created": nil},
	)

	if err := cursor.All(ctx, &t.hosts); err != nil {
		return fmt.Errorf("[!] Error while fetching new services: %w", err)
	}

	return nil
}

func (t *SubdomainEnumeration) insertSubs(ctx context.Context, op string, target m.Target, domain string) {
	var subdomains []interface{}
	cursor := t.db.Collection("subdomains")
	now := time.Now()

	subs := strings.Split(strings.TrimSpace(op), "\n")

	for _, sub := range subs {
		if sub == "" {
			continue
		}
		subdomains = append(subdomains, m.Subdomain{Target: target.ID, Subdomain: sub, Created: now})
	}

	breakAfterFirstFail := options.InsertMany().SetOrdered(false)

	val, err := t.db.Collection("subdomains").InsertMany(ctx, subdomains, breakAfterFirstFail)
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		t.notify.ErrNotif(fmt.Errorf("[!] Error while inserting subdomains to database: %w", err))
		return
	}

	log.Printf("[+] Found %d new subdomains for %s.\n", len(val.InsertedIDs), target.Name)

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

func (t *DnsResolveAll) fetchAssets(ctx context.Context) error {
	cursor, _ := t.db.Collection("subdomains").Find(ctx, bson.D{{}})
	if err := cursor.All(ctx, &t.subdomains); err != nil {
		return fmt.Errorf("[!] Error while fetching targets: %w", err)
	}

	return nil
}

func (t *HttpDiscoveryAll) fetchAssets(ctx context.Context) error {
	cursor, _ := t.db.Collection("http-services").Find(ctx, bson.M{})
	if err := cursor.All(ctx, &t.hosts); err != nil {
		return fmt.Errorf("[!] Error while fetching http services: %w", err)
	}

	fmt.Println("testttttttttttt")
	return nil
}

func tempFileNSubsMap(subs []m.Subdomain) (string, map[string]*m.Subdomain, error) {
	tempFile, err := os.CreateTemp("/tmp/", "subs")
	if err != nil {
		return "", nil, fmt.Errorf("[!] Error creating temp file: %w", err)
	}

	subsMap := make(map[string]*m.Subdomain, len(subs))

	for _, sub := range subs {
		tempFile.WriteString(fmt.Sprintf("%s\n", sub.Subdomain))
		subsMap[sub.Subdomain] = &sub
	}

	tempFile.Close()

	return tempFile.Name(), subsMap, nil
}

func tempFileNServicesMap(services []m.HttpService) (string, map[string]*m.HttpService, error) {
	tempFile, err := os.CreateTemp("/tmp/", "services")
	if err != nil {
		return "", nil, fmt.Errorf("[!] Error creating temp file: %w", err)
	}

	servicesMap := make(map[string]*m.HttpService, len(services))

	for _, service := range services {
		tempFile.WriteString(fmt.Sprintf("%s\n", service.Host))
		servicesMap[service.Host] = &service
	}

	tempFile.Close()

	return tempFile.Name(), servicesMap, nil
}

func createEmptyHttps(httpSlice *[]interface{}, sub m.Subdomain) {
	PORTS := []int{80, 443}
	now := time.Now()

	for _, port := range PORTS {
		*httpSlice = append(*httpSlice,
			m.HttpService{
				Subdomain: sub.ID,
				Host:      fmt.Sprintf("%s:%d", sub.Subdomain, port),
				IsActive:  false,
				Created:   nil,
				Updated:   now,
			},
		)
	}

}

func extractHost(host string) string {
	hostPattern := regexp.MustCompile(`^https?://(.*?:\d{1,5})$`)

	var found string

	found = hostPattern.FindString(host)
	if found != "" {
		return found
	}

	if strings.HasPrefix(host, "http:") {
		found = fmt.Sprintf("%s:%d", host[7:], 80)
	} else {
		found = fmt.Sprintf("%s:%d", host[8:], 443)
	}

	return found
}
