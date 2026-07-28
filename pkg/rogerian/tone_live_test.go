package rogerian

import (
	"strings"
	"testing"
)

func TestLiveStationStudentMessageSpanish(t *testing.T) {
	cases := []struct {
		status string
		frust  float32
		want   string
	}{
		{"in_progress", 0.2, "estación"},
		{"in_progress", 0.9, "no estás solo"},
		{"failed", 0.2, "reformular"},
		{"failed", 0.9, "no es un fallo tuyo"},
		{"expired", 0.5, "ya no está disponible"},
	}
	for _, tc := range cases {
		msg := LiveStationStudentMessage(tc.status, tc.frust)
		if msg == "" || !strings.Contains(strings.ToLower(msg), strings.ToLower(tc.want)) {
			t.Fatalf("status=%s frust=%v: want substring %q in %q", tc.status, tc.frust, tc.want, msg)
		}
		if strings.Contains(msg, "threshold") || strings.Contains(msg, "live station generation") {
			t.Fatalf("technical English leaked: %q", msg)
		}
	}
}
