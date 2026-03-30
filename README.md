<p align="center">
  <h1 align="center">codegraph-go</h1>
  <p align="center">Source code knowledge graph for smart code review context — zero dependencies, single binary</p>
</p>

<p align="center">
  <a href="https://github.com/JSLEEKR/codegraph-go/actions"><img src="https://img.shields.io/github/actions/workflow/status/JSLEEKR/codegraph-go/ci.yml?style=for-the-badge" alt="Build Status"></a>
  <a href="https://goreportcard.com/report/github.com/JSLEEKR/codegraph-go"><img src="https://goreportcard.com/badge/github.com/JSLEEKR/codegraph-go?style=for-the-badge" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=for-the-badge" alt="License"></a>
  <a href="https://github.com/JSLEEKR/codegraph-go"><img src="https://img.shields.io/badge/zero-dependencies-brightgreen?style=for-the-badge" alt="Zero Deps"></a>
</p>

---

## Why This Exists

AI coding assistants waste tokens reading entire codebases when reviewing changes. **codegraph-go** builds a structural knowledge graph of your code, then uses that graph to select only the most relevant context for a given diff — giving you **smart, token-efficient code review context**.

This is a Go reimplementation of [code-review-graph](https://github.com/tirth8205/code-review-graph) (3.7K stars, Python). Our version provides:

- **Zero external dependencies** — uses Go's `go/ast` for Go files, regex for others
- **Single static binary** — no Python, no pip, no tree-sitter C bindings
- **Concurrent parsing** — goroutine-per-file with configurable parallelism
- **Token budget optimization** — explicit knapsack-style context selection within a token limit
- **DOT graph export** — visualize your code's dependency graph with Graphviz

## Features

| Feature | Description |
|---------|-------------|
| **Multi-language parsing** | Go (via `go/ast`), Python, TypeScript, JavaScript |
| **Code knowledge graph** | Functions, classes, imports, call relationships, test coverage |
| **Diff-aware context** | Given a git diff, find the most relevant code context |
| **Risk scoring** | Security keywords, caller count, test coverage → priority ranking |
| **Token budget** | Select most relevant context within a configurable token limit |
| **DOT export** | Export graph in Graphviz DOT format for visualization |
| **JSON export** | Export stats and subgraphs as JSON |
| **Incremental updates** | Remove and re-parse only changed files |
| **Test gap detection** | Identify functions without test coverage in changed code |

## Installation

### From Source

```bash
go install github.com/JSLEEKR/codegraph-go/cmd/codegraph@latest
```

### Build from Repository

```bash
git clone https://github.com/JSLEEKR/codegraph-go.git
cd codegraph-go
go build -o codegraph ./cmd/codegraph/
```

## Quick Start

### 1. Build the Code Graph

```bash
# Parse all source files in the current directory
codegraph build-graph

# Or specify a repository root
codegraph build-graph --repo /path/to/project
```

Output:
```
Building code graph for /path/to/project...
Found 42 source files

Build complete in 85ms
  Files:     42
  Nodes:     287
  Edges:     534
  Languages: go, python, typescript
  Saved to:  /path/to/project/.codegraph/graph.json
```

### 2. Query the Graph

```bash
# Search by name
codegraph query --name "ParseFile"

# Filter by kind
codegraph query --kind Function --limit 10

# List all nodes in a file
codegraph query --file "parser/parser.go"

# JSON output
codegraph query --name "Server" --format json
```

Output:
```
Found 2 results:

  [f] ParseFile
    Qualified: parser/parser.go::ParseFile
    File:      parser/parser.go:35-68
    Params:    (filePath string, source []byte)
    Returns:   (*ParseResult, error)
    Calls:     parseGo, parsePython, parseTypeScript
    Called by: (none)

  [f] parseGo
    Qualified: parser/go_parser.go::parseGo
    File:      parser/go_parser.go:15-120
    Params:    (filePath string, source []byte)
    Returns:   ([]graph.Node, []graph.Edge, error)
```

### 3. Get Diff Context

```bash
# Find relevant context for recent changes
codegraph diff-context --base HEAD~1

# With custom token budget
codegraph diff-context --base main --budget 4000

# JSON output for programmatic use
codegraph diff-context --base HEAD~3 --format json
```

Output:
```
Changes in 3 files affecting 5 code entities

Impact radius: 12 nodes
Test gaps: 2 functions without tests
Token budget: 3200 / 4000 used

Functions missing tests:
  - validateInput (handler.go:45)
  - parseConfig (config.go:12)

Review priorities (by risk score):

## validateInput (Function) [risk: 0.50]
File: handler.go (lines 45-60)
Tokens: 120

​```
func validateInput(input string) error {
    if input == "" {
        return fmt.Errorf("empty input")
    }
    // ... sanitize ...
}
​```
```

### 4. View Statistics

```bash
codegraph stats
codegraph stats --format json
```

Output:
```
Graph Statistics:

  Total nodes:     287
  Total edges:     534
  Files:           42
  Languages:       go, python, typescript

  Nodes by kind:
    Function     142
    File         42
    Class        35
    Type         28
    Test         40

  Edges by kind:
    CALLS        215
    CONTAINS     187
    IMPORTS_FROM 89
    TESTED_BY    31
    INHERITS     12
```

### 5. Export Graph

```bash
# Export as DOT (for Graphviz)
codegraph export --format dot > graph.dot
dot -Tpng graph.dot -o graph.png

# Export filtered subgraph
codegraph export --format dot --filter "parser.go::ParseFile,parser.go::parseGo"

# Export as JSON
codegraph export --format json --output stats.json
```

## Architecture

```
codegraph-go/
├── graph/          # In-memory knowledge graph with JSON persistence
│   └── graph.go    # Node/Edge types, BFS impact analysis, save/load
├── parser/         # Multi-language source code parser
│   ├── parser.go       # Language detection, file-level orchestration
│   ├── go_parser.go    # Go parser using go/ast (stdlib)
│   └── regex_parser.go # Python/TypeScript parser using regex
├── diff/           # Git diff parsing and node mapping
│   └── diff.go     # Unified diff parser, line range overlap detection
├── context/        # Token-budget-aware context selection
│   └── context.go  # Risk scoring, greedy knapsack, context formatting
├── export/         # Graph export (DOT, JSON)
│   ├── dot.go      # Graphviz DOT format with color-coded node types
│   └── json.go     # JSON stats and subgraph export
└── cmd/codegraph/  # CLI entry point
    └── main.go     # Command router and flag parsing
```

### How It Works

```
Source Files → Parser (go/ast + regex) → Nodes + Edges
                                              ↓
                                    In-Memory Graph
                                              ↓
              Git Diff → Changed Ranges → Node Mapping → Risk Scoring
                                                              ↓
                                              Token Budget → Context Selection
                                                              ↓
                                                    Review Priorities
```

1. **Parsing**: Go files are parsed with `go/ast` for perfect fidelity. Python and TypeScript files use regex patterns to extract classes, functions, imports, and calls.

2. **Graph Construction**: Nodes (File, Class, Function, Type, Test) and Edges (CALLS, IMPORTS_FROM, INHERITS, IMPLEMENTS, CONTAINS, TESTED_BY, DEPENDS_ON) are stored in an in-memory graph with index maps for fast lookup.

3. **Diff Analysis**: Git diffs are parsed into line ranges. Each range is overlapped against graph nodes to find affected code entities.

4. **Risk Scoring**: Each affected node gets a risk score (0.0-1.0) based on:
   - Caller count (more callers = higher blast radius)
   - Test coverage (untested = higher risk)
   - Security keywords (auth, password, sql = higher risk)

5. **Context Selection**: Nodes are sorted by risk and selected greedily within the token budget. Source code is read from disk and included in the output.

### Node Types

| Kind | Description | DOT Shape |
|------|-------------|-----------|
| File | Source file | folder |
| Class | Class/struct definition | box |
| Function | Function/method | ellipse |
| Type | Interface/type alias | hexagon |
| Test | Test function | diamond |

### Edge Types

| Kind | Description | Example |
|------|-------------|---------|
| CALLS | Function invocation | `main() → helper()` |
| IMPORTS_FROM | Module import | `handler.go → fmt` |
| INHERITS | Class/struct embedding | `Dog → Animal` |
| IMPLEMENTS | Interface implementation | `Server → Handler` |
| CONTAINS | Structural containment | `file → function` |
| TESTED_BY | Test coverage link | `Parse() → TestParse()` |
| DEPENDS_ON | Dependency relationship | `module → package` |

## Language Support

| Language | Parser | Functions | Classes | Imports | Calls | Tests |
|----------|--------|-----------|---------|---------|-------|-------|
| Go | `go/ast` (stdlib) | Yes | Yes (structs) | Yes | Yes | Yes |
| Python | Regex | Yes | Yes | Yes | Yes | Yes |
| TypeScript | Regex | Yes | Yes | Yes | Yes | Yes |
| JavaScript | Regex | Yes | Yes | Yes | Yes | Yes |

### Go Parser Details (go/ast)

The Go parser provides first-class support via the standard library:
- Functions and methods with receiver types
- Struct and interface declarations
- Struct embedding (→ INHERITS edges)
- Import resolution
- Call extraction including package-qualified calls
- Test function detection (Test*, Benchmark*, Example*)
- Full parameter and return type extraction

### Python/TypeScript Parser Details (Regex)

The regex parsers extract:
- Class definitions with inheritance
- Function/method definitions with parameters
- Import statements
- Function calls (direct and method calls)
- Arrow functions (TypeScript)
- Interface and type alias declarations (TypeScript)
- Test file and test function detection

## Configuration

### Ignored Directories

The following directories are automatically skipped during parsing:

```
.git, node_modules, vendor, __pycache__, .codegraph,
.cache, dist, build, .next, .nuxt, target, bin,
.idea, .vscode, .vs
```

### Graph Storage

The graph is stored in `.codegraph/graph.json` in the repository root. This file can be committed to the repository for faster subsequent builds, or added to `.gitignore`.

## Comparison with code-review-graph (Python)

| Feature | code-review-graph (Python) | codegraph-go |
|---------|---------------------------|-------------|
| Dependencies | tree-sitter, networkx, mcp, watchdog | **Zero** (stdlib only) |
| Languages | 25+ (via tree-sitter) | 4 (Go, Python, TS, JS) |
| Distribution | pip install + tree-sitter compilation | **Single binary** |
| Parsing | tree-sitter (C bindings) | go/ast + regex |
| Storage | SQLite | In-memory + JSON |
| Concurrency | Single-threaded | **Goroutine-per-file** |
| Token Budget | Implicit (bounded BFS) | **Explicit knapsack** |
| Graph Export | HTML visualization | **DOT + JSON** |
| MCP Server | Yes (24+ tools) | No (CLI only) |
| File Watcher | Yes (watchdog) | No |

### When to Use Which

- **codegraph-go**: When you want a fast, zero-dependency CLI tool for code review context, especially for Go projects
- **code-review-graph**: When you need MCP integration, 25+ language support, or VS Code extension

## Development

```bash
# Run tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Build binary
go build -o codegraph ./cmd/codegraph/

# Run vet
go vet ./...
```

## License

MIT License — see [LICENSE](LICENSE) for details.

## Credits

Inspired by [code-review-graph](https://github.com/tirth8205/code-review-graph) by [tirth8205](https://github.com/tirth8205).
