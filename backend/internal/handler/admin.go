package handler

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	dataplatformv1 "github.com/bayu-aditya/data-platform/server/gen/dataplatform/v1"
	"github.com/bayu-aditya/data-platform/server/gen/dataplatform/v1/dataplatformv1connect"
	"github.com/bayu-aditya/data-platform/server/internal/repository"
	"github.com/bayu-aditya/data-platform/server/internal/usecase"
)

type adminHandler struct {
	environments usecase.EnvironmentUsecase
}

// NewAdminHandler returns the Connect handler for AdminService.
func NewAdminHandler(environments usecase.EnvironmentUsecase) dataplatformv1connect.AdminServiceHandler {
	return &adminHandler{environments: environments}
}

func (h *adminHandler) CreateEnvironment(ctx context.Context, req *connect.Request[dataplatformv1.CreateEnvironmentRequest]) (*connect.Response[dataplatformv1.CreateEnvironmentResponse], error) {
	env, err := h.environments.Create(ctx, req.Msg.GetName())
	if err != nil {
		return nil, ConnectError(ctx, err)
	}
	return connect.NewResponse(&dataplatformv1.CreateEnvironmentResponse{Environment: environmentProto(env)}), nil
}

func (h *adminHandler) ListEnvironments(ctx context.Context, _ *connect.Request[dataplatformv1.ListEnvironmentsRequest]) (*connect.Response[dataplatformv1.ListEnvironmentsResponse], error) {
	envs, err := h.environments.List(ctx)
	if err != nil {
		return nil, ConnectError(ctx, err)
	}
	res := &dataplatformv1.ListEnvironmentsResponse{Environments: make([]*dataplatformv1.Environment, 0, len(envs))}
	for _, env := range envs {
		res.Environments = append(res.Environments, environmentProto(env))
	}
	return connect.NewResponse(res), nil
}

func (h *adminHandler) DeleteEnvironment(ctx context.Context, req *connect.Request[dataplatformv1.DeleteEnvironmentRequest]) (*connect.Response[dataplatformv1.DeleteEnvironmentResponse], error) {
	if err := h.environments.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, ConnectError(ctx, err)
	}
	return connect.NewResponse(&dataplatformv1.DeleteEnvironmentResponse{}), nil
}

func environmentProto(env repository.Environment) *dataplatformv1.Environment {
	return &dataplatformv1.Environment{
		Id:        env.ID.String(),
		Name:      env.Name,
		CreatedAt: timestamppb.New(env.CreatedAt),
		UpdatedAt: timestamppb.New(env.UpdatedAt),
	}
}
