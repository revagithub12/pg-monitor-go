// Package auth provides JWT login and role-based access control (RBAC).
package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	RoleAdmin  = "admin"  // can read everything and manage alert rules
	RoleViewer = "viewer" // read-only
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type User struct {
	Username string
	Password string
	Role     string
}

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	secret []byte
	users  []User
}

func NewService(secret string, users ...User) *Service {
	return &Service{secret: []byte(secret), users: users}
}

// Login checks the credentials and returns a signed JWT valid for one hour.
func (s *Service) Login(username, password string) (string, error) {
	for _, u := range s.users {
		userOK := subtle.ConstantTimeCompare([]byte(u.Username), []byte(username)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(u.Password), []byte(password)) == 1
		if userOK && passOK {
			claims := Claims{
				Role: u.Role,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   u.Username,
					IssuedAt:  jwt.NewNumericDate(time.Now()),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}
			return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
		}
	}
	return "", ErrInvalidCredentials
}

// Parse validates the token signature and expiry and returns its claims.
func (s *Service) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// BearerToken extracts the token from the Authorization header.
func BearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if token, ok := strings.CutPrefix(header, "Bearer "); ok {
		return token
	}
	return ""
}

// Require wraps a handler so only valid tokens with one of the given roles can call it.
func (s *Service) Require(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := s.Parse(BearerToken(r))
		if err != nil {
			http.Error(w, `{"error":"invalid or missing token"}`, http.StatusUnauthorized)
			return
		}
		if !slices.Contains(roles, claims.Role) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
