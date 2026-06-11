package http

import (
	"net/http"

	"github.com/wrany/tracking-gateway/internal/usecase"
)

type MeHandler struct {
	me *usecase.MeUsecase
}

func NewMeHandler(me *usecase.MeUsecase) *MeHandler {
	return &MeHandler{me: me}
}

// GetMe godoc
// @Summary      Get current user
// @Tags         users
// @Produce      json
// @Success      200  {object}  MeEnv
// @Failure      401  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/me [get]
func (h *MeHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.me.GetMe(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"created_at": user.CreatedAt,
	})
}
