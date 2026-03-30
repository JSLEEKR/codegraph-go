package parser

import (
	"testing"

	"github.com/JSLEEKR/codegraph-go/graph"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want Language
	}{
		{"main.go", LangGo},
		{"app.py", LangPython},
		{"index.ts", LangTypeScript},
		{"index.tsx", LangTypeScript},
		{"app.js", LangJavaScript},
		{"app.jsx", LangJavaScript},
		{"module.mjs", LangJavaScript},
		{"module.cjs", LangJavaScript},
		{"README.md", LangUnknown},
		{"data.json", LangUnknown},
		{"Makefile", LangUnknown},
		{"path/to/main.go", LangGo},
		{"DIR/UPPER.PY", LangPython},
	}

	for _, tt := range tests {
		got := DetectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path string
		lang Language
		want bool
	}{
		{"main_test.go", LangGo, true},
		{"main.go", LangGo, false},
		{"test_utils.py", LangPython, true},
		{"utils_test.py", LangPython, true},
		{"utils.py", LangPython, false},
		{"app.test.ts", LangTypeScript, true},
		{"app.spec.ts", LangTypeScript, true},
		{"app.ts", LangTypeScript, false},
		{"tests/helper.go", LangGo, true},
		{"__tests__/app.js", LangJavaScript, true},
	}

	for _, tt := range tests {
		got := isTestFile(tt.path, tt.lang)
		if got != tt.want {
			t.Errorf("isTestFile(%q, %q) = %v, want %v", tt.path, tt.lang, got, tt.want)
		}
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a", 1},
		{"a\nb", 2},
		{"a\nb\nc\n", 4},
	}

	for _, tt := range tests {
		got := countLines([]byte(tt.input))
		if got != tt.want {
			t.Errorf("countLines(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestQualify(t *testing.T) {
	tests := []struct {
		file, parent, name, want string
	}{
		{"main.go", "", "foo", "main.go::foo"},
		{"main.go", "Server", "Handle", "main.go::Server.Handle"},
		{"pkg/util.go", "", "Helper", "pkg/util.go::Helper"},
	}

	for _, tt := range tests {
		got := Qualify(tt.file, tt.parent, tt.name)
		if got != tt.want {
			t.Errorf("Qualify(%q, %q, %q) = %q, want %q", tt.file, tt.parent, tt.name, got, tt.want)
		}
	}
}

func TestParseFileUnknownLanguage(t *testing.T) {
	result, err := ParseFile("readme.md", []byte("# Hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for unknown language")
	}
}

func TestParseFileGoBasic(t *testing.T) {
	src := `package main

import "fmt"

func hello() {
	fmt.Println("hello")
}

func main() {
	hello()
}
`
	result, err := ParseFile("main.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Language != LangGo {
		t.Errorf("expected language go, got %s", result.Language)
	}

	// Should have: 1 file node + 2 function nodes
	var funcNodes []graph.Node
	for _, n := range result.Nodes {
		if n.Kind == graph.KindFunction {
			funcNodes = append(funcNodes, n)
		}
	}
	if len(funcNodes) != 2 {
		t.Errorf("expected 2 function nodes, got %d", len(funcNodes))
	}

	// Should have import edge
	var importEdges []graph.Edge
	for _, e := range result.Edges {
		if e.Kind == graph.EdgeImportsFrom {
			importEdges = append(importEdges, e)
		}
	}
	if len(importEdges) != 1 {
		t.Errorf("expected 1 import edge, got %d", len(importEdges))
	}
}

func TestParseFileGoStruct(t *testing.T) {
	src := `package main

type Server struct {
	port int
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) Stop() {
}
`
	result, err := ParseFile("server.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var classNodes []graph.Node
	var funcNodes []graph.Node
	for _, n := range result.Nodes {
		switch n.Kind {
		case graph.KindClass:
			classNodes = append(classNodes, n)
		case graph.KindFunction:
			funcNodes = append(funcNodes, n)
		}
	}

	if len(classNodes) != 1 || classNodes[0].Name != "Server" {
		t.Errorf("expected 1 class node (Server), got %d: %v", len(classNodes), classNodes)
	}
	if len(funcNodes) != 2 {
		t.Errorf("expected 2 method nodes, got %d", len(funcNodes))
	}

	// Methods should have ParentName = Server
	for _, fn := range funcNodes {
		if fn.ParentName != "Server" {
			t.Errorf("expected parent Server for %s, got %q", fn.Name, fn.ParentName)
		}
	}
}

func TestParseFileGoTest(t *testing.T) {
	src := `package main

import "testing"

func TestAdd(t *testing.T) {
	result := Add(1, 2)
	if result != 3 {
		t.Error("wrong")
	}
}
`
	result, err := ParseFile("main_test.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var testNodes []graph.Node
	for _, n := range result.Nodes {
		if n.Kind == graph.KindTest {
			testNodes = append(testNodes, n)
		}
	}
	if len(testNodes) != 1 {
		t.Errorf("expected 1 test node, got %d", len(testNodes))
	}
	if len(testNodes) > 0 && !testNodes[0].IsTest {
		t.Error("test node should have IsTest=true")
	}
}

func TestParseFileGoInterface(t *testing.T) {
	src := `package main

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}
`
	result, err := ParseFile("io.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var typeNodes []graph.Node
	for _, n := range result.Nodes {
		if n.Kind == graph.KindType {
			typeNodes = append(typeNodes, n)
		}
	}
	if len(typeNodes) != 2 {
		t.Errorf("expected 2 type/interface nodes, got %d", len(typeNodes))
	}
}

func TestParseFileGoEmbedding(t *testing.T) {
	src := `package main

type Base struct {}

type Derived struct {
	Base
}
`
	result, err := ParseFile("embed.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var inheritEdges []graph.Edge
	for _, e := range result.Edges {
		if e.Kind == graph.EdgeInherits {
			inheritEdges = append(inheritEdges, e)
		}
	}
	if len(inheritEdges) != 1 {
		t.Errorf("expected 1 inherit edge, got %d", len(inheritEdges))
	}
}

func TestParseFileGoCallResolution(t *testing.T) {
	src := `package main

import "fmt"

func helper() {}

func main() {
	helper()
	fmt.Println("test")
}
`
	result, err := ParseFile("main.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var callEdges []graph.Edge
	for _, e := range result.Edges {
		if e.Kind == graph.EdgeCalls {
			callEdges = append(callEdges, e)
		}
	}
	if len(callEdges) < 2 {
		t.Errorf("expected at least 2 call edges, got %d", len(callEdges))
	}
}

func TestParseFilePythonBasic(t *testing.T) {
	src := `import os
from pathlib import Path

class FileProcessor:
    def __init__(self, path):
        self.path = path

    def process(self):
        return self.path

def helper():
    pass

def main():
    fp = FileProcessor("test")
    fp.process()
`
	result, err := ParseFile("app.py", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Language != LangPython {
		t.Errorf("expected python, got %s", result.Language)
	}

	// Check we got some nodes
	if len(result.Nodes) < 3 {
		t.Errorf("expected at least 3 nodes (file + class + functions), got %d", len(result.Nodes))
	}

	// Check imports
	var importEdges int
	for _, e := range result.Edges {
		if e.Kind == graph.EdgeImportsFrom {
			importEdges++
		}
	}
	if importEdges < 2 {
		t.Errorf("expected at least 2 import edges, got %d", importEdges)
	}
}

func TestParseFilePythonTest(t *testing.T) {
	src := `def test_addition():
    assert 1 + 1 == 2

def test_subtraction():
    assert 2 - 1 == 1
`
	result, err := ParseFile("test_math.py", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	// File should be detected as test
	if !result.Nodes[0].IsTest {
		t.Error("test file should be marked as test")
	}
}

func TestParseFileTypeScriptBasic(t *testing.T) {
	src := `import { readFile } from 'fs';

interface Config {
  port: number;
  host: string;
}

class Server {
  private config: Config;

  constructor(config: Config) {
    this.config = config;
  }

  start(): void {
    console.log('starting');
  }
}

function createServer(config: Config): Server {
  return new Server(config);
}

export const helper = (x: number) => x * 2;
`
	result, err := ParseFile("server.ts", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Language != LangTypeScript {
		t.Errorf("expected typescript, got %s", result.Language)
	}

	// Should have nodes for interface, class, function, arrow
	if len(result.Nodes) < 4 {
		t.Errorf("expected at least 4 nodes, got %d", len(result.Nodes))
	}

	// Should have import edge
	var importEdges int
	for _, e := range result.Edges {
		if e.Kind == graph.EdgeImportsFrom {
			importEdges++
		}
	}
	if importEdges < 1 {
		t.Errorf("expected at least 1 import edge, got %d", importEdges)
	}
}

func TestParseFileTypeScriptInheritance(t *testing.T) {
	src := `class Animal {
  name: string;
}

class Dog extends Animal {
  breed: string;
}
`
	result, err := ParseFile("animals.ts", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var inheritEdges []graph.Edge
	for _, e := range result.Edges {
		if e.Kind == graph.EdgeInherits {
			inheritEdges = append(inheritEdges, e)
		}
	}
	if len(inheritEdges) != 1 {
		t.Errorf("expected 1 inherit edge, got %d", len(inheritEdges))
	}
}

func TestParseFileJavaScript(t *testing.T) {
	src := `const express = require('express');

function handleRequest(req, res) {
  res.send('hello');
}

const app = express();
`
	result, err := ParseFile("app.js", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if result.Language != LangJavaScript {
		t.Errorf("expected javascript, got %s", result.Language)
	}
}

func TestParseFileGoParams(t *testing.T) {
	src := `package main

func add(a int, b int) int {
	return a + b
}
`
	result, err := ParseFile("math.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	for _, n := range result.Nodes {
		if n.Name == "add" {
			if n.Params == "" {
				t.Error("expected non-empty params for add()")
			}
			if n.ReturnType == "" {
				t.Error("expected non-empty return type for add()")
			}
		}
	}
}

func TestParseFileGoMultiReturn(t *testing.T) {
	src := `package main

func divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, nil
	}
	return a / b, nil
}
`
	result, err := ParseFile("math.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	for _, n := range result.Nodes {
		if n.Name == "divide" {
			if n.ReturnType == "" {
				t.Error("expected non-empty return type")
			}
		}
	}
}

func TestParseFileGoContainsEdges(t *testing.T) {
	src := `package main

func foo() {}
func bar() {}
`
	result, err := ParseFile("main.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var containsEdges []graph.Edge
	for _, e := range result.Edges {
		if e.Kind == graph.EdgeContains {
			containsEdges = append(containsEdges, e)
		}
	}
	if len(containsEdges) < 2 {
		t.Errorf("expected at least 2 CONTAINS edges (file->func), got %d", len(containsEdges))
	}
}

func TestParseFilePythonInheritance(t *testing.T) {
	src := `class Base:
    pass

class Derived(Base):
    pass
`
	result, err := ParseFile("classes.py", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var inheritEdges []graph.Edge
	for _, e := range result.Edges {
		if e.Kind == graph.EdgeInherits {
			inheritEdges = append(inheritEdges, e)
		}
	}
	if len(inheritEdges) != 1 {
		t.Errorf("expected 1 inherit edge, got %d", len(inheritEdges))
	}
}
