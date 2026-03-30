// Package export provides graph export functionality (DOT format).
package export

import (
	"fmt"
	"strings"

	"github.com/JSLEEKR/codegraph-go/graph"
)

// kindColors maps node kinds to graphviz colors.
var kindColors = map[graph.NodeKind]string{
	graph.KindFile:     "#4a90d9",
	graph.KindClass:    "#e8a838",
	graph.KindFunction: "#50b83c",
	graph.KindType:     "#9c6ade",
	graph.KindTest:     "#de3618",
}

// kindShapes maps node kinds to graphviz shapes.
var kindShapes = map[graph.NodeKind]string{
	graph.KindFile:     "folder",
	graph.KindClass:    "box",
	graph.KindFunction: "ellipse",
	graph.KindType:     "hexagon",
	graph.KindTest:     "diamond",
}

// edgeStyles maps edge kinds to graphviz styles.
var edgeStyles = map[graph.EdgeKind]string{
	graph.EdgeCalls:       "solid",
	graph.EdgeImportsFrom: "dashed",
	graph.EdgeInherits:    "bold",
	graph.EdgeImplements:  "dotted",
	graph.EdgeContains:    "solid",
	graph.EdgeTestedBy:    "dashed",
	graph.EdgeDependsOn:   "dashed",
}

var edgeColors = map[graph.EdgeKind]string{
	graph.EdgeCalls:       "#333333",
	graph.EdgeImportsFrom: "#4a90d9",
	graph.EdgeInherits:    "#e8a838",
	graph.EdgeImplements:  "#9c6ade",
	graph.EdgeContains:    "#999999",
	graph.EdgeTestedBy:    "#de3618",
	graph.EdgeDependsOn:   "#50b83c",
}

// ToDOT exports the graph in DOT format for Graphviz.
func ToDOT(g *graph.Graph, title string) string {
	if title == "" {
		title = "CodeGraph"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("digraph %q {\n", title))
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [fontname=\"Helvetica\", fontsize=10];\n")
	b.WriteString("  edge [fontname=\"Helvetica\", fontsize=8];\n\n")

	// Add legend
	b.WriteString("  subgraph cluster_legend {\n")
	b.WriteString("    label=\"Legend\";\n")
	b.WriteString("    style=dashed;\n")
	b.WriteString("    fontsize=8;\n")
	b.WriteString("    leg_file [label=\"File\" shape=folder fillcolor=\"#4a90d9\" style=filled fontcolor=white];\n")
	b.WriteString("    leg_class [label=\"Class\" shape=box fillcolor=\"#e8a838\" style=filled];\n")
	b.WriteString("    leg_func [label=\"Function\" shape=ellipse fillcolor=\"#50b83c\" style=filled fontcolor=white];\n")
	b.WriteString("    leg_type [label=\"Type\" shape=hexagon fillcolor=\"#9c6ade\" style=filled fontcolor=white];\n")
	b.WriteString("    leg_test [label=\"Test\" shape=diamond fillcolor=\"#de3618\" style=filled fontcolor=white];\n")
	b.WriteString("  }\n\n")

	// Add nodes
	nodes := g.AllNodes()
	nodeIDs := make(map[string]string) // qualified_name -> DOT id
	for i, n := range nodes {
		dotID := fmt.Sprintf("n%d", i)
		nodeIDs[n.QualifiedName] = dotID

		color := kindColors[n.Kind]
		shape := kindShapes[n.Kind]
		if color == "" {
			color = "#cccccc"
		}
		if shape == "" {
			shape = "ellipse"
		}

		label := escapeLabel(n.Name)
		b.WriteString(fmt.Sprintf("  %s [label=%q shape=%s fillcolor=%q style=filled",
			dotID, label, shape, color))
		if n.Kind != graph.KindClass {
			b.WriteString(" fontcolor=white")
		}
		b.WriteString("];\n")
	}

	b.WriteString("\n")

	// Add edges
	edges := g.AllEdges()
	for _, e := range edges {
		srcID := nodeIDs[e.SourceQualified]
		tgtID := nodeIDs[e.TargetQualified]
		if srcID == "" || tgtID == "" {
			continue
		}

		style := edgeStyles[e.Kind]
		color := edgeColors[e.Kind]
		if style == "" {
			style = "solid"
		}
		if color == "" {
			color = "#333333"
		}

		b.WriteString(fmt.Sprintf("  %s -> %s [label=%q style=%s color=%q];\n",
			srcID, tgtID, string(e.Kind), style, color))
	}

	b.WriteString("}\n")
	return b.String()
}

// ToFilteredDOT exports a subgraph containing only specified nodes and their edges.
func ToFilteredDOT(g *graph.Graph, qualifiedNames []string, title string) string {
	if title == "" {
		title = "CodeGraph (filtered)"
	}

	nameSet := make(map[string]bool, len(qualifiedNames))
	for _, qn := range qualifiedNames {
		nameSet[qn] = true
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("digraph %q {\n", title))
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [fontname=\"Helvetica\", fontsize=10];\n")
	b.WriteString("  edge [fontname=\"Helvetica\", fontsize=8];\n\n")

	nodeIDs := make(map[string]string)
	idx := 0
	nodes := g.AllNodes()
	for _, n := range nodes {
		if !nameSet[n.QualifiedName] {
			continue
		}
		dotID := fmt.Sprintf("n%d", idx)
		idx++
		nodeIDs[n.QualifiedName] = dotID

		color := kindColors[n.Kind]
		shape := kindShapes[n.Kind]
		if color == "" {
			color = "#cccccc"
		}
		if shape == "" {
			shape = "ellipse"
		}

		b.WriteString(fmt.Sprintf("  %s [label=%q shape=%s fillcolor=%q style=filled];\n",
			dotID, escapeLabel(n.Name), shape, color))
	}

	b.WriteString("\n")

	edges := g.AllEdges()
	for _, e := range edges {
		srcID := nodeIDs[e.SourceQualified]
		tgtID := nodeIDs[e.TargetQualified]
		if srcID == "" || tgtID == "" {
			continue
		}

		style := edgeStyles[e.Kind]
		color := edgeColors[e.Kind]
		if style == "" {
			style = "solid"
		}
		if color == "" {
			color = "#333333"
		}

		b.WriteString(fmt.Sprintf("  %s -> %s [label=%q style=%s color=%q];\n",
			srcID, tgtID, string(e.Kind), style, color))
	}

	b.WriteString("}\n")
	return b.String()
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > 40 {
		s = s[:37] + "..."
	}
	return s
}
