package usecase_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
	"github.com/bayu-aditya/data-platform/server/internal/repository"
	"github.com/bayu-aditya/data-platform/server/internal/repository/mocks"
	"github.com/bayu-aditya/data-platform/server/internal/usecase"
)

func TestEnvironmentCreateValidatesSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"simple", "staging", true},
		{"with digits and dash", "prod-2", true},
		{"two chars", "ab", true},
		{"63 chars", strings.Repeat("a", 63), true},
		{"empty", "", false},
		{"one char", "a", false},
		{"64 chars", strings.Repeat("a", 64), false},
		{"uppercase", "Staging", false},
		{"leading dash", "-staging", false},
		{"underscore", "stag_ing", false},
		{"space", "stag ing", false},
		{"trailing newline", "staging\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockEnvironmentRepository(t)
			if tt.valid {
				repo.EXPECT().Create(mock.Anything, tt.input).Return(repository.Environment{Name: tt.input}, nil).Once()
			}

			got, err := usecase.NewEnvironmentUsecase(repo).Create(t.Context(), tt.input)

			if tt.valid {
				if err != nil || got.Name != tt.input {
					t.Errorf("Create(%q) = %+v, %v; want success", tt.input, got, err)
				}
				return
			}
			if !domainerr.Is(err, domainerr.InvalidArgument) {
				t.Errorf("Create(%q) err = %v, want kind %v", tt.input, err, domainerr.InvalidArgument)
			}
		})
	}
}

func TestEnvironmentCreatePassesRepositoryErrorsOn(t *testing.T) {
	t.Parallel()

	dup := domainerr.New(domainerr.AlreadyExists, "environment already exists")
	repo := mocks.NewMockEnvironmentRepository(t)
	repo.EXPECT().Create(mock.Anything, "staging").Return(repository.Environment{}, dup).Once()

	_, err := usecase.NewEnvironmentUsecase(repo).Create(t.Context(), "staging")

	if !errors.Is(err, dup) {
		t.Errorf("err = %v, want %v", err, dup)
	}
}

func TestEnvironmentList(t *testing.T) {
	t.Parallel()

	want := []repository.Environment{{Name: "staging"}, {Name: "production"}}
	repo := mocks.NewMockEnvironmentRepository(t)
	repo.EXPECT().List(mock.Anything).Return(want, nil).Once()

	got, err := usecase.NewEnvironmentUsecase(repo).List(t.Context())

	if err != nil || len(got) != 2 || got[0].Name != "staging" {
		t.Errorf("List = %+v, %v; want %+v", got, err, want)
	}
}

func TestEnvironmentDelete(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	repo := mocks.NewMockEnvironmentRepository(t)
	repo.EXPECT().Delete(mock.Anything, id).Return(nil).Once()

	if err := usecase.NewEnvironmentUsecase(repo).Delete(t.Context(), id.String()); err != nil {
		t.Errorf("Delete: %v", err)
	}
}

func TestEnvironmentDeleteMalformedIDIsInvalidArgument(t *testing.T) {
	t.Parallel()

	repo := mocks.NewMockEnvironmentRepository(t)

	err := usecase.NewEnvironmentUsecase(repo).Delete(t.Context(), "not-a-uuid")

	if !domainerr.Is(err, domainerr.InvalidArgument) {
		t.Errorf("err = %v, want kind %v", err, domainerr.InvalidArgument)
	}
}
