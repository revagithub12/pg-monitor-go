package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestService() *Service {
	return NewService("test-secret",
		User{Username: "admin", Password: "pw1", Role: RoleAdmin},
		User{Username: "viewer", Password: "pw2", Role: RoleViewer},
	)
}

func TestLogin(t *testing.T) {
	s := newTestService()
	if _, err := s.Login("admin", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	token, err := s.Login("admin", "pw1")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.Parse(token)
	if err != nil || claims.Role != RoleAdmin {
		t.Fatalf("bad claims: %+v, %v", claims, err)
	}
}

func TestRequireRoles(t *testing.T) {
	s := newTestService()
	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }
	handler := s.Require(ok, RoleAdmin)

	call := func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec.Code
	}

	adminToken, _ := s.Login("admin", "pw1")
	viewerToken, _ := s.Login("viewer", "pw2")

	if code := call(""); code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d", code)
	}
	if code := call(viewerToken); code != http.StatusForbidden {
		t.Fatalf("viewer: got %d", code)
	}
	if code := call(adminToken); code != http.StatusNoContent {
		t.Fatalf("admin: got %d", code)
	}
}
