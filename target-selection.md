# V2 Target Selection: codegraph-go

## Original Project

| Field | Value |
|-------|-------|
| Name | code-review-graph |
| Full Name | tirth8205/code-review-graph |
| URL | https://github.com/tirth8205/code-review-graph |
| Description | Local knowledge graph for Claude Code. Builds a persistent map of your codebase so Claude reads only what matters -- 6.8x fewer tokens on reviews and up to 49x on daily coding tasks. |
| Language | Python |
| License | MIT |
| Stars | 3,731 |
| Forks | 331 |
| Created | 2026-02-26 |
| Age | 29 days |

## Trending Signals

| Signal | Score |
|--------|-------|
| Newcomer Score | 9.7 |
| Momentum Score | 9.0 |
| Trend Score | 5.6 |
| Signal Type | newcomer |
| Stars/Day Avg | 128.7 |
| Recent Commits (30d) | 127 |

## Why This Project

### Selection Criteria Match
- **Stars 1K-50K**: 3.7K stars -- within range
- **Core logic clear**: Tree-sitter parsing -> graph construction -> context retrieval
- **1-day reimplement**: Core algorithms (AST parsing, graph building, query engine) are well-scoped
- **Something to learn**: Incremental code graph construction, tree-sitter in Go, smart context selection algorithms

### Key Technical Concepts
1. **Tree-sitter parsing**: Parse source files into AST, extract functions/classes/imports/calls
2. **Knowledge graph construction**: Build directed graph of code relationships (calls, imports, inheritance, references)
3. **Incremental updates**: Only re-parse changed files, update graph edges efficiently
4. **Context retrieval**: Given a diff/query, walk the graph to find the minimal set of relevant code
5. **Token optimization**: Score and rank code sections by relevance, fit within token budgets

### Improvement Opportunities (Go advantages)
- **Performance**: Go's concurrency for parallel file parsing across large codebases
- **Single binary**: No Python environment setup, no pip dependencies
- **Memory efficiency**: Go's lower memory footprint for large graphs
- **Native tree-sitter**: go-tree-sitter bindings are mature and fast
- **Faster incremental updates**: goroutine-per-file parallel parsing
- **Smaller binary size**: vs Python + tree-sitter + networkx + all dependencies

### What We Learn
- Tree-sitter grammar integration in Go
- Graph data structures for code analysis (adjacency lists, topological sort)
- Diff-aware context selection algorithms
- Token budget optimization (knapsack-style relevance packing)

## Excluded Alternatives

| Project | Stars | Why Excluded |
|---------|-------|--------------|
| Scrapling | 33.5K | Browser-dependent (Playwright), web scraping is UI-heavy |
| opik | 18.5K | Dashboard/UI-heavy, too complex for 1 day |
| prompt-optimizer | 25.6K | Web app UI, core logic is LLM calls (not algorithmic) |
| edgequake | 1.5K | Too similar to lightrag-go (already done) |
| TrendRadar | 49.8K | Similar to our git-trend-sync |
| agent-orchestrator | 5.5K | Tightly coupled to specific tools (tmux, worktrees) |
| helicone | 5.4K | UI-heavy observability dashboard |

## Implementation Plan (Go)

### Name: codegraph-go

### Core Modules
1. **parser/**: Tree-sitter integration, multi-language support (Go, Python, TypeScript, Rust)
2. **graph/**: In-memory directed graph with persistence (JSON/binary serialization)
3. **index/**: File watcher + incremental update engine
4. **query/**: Context retrieval given diff hunks or symbol names
5. **cli/**: Command-line interface for build, query, serve

### Target: 80+ tests, TDD, comparison-report.md
