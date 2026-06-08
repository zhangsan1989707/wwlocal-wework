package middleware

import (
	"testing"
	"time"
)

func TestGenerateAndParseTokenCarriesUserClaims(t *testing.T) {
	token, err := GenerateToken(7, "alice", "dept_admin", "test-secret", time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := ParseToken(token, "test-secret")
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || claims.Role != "dept_admin" || claims.TokenType != "access" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
