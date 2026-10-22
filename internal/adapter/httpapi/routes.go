package httpapi

import (
	"net/http"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

func addRoutes(mux *http.ServeMux, a *app.Application) {
	mux.Handle("GET /healthz", handleHealth())
	mux.Handle("GET /api/version", handleVersion())
	mux.Handle("GET /api/capabilities", handleCapabilities(a))
	mux.Handle("GET /api/interfaces", handleInterfaces(a))
	mux.Handle("GET /api/inventory", handleInventory(a))
	mux.Handle("GET /api/tools", handleTools(a))
	mux.Handle("GET /api/jobs/{id}", handleJob(a))
	mux.Handle("GET /api/history", handleHistory(a))
	mux.Handle("GET /api/settings", handleSettings(a))
	mux.Handle("GET /api/wifi", handleWiFi(a))
}
