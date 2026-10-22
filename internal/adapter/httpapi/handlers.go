package httpapi

import (
	"net/http"
	"regexp"
	"runtime"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

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
