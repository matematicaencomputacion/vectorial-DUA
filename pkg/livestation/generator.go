package livestation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vectorial-dua/avlp/harness/telemetry"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/rogerian"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

// Result is a materialized live pedagogical station.
type Result struct {
	Node         vector.Node
	Content      string
	Sources      []string
	TrackingULID string
	Prompt       rogerian.PromptBundle
	Retrieved    []rag.ScoredChunk
}

// Generator turns a routing miss into a persisted live node via RAG + Rogers.
type Generator struct {
	Retriever   *rag.Retriever
	Nodes       *vector.Index
	Builder     rogerian.PromptBuilder
	Synthesizer rogerian.Synthesizer
	Tel         *telemetry.Collector
	Logf        func(format string, args ...any)
	TopK        int
}

// Request is the miss-path input.
type Request struct {
	StudentID      string
	DoubtText      string
	QueryEmbedding []float32
	Frustration    float32
	Dimension      string
	Format         string
	TrackingULID   string
}

var _ vector.LiveGenerator = (*Generator)(nil)

// GenerateLive adapts the generator directly to vector.Router's miss-path
// contract. Generate remains the richer API for callers that need prompt and
// retrieval details.
func (g *Generator) GenerateLive(ctx context.Context, req vector.LiveRequest) (vector.LiveResult, error) {
	res, err := g.Generate(ctx, Request{
		StudentID:      req.StudentID,
		DoubtText:      req.DoubtText,
		QueryEmbedding: req.QueryEmbedding,
		Frustration:    req.Frustration,
		Dimension:      req.Dimension,
		Format:         req.Format,
		TrackingULID:   req.TrackingULID,
	})
	if err != nil {
		return vector.LiveResult{}, err
	}
	return vector.LiveResult{
		Node:         res.Node,
		Content:      res.Content,
		Sources:      res.Sources,
		TrackingULID: res.TrackingULID,
	}, nil
}

// Generate retrieves context, synthesizes a station, and registers a live node.
func (g *Generator) Generate(ctx context.Context, req Request) (Result, error) {
	if g.Retriever == nil || g.Nodes == nil {
		return Result{}, fmt.Errorf("livestation: retriever and nodes index required")
	}
	if strings.TrimSpace(req.DoubtText) == "" {
		req.DoubtText = "Tengo un bloqueo y no entiendo este concepto; necesito una explicación básica."
	}
	if req.Dimension == "" {
		req.Dimension = string(dua.Representacion)
	}
	if req.Format == "" {
		req.Format = string(dua.Conceptual)
	}
	if req.TrackingULID == "" {
		id, err := vector.NewTrackingULID()
		if err != nil {
			return Result{}, err
		}
		req.TrackingULID = id
	}

	start := time.Now()
	hits, err := g.Retriever.RetrieveText(ctx, req.DoubtText)
	if err != nil {
		return Result{}, err
	}
	if g.Tel != nil {
		g.Tel.ObserveRouting(time.Since(start))
		g.Tel.Inc("rag_retrieve_total", 1)
	}

	prompt := g.Builder.Build(rogerian.BuildInput{
		DoubtText:   req.DoubtText,
		Frustration: req.Frustration,
		Dimension:   req.Dimension,
		Format:      req.Format,
		Chunks:      hits,
	})

	sources := rag.Sources(hits)
	content, model, synthesisErr := g.synthesize(ctx, prompt, hits)
	if synthesisErr != nil {
		g.logf("LLM synthesis failed; using extractive fallback: %v", synthesisErr)
		if g.Tel != nil {
			g.Tel.Inc("livestation_synthesis_fallback_total", 1)
		}
	} else if g.Synthesizer == nil {
		g.logf("LLM synthesizer unavailable; using extractive fallback")
	}

	emb := append([]float32(nil), req.QueryEmbedding...)
	if len(emb) == 0 {
		emb, err = g.Retriever.Embedder.Embed(ctx, req.DoubtText)
		if err != nil {
			return Result{}, err
		}
	}
	emb, err = vector.FitIndexEmbedding(emb, g.Nodes.Dims())
	if err != nil {
		return Result{}, fmt.Errorf("live station embedding: %w", err)
	}

	node, err := g.Nodes.RegisterLiveNode(req.Dimension, "adaptativo", req.Format, "live://stations/"+req.TrackingULID, emb)
	if err != nil {
		return Result{}, err
	}

	if g.Tel != nil {
		g.Tel.TraceLLM(telemetry.LLMSpan{
			Model:            model,
			Purpose:          "live_station",
			PromptTokens:     len(strings.Fields(prompt.FullPrompt)),
			CompletionTokens: len(strings.Fields(content)),
			LatencyMS:        time.Since(start).Milliseconds(),
			Success:          synthesisErr == nil,
			ErrorMessage:     errorMessage(synthesisErr),
			ParentRunID:      req.TrackingULID,
		})
		g.Tel.Inc("livestation_generated_total", 1)
	}

	return Result{
		Node:         node,
		Content:      content,
		Sources:      sources,
		TrackingULID: req.TrackingULID,
		Prompt:       prompt,
		Retrieved:    hits,
	}, nil
}

func (g *Generator) synthesize(
	ctx context.Context,
	p rogerian.PromptBundle,
	hits []rag.ScoredChunk,
) (content, model string, synthesisErr error) {
	model = "extractive-fallback"
	if g.Synthesizer != nil {
		generated, err := g.Synthesizer.Synthesize(ctx, p)
		if err == nil {
			return appendSources(generated, p.Sources), synthesizerModel(g.Synthesizer), nil
		}
		synthesisErr = err
	}
	return appendSources(synthesizeExtractive(p, hits), p.Sources), model, synthesisErr
}

func synthesizeExtractive(p rogerian.PromptBundle, hits []rag.ScoredChunk) string {
	var b strings.Builder
	b.WriteString("# Estación en vivo\n\n")
	b.WriteString("## Contención\n")
	b.WriteString(HintLine(p.Tone))
	b.WriteString("\n\n## Explicación anclada al contexto\n")
	if len(hits) == 0 {
		b.WriteString("No encontré material verificado en la base de conocimiento para esta duda. ")
		b.WriteString("Sigamos atomizando la pregunta juntos sin inventar detalles.\n")
	} else {
		// Use only chunk text — faithfulness by construction.
		for i, h := range hits {
			b.WriteString(fmt.Sprintf("### Fuente [%d]: %s\n\n", i+1, h.Chunk.Source))
			b.WriteString(strings.TrimSpace(h.Chunk.Text))
			b.WriteString("\n\n")
		}
	}
	b.WriteString("## Micro-ejercicio\n")
	switch p.Format {
	case string(dua.Practica):
		b.WriteString("En una celda del IDE, escribe un ejemplo mínimo relacionado con el contexto anterior y ejecútalo.\n")
	case string(dua.Visual):
		b.WriteString("Dibuja un diagrama de flujo de 3 cajas basado solo en las ideas de las fuentes citadas.\n")
	default:
		b.WriteString("Explica con tus palabras (2–3 frases) la idea principal de la fuente [1], sin agregar datos externos.\n")
	}
	return b.String()
}

func appendSources(content string, sources []string) string {
	var b strings.Builder
	b.WriteString(stripGeneratedSources(content))
	b.WriteString("\n\n## Fuentes\n")
	if len(sources) == 0 {
		b.WriteString("- Sin fuentes recuperadas.\n")
		return b.String()
	}
	for i, source := range sources {
		b.WriteString(fmt.Sprintf("- [%d] %s\n", i+1, source))
	}
	return b.String()
}

func stripGeneratedSources(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for i, line := range lines {
		heading := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if strings.EqualFold(heading, "Fuentes") {
			return strings.TrimSpace(strings.Join(lines[:i], "\n"))
		}
	}
	return strings.TrimSpace(content)
}

func synthesizerModel(synth rogerian.Synthesizer) string {
	type modelNamer interface {
		ModelName() string
	}
	if named, ok := synth.(modelNamer); ok {
		if model := strings.TrimSpace(named.ModelName()); model != "" {
			return model
		}
	}
	return "configured-synthesizer"
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (g *Generator) logf(format string, args ...any) {
	if g.Logf != nil {
		g.Logf(format, args...)
	}
}

// HintLine maps tone to a short facilitator line.
func HintLine(t rogerian.Tone) string {
	switch t {
	case rogerian.ToneValidate:
		return "Es normal no entender esto al principio; vamos a avanzar con material verificado, sin presión."
	case rogerian.ToneClarify:
		return "Reformulemos la duda con apoyo del material recuperado, paso a paso."
	case rogerian.ToneReframe:
		return "Miremos el concepto desde otro ángulo usando solo las fuentes citadas."
	default:
		return "Buen ritmo; aquí tienes una estación breve construida a partir del contexto recuperado."
	}
}
