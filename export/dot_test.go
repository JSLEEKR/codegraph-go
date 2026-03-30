package export

import (
	"strings"
	"testing"

	"github.com/JSLEEKR/codegraph-go/graph"
)

func TestToDOTEmpty(t *testing.T) {
	g := graph.New()
	dot := ToDOT(g, "TestGraph")
	if !strings.Contains(dot, "digraph") {
		t.Error("expected digraph keyword")
	}
	if !strings.Contains(dot, "TestGraph") {
		t.Error("expected graph title")
	}
}

func TestToDOTWithNodes(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a::foo", Kind: graph.KindFunction, Name: "foo", FilePath: "a"})
	g.AddNode(graph.Node{QualifiedName: "a::Bar", Kind: graph.KindClass, Name: "Bar", FilePath: "a"})

	dot := ToDOT(g, "")
	if !strings.Contains(dot, "foo") {
		t.Error("expected node label foo")
	}
	if !strings.Contains(dot, "Bar") {
		t.Error("expected node label Bar")
	}
	if !strings.Contains(dot, "ellipse") {
		t.Error("expected function shape ellipse")
	}
	if !strings.Contains(dot, "box") {
		t.Error("expected class shape box")
	}
}

func TestToDOTWithEdges(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a::f", Kind: graph.KindFunction, Name: "f", FilePath: "a"})
	g.AddNode(graph.Node{QualifiedName: "a::g", Kind: graph.KindFunction, Name: "g", FilePath: "a"})
	g.AddEdge(graph.Edge{Kind: graph.EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})

	dot := ToDOT(g, "test")
	if !strings.Contains(dot, "->") {
		t.Error("expected edge arrow")
	}
	if !strings.Contains(dot, "CALLS") {
		t.Error("expected CALLS label")
	}
}

func TestToDOTDefaultTitle(t *testing.T) {
	g := graph.New()
	dot := ToDOT(g, "")
	if !strings.Contains(dot, "CodeGraph") {
		t.Error("expected default title CodeGraph")
	}
}

func TestToDOTLegend(t *testing.T) {
	g := graph.New()
	dot := ToDOT(g, "test")
	if !strings.Contains(dot, "Legend") {
		t.Error("expected legend subgraph")
	}
}

func TestToDOTAllNodeKinds(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "f", Kind: graph.KindFile, Name: "f", FilePath: "f"})
	g.AddNode(graph.Node{QualifiedName: "c", Kind: graph.KindClass, Name: "c", FilePath: "c"})
	g.AddNode(graph.Node{QualifiedName: "fn", Kind: graph.KindFunction, Name: "fn", FilePath: "fn"})
	g.AddNode(graph.Node{QualifiedName: "ty", Kind: graph.KindType, Name: "ty", FilePath: "ty"})
	g.AddNode(graph.Node{QualifiedName: "te", Kind: graph.KindTest, Name: "te", FilePath: "te"})

	dot := ToDOT(g, "test")
	if !strings.Contains(dot, "folder") {
		t.Error("expected folder shape for File")
	}
	if !strings.Contains(dot, "hexagon") {
		t.Error("expected hexagon shape for Type")
	}
	if !strings.Contains(dot, "diamond") {
		t.Error("expected diamond shape for Test")
	}
}

func TestToFilteredDOT(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a::f", Kind: graph.KindFunction, Name: "f", FilePath: "a"})
	g.AddNode(graph.Node{QualifiedName: "a::g", Kind: graph.KindFunction, Name: "g", FilePath: "a"})
	g.AddNode(graph.Node{QualifiedName: "a::h", Kind: graph.KindFunction, Name: "h", FilePath: "a"})
	g.AddEdge(graph.Edge{Kind: graph.EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})
	g.AddEdge(graph.Edge{Kind: graph.EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::h"})

	dot := ToFilteredDOT(g, []string{"a::f", "a::g"}, "filtered")
	if !strings.Contains(dot, "f") {
		t.Error("expected f in filtered graph")
	}
	if !strings.Contains(dot, "g") {
		t.Error("expected g in filtered graph")
	}
	// h should not have a node definition (but might appear in edge)
}

func TestToFilteredDOTDefaultTitle(t *testing.T) {
	g := graph.New()
	dot := ToFilteredDOT(g, nil, "")
	if !strings.Contains(dot, "filtered") {
		t.Error("expected default filtered title")
	}
}

func TestEscapeLabel(t *testing.T) {
	tests := []struct {
		input string
		check string // substring that should appear
	}{
		{"hello", "hello"},
		{`say "hi"`, `say`},
		{"line1\nline2", "line1"},
		{strings.Repeat("a", 50), "..."}, // truncated
	}

	for _, tt := range tests {
		result := escapeLabel(tt.input)
		if !strings.Contains(result, tt.check) {
			t.Errorf("escapeLabel(%q) = %q, expected to contain %q", tt.input, result, tt.check)
		}
	}
}

func TestToDOTAllEdgeKinds(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a", Kind: graph.KindFunction, Name: "a", FilePath: "a"})
	g.AddNode(graph.Node{QualifiedName: "b", Kind: graph.KindFunction, Name: "b", FilePath: "b"})

	kinds := []graph.EdgeKind{
		graph.EdgeCalls, graph.EdgeImportsFrom, graph.EdgeInherits,
		graph.EdgeImplements, graph.EdgeContains, graph.EdgeTestedBy, graph.EdgeDependsOn,
	}

	for _, k := range kinds {
		g2 := graph.New()
		g2.AddNode(graph.Node{QualifiedName: "a", Kind: graph.KindFunction, Name: "a", FilePath: "a"})
		g2.AddNode(graph.Node{QualifiedName: "b", Kind: graph.KindFunction, Name: "b", FilePath: "b"})
		g2.AddEdge(graph.Edge{Kind: k, SourceQualified: "a", TargetQualified: "b"})

		dot := ToDOT(g2, "test")
		if !strings.Contains(dot, string(k)) {
			t.Errorf("expected edge kind %s in DOT output", k)
		}
	}
}
