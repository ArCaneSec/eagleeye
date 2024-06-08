package models

import (
	"strings"
	"time"
)

type jsonErrors map[string]map[string]string

type Target struct {
	Name       string   `json:"name"`
	Bounty     *bool    `json:"bounty"`
	Scope      []string `json:"scope"`
	OutOfScope []string `json:"outOfScope" bson:"outOfScope"`
	Source     string   `json:"source"`
}

func (t *Target) Validate() jsonErrors {
	errors := map[string]map[string]string{}

	if strings.TrimSpace(t.Name) == "" {
		errors["name"] = map[string]string{"error": "required"}
	}

	if t.Bounty == nil {
		errors["bounty"] = map[string]string{"error": "required"}
	}

	if len(t.Scope) == 0 {
		errors["scope"] = map[string]string{"error": "required."}
	}

	if t.Source != "hackerone" || t.Source != "bugcrowd" || t.Source != "integrity" || t.Source != "yeswehack" {
		errors["source"] = map[string]string{"error": "invalid value."}
	}

	return errors
}


type Subdomain struct {
	Subdomain string
	Created time.Time
}