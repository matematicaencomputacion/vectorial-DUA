package rogerian

import (
	"fmt"
	"strings"

	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/rag"
)

// PromptBundle is the grounded prompt for live-station synthesis.
type PromptBundle struct {
	Tone         Tone
	SystemStyle  string
	UserDoubt    string
	Context      string
	Sources      []string
	DUADim       string
	Format       string
	FullPrompt   string
	EmptyContext bool
}

// BuildInput feeds the person-centered prompt builder.
type BuildInput struct {
	DoubtText       string
	Frustration     float32
	Dimension       string // Representacion | Accion | Compromiso
	Format          string // visual | conceptual | practica
	Chunks          []rag.ScoredChunk
	AvailableTopics []string // curated titles for empty-context invitations
}

// PromptBuilder composes Rogers tone + RAG citations (no invention outside context).
type PromptBuilder struct{}

const maxTopicHints = 6

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

	if len(in.Chunks) == 0 {
		return buildEmptyContext(in, hint, dim, format)
	}

	contextBlock := strings.Join(ctxParts, "\n\n")
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
		Tone:         hint.Tone,
		SystemStyle:  system,
		UserDoubt:    in.DoubtText,
		Context:      contextBlock,
		Sources:      sources,
		DUADim:       dim,
		Format:       format,
		FullPrompt:   full,
		EmptyContext: false,
	}
}

func buildEmptyContext(in BuildInput, hint ScaffoldHint, dim, format string) PromptBundle {
	topics := formatTopicHints(in.AvailableTopics)
	emotion := "Acompañá con calidez, sin juzgar el punto de partida."
	switch hint.Tone {
	case ToneValidate:
		emotion = "Si hay frustración, validala en una frase breve y humana."
	case ToneClarify:
		emotion = "Invitá a acotar o reformular con suavidad."
	case ToneEncourage, ToneReframe:
		emotion = "Mantené un tono alentador y concreto."
	}
	system := strings.Join([]string{
		"Sos un tutor cercano que habla en español rioplatense, en segunda persona.",
		"Respondé solo al estudiante: cálido, directo, sin jerga de sistema.",
		"Prohibido mencionar: contexto, fuentes, DUA, dimensión, micro-ejercicio, rogeriano,",
		"plantillas, andamiaje, RAG, prompt, o cualquier estructura interna.",
		"No enumerés lo que no podés hacer. No dejes encabezados ni frases terminadas en ':' sin contenido después.",
		emotion,
	}, " ")

	var b strings.Builder
	b.WriteString(system)
	b.WriteString("\n\n## Duda del estudiante\n")
	b.WriteString(strings.TrimSpace(in.DoubtText))
	b.WriteString("\n\n## Instrucciones\n")
	b.WriteString("No hay material recuperado para esta duda. Escribí 2 a 4 frases que:\n")
	b.WriteString("1. Reconozcan la pregunta con naturalidad.\n")
	b.WriteString("2. Digan, sin tecnicismos, que ese tema no está en el material disponible ahora.\n")
	b.WriteString("3. Inviten a reformular o a preguntar por lo que sí está a mano.\n")
	if topics != "" {
		b.WriteString("Temas disponibles (mencionalos de forma concreta y breve): ")
		b.WriteString(topics)
		b.WriteString(".\n")
	}
	b.WriteString("No uses listas de lo imposible. No uses encabezados Markdown.\n")

	return PromptBundle{
		Tone:         hint.Tone,
		SystemStyle:  system,
		UserDoubt:    in.DoubtText,
		Context:      "",
		Sources:      nil,
		DUADim:       dim,
		Format:       format,
		FullPrompt:   b.String(),
		EmptyContext: true,
	}
}

func formatTopicHints(topics []string) string {
	var clean []string
	seen := map[string]struct{}{}
	for _, raw := range topics {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, t)
		if len(clean) >= maxTopicHints {
			break
		}
	}
	switch len(clean) {
	case 0:
		return ""
	case 1:
		return clean[0]
	case 2:
		return clean[0] + " o " + clean[1]
	default:
		return strings.Join(clean[:len(clean)-1], ", ") + " o " + clean[len(clean)-1]
	}
}
