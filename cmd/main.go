package main

import (
	m "EagleEye/internal/models"
	"EagleEye/pkg/tools"
	"flag"
	"fmt"
	"os"
	"time"

	"context"

	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func exit(msg string, exitValue int) {
	fmt.Println(msg)
	os.Exit(exitValue)
}

func handleAdd(db *gorm.DB, company string, domain string, ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	switch {
	case company != "" && domain == "":
		{
			fmt.Printf("[*] Creating %s company.\n", company)
			db.WithContext(ctx).Create(&m.Company{Name: company})
		}

	case domain != "" && company != "":
		{
			fmt.Printf("[*] Adding %s domain in %s's assets\n", domain, company)

			var c m.Company
			db.WithContext(ctx).Limit(1).Find(&c, "name = ?", company)
			if c.ID == 0 {
				exit(fmt.Sprintf("[!] Invalid company name: %s.", company), 1)
			}

			db.WithContext(ctx).Create(&m.Domain{Domain: domain, Company: c})
		}
	default:
		{
			exit("[!] You forgot to provide a company to add asset on.", 1)
		}
	}
}

func openDB(ctx context.Context) (*gorm.DB, error) {
	_, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=123 dbname=eagleeye"), &gorm.Config{})

	if err != nil {
		return nil, fmt.Errorf("[!] An error occured when attempting to connect to the database")
	}

	return db, nil
}

func handleMigrate(ctx context.Context, db *gorm.DB) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := db.WithContext(ctx).AutoMigrate(
		m.Company{},
		m.Domain{},
		m.Email{},
		m.Parameter{},
		m.Endpoint{}, m.Status{},
		m.Method{},
		m.DomainMethod{},
		m.DomainParameter{},
		m.EndpointMethod{},
		m.EndpointParameter{},
	); err != nil {
		exit(fmt.Sprintf("[!] An error occured when tried to migrate the schema, %s", err), 1)
	}

	fmt.Println("[*] Database schema has changes successfully.")
	os.Exit(0)
}

func handleRemove(db *gorm.DB, company string, domain string, ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	switch {
	case company != "":
		{
			fmt.Printf("[*] Removing %s company.\n", company)
			db.WithContext(ctx).Delete(&m.Company{}, "name = ?", company)
		}

	case domain != "":
		{
			fmt.Printf("[*] Removing %s domain.\n", domain)

			var c m.Company
			db.WithContext(ctx).Limit(1).Find(&c, "name = ?", company)
			if c.ID == 0 {
				exit(fmt.Sprintf("[!] Invalid company name: %s.", company), 1)
			}

			db.WithContext(ctx).Delete(&m.Domain{}, "domain = ?", domain)
		}
	default:
		{
			exit("[!] You forgot to provide a company to add asset on.", 1)
		}
	}
}

func flushDB(db *gorm.DB) {
	if err := db.Migrator().DropTable(
		m.Company{},
		m.Domain{},
		m.Email{},
		m.Parameter{},
		m.Endpoint{}, m.Status{},
		m.Method{},
		m.DomainMethod{},
		m.DomainParameter{},
		m.EndpointMethod{},
		m.EndpointParameter{}); err != nil {
		exit(fmt.Sprintf("[!] Error while flushing database: %s", err), 1)
	}
	exit("[*] Database has been flushed successfully.", 0)
}

func handleHunt(ctx context.Context, url string, crawl bool) {
	_, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	params := tools.FAllParams(url, crawl)
	fmt.Println("[*] There parameters have been found:\n", params)
}

func main() {
	ctx := context.Background()
	db, err := openDB(ctx)
	if err != nil {
		log.Fatal(err)
	}

	var (
		migrate bool
		flush   bool

		watch   string
		company string
		domain  string

		huntTarget string
		alone      bool
		crawl      bool
	)

	flag.BoolVar(&migrate, "migrate", false, "migrate schema to database.")
	flag.BoolVar(&flush, "flush", false, "flush database.")
	flag.StringVar(&watch, "watch", "", "watch for a single domain.")

	add := flag.NewFlagSet("add", flag.ExitOnError)
	add.StringVar(&company, "company", "", "add new company to database.")
	add.StringVar(&domain, "domain", "", "add new domain to company's assets.")

	remove := flag.NewFlagSet("remove", flag.ExitOnError)
	remove.StringVar(&company, "company", "", "remove company database.")
	remove.StringVar(&domain, "domain", "", "remove domain from company's assets.")

	hunt := flag.NewFlagSet("hunt", flag.ExitOnError)
	hunt.StringVar(&huntTarget, "target", "", "target domain to hunt.")
	hunt.BoolVar(&alone, "alone", false, "hunting for a single target which doesn't exists on DB.")
	hunt.BoolVar(&crawl, "crawl", true, "discover parameters in all js files as well, default=true.")

	flag.Parse()

	switch {

	case migrate:
		{
			handleMigrate(ctx, db)
		}

	case flush:
		{
			flushDB(db)
		}
	}

	if len(os.Args) < 3 {
		exit("Please use -h / --help for more information", 1)
	}

	switch os.Args[1] {
	case "add":
		{
			add.Parse(os.Args[2:])
			handleAdd(db, company, domain, ctx)
		}

	case "remove":
		{
			remove.Parse(os.Args[2:])
			handleRemove(db, company, domain, ctx)
		}

	case "hunt":
		{
			hunt.Parse(os.Args[2:])
			handleHunt(ctx, huntTarget, crawl)
		}
	}
}
