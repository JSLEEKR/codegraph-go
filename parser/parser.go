// Package parser extracts code structure from source files.
// Uses go/ast for Go files and regex-based extraction for Python and TypeScript.
package parser

import (
	"path/filepath"
	"strings"

	"github.com/JSLEEKR/codegraph-go/graph"
)

// Language represents a supported programming language.
type Language string

const (
	LangGo         Language = "go"
	LangPython     Language = "python"
	LangTypeScript Language = "typescript"
	LangJavaScript Language = "javascript"
	LangUnknown    Language = "unknown"
)

// ParseResult holds the extracted nodes and edges from a file.
type ParseResult struct {
	FilePath string
	Language Language
	Nodes    []graph.Node
	Edges    []graph.Edge
}

// DetectLanguage determines the language from a file extension.
func DetectLanguage(filePath string) Language {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return LangGo
	case ".py":
		return LangPython
	case ".ts", ".tsx":
		return LangTypeScript
	case ".js", ".jsx", ".mjs", ".cjs":
		return LangJavaScript
	default:
		return LangUnknown
	}
}

// ParseFile parses a source file and extracts code structure.
func ParseFile(filePath string, source []byte) (*ParseResult, error) {
	lang := DetectLanguage(filePath)
	if lang == LangUnknown {
		return nil, nil
	}

	var nodes []graph.Node
	var edges []graph.Edge

	// Add file node
	isTest := isTestFile(filePath, lang)
	fileNode := graph.Node{
		Kind:          graph.KindFile,
		Name:          filepath.Base(filePath),
		QualifiedName: filePath,
		FilePath:      filePath,
		LineStart:     1,
		LineEnd:       countLines(source),
		Language:      string(lang),
		IsTest:        isTest,
	}
	nodes = append(nodes, fileNode)

	switch lang {
	case LangGo:
		goNodes, goEdges, err := parseGo(filePath, source)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, goNodes...)
		edges = append(edges, goEdges...)
	case LangPython:
		pyNodes, pyEdges := parsePython(filePath, source)
		nodes = append(nodes, pyNodes...)
		edges = append(edges, pyEdges...)
	case LangTypeScript, LangJavaScript:
		tsNodes, tsEdges := parseTypeScript(filePath, source)
		nodes = append(nodes, tsNodes...)
		edges = append(edges, tsEdges...)
	}

	return &ParseResult{
		FilePath: filePath,
		Language: lang,
		Nodes:    nodes,
		Edges:    edges,
	}, nil
}

// Qualify creates a qualified name for a code entity.
func Qualify(filePath, parent, name string) string {
	if parent != "" {
		return filePath + "::" + parent + "." + name
	}
	return filePath + "::" + name
}

func isTestFile(filePath string, lang Language) bool {
	base := strings.ToLower(filepath.Base(filePath))
	dir := strings.ToLower(filepath.Dir(filePath))

	// Check directory
	if strings.Contains(dir, "test") || strings.Contains(dir, "spec") {
		return true
	}

	switch lang {
	case LangGo:
		return strings.HasSuffix(base, "_test.go")
	case LangPython:
		return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
	case LangTypeScript, LangJavaScript:
		return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
			strings.HasSuffix(base, "_test.ts") || strings.HasSuffix(base, "_test.js")
	}
	return false
}

func countLines(source []byte) int {
	if len(source) == 0 {
		return 0
	}
	count := 1
	for _, b := range source {
		if b == '\n' {
			count++
		}
	}
	return count
}
