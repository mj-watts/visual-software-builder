package studio

import "net/http"

func (a *App) groupRoutes() {
	a.route("GET /api/groups", 200, a.listGroups)
	a.route("POST /api/groups", 201, a.createGroup)
	a.route("PATCH /api/groups/{id}", 200, a.updateGroup)
	a.route("DELETE /api/groups/{id}", 200, a.deleteGroup)
	a.route("PUT /api/tickets/{id}/group", 200, a.assignGroup)
	a.route("POST /api/groups/{id}/run", 202, a.queueGroup)
}
func (a *App) listGroups(r *http.Request) (any, error) {
	id := projectID(r)
	if _, err := a.project(id, false); err != nil {
		return nil, err
	}
	values, err := queryJSON[Group](a.store, `SELECT data FROM groups ORDER BY rowid`)
	out := []Group{}
	for _, g := range values {
		if g.ProjectID == "" {
			g.ProjectID = "default"
		}
		if g.ProjectID == id {
			out = append(out, g)
		}
	}
	return out, err
}
func groupBody(r *http.Request) (Group, error) {
	g := Group{Color: "orange"}
	if err := decode(r, &g); err != nil {
		return g, err
	}
	return g, g.validate()
}
func (a *App) createGroup(r *http.Request) (any, error) {
	id := projectID(r)
	if _, err := a.project(id, false); err != nil {
		return nil, err
	}
	g, err := groupBody(r)
	if err != nil {
		return nil, err
	}
	g.ID = newID()
	g.ProjectID = id
	return g, a.store.saveGroup(g)
}
func (a *App) updateGroup(r *http.Request) (any, error) {
	old, err := a.group(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	g, err := groupBody(r)
	if err != nil {
		return nil, err
	}
	g.ID = old.ID
	g.ProjectID = old.ProjectID
	return g, a.store.saveGroup(g)
}
func (a *App) deleteGroup(r *http.Request) (any, error) {
	g, err := a.group(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	return map[string]string{"deleted": g.ID}, a.store.removeGroup(g.ID)
}
func (a *App) assignGroup(r *http.Request) (any, error) {
	t, err := a.ticket(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	v := struct {
		GroupID string `json:"group_id"`
	}{}
	if err = decode(r, &v); err != nil {
		return nil, err
	}
	if !textLength(v.GroupID, 0, 80) {
		return nil, problem(422, "Invalid group identifier")
	}
	if err = a.validateGroup(v.GroupID, t.ProjectID); err != nil {
		return nil, err
	}
	t.GroupID = v.GroupID
	return t, a.store.saveTicket(t)
}
