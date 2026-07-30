package rag_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/rag"
)

func TestNormalizeForEmbed(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "  VARIABLES  ", want: "variables"},
		{input: "ámbito qué", want: "ambito que"},
		{input: "vaaariables y scoope", want: "variables y scope"},
		{input: "archivo .env 2026", want: "archivo .env 2026"},
	}
	for _, tt := range tests {
		if got := rag.NormalizeForEmbed(tt.input); got != tt.want {
			t.Errorf("NormalizeForEmbed(%q)=%q want=%q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeForEmbedIsIdempotent(t *testing.T) {
	once := rag.NormalizeForEmbed("  Quééé VARIABLES  ")
	if twice := rag.NormalizeForEmbed(once); twice != once {
		t.Fatalf("once=%q twice=%q", once, twice)
	}
}

func TestHashEmbedderNormalizesSymmetrically(t *testing.T) {
	emb := rag.NewHashEmbedder(64)
	query, err := emb.Embed(context.Background(), "VAAARIABLES y SCOOPE")
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := emb.Embed(context.Background(), "variables y scope")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(query, descriptor) {
		t.Fatal("normalized query and descriptor embeddings differ")
	}
}
