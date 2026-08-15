package httpx

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestListenerHubSwitchesSameAddressWithoutRebind(t *testing.T) {
	hub, err := NewListenerHub(4)
	if err != nil {
		t.Fatalf("NewListenerHub() error = %v", err)
	}
	defer hub.Stop(context.Background())

	firstRoute, firstServer, firstDone := prepareHubServer(t, hub, "127.0.0.1:0", "first")
	if _, err := hub.Commit(firstRoute, nil); err != nil {
		t.Fatalf("Commit(first) error = %v", err)
	}
	address := firstRoute.BoundAddress().String()
	if body := getWithoutKeepAlive(t, address); body != "first" {
		t.Fatalf("first body = %q", body)
	}

	secondRoute, secondServer, secondDone := prepareHubServer(t, hub, firstRoute.ConfiguredAddress(), "second")
	if secondRoute.BoundAddress().String() != address {
		t.Fatalf("second bound address = %s, want %s", secondRoute.BoundAddress(), address)
	}
	if _, err := hub.Commit(secondRoute, firstRoute); err != nil {
		t.Fatalf("Commit(second) error = %v", err)
	}
	if body := getWithoutKeepAlive(t, address); body != "second" {
		t.Fatalf("second body = %q", body)
	}

	stopGenerationServer(t, firstRoute, firstServer, firstDone)
	stopCurrentServer(t, secondServer, secondDone)
}

func TestListenerHubAbortNewAddressReleasesBind(t *testing.T) {
	hub, err := NewListenerHub(1)
	if err != nil {
		t.Fatalf("NewListenerHub() error = %v", err)
	}
	route, err := hub.Prepare(t.Context(), "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	address := route.BoundAddress().String()
	if err := hub.Abort(route); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("address not released: %v", err)
	}
	_ = listener.Close()
}

func TestListenerHubKeepsActiveOldRequestPinnedDuringCommit(t *testing.T) {
	hub, err := NewListenerHub(4)
	if err != nil {
		t.Fatalf("NewListenerHub() error = %v", err)
	}
	defer hub.Stop(context.Background())
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	firstRoute, err := hub.Prepare(t.Context(), "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Prepare(first) error = %v", err)
	}
	firstServer, err := NewServer(&ServerConfig{Addr: "127.0.0.1:0"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		_, _ = io.WriteString(w, "first")
	}))
	if err != nil {
		t.Fatalf("NewServer(first) error = %v", err)
	}
	if err := firstServer.StartWithListener(t.Context(), firstRoute.Listener()); err != nil {
		t.Fatalf("StartWithListener(first) error = %v", err)
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- firstServer.Run(context.Background()) }()
	<-firstServer.Running()
	if _, err := hub.Commit(firstRoute, nil); err != nil {
		t.Fatalf("Commit(first) error = %v", err)
	}
	address := firstRoute.BoundAddress().String()
	responseDone := make(chan string, 1)
	go func() {
		response, requestErr := http.Get("http://" + address)
		if requestErr != nil {
			responseDone <- "error:" + requestErr.Error()
			return
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		responseDone <- string(body)
	}()
	<-requestStarted

	secondRoute, secondServer, secondDone := prepareHubServer(t, hub, firstRoute.ConfiguredAddress(), "second")
	if _, err := hub.Commit(secondRoute, firstRoute); err != nil {
		t.Fatalf("Commit(second) error = %v", err)
	}
	if body := getWithoutKeepAlive(t, address); body != "second" {
		t.Fatalf("new request body = %q", body)
	}
	stopDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := firstRoute.WaitDrained(ctx); err != nil {
			stopDone <- err
			return
		}
		stopDone <- firstServer.Stop(ctx)
	}()
	select {
	case err := <-stopDone:
		t.Fatalf("old generation stopped before active request completed: %v", err)
	default:
	}
	close(releaseRequest)
	if body := <-responseDone; body != "first" {
		t.Fatalf("old request body = %q", body)
	}
	if err := <-stopDone; err != nil {
		t.Fatalf("Stop(first) error = %v", err)
	}
	if err := <-firstDone; err != nil {
		t.Fatalf("Run(first) error = %v", err)
	}
	if err := hub.Release(firstRoute); err != nil {
		t.Fatalf("Release(first) error = %v", err)
	}
	stopCurrentServer(t, secondServer, secondDone)
}

func prepareHubServer(t *testing.T, hub *ListenerHub, address, body string) (*PreparedRoute, *Server, <-chan error) {
	t.Helper()
	route, err := hub.Prepare(t.Context(), address)
	if err != nil {
		t.Fatalf("Prepare(%s) error = %v", address, err)
	}
	server, err := NewServer(&ServerConfig{Addr: address}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if err := server.StartWithListener(t.Context(), route.Listener()); err != nil {
		t.Fatalf("StartWithListener() error = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Run(context.Background()) }()
	select {
	case <-server.Running():
	case <-time.After(time.Second):
		t.Fatal("server did not become serve-ready")
	}
	return route, server, done
}

func stopGenerationServer(t *testing.T, route *PreparedRoute, server *Server, done <-chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := route.WaitDrained(ctx); err != nil {
		t.Fatalf("WaitDrained() error = %v", err)
	}
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func stopCurrentServer(t *testing.T, server *Server, done <-chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func getWithoutKeepAlive(t *testing.T, address string) string {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: time.Second}
	response, err := client.Get(fmt.Sprintf("http://%s/", address))
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}
