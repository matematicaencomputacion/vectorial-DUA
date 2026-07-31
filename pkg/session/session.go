package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	RoleStudent = "student"
	RoleTeacher = "teacher"

	MetaStudentID = "avlp-student-id"
	MetaRole      = "avlp-role"
	MetaAuthMode  = "avlp-auth-mode"

	AuthModeSecure = "secure"
	AuthModeOpen   = "open"

	tokenVersion = "v1"
)

var (
	ErrInvalidToken = errors.New("invalid session token")
	ErrExpiredToken = errors.New("session token expired")
)

// Claims are the signed session payload.
type Claims struct {
	StudentID string `json:"sid"`
	Role      string `json:"role"`
	Exp       int64  `json:"exp"`
}

// Config holds session signing settings.
type Config struct {
	Secret    string
	TeacherKey string
	TTL       time.Duration
}

// FromEnv loads session config from AVLP_SESSION_SECRET / AVLP_TEACHER_KEY / AVLP_SESSION_TTL.
func FromEnv() Config {
	return Config{
		Secret:     strings.TrimSpace(os.Getenv("AVLP_SESSION_SECRET")),
		TeacherKey: strings.TrimSpace(os.Getenv("AVLP_TEACHER_KEY")),
		TTL:        TTLFromEnv(),
	}
}

// Secure reports whether HMAC sessions are required.
func (c Config) Secure() bool {
	return strings.TrimSpace(c.Secret) != ""
}

// SecureModeFromEnv is true when AVLP_SESSION_SECRET is set.
func SecureModeFromEnv() bool {
	return strings.TrimSpace(os.Getenv("AVLP_SESSION_SECRET")) != ""
}

// TTLFromEnv reads AVLP_SESSION_TTL (default 24h).
func TTLFromEnv() time.Duration {
	v := strings.TrimSpace(os.Getenv("AVLP_SESSION_TTL"))
	if v == "" {
		return 24 * time.Hour
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 24 * time.Hour
	}
	return d
}

// Issue creates a signed token for the student, optionally elevating to teacher.
func (c Config) Issue(studentID, teacherKey string, now time.Time) (token string, claims Claims, err error) {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return "", Claims{}, fmt.Errorf("student_id is required")
	}
	role := RoleStudent
	if c.TeacherKey != "" && teacherKey != "" &&
		subtle.ConstantTimeCompare([]byte(teacherKey), []byte(c.TeacherKey)) == 1 {
		role = RoleTeacher
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	claims = Claims{
		StudentID: studentID,
		Role:      role,
		Exp:       now.Add(ttl).Unix(),
	}
	if !c.Secure() {
		return "", claims, nil
	}
	token, err = c.sign(claims)
	return token, claims, err
}

// Verify checks a Bearer token and returns claims.
func (c Config) Verify(token string, now time.Time) (Claims, error) {
	if !c.Secure() {
		return Claims{}, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != tokenVersion {
		return Claims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, []byte(c.Secret))
	_, _ = mac.Write([]byte(tokenVersion + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.StudentID == "" || (claims.Role != RoleStudent && claims.Role != RoleTeacher) {
		return Claims{}, ErrInvalidToken
	}
	if claims.Exp > 0 && now.Unix() > claims.Exp {
		return Claims{}, ErrExpiredToken
	}
	return claims, nil
}

func (c Config) sign(claims Claims) (string, error) {
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(c.Secret))
	body := tokenVersion + "." + payload
	_, _ = mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

// ParseBoolEnv is a tiny helper for tests/docs.
func ParseBoolEnv(name string) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	return err == nil && b
}
