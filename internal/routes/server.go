package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Server struct {
	address string
	router  *chi.Mux
	db      *mongo.Database
}

func main() {
	r := chi.NewRouter()

	s := &Server{":5000", r, initDb()}

	r.Use(middleware.Logger)

	r.Post("/target/", s.CreateTarget)

	log.Printf("Listening on %s", s.address)
	http.ListenAndServe(s.address, s.router)
}

func initDb() *mongo.Database {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017/"))

	if err != nil {
		log.Fatalf("[!] Could not connect to database. err: %w", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil{
		log.Fatalf("[!] Database ping wasnt successfull. err: %w", err)
	}

	return client.Database("EagleEye")

}

func (s *Server) jsonEncode(w http.ResponseWriter, sCode int, data any) {
	w.Header().Set("Content-Type", "Application/json")
	w.WriteHeader(sCode)

	err := json.NewEncoder(w).Encode(data)

	if err != nil {
		log.Fatalf("[!] error while serializing data, err: %w", err)
		return
	}
}
