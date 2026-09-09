package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"os"
	"time"
)

type Server struct {
	http    *http.Server
	path    string
	session Session
}

// Start publishes a separate, loopback-only listener after the desktop API is
// ready. The token is scoped to the handler's automation allowlist.
func Start(appDir string, handler func(token string) http.Handler, report func(error)) (*Server, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	session := Session{Version: 1, URL: "http://" + listener.Addr().String(), Token: hex.EncodeToString(token[:])}
	api := handler(session.Token)
	s := &Server{path: SessionPath(appDir), session: session}
	s.http = &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8192,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != listener.Addr().String() || r.Header.Get("Origin") != "" {
				http.Error(w, "local automation clients only", http.StatusForbidden)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			api.ServeHTTP(w, r)
		}),
	}
	if err := writeSession(s.path, session); err != nil {
		listener.Close()
		return nil, err
	}
	go func() {
		if err := s.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = s.removeSession()
			if report != nil {
				report(err)
			}
		}
	}()
	return s, nil
}

func (s *Server) removeSession() error {
	current, err := ReadSession(s.path)
	if err != nil || current.Token != s.session.Token {
		return nil // Never remove a newer instance's discovery file.
	}
	err = os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Server) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	removeErr := s.removeSession()
	err := s.http.Shutdown(ctx)
	if err != nil {
		_ = s.http.Close()
	}
	return errors.Join(removeErr, err)
}
