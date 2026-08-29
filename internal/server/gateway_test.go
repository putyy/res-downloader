package server

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGatewayDispatchAndLifecycle(t *testing.T) {
	proxy := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "proxy")
	})
	gateway := New(
		func() Settings { return Settings{Host: "127.0.0.1", Port: "0"} },
		func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != "/api/test" {
				return false
			}
			_, _ = io.WriteString(w, "api")
			return true
		},
		func() http.Handler { return proxy },
		func(err error) { t.Errorf("serve gateway: %v", err) },
	)
	if err := gateway.Start(); err != nil {
		t.Fatal(err)
	}
	if !gateway.Active() || gateway.Address() == "" {
		t.Fatal("gateway did not retain its listener")
	}

	request, err := http.NewRequest(http.MethodGet, "http://"+gateway.Address()+"/api/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	// API routing intentionally uses the configured proxy port. A custom Host
	// header keeps this assertion valid when the listener requests port zero.
	request.Host = "127.0.0.1:0"
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || strings.TrimSpace(string(body)) != "api" {
		t.Fatalf("API response = %q, err=%v", body, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := gateway.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if gateway.Active() {
		t.Fatal("gateway remained active after shutdown")
	}
}
