package rogerian

import "context"

// Synthesizer turns a grounded PromptBundle into student-facing Markdown.
type Synthesizer interface {
	Synthesize(ctx context.Context, bundle PromptBundle) (string, error)
}
