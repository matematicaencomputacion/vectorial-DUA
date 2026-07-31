package livestation

import (
	"strings"
	"testing"
)

func TestSanitizeStudentContentTrimsDanglingColon(t *testing.T) {
	raw := "Hola.\n\n\nPrueba el siguiente micro-desafío a tu ritmo:\n"
	got := sanitizeStudentContent(raw)
	if strings.HasSuffix(got, ":") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Fatalf("blanks not collapsed: %q", got)
	}
	if !strings.Contains(got, "Hola.") {
		t.Fatalf("lost body: %q", got)
	}
}

func TestPruneEmptySectionsDropsHollowHeadings(t *testing.T) {
	raw := "# Estación\n\nCuerpo útil.\n\n## Micro-ejercicio\n\n## Fuentes\n\n## Contención\nTexto vivo.\n"
	got := pruneEmptySections(raw)
	if strings.Contains(got, "## Micro-ejercicio") || strings.Contains(got, "## Fuentes") {
		t.Fatalf("empty sections survived:\n%s", got)
	}
	if !strings.Contains(got, "## Contención") || !strings.Contains(got, "Texto vivo") {
		t.Fatalf("kept section lost:\n%s", got)
	}
}

func TestFinalizeContentOmitsEmptyFuentes(t *testing.T) {
	got := finalizeContent("Respuesta breve.\n\n## Micro-ejercicio\n\nAlgo:\n", nil)
	if strings.Contains(got, "## Fuentes") || strings.Contains(got, "## Micro-ejercicio") {
		t.Fatalf("unexpected sections:\n%s", got)
	}
	if strings.HasSuffix(strings.TrimSpace(got), ":") {
		t.Fatalf("dangling colon:\n%s", got)
	}
}
