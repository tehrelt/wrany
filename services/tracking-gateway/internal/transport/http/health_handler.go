package http

import (
	"encoding/json"
	"net/http"
)

// HealthzHandler godoc
// @Summary      Health check
// @Tags         system
// @Produce      json
// @Success      200  {object}  HealthzEnv
// @Router       /healthz [get]
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
