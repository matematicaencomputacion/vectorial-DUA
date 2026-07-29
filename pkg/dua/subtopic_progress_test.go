package dua_test

import (
	"reflect"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

func progressTestTree() *dua.DUAHierarchicalTree {
	return &dua.DUAHierarchicalTree{
		MainTopicTitle: "Tema",
		MacroMediaURL:  "https://example.test/macro",
		Subtopics: []dua.SubtopicNode{
			{
				SubtopicID: "root_a",
				Title:      "Raíz A",
				MediaURL:   "https://example.test/a",
				ChildSubtopics: []dua.SubtopicNode{
					{SubtopicID: "a_1", Title: "A 1", MediaURL: "https://example.test/a1"},
					{SubtopicID: "a_2", Title: "A 2", MediaURL: "https://example.test/a2"},
				},
			},
			{SubtopicID: "root_b", Title: "Raíz B", MediaURL: "https://example.test/b"},
		},
	}
}

func TestProgressForTreeVisitedPartialAndUnvisited(t *testing.T) {
	tests := []struct {
		name    string
		opened  map[string]struct{}
		wantA   dua.SubtopicProgressState
		wantB   dua.SubtopicProgressState
		wantIDs []string
	}{
		{
			name:    "unvisited",
			opened:  map[string]struct{}{},
			wantA:   dua.ProgressUnvisited,
			wantB:   dua.ProgressUnvisited,
			wantIDs: []string{},
		},
		{
			name:    "partial descendant",
			opened:  map[string]struct{}{"a_1": {}},
			wantA:   dua.ProgressPartial,
			wantB:   dua.ProgressUnvisited,
			wantIDs: []string{"a_1"},
		},
		{
			name: "visited whole branch",
			opened: map[string]struct{}{
				"root_a": {},
				"a_1":    {},
				"a_2":    {},
			},
			wantA:   dua.ProgressVisited,
			wantB:   dua.ProgressUnvisited,
			wantIDs: []string{"root_a", "a_1", "a_2"},
		},
		{
			name:    "visited leaf root",
			opened:  map[string]struct{}{"root_b": {}},
			wantA:   dua.ProgressUnvisited,
			wantB:   dua.ProgressVisited,
			wantIDs: []string{"root_b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dua.ProgressForTree(progressTestTree(), tt.opened)
			if got.TotalSubtopics != 4 {
				t.Fatalf("TotalSubtopics=%d want=4", got.TotalSubtopics)
			}
			if len(got.RootStates) != 2 {
				t.Fatalf("RootStates=%+v", got.RootStates)
			}
			if got.RootStates[0].State != tt.wantA || got.RootStates[1].State != tt.wantB {
				t.Fatalf("states=%+v want=(%s,%s)", got.RootStates, tt.wantA, tt.wantB)
			}
			if !reflect.DeepEqual(got.OpenedSubtopicIDs, tt.wantIDs) {
				t.Fatalf("OpenedSubtopicIDs=%v want=%v", got.OpenedSubtopicIDs, tt.wantIDs)
			}
		})
	}
}

func TestProgressForTreeIgnoresUnknownOpenedIDs(t *testing.T) {
	got := dua.ProgressForTree(progressTestTree(), map[string]struct{}{"deleted_subtopic": {}})
	if got.TotalSubtopics != 4 || len(got.OpenedSubtopicIDs) != 0 {
		t.Fatalf("got=%+v", got)
	}
	for _, root := range got.RootStates {
		if root.State != dua.ProgressUnvisited {
			t.Fatalf("unknown id changed root state: %+v", root)
		}
	}
}

func TestProgressForAutomovilSeedNestedTree(t *testing.T) {
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	node, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FC0")
	if !ok || node.Hierarchy == nil {
		t.Fatal("automovil hierarchy seed missing")
	}

	got := dua.ProgressForTree(node.Hierarchy, map[string]struct{}{
		"sub_motor":    {},
		"sub_4_ruedas": {},
	})
	if got.TotalSubtopics != 5 {
		t.Fatalf("TotalSubtopics=%d want=5", got.TotalSubtopics)
	}
	if want := []string{"sub_motor", "sub_4_ruedas"}; !reflect.DeepEqual(got.OpenedSubtopicIDs, want) {
		t.Fatalf("OpenedSubtopicIDs=%v want=%v", got.OpenedSubtopicIDs, want)
	}
	if len(got.RootStates) != 2 {
		t.Fatalf("RootStates=%+v", got.RootStates)
	}
	if got.RootStates[0].SubtopicID != "sub_caja_central" || got.RootStates[0].State != dua.ProgressPartial {
		t.Fatalf("caja central=%+v want partial", got.RootStates[0])
	}
	if got.RootStates[1].SubtopicID != "sub_4_ruedas" || got.RootStates[1].State != dua.ProgressVisited {
		t.Fatalf("ruedas=%+v want visited", got.RootStates[1])
	}
}
