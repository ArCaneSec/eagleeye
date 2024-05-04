package main

import (
	m "EagleEye/internal/models"
	"flag"
	"fmt"
	"os"
	"time"

	"golang.org/x/net/context"
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

func main() {
	ctx := context.Background()
	db, err := openDB(ctx)
	if err != nil {
		panic(err)
	}

	var (
		migrate bool
		flush   bool
		watch   string
		company string
		domain  string
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

	flag.Parse()

	if migrate {
		handleMigrate(ctx, db)
	}

	if flush {
		flushDB(db)
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
	}
}
