package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/wrany/tracking-gateway/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type RefreshTokenRepository interface {
	Save(ctx context.Context, token *domain.RefreshToken) error
	FindByTokenHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
}

type AuthConfig struct {
	JWTSecret  []byte
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type AuthUsecase struct {
	users  UserRepository
	tokens RefreshTokenRepository
	cfg    AuthConfig
}

func NewAuthUsecase(users UserRepository, tokens RefreshTokenRepository, cfg AuthConfig) *AuthUsecase {
	return &AuthUsecase{users: users, tokens: tokens, cfg: cfg}
}

type RegisterInput struct {
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

func (uc *AuthUsecase) Register(ctx context.Context, in RegisterInput) (*domain.TokenPair, error) {
	email := normalizeEmail(in.Email)
	if email == "" || len(in.Password) < 8 {
		return nil, ErrValidation
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := uc.users.Create(ctx, user); err != nil {
		return nil, err
	}

	return uc.issueTokenPair(ctx, user.ID)
}

func (uc *AuthUsecase) Login(ctx context.Context, in LoginInput) (*domain.TokenPair, error) {
	email := normalizeEmail(in.Email)

	user, err := uc.users.FindByEmail(ctx, email)
	if err != nil {
		// constant-time dummy compare prevents timing-based email enumeration
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$dummyhashfortimingnormalization"), []byte(in.Password))
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return uc.issueTokenPair(ctx, user.ID)
}

func (uc *AuthUsecase) Refresh(ctx context.Context, rawToken string) (*domain.TokenPair, error) {
	hash := HashToken(rawToken)

	rt, err := uc.tokens.FindByTokenHash(ctx, hash)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if time.Now().UTC().After(rt.ExpiresAt) {
		return nil, domain.ErrTokenExpired
	}
	if rt.RevokedAt != nil {
		return nil, domain.ErrTokenRevoked
	}

	if err := uc.tokens.Revoke(ctx, rt.ID); err != nil {
		return nil, err
	}

	return uc.issueTokenPair(ctx, rt.UserID)
}

func (uc *AuthUsecase) issueTokenPair(ctx context.Context, userID uuid.UUID) (*domain.TokenPair, error) {
	now := time.Now().UTC()

	accessToken, err := uc.signAccessToken(userID, now)
	if err != nil {
		return nil, err
	}

	rawRefresh, err := generateRandomToken()
	if err != nil {
		return nil, err
	}

	rt := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: HashToken(rawRefresh),
		ExpiresAt: now.Add(uc.cfg.RefreshTTL),
		CreatedAt: now,
	}
	if err := uc.tokens.Save(ctx, rt); err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	}, nil
}

func (uc *AuthUsecase) signAccessToken(userID uuid.UUID, now time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"iat": now.Unix(),
		"exp": now.Add(uc.cfg.AccessTTL).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(uc.cfg.JWTSecret)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
