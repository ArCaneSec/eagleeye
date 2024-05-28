package main

import (
	m "EagleEye/internal/models"
	"context"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"net/http"
	"time"
)

func (s *Server) createTarget(w http.ResponseWriter, r *http.Request) {
	var target m.Target
	err := json.NewDecoder(r.Body).Decode(&target)

	if err != nil {
		http.Error(w, "[!] invalid data.", http.StatusBadRequest)
		return
	}

	if errs := target.Validate(); len(errs) != 0 {
		s.jsonEncode(w, http.StatusBadRequest, errs)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = s.db.Collection("targets").InsertOne(ctx, target)

	if err != nil {
		errMessage := map[string]string{}

		if mongo.IsDuplicateKeyError(err) {
			errMessage["error"] = "[!] Target already exits."
		} else {
			errMessage["error"] = fmt.Sprintf("[!] Unexpected error occures, err: %w", err)
		}

		s.jsonEncode(w, http.StatusBadRequest, errMessage)
		return
	}

	s.jsonEncode(w, http.StatusCreated, map[string]string{"message": "created."})
}

func (s *Server) editTarget(w http.ResponseWriter, r *http.Request) {
	var target m.Target
	json.NewDecoder(r.Body).Decode(&target)

	if errs := target.Validate(); len(errs) != 0 {
		s.jsonEncode(w, http.StatusBadRequest, errs)
		return
	}
	update := bson.D{
		{"$set", bson.D{
			{"bounty", target.Bounty},
			{"scope", target.Scope},
			{"outOfScope", target.OutOfScope},
		}},
	}

	rs, err := s.db.Collection("targets").UpdateOne(queryContext(), bson.D{{"name", target.Name}}, update)
	if err != nil {
		s.jsonEncode(w, http.StatusBadGateway, err)
		return
	}

	message := map[string]string{"message": fmt.Sprintf("successfully updated %d record.", rs.MatchedCount)}
	s.jsonEncode(w, http.StatusAccepted, message)
}
