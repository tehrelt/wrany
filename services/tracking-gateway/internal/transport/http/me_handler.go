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
