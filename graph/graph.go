// Package graph provides an in-memory code knowledge graph with persistence.
package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// NodeKind represents the type of code entity.
type NodeKind string

const (
	KindFile     NodeKind = "File"
	KindClass    NodeKind = "Class"
	KindFunction NodeKind = "Function"
	KindType     NodeKind = "Type"
	KindTest     NodeKind = "Test"
)

// EdgeKind represents the type of relationship between nodes.
type EdgeKind string

const (
	EdgeCalls       EdgeKind = "CALLS"
	EdgeImportsFrom EdgeKind = "IMPORTS_FROM"
	EdgeInherits    EdgeKind = "INHERITS"
	EdgeImplements  EdgeKind = "IMPLEMENTS"
	EdgeContains    EdgeKind = "CONTAINS"
	EdgeTestedBy    EdgeKind = "TESTED_BY"
	EdgeDependsOn   EdgeKind = "DEPENDS_ON"
)

// Node represents a code entity in the graph.
type Node struct {
	ID            int      `json:"id"`
	Kind          NodeKind `json:"kind"`
	Name          string   `json:"name"`
	QualifiedName string   `json:"qualified_name"`
	FilePath      string   `json:"file_path"`
	LineStart     int      `json:"line_start"`
	LineEnd       int      `json:"line_end"`
	Language      string   `json:"language"`
	ParentName    string   `json:"parent_name,omitempty"`
	Params        string   `json:"params,omitempty"`
	ReturnType    string   `json:"return_type,omitempty"`
	IsTest        bool     `json:"is_test"`
}

// Edge represents a relationship between two nodes.
type Edge struct {
	ID              int      `json:"id"`
	Kind            EdgeKind `json:"kind"`
	SourceQualified string   `json:"source_qualified"`
	TargetQualified string   `json:"target_qualified"`
	FilePath        string   `json:"file_path"`
	Line            int      `json:"line"`
}

// Stats holds aggregate graph statistics.
type Stats struct {
	TotalNodes  int            `json:"total_nodes"`
	TotalEdges  int            `json:"total_edges"`
	NodesByKind map[string]int `json:"nodes_by_kind"`
	EdgesByKind map[string]int `json:"edges_by_kind"`
	Languages   []string       `json:"languages"`
	FilesCount  int            `json:"files_count"`
}

// Graph is a thread-safe in-memory code knowledge graph.
type Graph struct {
	mu       sync.RWMutex
	nodes    map[string]*Node // qualified_name -> Node
	edges    []*Edge
	nextNode int
	nextEdge int

	// indices for fast lookup
	nodesByFile map[string][]*Node   // file_path -> nodes
	edgesBySrc  map[string][]*Edge   // source_qualified -> edges
	edgesByTgt  map[string][]*Edge   // target_qualified -> edges
}

// New creates a new empty Graph.
func New() *Graph {
	return &Graph{
		nodes:       make(map[string]*Node),
		nodesByFile: make(map[string][]*Node),
		edgesBySrc:  make(map[string][]*Edge),
		edgesByTgt:  make(map[string][]*Edge),
	}
}

// AddNode adds or updates a node in the graph. Returns the node ID.
func (g *Graph) AddNode(n Node) int {
	g.mu.Lock()
	defer g.mu.Unlock()

	if existing, ok := g.nodes[n.QualifiedName]; ok {
		// Update existing node
		existing.Kind = n.Kind
		existing.Name = n.Name
		existing.FilePath = n.FilePath
		existing.LineStart = n.LineStart
		existing.LineEnd = n.LineEnd
		existing.Language = n.Language
		existing.ParentName = n.ParentName
		existing.Params = n.Params
		existing.ReturnType = n.ReturnType
		existing.IsTest = n.IsTest
		return existing.ID
	}

	g.nextNode++
	n.ID = g.nextNode
	nodeCopy := n
	g.nodes[n.QualifiedName] = &nodeCopy
	g.nodesByFile[n.FilePath] = append(g.nodesByFile[n.FilePath], &nodeCopy)
	return n.ID
}

// AddEdge adds an edge to the graph. Returns the edge ID.
func (g *Graph) AddEdge(e Edge) int {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Deduplicate: check if same edge already exists
	for _, existing := range g.edgesBySrc[e.SourceQualified] {
		if existing.TargetQualified == e.TargetQualified && existing.Kind == e.Kind {
			return existing.ID
		}
	}

	g.nextEdge++
	e.ID = g.nextEdge
	edgeCopy := e
	g.edges = append(g.edges, &edgeCopy)
	g.edgesBySrc[e.SourceQualified] = append(g.edgesBySrc[e.SourceQualified], &edgeCopy)
	g.edgesByTgt[e.TargetQualified] = append(g.edgesByTgt[e.TargetQualified], &edgeCopy)
	return e.ID
}

// GetNode returns a node by qualified name.
func (g *Graph) GetNode(qualifiedName string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[qualifiedName]
	if !ok {
		return nil, false
	}
	copy := *n
	return &copy, true
}

// GetNodesByFile returns all nodes in a given file.
func (g *Graph) GetNodesByFile(filePath string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	nodes := g.nodesByFile[filePath]
	result := make([]*Node, len(nodes))
	for i, n := range nodes {
		copy := *n
		result[i] = &copy
	}
	return result
}

// GetNodesByKind returns all nodes of the given kinds.
func (g *Graph) GetNodesByKind(kinds ...NodeKind) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	kindSet := make(map[NodeKind]bool, len(kinds))
	for _, k := range kinds {
		kindSet[k] = true
	}

	var result []*Node
	for _, n := range g.nodes {
		if kindSet[n.Kind] {
			copy := *n
			result = append(result, &copy)
		}
	}
	return result
}

// SearchNodes searches for nodes whose name contains the query string.
func (g *Graph) SearchNodes(query string, limit int) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	query = strings.ToLower(query)
	var result []*Node
	for _, n := range g.nodes {
		if strings.Contains(strings.ToLower(n.Name), query) ||
			strings.Contains(strings.ToLower(n.QualifiedName), query) {
			copy := *n
			result = append(result, &copy)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetEdgesBySource returns all edges originating from the given qualified name.
func (g *Graph) GetEdgesBySource(qualifiedName string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edges := g.edgesBySrc[qualifiedName]
	result := make([]*Edge, len(edges))
	for i, e := range edges {
		copy := *e
		result[i] = &copy
	}
	return result
}

// GetEdgesByTarget returns all edges targeting the given qualified name.
func (g *Graph) GetEdgesByTarget(qualifiedName string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edges := g.edgesByTgt[qualifiedName]
	result := make([]*Edge, len(edges))
	for i, e := range edges {
		copy := *e
		result[i] = &copy
	}
	return result
}

// RemoveFileData removes all nodes and edges associated with a file.
func (g *Graph) RemoveFileData(filePath string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Collect qualified names of nodes to remove
	toRemove := make(map[string]bool)
	for _, n := range g.nodesByFile[filePath] {
		toRemove[n.QualifiedName] = true
		delete(g.nodes, n.QualifiedName)
	}
	delete(g.nodesByFile, filePath)

	// Remove edges referencing these nodes
	g.edges = filterEdges(g.edges, toRemove)

	// Rebuild edge indices
	g.edgesBySrc = make(map[string][]*Edge)
	g.edgesByTgt = make(map[string][]*Edge)
	for _, e := range g.edges {
		g.edgesBySrc[e.SourceQualified] = append(g.edgesBySrc[e.SourceQualified], e)
		g.edgesByTgt[e.TargetQualified] = append(g.edgesByTgt[e.TargetQualified], e)
	}
}

func filterEdges(edges []*Edge, removedNodes map[string]bool) []*Edge {
	var result []*Edge
	for _, e := range edges {
		if !removedNodes[e.SourceQualified] && !removedNodes[e.TargetQualified] {
			result = append(result, e)
		}
	}
	return result
}

// GetImpactRadius performs BFS from changed files to find affected nodes.
func (g *Graph) GetImpactRadius(changedQualifiedNames []string, maxDepth, maxNodes int) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 2
	}
	if maxNodes <= 0 {
		maxNodes = 500
	}

	visited := make(map[string]bool)
	type queueItem struct {
		qn    string
		depth int
	}
	queue := make([]queueItem, 0, len(changedQualifiedNames))

	for _, qn := range changedQualifiedNames {
		if _, ok := g.nodes[qn]; ok {
			visited[qn] = true
			queue = append(queue, queueItem{qn, 0})
		}
	}

	for len(queue) > 0 && len(visited) < maxNodes {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxDepth {
			continue
		}

		// Forward edges (who does this node call?)
		for _, e := range g.edgesBySrc[item.qn] {
			if len(visited) >= maxNodes {
				break
			}
			if !visited[e.TargetQualified] {
				visited[e.TargetQualified] = true
				queue = append(queue, queueItem{e.TargetQualified, item.depth + 1})
			}
		}

		// Reverse edges (who calls this node?)
		for _, e := range g.edgesByTgt[item.qn] {
			if len(visited) >= maxNodes {
				break
			}
			if !visited[e.SourceQualified] {
				visited[e.SourceQualified] = true
				queue = append(queue, queueItem{e.SourceQualified, item.depth + 1})
			}
		}
	}

	var result []*Node
	for qn := range visited {
		if n, ok := g.nodes[qn]; ok {
			copy := *n
			result = append(result, &copy)
		}
	}
	return result
}

// GetCallerCount returns the number of unique callers of a node.
func (g *Graph) GetCallerCount(qualifiedName string) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.edgesByTgt[qualifiedName])
}

// HasTestCoverage checks if a node has any TESTED_BY edges.
// TESTED_BY edges: SourceQualified=function, TargetQualified=test_function
func (g *Graph) HasTestCoverage(qualifiedName string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, e := range g.edgesBySrc[qualifiedName] {
		if e.Kind == EdgeTestedBy {
			return true
		}
	}
	return false
}

// AllNodes returns all nodes in the graph.
func (g *Graph) AllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		copy := *n
		result = append(result, &copy)
	}
	return result
}

// AllEdges returns all edges in the graph.
func (g *Graph) AllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*Edge, 0, len(g.edges))
	for _, e := range g.edges {
		copy := *e
		result = append(result, &copy)
	}
	return result
}

// GetStats returns aggregate statistics about the graph.
func (g *Graph) GetStats() Stats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodesByKind := make(map[string]int)
	edgesByKind := make(map[string]int)
	langSet := make(map[string]bool)
	fileSet := make(map[string]bool)

	for _, n := range g.nodes {
		nodesByKind[string(n.Kind)]++
		if n.Language != "" {
			langSet[n.Language] = true
		}
		fileSet[n.FilePath] = true
	}
	for _, e := range g.edges {
		edgesByKind[string(e.Kind)]++
	}

	languages := make([]string, 0, len(langSet))
	for l := range langSet {
		languages = append(languages, l)
	}
	sort.Strings(languages)

	return Stats{
		TotalNodes:  len(g.nodes),
		TotalEdges:  len(g.edges),
		NodesByKind: nodesByKind,
		EdgesByKind: edgesByKind,
		Languages:   languages,
		FilesCount:  len(fileSet),
	}
}

// persistData is the JSON-serializable form of the graph.
type persistData struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

// Save persists the graph to a JSON file.
func (g *Graph) Save(path string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	data := persistData{
		Nodes: make([]*Node, 0, len(g.nodes)),
		Edges: make([]*Edge, 0, len(g.edges)),
	}
	for _, n := range g.nodes {
		data.Nodes = append(data.Nodes, n)
	}
	for _, e := range g.edges {
		data.Edges = append(data.Edges, e)
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}
	return os.WriteFile(path, b, 0644)
}

// Load reads a graph from a JSON file.
func Load(path string) (*Graph, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read graph file: %w", err)
	}

	var data persistData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("unmarshal graph: %w", err)
	}

	g := New()
	for _, n := range data.Nodes {
		g.nodes[n.QualifiedName] = n
		g.nodesByFile[n.FilePath] = append(g.nodesByFile[n.FilePath], n)
		if n.ID > g.nextNode {
			g.nextNode = n.ID
		}
	}
	for _, e := range data.Edges {
		g.edges = append(g.edges, e)
		g.edgesBySrc[e.SourceQualified] = append(g.edgesBySrc[e.SourceQualified], e)
		g.edgesByTgt[e.TargetQualified] = append(g.edgesByTgt[e.TargetQualified], e)
		if e.ID > g.nextEdge {
			g.nextEdge = e.ID
		}
	}
	return g, nil
}

// AllFiles returns all unique file paths in the graph.
func (g *Graph) AllFiles() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	files := make([]string, 0, len(g.nodesByFile))
	for f := range g.nodesByFile {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}
