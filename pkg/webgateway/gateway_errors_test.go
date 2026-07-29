package webgateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestStudentFacingGRPCMessageHidesDial(t *testing.T) {
	raw := `connection error: desc = "transport: Error while dialing: dial tcp 127.0.0.1:50051: connect: connection refused"`
	got := studentFacingGRPCMessage(codes.Unavailable, raw)
	if got != StudentUnavailableMessage {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "dial tcp") {
		t.Fatal("technical dial leaked")
	}
}

func TestWriteGRPCErrorTransportJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	err := status.Error(codes.Unavailable, `connection error: desc = "transport: Error while dialing: dial tcp 127.0.0.1:50051"`)
	writeGRPCError(rr, err)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rr.Code)
	}
	var body apiError
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Message != StudentUnavailableMessage {
		t.Fatalf("message=%q", body.Message)
	}
	if body.StudentMessage != StudentUnavailableMessage {
		t.Fatalf("student_message=%q", body.StudentMessage)
	}
	if strings.Contains(rr.Body.String(), "dial tcp") {
		t.Fatalf("dial leaked in body: %s", rr.Body.String())
	}
}
