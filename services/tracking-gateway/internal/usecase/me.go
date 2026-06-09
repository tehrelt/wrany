package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/wrany/tracking-gateway/internal/domain"
)

type MeUsecase struct {
	users UserRepository
}

func NewMeUsecase(users UserRepository) *MeUsecase {
	return &MeUsecase{users: users}
}

func (uc *MeUsecase) GetMe(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return uc.users.FindByID(ctx, userID)
}
