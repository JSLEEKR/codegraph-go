package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JSLEEKR/codegraph-go/diff"
	"github.com/JSLEEKR/codegraph-go/graph"
)

func TestComputeRiskScoreBasic(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a::foo", Kind: graph.KindFunction, Name: "foo", FilePath: "a"})

	risk := ComputeRiskScore(g, &graph.Node{QualifiedName: "a::foo", Name: "foo"})
	if risk.Total < 0 || risk.Total > 1.0 {
		t.Errorf("risk score out of range: %f", risk.Total)
	}
}

func TestComputeRiskScoreUntested(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a::foo", Kind: graph.KindFunction, Name: "foo", FilePath: "a"})

	risk := ComputeRiskScore(g, &graph.Node{QualifiedName: "a::foo", Name: "foo"})
	if risk.TestFactor != 0.30 {
		t.Errorf("expected test factor 0.30 for untested, got %f", risk.TestFactor)
	}
}

func TestComputeRiskScoreTested(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{QualifiedName: "a::foo", Kind: graph.KindFunction, Name: "foo", FilePath: "a"})
	g.AddEdge(graph.Edge{Kind: graph.EdgeTestedBy, SourceQualified: "a::foo", TargetQualified: "a::test_foo"})

	risk := ComputeRiskScore(g, &graph.Node{QualifiedName: "a::foo", Name: "foo"})
	if risk.TestFactor != 0.05 {
		t.Errorf("expected test factor 0.05 for tested, got %f", risk.TestFactor)
	}
}

func TestComputeRiskScoreSecurityKeyword(t *testing.T) {
	g := graph.New()

	risk := ComputeRiskScore(g, &graph.Node{QualifiedName: "a::authenticate", Name: "authenticate"})
	if risk.SecurityBonus != 0.20 {
		t.Errorf("expected security bonus 0.20 for 'authenticate', got %f", risk.SecurityBonus)
	}
}

func TestComputeRiskScoreNoSecurityKeyword(t *testing.T) {
	g := graph.New()

	risk := ComputeRiskScore(g, &graph.Node{QualifiedName: "a::helper", Name: "helper"})
	if risk.SecurityBonus != 0.0 {
		t.Errorf("expected no security bonus for 'helper', got %f", risk.SecurityBonus)
	}
}

func TestComputeRiskScoreCallerFactor(t *testing.T) {
	g := graph.New()
	for i := 0; i < 10; i++ {
		g.AddEdge(graph.Edge{
			Kind:            graph.EdgeCalls,
			SourceQualified: "a::caller_" + string(rune('A'+i)),
			TargetQualified: "a::target",
		})
	}

	risk := ComputeRiskScore(g, &graph.Node{QualifiedName: "a::target", Name: "target"})
	expected := 10.0 / 20.0 // 0.5 but capped at 0.25
	if risk.CallerFactor != expected && risk.CallerFactor > 0.25 {
		// CallerFactor is min(10/20, 0.25) = 0.25
	}
	if risk.CallerFactor > 0.25 {
		t.Errorf("caller factor should be capped at 0.25, got %f", risk.CallerFactor)
	}
}

func TestComputeRiskScoreCapped(t *testing.T) {
	g := graph.New()
	// Lots of callers + security keyword + untested
	for i := 0; i < 50; i++ {
		g.AddEdge(graph.Edge{
			Kind:            graph.EdgeCalls,
			SourceQualified: "a::c_" + string(rune(i)),
			TargetQualified: "a::auth_handler",
		})
	}

	risk := ComputeRiskScore(g, &graph.Node{QualifiedName: "a::auth_handler", Name: "auth_handler"})
	if risk.Total > 1.0 {
		t.Errorf("total risk should be capped at 1.0, got %f", risk.Total)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		min   int
		max   int
	}{
		{"", 0, 0},
		{"hello", 1, 3},
		{"func main() {\n\treturn\n}", 4, 10},
	}

	for _, tt := range tests {
		got := estimateTokens(tt.input)
		if got < tt.min || got > tt.max {
			t.Errorf("estimateTokens(%q) = %d, expected %d-%d", tt.input, got, tt.min, tt.max)
		}
	}
}

func TestReadSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(path, []byte(content), 0644)

	result := readSource("", path, 2, 4)
	if result != "line2\nline3\nline4" {
		t.Errorf("expected lines 2-4, got %q", result)
	}
}

func TestReadSourceMissing(t *testing.T) {
	result := readSource("", "/nonexistent/file.go", 1, 5)
	if result != "" {
		t.Error("expected empty string for nonexistent file")
	}
}

func TestReadSourceOutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("line1\nline2\n"), 0644)

	result := readSource("", path, 100, 200)
	if result != "" {
		t.Errorf("expected empty string for out of range, got %q", result)
	}
}

func TestAnalyzeChanges(t *testing.T) {
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
	g.AddEdge(graph.Edge{Kind: graph.EdgeCalls, SourceQualified: "main.go::foo", TargetQualified: "main.go::bar"})

	ranges := diff.ChangedRanges{
		"main.go": {{Start: 8, End: 12}},
	}

	result := AnalyzeChanges(g, ranges, "", 8000)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.ChangedNodes) == 0 {
		t.Error("expected changed nodes")
	}
	if result.TokenBudget != 8000 {
		t.Errorf("expected budget 8000, got %d", result.TokenBudget)
	}
}

func TestAnalyzeChangesTestGaps(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{
		QualifiedName: "a.go::untested",
		Kind:          graph.KindFunction,
		Name:          "untested",
		FilePath:      "a.go",
		LineStart:     1,
		LineEnd:       10,
	})
	g.AddNode(graph.Node{
		QualifiedName: "a.go::tested",
		Kind:          graph.KindFunction,
		Name:          "tested",
		FilePath:      "a.go",
		LineStart:     15,
		LineEnd:       25,
	})
	g.AddEdge(graph.Edge{Kind: graph.EdgeTestedBy, SourceQualified: "a.go::tested", TargetQualified: "a.go::test_tested"})

	ranges := diff.ChangedRanges{
		"a.go": {{Start: 1, End: 25}},
	}

	result := AnalyzeChanges(g, ranges, "", 8000)
	// untested should be in test gaps, tested should not
	gapNames := make(map[string]bool)
	for _, n := range result.TestGaps {
		gapNames[n.Name] = true
	}
	if !gapNames["untested"] {
		t.Error("expected 'untested' in test gaps")
	}
	if gapNames["tested"] {
		t.Error("'tested' should not be in test gaps")
	}
}

func TestAnalyzeChangesDefaultBudget(t *testing.T) {
	g := graph.New()
	result := AnalyzeChanges(g, diff.ChangedRanges{}, "", 0)
	if result.TokenBudget != 8000 {
		t.Errorf("expected default budget 8000, got %d", result.TokenBudget)
	}
}

func TestSelectContext(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.Node{
		QualifiedName: "a.go::foo",
		Kind:          graph.KindFunction,
		Name:          "foo",
		FilePath:      "a.go",
		LineStart:     1,
		LineEnd:       5,
	})
	g.AddNode(graph.Node{
		QualifiedName: "a.go::bar",
		Kind:          graph.KindFunction,
		Name:          "bar",
		FilePath:      "a.go",
		LineStart:     10,
		LineEnd:       15,
	})

	entries := SelectContext(g, []string{"a.go::foo", "a.go::bar"}, "", 8000)
	if len(entries) > 2 {
		t.Errorf("expected at most 2 entries, got %d", len(entries))
	}
}

func TestSelectContextUnknownNode(t *testing.T) {
	g := graph.New()
	entries := SelectContext(g, []string{"nonexistent"}, "", 8000)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for unknown node, got %d", len(entries))
	}
}

func TestFormatContext(t *testing.T) {
	entries := []ContextEntry{
		{
			Node: &graph.Node{
				Name:      "foo",
				Kind:      graph.KindFunction,
				FilePath:  "main.go",
				LineStart: 1,
				LineEnd:   10,
			},
			Risk:       RiskScore{Total: 0.5},
			Source:     "func foo() {}",
			TokenCount: 4,
		},
	}

	output := FormatContext(entries)
	if output == "" {
		t.Error("expected non-empty output")
	}
	if !contains(output, "foo") {
		t.Error("expected output to contain 'foo'")
	}
	if !contains(output, "0.50") {
		t.Error("expected output to contain risk score")
	}
}

func TestSecurityKeywords(t *testing.T) {
	keywords := []string{"auth", "password", "token", "encrypt", "sql", "admin"}
	for _, kw := range keywords {
		if !SecurityKeywords[kw] {
			t.Errorf("expected %q to be a security keyword", kw)
		}
	}

	nonKeywords := []string{"hello", "print", "map", "slice"}
	for _, nk := range nonKeywords {
		if SecurityKeywords[nk] {
			t.Errorf("expected %q to NOT be a security keyword", nk)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
