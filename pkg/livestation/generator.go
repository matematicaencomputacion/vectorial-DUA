package livestation

import (
	"context"
	"fmt"
	"path"
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
	Retriever       *rag.Retriever
	Nodes           *vector.Index
	Builder         rogerian.PromptBuilder
	Synthesizer     rogerian.Synthesizer
	AvailableTopics []string // curated titles for empty-context invitations
	Tel             *telemetry.Collector
	Logf            func(format string, args ...any)
	TopK            int
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
		DoubtText:       req.DoubtText,
		Frustration:     req.Frustration,
		Dimension:       req.Dimension,
		Format:          req.Format,
		Chunks:          hits,
		AvailableTopics: g.topicHints(),
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

func (g *Generator) topicHints() []string {
	if len(g.AvailableTopics) > 0 {
		return append([]string(nil), g.AvailableTopics...)
	}
	return curatedLabelsFromIndex(g.Nodes)
}

func curatedLabelsFromIndex(idx *vector.Index) []string {
	if idx == nil {
		return nil
	}
	var out []string
	for _, n := range idx.Nodes() {
		if n.IsLiveGenerated {
			continue
		}
		if label := humanizeResourceURL(n.ResourceURL); label != "" {
			out = append(out, label)
		}
	}
	return out
}

func humanizeResourceURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "live://") {
		return ""
	}
	base := path.Base(raw)
	base = strings.TrimSuffix(base, path.Ext(base))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	return strings.TrimSpace(base)
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
			return finalizeContent(generated, p.Sources), synthesizerModel(g.Synthesizer), nil
		}
		synthesisErr = err
	}
	return finalizeContent(synthesizeExtractive(p, hits, g.topicHints()), p.Sources), model, synthesisErr
}

func synthesizeExtractive(p rogerian.PromptBundle, hits []rag.ScoredChunk, topics []string) string {
	var b strings.Builder
	b.WriteString("# Estación en vivo\n\n")
	if len(hits) == 0 {
		b.WriteString("Gracias por tu pregunta.\n\n")
		b.WriteString("No encontré material verificado en la base de conocimiento para esta duda. ")
		b.WriteString("Si querés, reformulá la pregunta o pedime algo de lo que sí tengo a mano")
		if hints := formatTopicList(topics); hints != "" {
			b.WriteString(": ")
			b.WriteString(hints)
			b.WriteString(".")
		} else {
			b.WriteString(".")
		}
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString("## Contención\n")
	b.WriteString(HintLine(p.Tone))
	b.WriteString("\n\n## Explicación anclada al material\n")
	for i, h := range hits {
		b.WriteString(fmt.Sprintf("### Fuente [%d]: %s\n\n", i+1, h.Chunk.Source))
		b.WriteString(strings.TrimSpace(h.Chunk.Text))
		b.WriteString("\n\n")
	}
	exercise := ""
	switch p.Format {
	case string(dua.Practica):
		exercise = "En una celda del IDE, escribe un ejemplo mínimo relacionado con el material anterior y ejecútalo."
	case string(dua.Visual):
		exercise = "Dibuja un diagrama de flujo de 3 cajas basado solo en las ideas recuperadas."
	default:
		exercise = "Explica con tus palabras (2–3 frases) la idea principal de la fuente [1], sin agregar datos externos."
	}
	if strings.TrimSpace(exercise) != "" {
		b.WriteString("## Micro-ejercicio\n")
		b.WriteString(exercise)
		b.WriteString("\n")
	}
	return b.String()
}

func formatTopicList(topics []string) string {
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
		if len(clean) >= 6 {
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

func finalizeContent(content string, sources []string) string {
	body := pruneEmptySections(sanitizeStudentContent(stripGeneratedSources(content)))
	return appendSources(body, sources)
}

func appendSources(content string, sources []string) string {
	content = strings.TrimSpace(content)
	if len(sources) == 0 {
		return content
	}
	var b strings.Builder
	b.WriteString(content)
	b.WriteString("\n\n## Fuentes\n")
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

// sanitizeStudentContent trims dangling colon-only tails and collapses blank lines.
func sanitizeStudentContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "" || strings.HasSuffix(last, ":") {
			lines = lines[:len(lines)-1]
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// pruneEmptySections drops known optional ## sections whose body is empty.
func pruneEmptySections(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		if name, ok := optionalSectionHeading(line); ok {
			j := i + 1
			for j < len(lines) && !isMarkdownHeading(lines[j]) {
				j++
			}
			body := strings.TrimSpace(strings.Join(lines[i+1:j], "\n"))
			if body != "" {
				out = append(out, line)
				out = append(out, lines[i+1:j]...)
			} else {
				_ = name
			}
			i = j
			continue
		}
		out = append(out, line)
		i++
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func optionalSectionHeading(line string) (string, bool) {
	trim := strings.TrimSpace(line)
	if !strings.HasPrefix(trim, "##") || strings.HasPrefix(trim, "###") {
		return "", false
	}
	heading := strings.TrimSpace(strings.TrimLeft(trim, "#"))
	switch strings.ToLower(heading) {
	case "fuentes", "micro-ejercicio", "micro ejercicio", "contención", "contencion":
		return heading, true
	default:
		return "", false
	}
}

func isMarkdownHeading(line string) bool {
	trim := strings.TrimSpace(line)
	if !strings.HasPrefix(trim, "#") {
		return false
	}
	return strings.TrimSpace(strings.TrimLeft(trim, "#")) != ""
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
		return "Buen ritmo; aquí tienes una estación breve construida a partir del material recuperado."
	}
}
