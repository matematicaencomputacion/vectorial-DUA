package dua

// ResolveBotoneraDelta returns the resolved preference delta for a botonera
// selection. Empty result means "nothing to apply" and is not an error.
//
// Resolution order:
//  1. clientDelta (if provided) — always wins when non-empty
//  2. schema variant preference_delta
//  3. empty (including legacy flat Botonera matches)
//
// Legacy InteractiveVideoNode.Botonera entries may carry vector_delta in
// content-embedding space. Those MUST NEVER be applied to V_e (VeDims=5).
// When the touch matches only a legacy id_btn and the client sent no
// preference_delta, this returns empty → Ack neutro without Apply.
func ResolveBotoneraDelta(node *InteractiveVideoNode, variantID string, clientDelta []float32) []float32 {
	if len(clientDelta) > 0 {
		return append([]float32(nil), clientDelta...)
	}
	if node == nil || variantID == "" {
		return nil
	}
	schema := node.BotoneraSchema
	if schema == nil {
		// Legacy-only node: existence validated elsewhere; no V_e delta without client.
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
	// Matched as legacy id_btn only (or unknown schema variant) → no V_e delta.
	return nil
}

// HasBotoneraVariant reports whether variantID (and formatID for combined) exists
// in the node's botonera_schema OR as a legacy flat Botonera id_btn.
func HasBotoneraVariant(node *InteractiveVideoNode, variantID, formatID string) bool {
	if node == nil || variantID == "" {
		return false
	}
	if hasSchemaVariant(node.BotoneraSchema, variantID, formatID) {
		return true
	}
	for _, b := range node.Botonera {
		if b.IDBtn == variantID {
			return true
		}
	}
	return false
}

func hasSchemaVariant(schema *DUANodeBotonera, variantID, formatID string) bool {
	if schema == nil || variantID == "" {
		return false
	}
	switch schema.Kind {
	case SchemaDepth:
		for _, opt := range schema.DepthOptions {
			if opt.VariantID == variantID {
				return true
			}
		}
	case SchemaCognitive:
		for _, opt := range schema.CognitiveOptions {
			if opt.VariantID == variantID {
				return true
			}
		}
	case SchemaEmergency:
		for _, opt := range schema.EmergencyOptions {
			if opt.VariantID == variantID {
				return true
			}
		}
	case SchemaCombined:
		if formatID == "" {
			return false
		}
		_, ok := schema.ResolveCombined(variantID, formatID)
		return ok
	case SchemaFlat:
		for _, btn := range schema.FlatButtons {
			if btn.IDBtn == variantID {
				return true
			}
		}
	}
	return false
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
