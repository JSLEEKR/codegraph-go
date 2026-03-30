package graph

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Fatal("New() returned nil")
	}
	stats := g.GetStats()
	if stats.TotalNodes != 0 {
		t.Errorf("expected 0 nodes, got %d", stats.TotalNodes)
	}
	if stats.TotalEdges != 0 {
		t.Errorf("expected 0 edges, got %d", stats.TotalEdges)
	}
}

func TestAddNode(t *testing.T) {
	g := New()
	id := g.AddNode(Node{
		Kind:          KindFunction,
		Name:          "foo",
		QualifiedName: "main.go::foo",
		FilePath:      "main.go",
		LineStart:     1,
		LineEnd:       10,
		Language:      "go",
	})
	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}

	n, ok := g.GetNode("main.go::foo")
	if !ok {
		t.Fatal("node not found")
	}
	if n.Name != "foo" {
		t.Errorf("expected name foo, got %s", n.Name)
	}
}

func TestAddNodeUpdate(t *testing.T) {
	g := New()
	g.AddNode(Node{
		Kind:          KindFunction,
		Name:          "foo",
		QualifiedName: "main.go::foo",
		FilePath:      "main.go",
		LineStart:     1,
		LineEnd:       10,
		Language:      "go",
	})

	// Update same node
	id2 := g.AddNode(Node{
		Kind:          KindFunction,
		Name:          "foo",
		QualifiedName: "main.go::foo",
		FilePath:      "main.go",
		LineStart:     5,
		LineEnd:       20,
		Language:      "go",
	})
	if id2 != 1 {
		t.Errorf("expected same id 1, got %d", id2)
	}

	n, _ := g.GetNode("main.go::foo")
	if n.LineStart != 5 || n.LineEnd != 20 {
		t.Errorf("node not updated: lines %d-%d", n.LineStart, n.LineEnd)
	}

	stats := g.GetStats()
	if stats.TotalNodes != 1 {
		t.Errorf("expected 1 node after update, got %d", stats.TotalNodes)
	}
}

func TestAddEdge(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a.go::foo", Kind: KindFunction, Name: "foo", FilePath: "a.go"})
	g.AddNode(Node{QualifiedName: "a.go::bar", Kind: KindFunction, Name: "bar", FilePath: "a.go"})

	id := g.AddEdge(Edge{
		Kind:            EdgeCalls,
		SourceQualified: "a.go::foo",
		TargetQualified: "a.go::bar",
		FilePath:        "a.go",
		Line:            5,
	})
	if id != 1 {
		t.Errorf("expected edge id 1, got %d", id)
	}

	stats := g.GetStats()
	if stats.TotalEdges != 1 {
		t.Errorf("expected 1 edge, got %d", stats.TotalEdges)
	}
}

func TestAddEdgeDeduplicate(t *testing.T) {
	g := New()
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})
	stats := g.GetStats()
	if stats.TotalEdges != 1 {
		t.Errorf("expected 1 edge (deduplicated), got %d", stats.TotalEdges)
	}
}

func TestAddEdgeDifferentKinds(t *testing.T) {
	g := New()
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})
	g.AddEdge(Edge{Kind: EdgeContains, SourceQualified: "a::f", TargetQualified: "a::g"})
	stats := g.GetStats()
	if stats.TotalEdges != 2 {
		t.Errorf("expected 2 edges (different kinds), got %d", stats.TotalEdges)
	}
}

func TestGetNodesByFile(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a.go::foo", Kind: KindFunction, Name: "foo", FilePath: "a.go"})
	g.AddNode(Node{QualifiedName: "a.go::bar", Kind: KindFunction, Name: "bar", FilePath: "a.go"})
	g.AddNode(Node{QualifiedName: "b.go::baz", Kind: KindFunction, Name: "baz", FilePath: "b.go"})

	nodes := g.GetNodesByFile("a.go")
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes in a.go, got %d", len(nodes))
	}

	nodes = g.GetNodesByFile("b.go")
	if len(nodes) != 1 {
		t.Errorf("expected 1 node in b.go, got %d", len(nodes))
	}

	nodes = g.GetNodesByFile("c.go")
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes in c.go, got %d", len(nodes))
	}
}

func TestGetNodesByKind(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a::f", Kind: KindFunction, Name: "f", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::C", Kind: KindClass, Name: "C", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::T", Kind: KindTest, Name: "T", FilePath: "a"})

	funcs := g.GetNodesByKind(KindFunction)
	if len(funcs) != 1 {
		t.Errorf("expected 1 function, got %d", len(funcs))
	}

	both := g.GetNodesByKind(KindFunction, KindTest)
	if len(both) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(both))
	}
}

func TestSearchNodes(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a::ParseFile", Kind: KindFunction, Name: "ParseFile", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::parseGo", Kind: KindFunction, Name: "parseGo", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::something", Kind: KindFunction, Name: "something", FilePath: "a"})

	results := g.SearchNodes("parse", 10)
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'parse', got %d", len(results))
	}
}

func TestSearchNodesLimit(t *testing.T) {
	g := New()
	for i := 0; i < 10; i++ {
		g.AddNode(Node{QualifiedName: "a::func_" + string(rune('a'+i)), Kind: KindFunction, Name: "func_" + string(rune('a'+i)), FilePath: "a"})
	}
	results := g.SearchNodes("func", 3)
	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

func TestGetEdgesBySource(t *testing.T) {
	g := New()
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::h"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::g", TargetQualified: "a::h"})

	edges := g.GetEdgesBySource("a::f")
	if len(edges) != 2 {
		t.Errorf("expected 2 edges from a::f, got %d", len(edges))
	}
}

func TestGetEdgesByTarget(t *testing.T) {
	g := New()
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::h"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::g", TargetQualified: "a::h"})

	edges := g.GetEdgesByTarget("a::h")
	if len(edges) != 2 {
		t.Errorf("expected 2 edges to a::h, got %d", len(edges))
	}
}

func TestRemoveFileData(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a.go::f", Kind: KindFunction, Name: "f", FilePath: "a.go"})
	g.AddNode(Node{QualifiedName: "a.go::g", Kind: KindFunction, Name: "g", FilePath: "a.go"})
	g.AddNode(Node{QualifiedName: "b.go::h", Kind: KindFunction, Name: "h", FilePath: "b.go"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a.go::f", TargetQualified: "a.go::g"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a.go::f", TargetQualified: "b.go::h"})

	g.RemoveFileData("a.go")

	stats := g.GetStats()
	if stats.TotalNodes != 1 {
		t.Errorf("expected 1 node after removal, got %d", stats.TotalNodes)
	}
	if stats.TotalEdges != 0 {
		t.Errorf("expected 0 edges after removal, got %d", stats.TotalEdges)
	}

	_, ok := g.GetNode("a.go::f")
	if ok {
		t.Error("a.go::f should have been removed")
	}

	_, ok = g.GetNode("b.go::h")
	if !ok {
		t.Error("b.go::h should still exist")
	}
}

func TestGetImpactRadius(t *testing.T) {
	g := New()
	// Build a chain: f -> g -> h -> i
	g.AddNode(Node{QualifiedName: "a::f", Kind: KindFunction, Name: "f", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::g", Kind: KindFunction, Name: "g", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::h", Kind: KindFunction, Name: "h", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::i", Kind: KindFunction, Name: "i", FilePath: "a"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::g", TargetQualified: "a::h"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::h", TargetQualified: "a::i"})

	// Impact from f with depth 2 should reach g, h but not i
	nodes := g.GetImpactRadius([]string{"a::f"}, 2, 500)
	names := nodeNames(nodes)
	if !contains(names, "f") || !contains(names, "g") || !contains(names, "h") {
		t.Errorf("expected f, g, h in impact radius, got %v", names)
	}
	if contains(names, "i") {
		t.Error("i should not be in impact radius at depth 2")
	}
}

func TestGetImpactRadiusReverse(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a::f", Kind: KindFunction, Name: "f", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::g", Kind: KindFunction, Name: "g", FilePath: "a"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})

	// Impact from g should also find f (reverse edge)
	nodes := g.GetImpactRadius([]string{"a::g"}, 1, 500)
	names := nodeNames(nodes)
	if !contains(names, "f") {
		t.Error("expected f in reverse impact from g")
	}
}

func TestGetImpactRadiusMaxNodes(t *testing.T) {
	g := New()
	// Add many nodes connected to a hub
	g.AddNode(Node{QualifiedName: "hub", Kind: KindFunction, Name: "hub", FilePath: "a"})
	for i := 0; i < 20; i++ {
		qn := "a::n" + string(rune('A'+i))
		g.AddNode(Node{QualifiedName: qn, Kind: KindFunction, Name: "n" + string(rune('A'+i)), FilePath: "a"})
		g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "hub", TargetQualified: qn})
	}

	nodes := g.GetImpactRadius([]string{"hub"}, 2, 5)
	if len(nodes) > 5 {
		t.Errorf("expected at most 5 nodes, got %d", len(nodes))
	}
}

func TestGetCallerCount(t *testing.T) {
	g := New()
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::target"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::g", TargetQualified: "a::target"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::h", TargetQualified: "a::target"})

	count := g.GetCallerCount("a::target")
	if count != 3 {
		t.Errorf("expected 3 callers, got %d", count)
	}
}

func TestHasTestCoverage(t *testing.T) {
	g := New()
	g.AddEdge(Edge{Kind: EdgeTestedBy, SourceQualified: "a::foo", TargetQualified: "a::test_foo"})

	if !g.HasTestCoverage("a::foo") {
		t.Error("expected foo to have test coverage")
	}
	if g.HasTestCoverage("a::bar") {
		t.Error("expected bar to not have test coverage")
	}
}

func TestAllNodes(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a::f", Kind: KindFunction, Name: "f", FilePath: "a"})
	g.AddNode(Node{QualifiedName: "a::g", Kind: KindFunction, Name: "g", FilePath: "a"})

	nodes := g.AllNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestAllEdges(t *testing.T) {
	g := New()
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g"})
	g.AddEdge(Edge{Kind: EdgeContains, SourceQualified: "a", TargetQualified: "a::f"})

	edges := g.AllEdges()
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}

func TestGetStats(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a::f", Kind: KindFunction, Name: "f", FilePath: "a", Language: "go"})
	g.AddNode(Node{QualifiedName: "b::g", Kind: KindClass, Name: "g", FilePath: "b", Language: "python"})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "b::g"})

	stats := g.GetStats()
	if stats.TotalNodes != 2 {
		t.Errorf("expected 2 nodes, got %d", stats.TotalNodes)
	}
	if stats.TotalEdges != 1 {
		t.Errorf("expected 1 edge, got %d", stats.TotalEdges)
	}
	if len(stats.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d", len(stats.Languages))
	}
	if stats.NodesByKind["Function"] != 1 {
		t.Errorf("expected 1 Function node, got %d", stats.NodesByKind["Function"])
	}
	if stats.EdgesByKind["CALLS"] != 1 {
		t.Errorf("expected 1 CALLS edge, got %d", stats.EdgesByKind["CALLS"])
	}
}

func TestAllFiles(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a::f", FilePath: "a.go", Kind: KindFunction, Name: "f"})
	g.AddNode(Node{QualifiedName: "b::g", FilePath: "b.go", Kind: KindFunction, Name: "g"})
	g.AddNode(Node{QualifiedName: "a::h", FilePath: "a.go", Kind: KindFunction, Name: "h"})

	files := g.AllFiles()
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_graph.json")

	g := New()
	g.AddNode(Node{QualifiedName: "a::f", Kind: KindFunction, Name: "f", FilePath: "a", Language: "go", LineStart: 1, LineEnd: 10})
	g.AddNode(Node{QualifiedName: "a::g", Kind: KindFunction, Name: "g", FilePath: "a", Language: "go", LineStart: 12, LineEnd: 20})
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g", FilePath: "a", Line: 5})

	if err := g.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	g2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stats := g2.GetStats()
	if stats.TotalNodes != 2 {
		t.Errorf("loaded graph: expected 2 nodes, got %d", stats.TotalNodes)
	}
	if stats.TotalEdges != 1 {
		t.Errorf("loaded graph: expected 1 edge, got %d", stats.TotalEdges)
	}

	n, ok := g2.GetNode("a::f")
	if !ok {
		t.Fatal("loaded graph: node a::f not found")
	}
	if n.LineStart != 1 || n.LineEnd != 10 {
		t.Errorf("loaded graph: wrong line range: %d-%d", n.LineStart, n.LineEnd)
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/graph.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetNodeNotFound(t *testing.T) {
	g := New()
	_, ok := g.GetNode("nonexistent")
	if ok {
		t.Error("expected false for nonexistent node")
	}
}

func TestNodeCopyIsolation(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a::f", Kind: KindFunction, Name: "f", FilePath: "a"})

	n, _ := g.GetNode("a::f")
	n.Name = "modified"

	n2, _ := g.GetNode("a::f")
	if n2.Name != "f" {
		t.Error("modification of returned node should not affect graph")
	}
}

func TestEdgeCopyIsolation(t *testing.T) {
	g := New()
	g.AddEdge(Edge{Kind: EdgeCalls, SourceQualified: "a::f", TargetQualified: "a::g", Line: 5})

	edges := g.GetEdgesBySource("a::f")
	edges[0].Line = 999

	edges2 := g.GetEdgesBySource("a::f")
	if edges2[0].Line != 5 {
		t.Error("modification of returned edge should not affect graph")
	}
}

func TestGetImpactRadiusUnknownNode(t *testing.T) {
	g := New()
	nodes := g.GetImpactRadius([]string{"nonexistent"}, 2, 500)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for unknown QN, got %d", len(nodes))
	}
}

func TestGetImpactRadiusDefaults(t *testing.T) {
	g := New()
	g.AddNode(Node{QualifiedName: "a::f", Kind: KindFunction, Name: "f", FilePath: "a"})
	// Should use defaults for 0 values
	nodes := g.GetImpactRadius([]string{"a::f"}, 0, 0)
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
}

// helpers

func nodeNames(nodes []*Node) []string {
	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	sort.Strings(names)
	return names
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
