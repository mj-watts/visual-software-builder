package studio

import "net/http"

func (a *App) projectRoutes() {
	a.route("GET /api/projects", 200, a.listProjects)
	a.route("POST /api/projects", 201, a.createProject)
	a.route("GET /api/projects/{id}", 200, func(r *http.Request) (any, error) { return a.project(r.PathValue("id"), false) })
	a.route("PUT /api/projects/{id}", 200, a.updateProject)
	a.route("DELETE /api/projects/{id}", 200, a.deleteProject)
	a.route("POST /api/projects/{id}/restore", 200, a.restoreProject)
	a.route("GET /api/project", 200, func(r *http.Request) (any, error) { v, err := a.project("default", false); return v.Project, err })
	a.route("PUT /api/project", 200, func(r *http.Request) (any, error) {
		r.SetPathValue("id", "default")
		v, err := a.updateProject(r)
		if err != nil {
			return nil, err
		}
		return v.(ManagedProject).Project, nil
	})
}
func (a *App) listProjects(r *http.Request) (any, error) {
	include, err := queryBool(r, "include_deleted")
	if err != nil {
		return nil, err
	}
	values, err := queryJSON[ManagedProject](a.store, `SELECT data FROM projects ORDER BY rowid`)
	out := []ManagedProject{}
	for _, v := range values {
		if include || !v.Deleted {
			out = append(out, v)
		}
	}
	return out, err
}
func projectBody(r *http.Request) (Project, error) {
	p := Project{Name: "My first project"}
	if err := decode(r, &p); err != nil {
		return p, err
	}
	return p, p.validate()
}
func (a *App) createProject(r *http.Request) (any, error) {
	p, err := projectBody(r)
	if err != nil {
		return nil, err
	}
	v := ManagedProject{Project: p, ID: newID()}
	return v, a.store.saveProject(v)
}
func (a *App) updateProject(r *http.Request) (any, error) {
	v, err := a.project(r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	p, err := projectBody(r)
	if err != nil {
		return nil, err
	}
	v.Project = p
	return v, a.store.saveProject(v)
}
func (a *App) deleteProject(r *http.Request) (any, error) {
	v, err := a.project(r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	tickets, err := a.store.tickets(v.ID, false)
	if err != nil {
		return nil, err
	}
	for _, t := range tickets {
		if err = idle(t); err != nil {
			return nil, err
		}
	}
	v.Deleted = true
	return v, a.store.saveProject(v)
}
func (a *App) restoreProject(r *http.Request) (any, error) {
	v, err := a.project(r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	v.Deleted = false
	return v, a.store.saveProject(v)
}
