package dua

// LayoutType selects Master UI composition for a pedagogical node.
type LayoutType string

const (
	LayoutUnspecified          LayoutType = ""
	LayoutInteractiveDashboard LayoutType = "interactive_dashboard"
)

// ValidLayout reports whether l is a known layout.
func ValidLayout(l string) bool {
	switch LayoutType(l) {
	case LayoutInteractiveDashboard:
		return true
	default:
		return false
	}
}
