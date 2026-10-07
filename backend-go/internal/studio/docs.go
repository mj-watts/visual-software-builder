package studio

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openAPI []byte

const swaggerHTML = `<!doctype html><html><head><title>Swimlane Go API - Swagger UI</title><link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css"></head><body><div id="swagger-ui"></div><script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script><script>SwaggerUIBundle({url:'/openapi.json',dom_id:'#swagger-ui',deepLinking:true,docExpansion:'none'})</script></body></html>`

func (a *App) docsRoutes() {
	a.mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(openAPI); err != nil {
			return
		}
	})
	a.mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write([]byte(swaggerHTML)); err != nil {
			return
		}
	})
}
