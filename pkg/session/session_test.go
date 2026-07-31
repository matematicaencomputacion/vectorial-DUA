package session_test

import (
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/pkg/session"
)

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	cfg := session.Config{Secret: "test-secret-at-least-32-chars-long", TTL: time.Hour}
	now := time.Unix(1_700_000_000, 0).UTC()
	tok, claims, err := cfg.Issue("stu-a", "", now)
	if err != nil || tok == "" || claims.Role != session.RoleStudent {
		t.Fatalf("issue: tok=%q claims=%+v err=%v", tok, claims, err)
	}
	got, err := cfg.Verify(tok, now.Add(time.Minute))
	if err != nil || got.StudentID != "stu-a" {
		t.Fatalf("verify: %+v err=%v", got, err)
	}
}

func TestTeacherKeyElevatesRole(t *testing.T) {
	cfg := session.Config{Secret: "secret", TeacherKey: "institute", TTL: time.Hour}
	now := time.Now().UTC()
	_, claims, err := cfg.Issue("stu-t", "institute", now)
	if err != nil || claims.Role != session.RoleTeacher {
		t.Fatalf("expected teacher, got %+v err=%v", claims, err)
	}
	_, claims, err = cfg.Issue("stu-t", "wrong", now)
	if err != nil || claims.Role != session.RoleStudent {
		t.Fatalf("expected student on bad key, got %+v err=%v", claims, err)
	}
}

func TestOpenModeIssueHasEmptyToken(t *testing.T) {
	cfg := session.Config{}
	tok, claims, err := cfg.Issue("stu", "", time.Now().UTC())
	if err != nil || tok != "" || claims.StudentID != "stu" {
		t.Fatalf("open mode: tok=%q claims=%+v err=%v", tok, claims, err)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	cfg := session.Config{Secret: "secret", TTL: time.Second}
	now := time.Unix(1_700_000_000, 0).UTC()
	tok, _, err := cfg.Issue("stu", "", now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.Verify(tok, now.Add(2*time.Second))
	if err != session.ErrExpiredToken {
		t.Fatalf("want expired, got %v", err)
	}
}
