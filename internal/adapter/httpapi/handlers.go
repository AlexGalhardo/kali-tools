package httpapi

import (
	"net/http"
	"regexp"
	"runtime"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

const maxTLSAuditBody = 4 << 10
const maxDNSInspectBody = 8 << 10
const maxLocalDiscoveryBody = 4 << 10
const maxSNMPBody = 4 << 10

func handleTLSAudit(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := contract.TLSAuditRequest{Port: 443, TimeoutMS: 5000, Lint: true}
		if err := decodeJSON(w, r, &request, maxTLSAuditBody); err != nil {
			writeError(w, r, err)
			return
		}
		value, err := a.AuditTLS(r.Context(), request.Request())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromTLSAudit(value))
	}
}

func handleDNSInspect(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request contract.DNSInspectRequest
		if err := decodeJSON(w, r, &request, maxDNSInspectBody); err != nil {
			writeError(w, r, err)
			return
		}
		value, err := a.InspectDNS(r.Context(), request.Request())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromDNSInspect(value))
	}
}

func handleLocalDiscovery(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request contract.LocalDiscoveryRequest
		if err := decodeJSON(w, r, &request, maxLocalDiscoveryBody); err != nil {
			writeError(w, r, err)
			return
		}
		value, err := a.DiscoverLocal(r.Context(), request.Request())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromLocalDiscovery(value))
	}
}

func handleSNMP(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request contract.SNMPRequest
		if err := decodeJSON(w, r, &request, maxSNMPBody); err != nil {
			writeError(w, r, err)
			return
		}
		query, err := request.Request()
		request.Community = ""
		if request.V3 != nil {
			request.V3.AuthPass = ""
			request.V3.PrivPass = ""
		}
		if err != nil {
			writeError(w, r, err)
			return
		}
		value, err := a.QuerySNMP(r.Context(), query)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromSNMP(value))
	}
}

func handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
func handleVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": domain.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "mode": "server"})
	}
}
func handleCapabilities(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, contract.FromCapabilities(a.Capabilities(r.Context())))
	}
}

func handleCaptureBackends(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, contract.FromCapabilities(a.CaptureBackendStatus(r.Context())))
	}
}
func handleInterfaces(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, err := a.Interfaces(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromInterfaces(value))
	}
}

func handleConnections(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, err := a.Connections(r.Context())
		writeConverted(w, r, value, err, contract.FromConnectionTable)
	}
}

// writeConverted keeps the error check out of the caller: on platforms where
// the provider is an unsupported stub, staticcheck (SA4023) otherwise flags
// the caller's err != nil as always true.
func writeConverted[T, U any](w http.ResponseWriter, r *http.Request, value T, err error, convert func(T) U) {
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, convert(value))
}

func handleThroughput(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DurationMS int64  `json:"duration_ms"`
			IperfJSON  string `json:"iperf_json"`
		}
		if err := decodeJSON(w, r, &body, maxJSONBody); err != nil {
			writeError(w, r, err)
			return
		}
		var duration time.Duration
		if body.DurationMS != 0 {
			if body.DurationMS < 1 || body.DurationMS > 10_000 {
				writeError(w, r, domain.Invalid("throughput duration must be 1ms..10s"))
				return
			}
			duration = time.Duration(body.DurationMS) * time.Millisecond
		}
		value, err := a.Throughput(r.Context(), duration, []byte(body.IperfJSON))
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromThroughput(value))
	}
}

func handleNeighbors(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		family := r.URL.Query().Get("family")
		if family == "" {
			family = "all"
		}
		value, err := a.Neighbors(r.Context(), family)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromNeighborTable(value))
	}
}
func handleInventory(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, err := a.Inventory(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromInventory(value))
	}
}
func handleTools(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, err := a.Tools(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromTools(value))
	}
}
func handleHistory(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, err := a.History(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromStatuses(value))
	}
}
func handleSettings(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, err := a.Settings(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromSettings(value))
	}
}
func handleWiFi(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, err := a.CurrentWiFi(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromWiFi(value))
	}
}

var validJobID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func requestJobID(r *http.Request) (job.ID, error) {
	id := r.PathValue("id")
	if !validJobID.MatchString(id) {
		return "", domain.Invalid("job ID must be 32 lowercase hexadecimal characters")
	}
	return job.ID(id), nil
}

func handleJob(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := requestJobID(r)
		if err != nil {
			writeError(w, r, err)
			return
		}
		value, ok := a.JobSnapshot(r.Context(), id)
		if !ok {
			writeError(w, r, &domain.Error{Code: domain.CodeNotFound, Message: "job not found"})
			return
		}
		writeJSON(w, http.StatusOK, contract.FromStatus(value))
	}
}

func handleDiagnose(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request contract.DiagnosticRequest
		if err := decodeJSON(w, r, &request, maxJSONBody); err != nil {
			writeError(w, r, err)
			return
		}
		value, err := a.Diagnose(r.Context(), request.Request())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromDiagnosticResult(value))
	}
}

func handleStartDiagnostic(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request contract.DiagnosticRequest
		if err := decodeJSON(w, r, &request, maxJSONBody); err != nil {
			writeError(w, r, err)
			return
		}
		value, err := a.StartDiagnostic(r.Context(), request.Request())
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, contract.FromStatus(value))
	}
}

func handleCancel(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := decodeEmpty(w, r); err != nil {
			writeError(w, r, err)
			return
		}
		id, err := requestJobID(r)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"cancelled": a.CancelJob(r.Context(), id)})
	}
}

func handleClearHistory(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := decodeEmpty(w, r); err != nil {
			writeError(w, r, err)
			return
		}
		if err := a.ClearHistory(r.Context()); err != nil {
			writeError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleSaveSettings(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request contract.Settings
		if err := decodeJSON(w, r, &request, maxJSONBody); err != nil {
			writeError(w, r, err)
			return
		}
		if err := a.SaveSettings(r.Context(), request.AppSettings()); err != nil {
			writeError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleReveal(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := decodeEmpty(w, r); err != nil {
			writeError(w, r, err)
			return
		}
		value, err := a.RevealWiFi(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		writeJSON(w, http.StatusOK, contract.FromWiFiSecret(value))
	}
}

type scopeRequest struct {
	CIDR  string `json:"cidr"`
	Ports []int  `json:"ports"`
}

func handleDiscover(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request scopeRequest
		if err := decodeJSON(w, r, &request, maxJSONBody); err != nil {
			writeError(w, r, err)
			return
		}
		value, truncated, err := a.Discover(r.Context(), request.CIDR, request.Ports, app.DefaultDiscoveryTimeout)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromDiscovery(value, truncated))
	}
}

func handleInspect(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request scopeRequest
		if err := decodeJSON(w, r, &request, maxJSONBody); err != nil {
			writeError(w, r, err)
			return
		}
		value, err := a.InspectServices(r.Context(), request.CIDR, request.Ports)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromServiceInspection(value))
	}
}

func handleStartCapture(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request contract.LiveCaptureRequest
		if err := decodeJSON(w, r, &request, maxJSONBody); err != nil {
			writeError(w, r, err)
			return
		}
		if err := a.StartCapture(r.Context(), request.LiveRequest()); err != nil {
			writeError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func handleStopCapture(a *app.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := decodeEmpty(w, r); err != nil {
			writeError(w, r, err)
			return
		}
		a.StopCapture(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}
}
