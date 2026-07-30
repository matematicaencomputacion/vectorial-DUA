package rogerian

import (
	"fmt"
	"strings"

	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/rag"
)

// PromptBundle is the grounded prompt for live-station synthesis.
type PromptBundle struct {
	Tone        Tone
	SystemStyle string
	UserDoubt   string
	Context     string
	Sources     []string
	DUADim      string
	Format      string
	FullPrompt  string
}

// BuildInput feeds the person-centered prompt builder.
type BuildInput struct {
	DoubtText   string
	Frustration float32
	Dimension   string // Representacion | Accion | Compromiso
	Format      string // visual | conceptual | practica
	Chunks      []rag.ScoredChunk
}

// PromptBuilder composes Rogers tone + RAG citations (no invention outside context).
type PromptBuilder struct{}

// Build creates a PromptBundle constrained to retrieved chunks.
func (PromptBuilder) Build(in BuildInput) PromptBundle {
	hint := HintForFrustration(in.Frustration)
	dim := in.Dimension
	if !dua.ValidDimension(dim) {
		dim = string(dua.Representacion)
	}
	format := in.Format
	if format == "" {
		format = string(dua.Conceptual)
	}

	var ctxParts []string
	var sources []string
	seen := map[string]struct{}{}
	for i, h := range in.Chunks {
		cite := fmt.Sprintf("[%d] (%s) %s\n%s", i+1, h.Chunk.Source, h.Chunk.Title, strings.TrimSpace(h.Chunk.Text))
		ctxParts = append(ctxParts, cite)
		if _, ok := seen[h.Chunk.Source]; !ok {
			seen[h.Chunk.Source] = struct{}{}
			sources = append(sources, h.Chunk.Source)
		}
	}
	contextBlock := strings.Join(ctxParts, "\n\n")
	if contextBlock == "" {
		contextBlock = "(sin contexto recuperado)"
	}

	system := strings.Join([]string{
		"Eres un facilitador rogeriano de aprendizaje (Carl Rogers + DUA).",
		"NO juzgues el punto de partida del estudiante.",
		"Usa ÚNICAMENTE el contexto citado; no uses conocimiento previo ni completes vacíos.",
		"Si el contexto falta o no alcanza, dilo con honestidad.",
		fmt.Sprintf("Dimensión DUA: %s. Formato preferido: %s.", dim, format),
		fmt.Sprintf("Tono: %s — %s", hint.Tone, hint.Message),
	}, " ")

	full := fmt.Sprintf(`%s

## Duda del estudiante
%s

## Contexto verificado (RAG)
%s

## Instrucciones
1. Valida la emoción/bloqueo brevemente.
2. Explica el concepto con el formato DUA pedido.
3. Cita las fuentes [#] usadas.
4. Propón un micro-ejercicio seguro (sin presión).
5. No agregues una sección Fuentes; la aplicación la construye.
`, system, strings.TrimSpace(in.DoubtText), contextBlock)

	return PromptBundle{
		Tone:        hint.Tone,
		SystemStyle: system,
		UserDoubt:   in.DoubtText,
		Context:     contextBlock,
		Sources:     sources,
		DUADim:      dim,
		Format:      format,
		FullPrompt:  full,
	}
}
