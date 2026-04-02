package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gabehf/koito/engine/middleware"
	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/models"
	"github.com/google/uuid"
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

func assertStatusAndContains(t *testing.T, rr *httptest.ResponseRecorder, code int, contains string) {
	t.Helper()
	if rr.Code != code {
		t.Fatalf("expected %d, got %d", code, rr.Code)
	}
	if contains != "" && !strings.Contains(rr.Body.String(), contains) {
		t.Fatalf("expected body to contain %q, got %s", contains, rr.Body.String())
	}
}

func TestGetArtistHandler(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		store           ArtistStore
		expectedCode    int
		expectedContain string
	}{
		{"MissingID", "/artist", nil, http.StatusBadRequest, ""},
		{"InvalidID", "/artist?id=abc", nil, http.StatusBadRequest, ""},
		{"Success", "/artist?id=5", artistStoreMock{}, http.StatusOK, "Test Artist"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			http.HandlerFunc(GetArtistHandler(tc.store)).ServeHTTP(rr, req)
			assertStatusAndContains(t, rr, tc.expectedCode, tc.expectedContain)
		})
	}
}

func TestGetTrackHandler(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		store           TrackStore
		expectedCode    int
		expectedContain string
	}{
		{"MissingID", "/track", nil, http.StatusBadRequest, ""},
		{"InvalidID", "/track?id=xyz", nil, http.StatusBadRequest, ""},
		{"Success", "/track?id=7", trackStoreMock{}, http.StatusOK, "Test Track"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			http.HandlerFunc(GetTrackHandler(tc.store)).ServeHTTP(rr, req)
			assertStatusAndContains(t, rr, tc.expectedCode, tc.expectedContain)
		})
	}
}

func TestLoginHandler_Success(t *testing.T) {
	// prepare hashed password
	pass := []byte("secretpass")
	hashed, _ := bcrypt.GenerateFromPassword(pass, bcrypt.DefaultCost)

	store := &loginStoreMock{user: &models.User{ID: 3, Username: "bob", Password: hashed}}

	form := "username=bob&password=secretpass"
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	LoginHandler(store).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	// cookie set
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "koito_session" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected koito_session cookie to be set")
	}
}

// --- minimal mocks ---

type artistStoreMock struct{}

func (artistStoreMock) GetArtist(ctx context.Context, opts db.GetArtistOpts) (*models.Artist, error) {
	return &models.Artist{ID: opts.ID, Name: "Test Artist"}, nil
}

type trackStoreMock struct{}

func (trackStoreMock) GetTrack(ctx context.Context, opts db.GetTrackOpts) (*models.Track, error) {
	return &models.Track{ID: opts.ID, Title: "Test Track"}, nil
}

type loginStoreMock struct{ user *models.User }

func (l *loginStoreMock) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	if l.user != nil && l.user.Username == username {
		return l.user, nil
	}
	return nil, nil
}
func (l *loginStoreMock) SaveSession(ctx context.Context, userId int32, expiresAt time.Time, persistent bool) (*models.Session, error) {
	return &models.Session{ID: uuid.New(), UserID: userId, ExpiresAt: expiresAt, Persistent: persistent}, nil
}

// --- alias tests ---

func TestGetAliasesHandler_MissingAllIDs_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/aliases", nil)

	GetAliasesHandler(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "must be provided") {
		t.Fatalf("expected error message about missing IDs, got %s", rr.Body.String())
	}
}

func TestGetAliasesHandler_InvalidArtistID_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/aliases?artist_id=invalid", nil)

	GetAliasesHandler(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "invalid") {
		t.Fatalf("expected error message about invalid ID, got %s", rr.Body.String())
	}
}

func TestGetAliasesHandler_InvalidAlbumID_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/aliases?album_id=notanumber", nil)

	GetAliasesHandler(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetAliasesHandler_InvalidTrackID_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/aliases?track_id=xyz", nil)

	GetAliasesHandler(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetAliasesHandler_MultipleIDs_Returns400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/aliases?artist_id=1&album_id=2", nil)

	GetAliasesHandler(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "only one") {
		t.Fatalf("expected error message about multiple IDs, got %s", rr.Body.String())
	}
}
