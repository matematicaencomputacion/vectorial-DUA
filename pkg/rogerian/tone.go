// Package rogerian provides person-centered scaffolding helpers (Carl Rogers).
// The Agent panel consumes these tones when detecting cognitive/emotional blocks.
package rogerian

// Tone guides facilitator language without judging the learner's starting point.
type Tone string

const (
	ToneValidate    Tone = "validate"
	ToneClarify     Tone = "clarify"
	ToneEncourage   Tone = "encourage"
	ToneReframe     Tone = "reframe"
)

// ScaffoldHint suggests how the Agent should respond to a block.
type ScaffoldHint struct {
	Tone    Tone
	Message string
}

// HintForFrustration maps affective load to a non-judgmental facilitator hint.
func HintForFrustration(level float32) ScaffoldHint {
	switch {
	case level >= 0.75:
		return ScaffoldHint{
			Tone:    ToneValidate,
			Message: "Es normal no entender esto al principio; vamos a atomizar la duda juntos.",
		}
	case level >= 0.4:
		return ScaffoldHint{
			Tone:    ToneClarify,
			Message: "Reformulemos tu pregunta en una pieza más pequeña y concreta.",
		}
	default:
		return ScaffoldHint{
			Tone:    ToneEncourage,
			Message: "Buen progreso; prueba el siguiente micro-desafío a tu ritmo.",
		}
	}
}

// LiveStationStudentMessage returns Spanish, person-centered copy for pending stations.
// status: in_progress | failed | expired | ready (ready uses HintForFrustration).
func LiveStationStudentMessage(status string, frustration float32) string {
	switch status {
	case "ready":
		return HintForFrustration(frustration).Message
	case "in_progress":
		if frustration >= 0.75 {
			return "Estamos preparando una estación para tu duda; en un momento va a estar lista. Tu pregunta vale la pena — no estás solo en esto."
		}
		return "Estamos preparando una estación para tu duda; en un momento va a estar lista. Tu pregunta vale la pena."
	case "expired":
		return "Esa estación ya no está disponible. No encontré material verificado a tiempo; probemos reformular la duda juntos."
	default: // failed and unknown
		if frustration >= 0.75 {
			return "No encontré material verificado todavía; no es un fallo tuyo. Probemos reformular la duda juntos."
		}
		return "No encontré material verificado todavía; probemos reformular la duda juntos."
	}
}
