package session

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ctxKey int

const identityKey ctxKey = 1

// Identity is the verified caller attached to a request context.
type Identity struct {
	StudentID string
	Role      string
	Secure    bool
}

// WithIdentity stores a verified identity on ctx.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFromContext returns the identity if present.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// AppendOutgoingMetadata attaches auth metadata for a gRPC client call.
func AppendOutgoingMetadata(ctx context.Context, id Identity) context.Context {
	pairs := []string{
		MetaAuthMode, AuthModeOpen,
	}
	if id.Secure {
		pairs = []string{
			MetaAuthMode, AuthModeSecure,
			MetaStudentID, id.StudentID,
			MetaRole, id.Role,
		}
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

// RequireSecureIdentity reads incoming metadata when the process is in secure mode.
// In open mode it returns a zero identity with Secure=false and ok=true.
func RequireSecureIdentity(ctx context.Context) (Identity, error) {
	if !SecureModeFromEnv() {
		return Identity{Secure: false}, nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return Identity{}, status.Error(codes.Unauthenticated, "authentication required")
	}
	mode := firstMD(md, MetaAuthMode)
	if mode != AuthModeSecure {
		return Identity{}, status.Error(codes.Unauthenticated, "authentication required")
	}
	sid := firstMD(md, MetaStudentID)
	role := firstMD(md, MetaRole)
	if sid == "" || (role != RoleStudent && role != RoleTeacher) {
		return Identity{}, status.Error(codes.Unauthenticated, "authentication required")
	}
	return Identity{StudentID: sid, Role: role, Secure: true}, nil
}

// ResolveStudentID applies precedence: verified metadata wins; body mismatch → NotFound.
func ResolveStudentID(id Identity, requested string) (string, error) {
	requested = trim(requested)
	if !id.Secure {
		if requested == "" {
			return "", status.Error(codes.InvalidArgument, "student_id is required")
		}
		return requested, nil
	}
	if requested != "" && requested != id.StudentID {
		return "", status.Error(codes.NotFound, "no encontrado")
	}
	return id.StudentID, nil
}

func firstMD(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
