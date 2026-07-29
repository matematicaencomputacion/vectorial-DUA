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

// NodeSubtopicProgress is the aggregate state of any node in the hierarchy.
type NodeSubtopicProgress struct {
	SubtopicID      string
	Title           string
	State           SubtopicProgressState
	OpenedInSubtree int
	TotalInSubtree  int
}

// TreeProgress is a stable snapshot of progress through a hierarchy.
type TreeProgress struct {
	OpenedSubtopicIDs []string
	TotalSubtopics    int
	RootStates        []RootSubtopicProgress
	NodeStates        []NodeSubtopicProgress
}

// ProgressForTree aggregates an opened-id set without reading or mutating a
// store. Unknown opened ids are ignored; output ids follow hierarchy pre-order.
func ProgressForTree(tree *DUAHierarchicalTree, openedSet map[string]struct{}) TreeProgress {
	progress := TreeProgress{
		OpenedSubtopicIDs: []string{},
		RootStates:        []RootSubtopicProgress{},
		NodeStates:        []NodeSubtopicProgress{},
	}
	if tree == nil {
		return progress
	}

	for i := range tree.Subtopics {
		root := &tree.Subtopics[i]
		total, opened := progressInSubtree(root, openedSet, &progress.OpenedSubtopicIDs, &progress.NodeStates)
		progress.TotalSubtopics += total

		progress.RootStates = append(progress.RootStates, RootSubtopicProgress{
			SubtopicID: root.SubtopicID,
			Title:      root.Title,
			State:      progressState(total, opened),
		})
	}
	return progress
}

func progressInSubtree(
	node *SubtopicNode,
	openedSet map[string]struct{},
	openedIDs *[]string,
	nodeStates *[]NodeSubtopicProgress,
) (total, opened int) {
	if node == nil {
		return 0, 0
	}
	stateIndex := len(*nodeStates)
	*nodeStates = append(*nodeStates, NodeSubtopicProgress{
		SubtopicID: node.SubtopicID,
		Title:      node.Title,
	})
	total = 1
	if _, ok := openedSet[node.SubtopicID]; ok {
		opened = 1
		*openedIDs = append(*openedIDs, node.SubtopicID)
	}
	for i := range node.ChildSubtopics {
		childTotal, childOpened := progressInSubtree(
			&node.ChildSubtopics[i],
			openedSet,
			openedIDs,
			nodeStates,
		)
		total += childTotal
		opened += childOpened
	}
	(*nodeStates)[stateIndex].State = progressState(total, opened)
	(*nodeStates)[stateIndex].OpenedInSubtree = opened
	(*nodeStates)[stateIndex].TotalInSubtree = total
	return total, opened
}

func progressState(total, opened int) SubtopicProgressState {
	switch {
	case total > 0 && opened == total:
		return ProgressVisited
	case opened > 0:
		return ProgressPartial
	default:
		return ProgressUnvisited
	}
}
