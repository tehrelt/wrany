package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/wrany/tracking-gateway/internal/usecase"
)

type DeviceHandler struct {
	devices *usecase.DeviceUsecase
}

func NewDeviceHandler(devices *usecase.DeviceUsecase) *DeviceHandler {
	return &DeviceHandler{devices: devices}
}

func (h *DeviceHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		DeviceID string  `json:"device_id"`
		Name     *string `json:"name"`
		Platform *string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deviceID, err := uuid.Parse(body.DeviceID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "device_id must be a valid UUID")
		return
	}

	device, err := h.devices.RegisterDevice(r.Context(), usecase.RegisterDeviceInput{
		UserID:   userID,
		DeviceID: deviceID,
		Name:     body.Name,
		Platform: body.Platform,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, device)
}

func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	devices, err := h.devices.ListDevices(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, devices)
}
