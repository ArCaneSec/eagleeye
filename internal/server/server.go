package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ArCaneSec/eagleeye/internal/jobs"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Server struct {
	db        *mongo.Database
	scheduler *jobs.Scheduler
}

func InitializeEagleEye() {
	godotenv.Load("../../.env")
	r := chi.NewRouter()

	var wg sync.WaitGroup

	httpServer := http.Server{Addr: "127.0.0.1:5000", Handler: r}

	s := &Server{db: initDb()}
	s.scheduler = jobs.ScheduleJobs(s.db, &wg)

	r.Use(middleware.Logger)

	r.Post("/target/", s.createTarget)
	r.Put("/target/", s.editTarget)
	r.Post("/job/{id:[0-9]{1,3}}", s.activeJob)
	r.Delete("/job/{id:[0-9]{1,3}}", s.deactiveJob)
	r.Delete("/job/", s.deactiveAll)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)
	var interruptCount int

	go func() {
		for {
			<-sig
			interruptCount++
			if interruptCount == 1 {
				go func() {
					err := httpServer.Shutdown(context.Background())
					if err != nil {
						log.Fatal(err)
					}

					s.scheduler.Shutdown()

				}()
			} else {
				log.Fatal("[!] Received second signal, terminating immediately")
			}
		}
	}()

	log.Printf("Listening on %s", httpServer.Addr)

	if err := httpServer.ListenAndServe(); err != nil {
		log.Println("[~] Http server closed, preparing graceful shutdown...")
		wg.Wait()
	}

	log.Println("[#] Eagle's going to sleep, cya!")
}

func initDb() *mongo.Database {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017/"))

	if err != nil {
		log.Fatalf("[!] Could not connect to database. err: %w", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("[!] Database ping wasnt successfull. err: %w", err)
	}

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	db := client.Database("EagleEye")
	_, err = db.Collection("targets").Indexes().CreateOne(ctx, indexModel)

	if err != nil {
		log.Fatalf("[!] An error occured when tried to create index for targets collection, err: %w", err)
	}

	sdIndexModel := mongo.IndexModel{
		Keys:    bson.D{{"subdomain", 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err = db.Collection("subdomains").Indexes().CreateOne(ctx, sdIndexModel)
	if err != nil {
		log.Fatalf("[!] Error while tried to create index for subdomains collection, err: %w", err)
	}

	hsIndexModel := mongo.IndexModel{
		Keys:    bson.D{{"host", 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err = db.Collection("http-services").Indexes().CreateOne(ctx, hsIndexModel)
	if err != nil {
		log.Fatalf(
			"[!] Error while tried to create index for http-services collection, err: %w",
			err,
		)
	}

	return db

}

func (s *Server) jsonEncode(w http.ResponseWriter, sCode int, data any) {
	w.Header().Set("Content-Type", "Application/json")
	w.WriteHeader(sCode)

	if err, ok := data.(error); ok {
		data = map[string]any{"error": err.Error()}
	} else if str, ok := data.(string); ok {
		data = map[string]any{"message": str}
	}

	err := json.NewEncoder(w).Encode(data)

	if err != nil {
		log.Fatalf("[!] error while serializing data, err: %w", err)
		return
	}
}
