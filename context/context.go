// Package context provides token-budget-aware context selection from a code graph.
package context

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JSLEEKR/codegraph-go/diff"
	"github.com/JSLEEKR/codegraph-go/graph"
)

// SecurityKeywords are terms that increase risk score for changed code.
var SecurityKeywords = map[string]bool{
	"auth": true, "login": true, "password": true, "token": true, "session": true,
	"crypt": true, "secret": true, "encrypt": true, "decrypt": true, "hash": true,
	"sign": true, "verify": true, "sql": true, "query": true, "execute": true,
	"connect": true, "socket": true, "request": true, "http": true,
	"sanitize": true, "validate": true, "admin": true, "privilege": true,
	"credential": true, "permission": true, "key": true, "cert": true,
	"cookie": true, "csrf": true, "xss": true, "injection": true,
}

// RiskScore holds the computed risk score and its breakdown.
type RiskScore struct {
	Total         float64 `json:"total"`
	CallerFactor  float64 `json:"caller_factor"`
	TestFactor    float64 `json:"test_factor"`
	SecurityBonus float64 `json:"security_bonus"`
}

// ContextEntry represents a piece of code context selected for review.
type ContextEntry struct {
	Node       *graph.Node `json:"node"`
	Risk       RiskScore   `json:"risk"`
	Source     string      `json:"source,omitempty"`
	TokenCount int         `json:"token_count"`
}

// AnalysisResult holds the full analysis of code changes.
type AnalysisResult struct {
	ChangedNodes   []*graph.Node  `json:"changed_nodes"`
	ImpactedNodes  []*graph.Node  `json:"impacted_nodes"`
	TestGaps       []*graph.Node  `json:"test_gaps"`
	ReviewPriority []ContextEntry `json:"review_priority"`
	TokensUsed     int            `json:"tokens_used"`
	TokenBudget    int            `json:"token_budget"`
}

// ComputeRiskScore calculates a risk score for a node (0.0 to 1.0).
func ComputeRiskScore(g *graph.Graph, node *graph.Node) RiskScore {
	var score RiskScore

	// Caller count factor: more callers = higher blast radius
	callerCount := g.GetCallerCount(node.QualifiedName)
	score.CallerFactor = math.Min(float64(callerCount)/20.0, 0.25)

	// Test coverage factor
	if g.HasTestCoverage(node.QualifiedName) {
		score.TestFactor = 0.05
	} else {
		score.TestFactor = 0.30
	}

	// Security keyword bonus
	nameLower := strings.ToLower(node.Name)
	qnLower := strings.ToLower(node.QualifiedName)
	for keyword := range SecurityKeywords {
		if strings.Contains(nameLower, keyword) || strings.Contains(qnLower, keyword) {
			score.SecurityBonus = 0.20
			break
		}
	}

	score.Total = math.Min(score.CallerFactor+score.TestFactor+score.SecurityBonus, 1.0)
	return score
}

// AnalyzeChanges performs full impact analysis given changed ranges.
func AnalyzeChanges(g *graph.Graph, ranges diff.ChangedRanges, repoRoot string, tokenBudget int) *AnalysisResult {
	if tokenBudget <= 0 {
		tokenBudget = 8000
	}

	// Map changes to nodes
	changedNodes := diff.MapChangesToNodes(g, ranges)

	// Filter to meaningful kinds
	var meaningful []*graph.Node
	for _, n := range changedNodes {
		if n.Kind == graph.KindFunction || n.Kind == graph.KindTest ||
			n.Kind == graph.KindClass || n.Kind == graph.KindType {
			meaningful = append(meaningful, n)
		}
	}

	// Get impact radius
	var changedQNs []string
	for _, n := range meaningful {
		changedQNs = append(changedQNs, n.QualifiedName)
	}
	impactedNodes := g.GetImpactRadius(changedQNs, 2, 500)

	// Find test gaps
	var testGaps []*graph.Node
	for _, n := range meaningful {
		if n.Kind == graph.KindFunction && !n.IsTest {
			if !g.HasTestCoverage(n.QualifiedName) {
				testGaps = append(testGaps, n)
			}
		}
	}

	// Compute risk scores and build context entries
	var entries []ContextEntry
	for _, n := range meaningful {
		risk := ComputeRiskScore(g, n)
		source := readSource(repoRoot, n.FilePath, n.LineStart, n.LineEnd)
		tokens := estimateTokens(source)

		entries = append(entries, ContextEntry{
			Node:       n,
			Risk:       risk,
			Source:     source,
			TokenCount: tokens,
		})
	}

	// Sort by risk score descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Risk.Total > entries[j].Risk.Total
	})

	// Select entries within token budget (greedy knapsack)
	var selected []ContextEntry
	tokensUsed := 0
	for _, entry := range entries {
		if tokensUsed+entry.TokenCount <= tokenBudget {
			selected = append(selected, entry)
			tokensUsed += entry.TokenCount
		}
	}

	return &AnalysisResult{
		ChangedNodes:   changedNodes,
		ImpactedNodes:  impactedNodes,
		TestGaps:       testGaps,
		ReviewPriority: selected,
		TokensUsed:     tokensUsed,
		TokenBudget:    tokenBudget,
	}
}

// SelectContext selects the most relevant code context within a token budget.
func SelectContext(g *graph.Graph, qualifiedNames []string, repoRoot string, tokenBudget int) []ContextEntry {
	if tokenBudget <= 0 {
		tokenBudget = 8000
	}

	type scored struct {
		node  *graph.Node
		risk  RiskScore
		src   string
		toks  int
	}

	var items []scored
	for _, qn := range qualifiedNames {
		node, ok := g.GetNode(qn)
		if !ok {
			continue
		}
		risk := ComputeRiskScore(g, node)
		src := readSource(repoRoot, node.FilePath, node.LineStart, node.LineEnd)
		toks := estimateTokens(src)
		items = append(items, scored{node, risk, src, toks})
	}

	// Sort by risk descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].risk.Total > items[j].risk.Total
	})

	var result []ContextEntry
	used := 0
	for _, item := range items {
		if used+item.toks <= tokenBudget {
			result = append(result, ContextEntry{
				Node:       item.node,
				Risk:       item.risk,
				Source:     item.src,
				TokenCount: item.toks,
			})
			used += item.toks
		}
	}

	return result
}

// estimateTokens approximates token count using ~4 chars per token.
func estimateTokens(source string) int {
	if source == "" {
		return 0
	}
	// Rough estimate: ~4 characters per token for code
	return (len(source) + 3) / 4
}

// readSource reads source code lines from a file.
func readSource(repoRoot, filePath string, startLine, endLine int) string {
	fullPath := filePath
	if repoRoot != "" && !filepath.IsAbs(filePath) {
		fullPath = filepath.Join(repoRoot, filePath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > len(lines) {
		return ""
	}

	selected := lines[startLine-1 : endLine]
	return strings.Join(selected, "\n")
}

// FormatContext formats context entries for display.
func FormatContext(entries []ContextEntry) string {
	var b strings.Builder

	for i, entry := range entries {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		b.WriteString(fmt.Sprintf("## %s (%s) [risk: %.2f]\n",
			entry.Node.Name, entry.Node.Kind, entry.Risk.Total))
		b.WriteString(fmt.Sprintf("File: %s (lines %d-%d)\n",
			entry.Node.FilePath, entry.Node.LineStart, entry.Node.LineEnd))
		b.WriteString(fmt.Sprintf("Tokens: %d\n\n", entry.TokenCount))
		if entry.Source != "" {
			b.WriteString("```\n")
			b.WriteString(entry.Source)
			b.WriteString("\n```\n")
		}
	}

	return b.String()
}
