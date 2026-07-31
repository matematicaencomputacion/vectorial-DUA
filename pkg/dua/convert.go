package dua

import (
	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
)

// ToProto converts a domain interactive node to protobuf.
func ToProto(n *InteractiveVideoNode) *vectorv1.InteractiveVideoNode {
	if n == nil {
		return nil
	}
	out := &vectorv1.InteractiveVideoNode{
		NodeId:                   n.NodeID,
		DimensionDua:             n.DimensionDUA,
		Titulo:                   n.Titulo,
		LayoutType:               vectorv1.LayoutType_LAYOUT_TYPE_INTERACTIVE_DASHBOARD,
		StageMediaDefault:        n.StageMediaDefault,
		Embedding:                append([]float32(nil), n.Embedding...),
		BotoneraSchema:           botoneraSchemaToProto(n.BotoneraSchema),
		Hierarchy:                hierarchyToProto(n.Hierarchy),
		StageMarkdownDefault:     n.StageMarkdownDefault,
		RetrievedSources:         append([]string(nil), n.RetrievedSources...),
		PromotedFromTrackingUlid: n.PromotedFromTrackingULID,
		CaptionsUrl:              n.CaptionsURL,
		Transcript:               n.Transcript,
		AltText:                  n.AltText,
		AudioDescriptionUrl:      n.AudioDescriptionURL,
	}
	for _, b := range n.Botonera {
		out.Botonera = append(out.Botonera, buttonToProto(b))
	}
	return out
}

func hierarchyToProto(t *DUAHierarchicalTree) *vectorv1.DUAHierarchicalTree {
	if t == nil {
		return nil
	}
	out := &vectorv1.DUAHierarchicalTree{
		MainTopicTitle: t.MainTopicTitle,
		MacroMediaUrl:  t.MacroMediaURL,
	}
	for _, s := range t.Subtopics {
		out.Subtopics = append(out.Subtopics, subtopicToProto(s))
	}
	return out
}

func subtopicToProto(s SubtopicNode) *vectorv1.SubtopicNode {
	out := &vectorv1.SubtopicNode{
		SubtopicId:           s.SubtopicID,
		Title:                s.Title,
		DepthLevel:           depthToProto(s.DepthLevel),
		IsOptional:           s.IsOptional,
		MediaUrl:             s.MediaURL,
		DurationSeconds:      s.DurationSeconds,
		OrbitDelta:           append([]float32(nil), s.OrbitDelta...),
		CaptionsUrl:          s.CaptionsURL,
		Transcript:           s.Transcript,
		AltText:              s.AltText,
		AudioDescriptionUrl:  s.AudioDescriptionURL,
	}
	for _, c := range s.ChildSubtopics {
		out.ChildSubtopics = append(out.ChildSubtopics, subtopicToProto(c))
	}
	return out
}

func depthToProto(d SubtopicDepthLevel) vectorv1.SubtopicDepth {
	switch d {
	case DepthComponent:
		return vectorv1.SubtopicDepth_SUBTOPIC_DEPTH_COMPONENT
	case DepthMicro:
		return vectorv1.SubtopicDepth_SUBTOPIC_DEPTH_MICRO
	default:
		return vectorv1.SubtopicDepth_SUBTOPIC_DEPTH_MACRO
	}
}

// ButtonToProto converts a domain button to protobuf.
func ButtonToProto(b InteractiveButton) *vectorv1.InteractiveButton {
	return buttonToProto(b)
}

func buttonToProto(b InteractiveButton) *vectorv1.InteractiveButton {
	return &vectorv1.InteractiveButton{
		IdBtn:           b.IDBtn,
		Label:           b.Label,
		ActionType:      actionToProto(b.ActionType),
		MediaUrl:        b.MediaURL,
		TimestampStart:  b.TimestampStart,
		TimestampEnd:    b.TimestampEnd,
		CellCode:        b.CellCode,
		VectorDelta:     append([]float32(nil), b.VectorDelta...),
		IsLiveGenerated: b.IsLiveGenerated,
	}
}

func actionToProto(a ButtonActionType) vectorv1.ButtonActionType {
	switch a {
	case ActionOpenIDECell:
		return vectorv1.ButtonActionType_BUTTON_ACTION_OPEN_IDE_CELL
	case ActionAskAgent:
		return vectorv1.ButtonActionType_BUTTON_ACTION_ASK_AGENT
	default:
		return vectorv1.ButtonActionType_BUTTON_ACTION_PLAY_CLIP
	}
}

func botoneraSchemaToProto(b *DUANodeBotonera) *vectorv1.DUANodeBotonera {
	if b == nil {
		return nil
	}
	out := &vectorv1.DUANodeBotonera{
		Kind:       schemaKindToProto(b.Kind),
		TopicTitle: b.TopicTitle,
		DepthAxis:  append([]string(nil), b.DepthAxis...),
		FormatAxis: append([]string(nil), b.FormatAxis...),
	}
	for _, btn := range b.FlatButtons {
		out.FlatButtons = append(out.FlatButtons, buttonToProto(btn))
	}
	for _, d := range b.DepthOptions {
		out.DepthOptions = append(out.DepthOptions, &vectorv1.DepthVariant{
			VariantId:            d.VariantID,
			Label:                d.Label,
			MediaUrl:             d.MediaURL,
			DurationSeconds:      d.DurationSeconds,
			FormatType:           mediaFormatToProto(d.FormatType),
			PreferenceDelta:      append([]float32(nil), d.PreferenceDelta...),
			CaptionsUrl:          d.CaptionsURL,
			Transcript:           d.Transcript,
			AltText:              d.AltText,
			AudioDescriptionUrl:  d.AudioDescriptionURL,
		})
	}
	for _, c := range b.CognitiveOptions {
		out.CognitiveOptions = append(out.CognitiveOptions, &vectorv1.CognitiveVariant{
			VariantId:            c.VariantID,
			Label:                c.Label,
			MediaUrl:             c.MediaURL,
			CellCode:             c.CellCode,
			FormatType:           mediaFormatToProto(c.FormatType),
			PreferenceDelta:      append([]float32(nil), c.PreferenceDelta...),
			CaptionsUrl:          c.CaptionsURL,
			Transcript:           c.Transcript,
			AltText:              c.AltText,
			AudioDescriptionUrl:  c.AudioDescriptionURL,
		})
	}
	for _, e := range b.EmergencyOptions {
		out.EmergencyOptions = append(out.EmergencyOptions, &vectorv1.EmergencyVariant{
			VariantId:            e.VariantID,
			Label:                e.Label,
			MediaUrl:             e.MediaURL,
			HintText:             e.HintText,
			WalkthroughUrl:       e.WalkthroughURL,
			FormatType:           mediaFormatToProto(e.FormatType),
			PreferenceDelta:      append([]float32(nil), e.PreferenceDelta...),
			CaptionsUrl:          e.CaptionsURL,
			Transcript:           e.Transcript,
			AltText:              e.AltText,
			AudioDescriptionUrl:  e.AudioDescriptionURL,
		})
	}
	for _, cell := range b.MatrixCells {
		out.MatrixCells = append(out.MatrixCells, &vectorv1.CombinedCell{
			DepthId:              cell.DepthID,
			FormatId:             cell.FormatID,
			MediaUrl:             cell.MediaURL,
			CellCode:             cell.CellCode,
			DurationSeconds:      cell.DurationSeconds,
			FormatType:           mediaFormatToProto(cell.FormatType),
			CaptionsUrl:          cell.CaptionsURL,
			Transcript:           cell.Transcript,
			AltText:              cell.AltText,
			AudioDescriptionUrl:  cell.AudioDescriptionURL,
		})
	}
	return out
}

func schemaKindToProto(k BotoneraSchemaKind) vectorv1.BotoneraSchemaKind {
	switch k {
	case SchemaDepth:
		return vectorv1.BotoneraSchemaKind_BOTONERA_SCHEMA_DEPTH
	case SchemaCognitive:
		return vectorv1.BotoneraSchemaKind_BOTONERA_SCHEMA_COGNITIVE
	case SchemaEmergency:
		return vectorv1.BotoneraSchemaKind_BOTONERA_SCHEMA_EMERGENCY
	case SchemaCombined:
		return vectorv1.BotoneraSchemaKind_BOTONERA_SCHEMA_COMBINED
	default:
		return vectorv1.BotoneraSchemaKind_BOTONERA_SCHEMA_FLAT
	}
}

func mediaFormatToProto(f MediaFormatType) vectorv1.MediaFormatType {
	switch f {
	case MediaLiveCode:
		return vectorv1.MediaFormatType_MEDIA_FORMAT_LIVE_CODE
	case MediaDiagram:
		return vectorv1.MediaFormatType_MEDIA_FORMAT_DIAGRAM
	case MediaAudioDebate:
		return vectorv1.MediaFormatType_MEDIA_FORMAT_AUDIO_DEBATE
	case MediaInteractiveCanvas:
		return vectorv1.MediaFormatType_MEDIA_FORMAT_INTERACTIVE_CANVAS
	case MediaTextHint:
		return vectorv1.MediaFormatType_MEDIA_FORMAT_TEXT_HINT
	case MediaVideo:
		return vectorv1.MediaFormatType_MEDIA_FORMAT_VIDEO
	default:
		return vectorv1.MediaFormatType_MEDIA_FORMAT_UNSPECIFIED
	}
}
