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

// RegisterDevice godoc
// @Summary      Register a device
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        body  body      RegisterDeviceReq  true  "Device registration"
// @Success      201   {object}  DeviceEnv
// @Failure      400   {object}  ApiError
// @Failure      401   {object}  ApiError
// @Failure      422   {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/devices/register [post]
func (h *DeviceHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body RegisterDeviceReq
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

// ListDevices godoc
// @Summary      List current user's devices
// @Tags         devices
// @Produce      json
// @Success      200  {object}  DevicesEnv
// @Failure      401  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/devices [get]
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
