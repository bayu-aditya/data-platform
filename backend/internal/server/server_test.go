package server_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"

	"github.com/bayu-aditya/data-platform/server/internal/server"
)

// startServer boots the server on a free port and returns its base URL.
func startServer(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}

	srv := server.New(server.Config{Addr: addr})

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("serve: %v", err)
		}
	})

	waitReady(t, addr)

	return "http://" + addr
}

func waitReady(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s never became ready", addr)
}

// grpcClient talks the gRPC protocol over unencrypted HTTP/2, the way an
// external gRPC client would.
func grpcClient(baseURL string) *grpchealth.Client {
	transport := &http.Transport{}
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols

	return grpchealth.NewClient(
		&http.Client{Transport: transport},
		baseURL,
		connect.WithGRPC(),
	)
}

func TestCheckProcessIsServing(t *testing.T) {
	t.Parallel()

	client := grpcClient(startServer(t))

	res, err := client.Check(context.Background(), &grpchealth.CheckRequest{})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if res.Status != grpchealth.StatusServing {
		t.Errorf("status = %v, want %v", res.Status, grpchealth.StatusServing)
	}
}

func TestCheckUnknownServiceIsNotFound(t *testing.T) {
	t.Parallel()

	client := grpcClient(startServer(t))

	_, err := client.Check(context.Background(), &grpchealth.CheckRequest{
		Service: "data_platform.v1.NoSuchService",
	})
	if got, want := connect.CodeOf(err), connect.CodeNotFound; got != want {
		t.Errorf("code = %v, want %v (err: %v)", got, want, err)
	}
}
