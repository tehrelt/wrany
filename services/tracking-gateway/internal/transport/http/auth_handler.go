package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

type AuthHandler struct {
	auth *usecase.AuthUsecase
}

func NewAuthHandler(auth *usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Register godoc
// @Summary      Register a new user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      swRegisterReq  true  "Registration credentials"
// @Success      201   {object}  swTokenPairEnv
// @Failure      400   {object}  swErr
// @Failure      409   {object}  swErr
// @Failure      422   {object}  swErr
// @Router       /v1/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body swRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pair, err := h.auth.Register(r.Context(), usecase.RegisterInput{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmailTaken):
			slog.Warn("register: email taken", "email", body.Email)
			writeError(w, http.StatusConflict, "unable to register with provided credentials")
		case errors.Is(err, usecase.ErrValidation):
			slog.Warn("register: validation failed", "email", body.Email)
			writeError(w, http.StatusUnprocessableEntity, "invalid email or password")
		default:
			slog.Error("register: internal", "email", body.Email, "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	slog.Info("register: ok", "email", body.Email)
	writeJSON(w, http.StatusCreated, pair)
}

// Login godoc
// @Summary      Login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      swLoginReq  true  "Login credentials"
// @Success      200   {object}  swTokenPairEnv
// @Failure      400   {object}  swErr
// @Failure      401   {object}  swErr
// @Router       /v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body swLoginReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pair, err := h.auth.Login(r.Context(), usecase.LoginInput{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		slog.Warn("login: failed", "email", body.Email, "err", err)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	slog.Info("login: ok", "email", body.Email)
	writeJSON(w, http.StatusOK, pair)
}

// Refresh godoc
// @Summary      Refresh access token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      swRefreshReq  true  "Refresh token"
// @Success      200   {object}  swTokenPairEnv
// @Failure      400   {object}  swErr
// @Failure      401   {object}  swErr
// @Router       /v1/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body swRefreshReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pair, err := h.auth.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	writeJSON(w, http.StatusOK, pair)
}
