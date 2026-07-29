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
	Retriever *rag.Retriever
	Nodes     *vector.Index
	Builder   rogerian.PromptBuilder
	Tel       *telemetry.Collector
	TopK      int
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

	content := synthesize(prompt, hits)
	sources := rag.Sources(hits)

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
			Model:            "template-livestation",
			Purpose:          "live_station",
			PromptTokens:     len(strings.Fields(prompt.FullPrompt)),
			CompletionTokens: len(strings.Fields(content)),
			LatencyMS:        time.Since(start).Milliseconds(),
			Success:          true,
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

func synthesize(p rogerian.PromptBundle, hits []rag.ScoredChunk) string {
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
	b.WriteString("\n## Fuentes\n")
	for i, s := range p.Sources {
		b.WriteString(fmt.Sprintf("- [%d] %s\n", i+1, s))
	}
	return b.String()
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
