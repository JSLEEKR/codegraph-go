# Study Notes: code-review-graph (tirth8205)

## Project Overview

- **Name**: code-review-graph
- **URL**: https://github.com/tirth8205/code-review-graph
- **Stars**: 3.7K (newcomer, 29 days old, ~128 stars/day)
- **Language**: Python 3.10+
- **Version**: 2.0.0
- **License**: MIT
- **Purpose**: Persistent incremental knowledge graph for token-efficient, context-aware code reviews. Parses source code into a structural graph, then uses that graph to select only the relevant context for AI code review -- claiming 6.8x fewer tokens on average.

---

## Architecture

### Module Structure

```
code_review_graph/
  __init__.py, __main__.py
  cli.py          — CLI entry point (build, update, detect-changes, visualize, etc.)
  parser.py       — Tree-sitter AST parsing, multi-language extraction
  graph.py        — SQLite-backed GraphStore (nodes, edges, queries, impact analysis)
  changes.py      — Git diff parsing, node mapping, risk scoring, context retrieval
  constants.py    — Security keywords frozenset
  communities.py  — Community detection (optional, igraph)
  embeddings.py   — Semantic search (optional, sentence-transformers)
  eval/           — Benchmark suite (token efficiency, impact accuracy, etc.)
```

### Data Flow

```
Source Files
  → parser.py (Tree-sitter AST → NodeInfo + EdgeInfo)
  → graph.py (SQLite store: upsert nodes/edges)
  → changes.py (git diff → changed ranges → map to nodes → risk score → context)
  → CLI / MCP server (expose to AI tools)
```

---

## Core Algorithm

### 1. Parsing (parser.py)

**Tree-sitter integration**: Uses `tree-sitter` and `tree-sitter-language-pack` to parse 25+ languages into ASTs.

**Extraction pipeline per file**:
1. Detect language from file extension
2. Parse source bytes into AST via tree-sitter
3. Create File node (with test status)
4. Pre-scan for imports and defined names (scope collection)
5. Recursive AST walk: extract classes, functions, imports, calls
6. Resolve call targets to qualified names
7. Generate TESTED_BY edges from test calls

**Node types**: File, Class, Function, Type, Test

**Edge types**: CALLS, IMPORTS_FROM, INHERITS, IMPLEMENTS, CONTAINS, TESTED_BY, DEPENDS_ON

**Qualification format**: `filepath::ClassName.method_name` or `filepath::function_name`

**Language-specific handlers**:
- Vue SFC: extract `<script>` blocks, adjust line numbers
- R: handle `<-` assignment, `setClass`/`setRefClass`
- Solidity: state variables, emit statements, modifiers
- TypeScript: path alias resolution via tsconfig.json

**Optimizations**:
- AST depth guard: max 180 levels
- Module resolution cache: evicts at 15K entries
- Test description capping: 200 chars

### 2. Graph Construction (graph.py)

**Storage**: SQLite with WAL journal mode for concurrent access.

**Tables**:
- `nodes`: id, kind, name, qualified_name (UNIQUE), file_path, line_start, line_end, language, parent_name, params, return_type, is_test, file_hash, extra (JSON)
- `edges`: id, kind, source_qualified, target_qualified, file_path, line, extra (JSON)
- `metadata`: key-value store (schema version, etc.)

**Key indices**: file_path, kind, qualified_name, source/target qualified names

**GraphStore class** (context manager):
- Write: `upsert_node()`, `upsert_edge()`, `remove_file_data()`, `store_file_nodes_edges()`
- Read: `get_node()`, `get_nodes_by_file()`, `search_nodes()`, `get_edges_by_source/target()`
- Traversal: `get_impact_radius()` -- BFS via NetworkX, respects depth/node limits
- Stats: `get_stats()`, `get_all_files()`
- Communities: `get_node_community_id()`, `get_community_member_qns()`

**Batch safety**: Splits queries > 450 items to stay under SQLite 999-variable limit.

**Thread safety**: `threading.Lock` for NetworkX graph cache.

### 3. Diff-Aware Context Retrieval (changes.py)

**Step 1: Parse git diff**
- `parse_git_diff_ranges(repo_root, base="HEAD~1")` -- runs `git diff --unified=0`
- `_parse_unified_diff(diff_text)` -- extracts `@@ -old,count +new,count @@` hunks
- Returns: `dict[str, list[tuple[int, int]]]` (file -> line ranges)
- Ref validation via regex, 30s timeout (configurable via `CRG_GIT_TIMEOUT` env var)

**Step 2: Map changes to nodes**
- `map_changes_to_nodes(store, changed_ranges)` -- overlap detection
- Condition: `node.line_start <= range_end AND node.line_end >= range_start`
- Deduplicates by qualified_name

**Step 3: Risk scoring**
- `compute_risk_score(store, node)` -- normalized 0.0-1.0 from 5 factors:
  - Flow participation: 0.05/flow, cap 0.25
  - Community crossing: 0.05/cross-community caller, cap 0.15
  - Test coverage: 0.30 (untested) / 0.05 (tested)
  - Security keywords: +0.20 if name matches
  - Caller count: `count/20`, cap 0.10

**Step 4: Analyze changes**
- Filters to Function/Test/Class kinds
- Retrieves affected flows
- Identifies test gaps (no TESTED_BY edges)
- Returns top 10 by risk score as review priorities

### 4. Token Budget Optimization

Token optimization is implicit: by selecting only the relevant subgraph (changed nodes + their callers/callees within max_depth), the system drastically reduces the context window needed. The `get_impact_radius()` method with `max_depth=2` and `max_nodes=500` bounds the output.

No explicit knapsack/packing algorithm found -- the optimization is structural (graph traversal bounded by depth/node count).

---

## Dependencies

### Required
- `tree-sitter` (>=0.23.0) + `tree-sitter-language-pack` -- AST parsing
- `networkx` (>=3.2) -- graph traversal, BFS, impact radius
- `mcp` + `fastmcp` -- MCP server protocol
- `watchdog` (>=4.0.0) -- file system watching

### Optional
- `sentence-transformers` / `google-generativeai` -- semantic embeddings
- `igraph` -- community detection
- `matplotlib` + `pyyaml` -- evaluation benchmarks
- `ollama` -- wiki generation

---

## Weaknesses & Improvement Opportunities

### 1. Heavy Dependencies
- Tree-sitter requires C bindings + language grammars (~200MB+ installed)
- NetworkX is pure Python, slow for large graphs
- Full install pulls in torch, numpy, transformers for optional features
- **Go advantage**: `go/ast` is stdlib, no C deps; custom graph is faster than NetworkX

### 2. Performance
- Single-threaded parsing (no parallel file processing)
- NetworkX BFS is O(V+E) but with Python overhead
- SQLite writes are serialized
- **Go advantage**: goroutine-per-file parsing, native concurrency

### 3. Language Support Coupling
- All 25 languages handled in one massive parser.py file
- Adding a language requires understanding tree-sitter node types
- **Go advantage**: go/ast for Go files is first-class; regex for others is simpler

### 4. Memory Usage
- NetworkX graph cached in memory alongside SQLite
- Large monorepos can exceed memory
- **Go advantage**: lower per-object overhead, more efficient structs

### 5. Distribution
- Requires Python 3.10+ environment, pip install, tree-sitter compilation
- **Go advantage**: single static binary, zero runtime deps

### 6. No Token Budget Packing
- No explicit algorithm to fit context within a token limit
- Just bounded BFS -- may overshoot or undershoot
- **Our improvement**: implement actual token-aware context selection

### 7. Risk Scoring is Simple
- 5-factor linear combination with hard caps
- No learning or adaptation
- Could be improved with configurable weights

### 8. SQLite Lock Contention
- WAL mode helps but heavy concurrent writes can still block
- In-memory graph option could be faster for smaller projects

---

## Key Takeaways for Reimplementation

1. **Core value**: Map git diffs to affected code entities, select minimal context for review
2. **Must-have**: Parse -> Graph -> Diff -> Context pipeline
3. **Our approach**: go/ast for Go (perfect fidelity), regex for Python/TS (good enough)
4. **Skip**: MCP server, VS Code extension, community detection, embeddings, wiki
5. **Add**: Explicit token budget optimization, DOT export, parallel parsing
6. **Data structures**: In-memory graph (no SQLite needed for CLI tool), JSON persistence
