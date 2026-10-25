package httpapi

import (
	"net/http"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

func addRoutes(mux *http.ServeMux, a *app.Application, s *Server) {
	mux.Handle("POST /api/session", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out := &statusWriter{ResponseWriter: w}
		s.session.exchange(out, r)
		if out.status == http.StatusUnauthorized {
			s.log.Info("session exchange rejected")
		}
	}))
	mux.Handle("GET /healthz", emptyRequest(handleHealth()))
	mux.Handle("GET /api/version", emptyRequest(handleVersion()))
	mux.Handle("GET /api/capabilities", emptyRequest(handleCapabilities(a)))
	mux.Handle("GET /api/capture/backends", emptyRequest(handleCaptureBackends(a)))
	mux.Handle("GET /api/interfaces", emptyRequest(handleInterfaces(a)))
	mux.Handle("GET /api/neighbors", emptyRequest(handleNeighbors(a)))
	mux.Handle("GET /api/connections", emptyRequest(handleConnections(a)))
	mux.Handle("POST /api/throughput", handleThroughput(a))
	mux.Handle("GET /api/inventory", emptyRequest(handleInventory(a)))
	mux.Handle("GET /api/tools", emptyRequest(handleTools(a)))
	mux.Handle("GET /api/jobs/{id}", emptyRequest(handleJob(a)))
	mux.Handle("GET /api/history", emptyRequest(handleHistory(a)))
	mux.Handle("GET /api/settings", emptyRequest(handleSettings(a)))
	mux.Handle("GET /api/legal", emptyRequest(handleLegalNotice(a)))
	mux.Handle("POST /api/legal/accept", handleAcceptLegalNotice(a))
	mux.Handle("GET /api/wifi", emptyRequest(handleWiFi(a)))
	mux.Handle("POST /api/diagnose", handleDiagnose(a))
	mux.Handle("POST /api/tls/audit", handleTLSAudit(a))
	mux.Handle("POST /api/dns/inspect", handleDNSInspect(a))
	mux.Handle("POST /api/discovery/local", handleLocalDiscovery(a))
	mux.Handle("POST /api/snmp", handleSNMP(a))
	mux.Handle("POST /api/diagnostics", handleStartDiagnostic(a))
	mux.Handle("POST /api/jobs/{id}/cancel", handleCancel(a))
	mux.Handle("GET /api/events", emptyRequest(s.handleEvents()))
	mux.Handle("DELETE /api/history", handleClearHistory(a))
	mux.Handle("PUT /api/settings", handleSaveSettings(a))
	mux.Handle("POST /api/wifi/reveal", handleReveal(a))
	mux.Handle("POST /api/discovery", handleDiscover(a))
	mux.Handle("POST /api/inspect", handleInspect(a))
	mux.Handle("POST /api/capture/start", handleStartCapture(a))
	mux.Handle("POST /api/capture/stop", handleStopCapture(a))
	mux.Handle("POST /api/capture/import", s.handleImport())
	mux.Handle("GET /api/capture/export", emptyRequest(s.handleExport()))
	mux.Handle("GET /api/doctor/bundle", emptyRequest(s.handleBundle()))
}
