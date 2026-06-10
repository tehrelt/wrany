package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// --- mock UserRepository ---

type mockUserRepo struct {
	users  map[string]*domain.User
	byID   map[uuid.UUID]*domain.User
	createErr error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*domain.User),
		byID:  make(map[uuid.UUID]*domain.User),
	}
}

func (m *mockUserRepo) Create(_ context.Context, u *domain.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.users[u.Email]; exists {
		return domain.ErrEmailTaken
	}
	m.users[u.Email] = u
	m.byID[u.ID] = u
	return nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

// --- mock RefreshTokenRepository ---

type mockTokenRepo struct {
	tokens map[string]*domain.RefreshToken
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{tokens: make(map[string]*domain.RefreshToken)}
}

func (m *mockTokenRepo) Save(_ context.Context, t *domain.RefreshToken) error {
	m.tokens[t.TokenHash] = t
	return nil
}

func (m *mockTokenRepo) FindByTokenHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	t, ok := m.tokens[hash]
	if !ok {
		return nil, domain.ErrTokenNotFound
	}
	return t, nil
}

func (m *mockTokenRepo) Revoke(_ context.Context, id uuid.UUID) error {
	for _, t := range m.tokens {
		if t.ID == id {
			now := time.Now().UTC()
			t.RevokedAt = &now
			return nil
		}
	}
	return nil
}

// --- helpers ---

func newTestAuthUsecase(users *mockUserRepo, tokens *mockTokenRepo) *usecase.AuthUsecase {
	return usecase.NewAuthUsecase(users, tokens, usecase.AuthConfig{
		JWTSecret:  []byte("test-secret-at-least-32-bytes-long!!"),
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 168 * time.Hour,
	})
}

// --- Register ---

func TestAuthUsecase_Register_Success(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())

	pair, err := uc.Register(context.Background(), usecase.RegisterInput{
		Email:    "user@example.com",
		Password: "password123",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

func TestAuthUsecase_Register_EmailNormalized(t *testing.T) {
	users := newMockUserRepo()
	uc := newTestAuthUsecase(users, newMockTokenRepo())

	_, err := uc.Register(context.Background(), usecase.RegisterInput{
		Email:    "  USER@EXAMPLE.COM  ",
		Password: "password123",
	})
	require.NoError(t, err)

	_, ok := users.users["user@example.com"]
	assert.True(t, ok, "email should be stored normalized (lowercase, trimmed)")
}

func TestAuthUsecase_Register_DuplicateEmail(t *testing.T) {
	users := newMockUserRepo()
	uc := newTestAuthUsecase(users, newMockTokenRepo())

	_, err := uc.Register(context.Background(), usecase.RegisterInput{Email: "a@b.com", Password: "password123"})
	require.NoError(t, err)

	_, err = uc.Register(context.Background(), usecase.RegisterInput{Email: "a@b.com", Password: "other1234"})
	assert.True(t, errors.Is(err, domain.ErrEmailTaken))
}

func TestAuthUsecase_Register_ShortPassword(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())

	_, err := uc.Register(context.Background(), usecase.RegisterInput{Email: "a@b.com", Password: "short"})
	assert.ErrorIs(t, err, usecase.ErrValidation)
}

func TestAuthUsecase_Register_EmptyEmail(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())

	_, err := uc.Register(context.Background(), usecase.RegisterInput{Email: "   ", Password: "password123"})
	assert.ErrorIs(t, err, usecase.ErrValidation)
}

// --- Login ---

func TestAuthUsecase_Login_Success(t *testing.T) {
	users := newMockUserRepo()
	tokens := newMockTokenRepo()
	uc := newTestAuthUsecase(users, tokens)

	_, err := uc.Register(context.Background(), usecase.RegisterInput{Email: "u@x.com", Password: "pass1234"})
	require.NoError(t, err)

	pair, err := uc.Login(context.Background(), usecase.LoginInput{Email: "u@x.com", Password: "pass1234"})
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
}

func TestAuthUsecase_Login_WrongPassword(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())
	_, _ = uc.Register(context.Background(), usecase.RegisterInput{Email: "u@x.com", Password: "pass1234"})

	_, err := uc.Login(context.Background(), usecase.LoginInput{Email: "u@x.com", Password: "wrongpass"})
	assert.ErrorIs(t, err, usecase.ErrInvalidCredentials)
}

func TestAuthUsecase_Login_UnknownEmail(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())

	_, err := uc.Login(context.Background(), usecase.LoginInput{Email: "nobody@x.com", Password: "pass1234"})
	assert.ErrorIs(t, err, usecase.ErrInvalidCredentials)
}

func TestAuthUsecase_Login_SameErrorForWrongPasswordAndUnknownEmail(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())
	_, _ = uc.Register(context.Background(), usecase.RegisterInput{Email: "u@x.com", Password: "pass1234"})

	errWrong, _ := func() (error, *domain.TokenPair) {
		p, e := uc.Login(context.Background(), usecase.LoginInput{Email: "u@x.com", Password: "bad"})
		return e, p
	}()
	errUnknown, _ := func() (error, *domain.TokenPair) {
		p, e := uc.Login(context.Background(), usecase.LoginInput{Email: "no@x.com", Password: "bad"})
		return e, p
	}()

	assert.Equal(t, errWrong.Error(), errUnknown.Error(), "error message must be identical")
}

// --- Refresh ---

func TestAuthUsecase_Refresh_Success(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())

	pair, err := uc.Register(context.Background(), usecase.RegisterInput{Email: "u@x.com", Password: "pass1234"})
	require.NoError(t, err)

	newPair, err := uc.Refresh(context.Background(), pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newPair.AccessToken)
	assert.NotEqual(t, pair.RefreshToken, newPair.RefreshToken, "refresh token must be rotated")
}

func TestAuthUsecase_Refresh_RevokedToken(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())

	pair, _ := uc.Register(context.Background(), usecase.RegisterInput{Email: "u@x.com", Password: "pass1234"})

	// first refresh revokes the original token
	_, err := uc.Refresh(context.Background(), pair.RefreshToken)
	require.NoError(t, err)

	// second refresh with same (now revoked) token must fail
	_, err = uc.Refresh(context.Background(), pair.RefreshToken)
	assert.ErrorIs(t, err, domain.ErrTokenRevoked)
}

func TestAuthUsecase_Refresh_ExpiredToken(t *testing.T) {
	tokens := newMockTokenRepo()
	uc := newTestAuthUsecase(newMockUserRepo(), tokens)

	pair, _ := uc.Register(context.Background(), usecase.RegisterInput{Email: "u@x.com", Password: "pass1234"})

	// manually expire the token
	hash := usecase.HashToken(pair.RefreshToken)
	token := tokens.tokens[hash]
	token.ExpiresAt = time.Now().UTC().Add(-time.Hour)

	_, err := uc.Refresh(context.Background(), pair.RefreshToken)
	assert.ErrorIs(t, err, domain.ErrTokenExpired)
}

func TestAuthUsecase_Refresh_UnknownToken(t *testing.T) {
	uc := newTestAuthUsecase(newMockUserRepo(), newMockTokenRepo())

	_, err := uc.Refresh(context.Background(), "completely-unknown-token")
	assert.ErrorIs(t, err, usecase.ErrInvalidCredentials)
}
