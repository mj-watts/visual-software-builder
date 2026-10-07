package studio

import (
	_ "embed"
	"encoding/json"
)

//go:embed seed.json
var seedJSON []byte

func (a *App) Seed() error {
	var n int
	if err := a.store.db.QueryRow(`SELECT count(*) FROM tickets`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := a.project("default", false); err != nil {
		return nil
	}
	var values []Ticket
	if err := json.Unmarshal(seedJSON, &values); err != nil {
		return err
	}
	for _, v := range values {
		t, err := a.store.create(v.Draft, "default")
		if err != nil {
			return err
		}
		t.Status = v.Status
		t.Response = v.Response
		t.Files = v.Files
		if err = a.store.saveTicket(t); err != nil {
			return err
		}
	}
	return nil
}
