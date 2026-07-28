package dua

// ResolveBotoneraDelta returns the resolved preference delta for a botonera
// selection. Empty result means "nothing to apply" and is not an error.
//
// Resolution order:
//  1. clientDelta (if provided)
//  2. schema variant delta
//  3. empty
func ResolveBotoneraDelta(schema *DUANodeBotonera, variantID string, clientDelta []float32) []float32 {
	if len(clientDelta) > 0 {
		return append([]float32(nil), clientDelta...)
	}
	if schema == nil || variantID == "" {
		return nil
	}

	switch schema.Kind {
	case SchemaDepth:
		for _, opt := range schema.DepthOptions {
			if opt.VariantID == variantID {
				return append([]float32(nil), opt.PreferenceDelta...)
			}
		}
	case SchemaCognitive:
		for _, opt := range schema.CognitiveOptions {
			if opt.VariantID == variantID {
				return append([]float32(nil), opt.PreferenceDelta...)
			}
		}
	case SchemaEmergency:
		for _, opt := range schema.EmergencyOptions {
			if opt.VariantID == variantID {
				return append([]float32(nil), opt.PreferenceDelta...)
			}
		}
	}
	return nil
}

// ResolveSubtopicDelta returns the resolved preference delta for subtopic touch.
// Empty result means "nothing to apply" and is not an error.
//
// Resolution order:
//  1. clientDelta (if provided)
//  2. subtopic orbit_delta
//  3. empty
func ResolveSubtopicDelta(tree *DUAHierarchicalTree, subtopicID string, clientDelta []float32) []float32 {
	if len(clientDelta) > 0 {
		return append([]float32(nil), clientDelta...)
	}
	if tree == nil || subtopicID == "" {
		return nil
	}
	node, ok := tree.FindByID(subtopicID)
	if !ok || len(node.OrbitDelta) == 0 {
		return nil
	}
	return append([]float32(nil), node.OrbitDelta...)
}
