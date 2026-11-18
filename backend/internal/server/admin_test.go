package server_test

import (
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	dataplatformv1 "github.com/bayu-aditya/data-platform/server/gen/dataplatform/v1"
	"github.com/bayu-aditya/data-platform/server/gen/dataplatform/v1/dataplatformv1connect"
	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
	"github.com/bayu-aditya/data-platform/server/internal/repository"
	"github.com/bayu-aditya/data-platform/server/internal/server"
	"github.com/bayu-aditya/data-platform/server/internal/usecase/mocks"
)

// startAdmin boots a server whose environment usecase is envs and returns an
// AdminService client speaking gRPC over unencrypted HTTP/2.
func startAdmin(t *testing.T, envs *mocks.MockEnvironmentUsecase) dataplatformv1connect.AdminServiceClient {
	t.Helper()

	baseURL := startServerWith(t, server.Config{Environments: envs})

	transport := &http.Transport{}
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols

	return dataplatformv1connect.NewAdminServiceClient(
		&http.Client{Transport: transport},
		baseURL,
		connect.WithGRPC(),
	)
}

func TestCreateEnvironment(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	envs := mocks.NewMockEnvironmentUsecase(t)
	envs.EXPECT().Create(mock.Anything, "staging").Return(repository.Environment{
		Model: repository.Model{ID: id, CreatedAt: created, UpdatedAt: created},
		Name:  "staging",
	}, nil).Once()

	res, err := startAdmin(t, envs).CreateEnvironment(t.Context(),
		connect.NewRequest(&dataplatformv1.CreateEnvironmentRequest{Name: "staging"}))
	if err != nil {
		t.Fatalf("CreateEnvironment: %v", err)
	}

	env := res.Msg.GetEnvironment()
	if env.GetId() != id.String() || env.GetName() != "staging" || !env.GetCreatedAt().AsTime().Equal(created) {
		t.Errorf("environment = %v", env)
	}
}

func TestCreateEnvironmentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"invalid slug", domainerr.New(domainerr.InvalidArgument, "bad name"), connect.CodeInvalidArgument},
		{"duplicate", domainerr.New(domainerr.AlreadyExists, "exists"), connect.CodeAlreadyExists},
		{"unclassified", assertErr("boom"), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			envs := mocks.NewMockEnvironmentUsecase(t)
			envs.EXPECT().Create(mock.Anything, "x").Return(repository.Environment{}, tt.err).Once()

			_, err := startAdmin(t, envs).CreateEnvironment(t.Context(),
				connect.NewRequest(&dataplatformv1.CreateEnvironmentRequest{Name: "x"}))

			if got := connect.CodeOf(err); got != tt.want {
				t.Errorf("code = %v, want %v (err: %v)", got, tt.want, err)
			}
		})
	}
}

func TestListEnvironments(t *testing.T) {
	t.Parallel()

	envs := mocks.NewMockEnvironmentUsecase(t)
	envs.EXPECT().List(mock.Anything).Return([]repository.Environment{{Name: "staging"}, {Name: "production"}}, nil).Once()

	res, err := startAdmin(t, envs).ListEnvironments(t.Context(),
		connect.NewRequest(&dataplatformv1.ListEnvironmentsRequest{}))
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}

	got := res.Msg.GetEnvironments()
	if len(got) != 2 || got[0].GetName() != "staging" || got[1].GetName() != "production" {
		t.Errorf("environments = %v", got)
	}
}

func TestDeleteEnvironment(t *testing.T) {
	t.Parallel()

	envs := mocks.NewMockEnvironmentUsecase(t)
	envs.EXPECT().Delete(mock.Anything, "some-id").Return(nil).Once()

	_, err := startAdmin(t, envs).DeleteEnvironment(t.Context(),
		connect.NewRequest(&dataplatformv1.DeleteEnvironmentRequest{Id: "some-id"}))
	if err != nil {
		t.Errorf("DeleteEnvironment: %v", err)
	}
}

func TestDeleteEnvironmentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"unknown id", domainerr.New(domainerr.NotFound, "not found"), connect.CodeNotFound},
		{"referenced", domainerr.New(domainerr.FailedPrecondition, "in use"), connect.CodeFailedPrecondition},
		{"malformed id", domainerr.New(domainerr.InvalidArgument, "bad id"), connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			envs := mocks.NewMockEnvironmentUsecase(t)
			envs.EXPECT().Delete(mock.Anything, "id").Return(tt.err).Once()

			_, err := startAdmin(t, envs).DeleteEnvironment(t.Context(),
				connect.NewRequest(&dataplatformv1.DeleteEnvironmentRequest{Id: "id"}))

			if got := connect.CodeOf(err); got != tt.want {
				t.Errorf("code = %v, want %v (err: %v)", got, tt.want, err)
			}
		})
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
