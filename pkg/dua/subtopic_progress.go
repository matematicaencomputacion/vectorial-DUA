package dua

// SubtopicProgressState describes how much of a root branch the student opened.
type SubtopicProgressState string

const (
	ProgressVisited   SubtopicProgressState = "visited"
	ProgressPartial   SubtopicProgressState = "partial"
	ProgressUnvisited SubtopicProgressState = "unvisited"
)

// RootSubtopicProgress is the aggregate state of one root branch.
type RootSubtopicProgress struct {
	SubtopicID string
	Title      string
	State      SubtopicProgressState
}

// TreeProgress is a stable snapshot of progress through a hierarchy.
type TreeProgress struct {
	OpenedSubtopicIDs []string
	TotalSubtopics    int
	RootStates        []RootSubtopicProgress
}

// ProgressForTree aggregates an opened-id set without reading or mutating a
// store. Unknown opened ids are ignored; output ids follow hierarchy pre-order.
func ProgressForTree(tree *DUAHierarchicalTree, openedSet map[string]struct{}) TreeProgress {
	progress := TreeProgress{
		OpenedSubtopicIDs: []string{},
		RootStates:        []RootSubtopicProgress{},
	}
	if tree == nil {
		return progress
	}

	for i := range tree.Subtopics {
		root := &tree.Subtopics[i]
		total, opened := progressInSubtree(root, openedSet, &progress.OpenedSubtopicIDs)
		progress.TotalSubtopics += total

		state := ProgressUnvisited
		switch {
		case total > 0 && opened == total:
			state = ProgressVisited
		case opened > 0:
			state = ProgressPartial
		}
		progress.RootStates = append(progress.RootStates, RootSubtopicProgress{
			SubtopicID: root.SubtopicID,
			Title:      root.Title,
			State:      state,
		})
	}
	return progress
}

func progressInSubtree(node *SubtopicNode, openedSet map[string]struct{}, openedIDs *[]string) (total, opened int) {
	if node == nil {
		return 0, 0
	}
	total = 1
	if _, ok := openedSet[node.SubtopicID]; ok {
		opened = 1
		*openedIDs = append(*openedIDs, node.SubtopicID)
	}
	for i := range node.ChildSubtopics {
		childTotal, childOpened := progressInSubtree(&node.ChildSubtopics[i], openedSet, openedIDs)
		total += childTotal
		opened += childOpened
	}
	return total, opened
}
