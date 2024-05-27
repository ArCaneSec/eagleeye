package main

import (
	m "EagleEye/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *Server) CreateTarget(w http.ResponseWriter, r *http.Request) {
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
		s.jsonEncode(w, http.StatusBadRequest, err)
		fmt.Println(err)
		return
	}

	s.jsonEncode(w, http.StatusCreated, map[string]string{"message": "created."})
}
