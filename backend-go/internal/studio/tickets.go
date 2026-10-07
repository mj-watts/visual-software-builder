package studio

import (
	"encoding/json"
	"net/http"
)

func (a *App) ticketRoutes() {
	a.route("GET /api/tickets", 200, a.listTickets)
	a.route("POST /api/tickets", 201, a.createTicket)
	a.route("GET /api/tickets/{id}", 200, func(r *http.Request) (any, error) { return a.ticket(r.PathValue("id")) })
	a.route("PATCH /api/tickets/{id}", 200, a.updateTicket)
	a.route("DELETE /api/tickets/{id}", 200, a.deleteTicket)
	a.route("POST /api/tickets/delete", 200, a.bulkDelete)
	a.route("POST /api/tickets/{id}/restore", 200, a.restoreTicket)
}
func (a *App) listTickets(r *http.Request) (any, error) {
	id := projectID(r)
	if _, err := a.project(id, false); err != nil {
		return nil, err
	}
	deleted, err := queryBool(r, "deleted")
	if err != nil {
		return nil, err
	}
	return a.store.tickets(id, deleted)
}
func (a *App) createTicket(r *http.Request) (any, error) {
	id := projectID(r)
	if _, err := a.project(id, false); err != nil {
		return nil, err
	}
	d := defaultDraft()
	if err := decode(r, &d); err != nil {
		return nil, err
	}
	if err := d.validate(); err != nil {
		return nil, err
	}
	if err := a.validateGroup(d.GroupID, id); err != nil {
		return nil, err
	}
	return a.store.create(d, id)
}
func (a *App) updateTicket(r *http.Request) (any, error) {
	t, err := a.ticket(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if err = editable(t); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err = decode(r, &fields); err != nil {
		return nil, err
	}
	d, err := patchDraft(t.Draft, fields)
	if err != nil {
		return nil, err
	}
	if err = a.validateGroup(d.GroupID, t.ProjectID); err != nil {
		return nil, err
	}
	t.Draft = d
	return t, a.store.saveTicket(t)
}
func patchDraft(d Draft, fields map[string]json.RawMessage) (Draft, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return d, err
	}
	var current map[string]json.RawMessage
	if err = json.Unmarshal(raw, &current); err != nil {
		return d, err
	}
	for k, v := range fields {
		if _, ok := current[k]; ok && string(v) != "null" {
			current[k] = v
		}
	}
	raw, err = json.Marshal(current)
	if err != nil {
		return d, err
	}
	if err = json.Unmarshal(raw, &d); err != nil {
		return d, problem(422, "Invalid ticket fields or attachments")
	}
	return d, d.validate()
}
func (a *App) deleteTicket(r *http.Request) (any, error) {
	t, err := a.ticket(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if err = idle(t); err != nil {
		return nil, err
	}
	return map[string]string{"deleted": t.ID}, a.store.setDeleted([]string{t.ID}, true)
}
func (a *App) restoreTicket(r *http.Request) (any, error) {
	t, err := a.store.ticket(r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if _, err = a.project(t.ProjectID, false); err != nil {
		return nil, err
	}
	return t, a.store.setDeleted([]string{t.ID}, false)
}
func (a *App) bulkDelete(r *http.Request) (any, error) {
	v := struct {
		IDs       []string `json:"ids"`
		ProjectID string   `json:"project_id"`
	}{ProjectID: "default"}
	if err := decode(r, &v); err != nil {
		return nil, err
	}
	if len(v.IDs) == 0 || len(v.IDs) > 200 {
		return nil, problem(422, "Select between 1 and 200 tickets")
	}
	if _, err := a.project(v.ProjectID, false); err != nil {
		return nil, err
	}
	ids, err := a.validateDeletion(v.IDs, v.ProjectID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"deleted": ids}, a.store.setDeleted(ids, true)
}
func (a *App) validateDeletion(ids []string, project string) ([]string, error) {
	unique := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		t, err := a.ticket(id)
		if err != nil {
			return nil, err
		}
		if t.ProjectID != project {
			return nil, problem(422, "Selected tickets must belong to the current project.")
		}
		if err = idle(t); err != nil {
			return nil, err
		}
		unique = append(unique, id)
	}
	return unique, nil
}
