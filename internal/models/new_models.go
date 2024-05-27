package models

type Target struct {
	Name       string   `json:"name"`
	Bounty     *bool    `json:"bounty"`
	Scope      []string `json:"scope"`
	OutOfScope []string `json:"outOfScope"`
	Source     string   `json:"source"`
}

func (t *Target) Validate() map[string]string {
	errors := map[string]string{}

	if t.Name == "" {
		errors["name"] = "required."
	}

	if t.Bounty == nil {
		errors["bounty"] = "required."
	}

	if len(t.Scope) == 0 {
		errors["scope"] = "required."
	}

	if t.Source == "" {
		errors["source"] = "required."
	}

	return errors
}
