package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabehf/koito/engine/middleware"
	"github.com/gabehf/koito/internal/models"
)

// Focused, small test set that avoids needing a full db.DB mock.

func TestHealthHandler_Returns200(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	HealthHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestMeHandler_Unauthorized(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)

	// MeHandler does not use the store for the unauthorized path, pass nil
	MeHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestMeHandler_Success(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)

	user := &models.User{ID: 1, Username: "testuser"}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)

	MeHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "testuser") {
		t.Fatalf("expected response to contain username, got %s", rr.Body.String())
	}
}

func TestGetArtistHandler_MissingID_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/artist", nil)

	// Handler will validate query param before touching the store; pass nil
	http.HandlerFunc(GetArtistHandler(nil)).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetArtistHandler_InvalidID_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/artist?id=abc", nil)

	http.HandlerFunc(GetArtistHandler(nil)).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", rr.Code)
	}
}

func TestGetTrackHandler_MissingID_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/track", nil)

	http.HandlerFunc(GetTrackHandler(nil)).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetTrackHandler_InvalidID_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/track?id=xyz", nil)

	http.HandlerFunc(GetTrackHandler(nil)).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", rr.Code)
	}
}
