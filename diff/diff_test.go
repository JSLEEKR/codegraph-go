package diff

import (
	"testing"

	"github.com/JSLEEKR/codegraph-go/graph"
)

func TestParseUnifiedDiffBasic(t *testing.T) {
	diffText := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -10,3 +10,5 @@ func main() {
`
	ranges := ParseUnifiedDiff(diffText)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 file, got %d", len(ranges))
	}
	lr := ranges["main.go"]
	if len(lr) != 1 {
		t.Fatalf("expected 1 range, got %d", len(lr))
	}
	if lr[0].Start != 10 || lr[0].End != 14 {
		t.Errorf("expected range 10-14, got %d-%d", lr[0].Start, lr[0].End)
	}
}

func TestParseUnifiedDiffMultipleHunks(t *testing.T) {
	diffText := `diff --git a/util.go b/util.go
--- a/util.go
+++ b/util.go
@@ -5,2 +5,3 @@ func helper() {
@@ -20,1 +21,4 @@ func another() {
`
	ranges := ParseUnifiedDiff(diffText)
	lr := ranges["util.go"]
	if len(lr) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(lr))
	}
	if lr[0].Start != 5 || lr[0].End != 7 {
		t.Errorf("hunk 1: expected 5-7, got %d-%d", lr[0].Start, lr[0].End)
	}
	if lr[1].Start != 21 || lr[1].End != 24 {
		t.Errorf("hunk 2: expected 21-24, got %d-%d", lr[1].Start, lr[1].End)
	}
}

func TestParseUnifiedDiffMultipleFiles(t *testing.T) {
	diffText := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,2 @@
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -10,1 +10,1 @@
`
	ranges := ParseUnifiedDiff(diffText)
	if len(ranges) != 2 {
		t.Errorf("expected 2 files, got %d", len(ranges))
	}
	if _, ok := ranges["a.go"]; !ok {
		t.Error("a.go not found in ranges")
	}
	if _, ok := ranges["b.go"]; !ok {
		t.Error("b.go not found in ranges")
	}
}

func TestParseUnifiedDiffDeletion(t *testing.T) {
	diffText := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -5,3 +5,0 @@ func old() {
`
	ranges := ParseUnifiedDiff(diffText)
	lr := ranges["main.go"]
	if len(lr) != 1 {
		t.Fatalf("expected 1 range, got %d", len(lr))
	}
	// Zero count should be treated as single line
	if lr[0].Start != 5 || lr[0].End != 5 {
		t.Errorf("expected range 5-5, got %d-%d", lr[0].Start, lr[0].End)
	}
}

func TestParseUnifiedDiffSingleLine(t *testing.T) {
	diffText := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1 @@
`
	ranges := ParseUnifiedDiff(diffText)
	lr := ranges["main.go"]
	if len(lr) != 1 {
		t.Fatalf("expected 1 range, got %d", len(lr))
	}
	if lr[0].Start != 1 || lr[0].End != 1 {
		t.Errorf("expected range 1-1, got %d-%d", lr[0].Start, lr[0].End)
	}
}

func TestParseUnifiedDiffEmpty(t *testing.T) {
	ranges := ParseUnifiedDiff("")
	if len(ranges) != 0 {
		t.Errorf("expected 0 files, got %d", len(ranges))
	}
}

func TestMapChangesToNodes(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{
		QualifiedName: "main.go",
		Kind:          graph.KindFile,
		Name:          "main.go",
		FilePath:      "main.go",
		LineStart:     1,
		LineEnd:       50,
	})
	g.AddNode(graph.Node{
		QualifiedName: "main.go::foo",
		Kind:          graph.KindFunction,
		Name:          "foo",
		FilePath:      "main.go",
		LineStart:     5,
		LineEnd:       15,
	})
	g.AddNode(graph.Node{
		QualifiedName: "main.go::bar",
		Kind:          graph.KindFunction,
		Name:          "bar",
		FilePath:      "main.go",
		LineStart:     20,
		LineEnd:       30,
	})
	g.AddNode(graph.Node{
		QualifiedName: "main.go::baz",
		Kind:          graph.KindFunction,
		Name:          "baz",
		FilePath:      "main.go",
		LineStart:     35,
		LineEnd:       45,
	})

	// Change affects lines 10-12 (inside foo) and 22-25 (inside bar)
	ranges := ChangedRanges{
		"main.go": {
			{Start: 10, End: 12},
			{Start: 22, End: 25},
		},
	}

	nodes := MapChangesToNodes(g, ranges)
	if len(nodes) != 2 {
		t.Errorf("expected 2 affected nodes, got %d", len(nodes))
	}

	names := make(map[string]bool)
	for _, n := range nodes {
		names[n.Name] = true
	}
	if !names["foo"] {
		t.Error("expected foo to be affected")
	}
	if !names["bar"] {
		t.Error("expected bar to be affected")
	}
	if names["baz"] {
		t.Error("baz should not be affected")
	}
}

func TestMapChangesToNodesNoOverlap(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{
		QualifiedName: "a.go::foo",
		Kind:          graph.KindFunction,
		Name:          "foo",
		FilePath:      "a.go",
		LineStart:     5,
		LineEnd:       10,
	})

	ranges := ChangedRanges{
		"a.go": {{Start: 20, End: 25}},
	}

	nodes := MapChangesToNodes(g, ranges)
	if len(nodes) != 0 {
		t.Errorf("expected 0 affected nodes, got %d", len(nodes))
	}
}

func TestMapChangesToNodesDeduplicate(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{
		QualifiedName: "a.go::foo",
		Kind:          graph.KindFunction,
		Name:          "foo",
		FilePath:      "a.go",
		LineStart:     5,
		LineEnd:       20,
	})

	// Two ranges both inside foo
	ranges := ChangedRanges{
		"a.go": {
			{Start: 7, End: 8},
			{Start: 15, End: 16},
		},
	}

	nodes := MapChangesToNodes(g, ranges)
	if len(nodes) != 1 {
		t.Errorf("expected 1 node (deduplicated), got %d", len(nodes))
	}
}

func TestMapChangesToNodesSkipsFileNodes(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{
		QualifiedName: "a.go",
		Kind:          graph.KindFile,
		Name:          "a.go",
		FilePath:      "a.go",
		LineStart:     1,
		LineEnd:       100,
	})

	ranges := ChangedRanges{
		"a.go": {{Start: 1, End: 50}},
	}

	nodes := MapChangesToNodes(g, ranges)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes (file nodes skipped), got %d", len(nodes))
	}
}

func TestGetChangedFiles(t *testing.T) {
	ranges := ChangedRanges{
		"a.go": {{Start: 1, End: 5}},
		"b.go": {{Start: 10, End: 15}},
	}

	files := GetChangedFiles(ranges)
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestSafeRefValidation(t *testing.T) {
	tests := []struct {
		ref  string
		safe bool
	}{
		{"HEAD~1", true},
		{"main", true},
		{"origin/main", true},
		{"v1.0.0", true},
		{"abc123", true},
		{"HEAD^", true},
		{"HEAD@{1}", true},
		{"; rm -rf /", false},
		{"$(evil)", false},
		{"`whoami`", false},
		{"HEAD && echo pwned", false},
	}

	for _, tt := range tests {
		got := safeRef.MatchString(tt.ref)
		if got != tt.safe {
			t.Errorf("safeRef(%q) = %v, want %v", tt.ref, got, tt.safe)
		}
	}
}

func TestMapChangesToNodesUnknownFile(t *testing.T) {
	g := graph.New()
	ranges := ChangedRanges{
		"nonexistent.go": {{Start: 1, End: 10}},
	}

	nodes := MapChangesToNodes(g, ranges)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for unknown file, got %d", len(nodes))
	}
}

func TestParseUnifiedDiffRealWorld(t *testing.T) {
	diffText := `diff --git a/graph/graph.go b/graph/graph.go
index abc123..def456 100644
--- a/graph/graph.go
+++ b/graph/graph.go
@@ -45,6 +45,8 @@ type Graph struct {
+	// new field
+	version int
@@ -100,3 +102,7 @@ func (g *Graph) AddNode(n Node) int {
+	// validation
+	if n.Name == "" {
+		return -1
+	}
diff --git a/parser/parser.go b/parser/parser.go
index 111222..333444 100644
--- a/parser/parser.go
+++ b/parser/parser.go
@@ -10,1 +10,3 @@ func ParseFile(path string) {
+	// new code
+	validate(path)
`
	ranges := ParseUnifiedDiff(diffText)
	if len(ranges) != 2 {
		t.Fatalf("expected 2 files, got %d", len(ranges))
	}

	graphRanges := ranges["graph/graph.go"]
	if len(graphRanges) != 2 {
		t.Errorf("expected 2 hunks for graph.go, got %d", len(graphRanges))
	}

	parserRanges := ranges["parser/parser.go"]
	if len(parserRanges) != 1 {
		t.Errorf("expected 1 hunk for parser.go, got %d", len(parserRanges))
	}
}
