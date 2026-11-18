package usecase_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
	"github.com/bayu-aditya/data-platform/server/internal/repository/mocks"
	"github.com/bayu-aditya/data-platform/server/internal/usecase"
)

func TestCheckMetadataDBHealthy(t *testing.T) {
	t.Parallel()

	repo := mocks.NewMockHealthRepository(t)
	repo.EXPECT().Ping(mock.Anything).Return(nil).Once()

	if err := usecase.NewHealthUsecase(repo).CheckMetadataDB(t.Context()); err != nil {
		t.Errorf("CheckMetadataDB: %v", err)
	}
}

func TestCheckMetadataDBUnreachableIsUnavailable(t *testing.T) {
	t.Parallel()

	cause := errors.New("connection refused")
	repo := mocks.NewMockHealthRepository(t)
	repo.EXPECT().Ping(mock.Anything).Return(cause).Once()

	err := usecase.NewHealthUsecase(repo).CheckMetadataDB(t.Context())

	if !domainerr.Is(err, domainerr.Unavailable) {
		t.Errorf("err = %v, want kind %v", err, domainerr.Unavailable)
	}
	if !errors.Is(err, cause) {
		t.Errorf("err = %v, want it to wrap %v", err, cause)
	}
}
