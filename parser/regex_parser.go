package parser

import (
	"regexp"
	"strings"

	"github.com/JSLEEKR/codegraph-go/graph"
)

// Python patterns
var (
	pyClassRe    = regexp.MustCompile(`^(\s*)class\s+(\w+)\s*(?:\(([^)]*)\))?\s*:`)
	pyFuncRe     = regexp.MustCompile(`^(\s*)def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*(\S+))?\s*:`)
	pyImportRe   = regexp.MustCompile(`^(?:from\s+(\S+)\s+)?import\s+(.+)`)
	pyCallRe     = regexp.MustCompile(`(?:^|[^.\w])(\w+)\s*\(`)
	pyMethodCall = regexp.MustCompile(`(\w+)\.(\w+)\s*\(`)
	pyDecorRe    = regexp.MustCompile(`^(\s*)@(\w+)`)
)

// TypeScript/JavaScript patterns
var (
	tsClassRe    = regexp.MustCompile(`^(?:export\s+)?(?:abstract\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?(?:\s+implements\s+([^{]+))?`)
	tsFuncRe     = regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*(?:<[^>]+>)?\s*\(([^)]*)\)(?:\s*:\s*(\S+))?`)
	tsMethodRe   = regexp.MustCompile(`^\s+(?:(?:public|private|protected|static|async|readonly)\s+)*(\w+)\s*(?:<[^>]+>)?\s*\(([^)]*)\)(?:\s*:\s*(\S+))?`)
	tsArrowRe    = regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::\s*\S+\s*)?=\s*(?:async\s+)?(?:\([^)]*\)|[^=]+)\s*=>`)
	tsImportRe   = regexp.MustCompile(`^import\s+(?:(?:\{[^}]+\}|[\w*]+)\s+from\s+)?['"]([^'"]+)['"]`)
	tsCallRe     = regexp.MustCompile(`(?:^|[^.\w])(\w+)\s*\(`)
	tsMethodCallRe = regexp.MustCompile(`(\w+)\.(\w+)\s*\(`)
	tsInterfaceRe  = regexp.MustCompile(`^(?:export\s+)?interface\s+(\w+)(?:\s+extends\s+([^{]+))?`)
	tsTypeRe       = regexp.MustCompile(`^(?:export\s+)?type\s+(\w+)`)
)

// parsePython extracts code structure from Python source using regex.
func parsePython(filePath string, source []byte) ([]graph.Node, []graph.Edge) {
	lines := strings.Split(string(source), "\n")
	isTest := isTestFile(filePath, LangPython)

	var nodes []graph.Node
	var edges []graph.Edge

	// First pass: collect classes and functions
	var scopes []scopeEntry
	definedNames := make(map[string]string) // name -> qualifiedName
	importMap := make(map[string]string)    // alias -> module

	for i, line := range lines {
		lineNum := i + 1

		// Imports
		if m := pyImportRe.FindStringSubmatch(line); m != nil {
			fromModule := m[1]
			imports := m[2]
			if fromModule != "" {
				for _, imp := range strings.Split(imports, ",") {
					imp = strings.TrimSpace(imp)
					parts := strings.SplitN(imp, " as ", 2)
					name := strings.TrimSpace(parts[0])
					alias := name
					if len(parts) > 1 {
						alias = strings.TrimSpace(parts[1])
					}
					importMap[alias] = fromModule + "." + name
					edges = append(edges, graph.Edge{
						Kind:            graph.EdgeImportsFrom,
						SourceQualified: filePath,
						TargetQualified: fromModule,
						FilePath:        filePath,
						Line:            lineNum,
					})
				}
			} else {
				for _, imp := range strings.Split(imports, ",") {
					imp = strings.TrimSpace(imp)
					parts := strings.SplitN(imp, " as ", 2)
					name := strings.TrimSpace(parts[0])
					alias := name
					if len(parts) > 1 {
						alias = strings.TrimSpace(parts[1])
					}
					importMap[alias] = name
					edges = append(edges, graph.Edge{
						Kind:            graph.EdgeImportsFrom,
						SourceQualified: filePath,
						TargetQualified: name,
						FilePath:        filePath,
						Line:            lineNum,
					})
				}
			}
			continue
		}

		// Classes
		if m := pyClassRe.FindStringSubmatch(line); m != nil {
			indent := len(m[1])
			className := m[2]
			bases := m[3]

			// Close scopes at same or lower indent
			scopes = closeScopes(scopes, indent, lines, i, &nodes)

			qn := currentQualifiedName(filePath, scopes, className)
			definedNames[className] = qn

			scopes = append(scopes, scopeEntry{
				name:   className,
				indent: indent,
				kind:   graph.KindClass,
				start:  lineNum,
			})

			// Add CONTAINS edge
			containerQN := filePath
			if len(scopes) > 1 {
				containerQN = currentQualifiedName(filePath, scopes[:len(scopes)-1], scopes[len(scopes)-2].name)
			}
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeContains,
				SourceQualified: containerQN,
				TargetQualified: qn,
				FilePath:        filePath,
				Line:            lineNum,
			})

			// Inheritance
			if bases != "" {
				for _, base := range strings.Split(bases, ",") {
					base = strings.TrimSpace(base)
					if base != "" && base != "object" {
						edges = append(edges, graph.Edge{
							Kind:            graph.EdgeInherits,
							SourceQualified: qn,
							TargetQualified: resolveTarget(filePath, base, importMap),
							FilePath:        filePath,
							Line:            lineNum,
						})
					}
				}
			}
			continue
		}

		// Functions/methods
		if m := pyFuncRe.FindStringSubmatch(line); m != nil {
			indent := len(m[1])
			funcName := m[2]
			params := m[3]
			retType := m[4]

			scopes = closeScopes(scopes, indent, lines, i, &nodes)

			funcKind := graph.KindFunction
			if isTest && strings.HasPrefix(funcName, "test_") {
				funcKind = graph.KindTest
			}

			var parentName string
			if len(scopes) > 0 && scopes[len(scopes)-1].kind == graph.KindClass {
				parentName = scopes[len(scopes)-1].name
			}

			qn := currentQualifiedName(filePath, scopes, funcName)
			definedNames[funcName] = qn

			scopes = append(scopes, scopeEntry{
				name:   funcName,
				indent: indent,
				kind:   funcKind,
				start:  lineNum,
			})

			_ = params  // stored in node
			_ = retType // stored in node

			containerQN := filePath
			if parentName != "" {
				containerQN = Qualify(filePath, "", parentName)
			}
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeContains,
				SourceQualified: containerQN,
				TargetQualified: qn,
				FilePath:        filePath,
				Line:            lineNum,
			})

			// We'll create the node when the scope closes to get line_end
			continue
		}
	}

	// Close remaining scopes
	scopes = closeScopes(scopes, -1, lines, len(lines), &nodes)

	// Second pass: extract calls
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "import") || strings.HasPrefix(trimmed, "from") {
			continue
		}

		// Find which function this line belongs to
		callerQN := findEnclosingFunc(filePath, nodes, lineNum)
		if callerQN == "" {
			continue
		}

		// Method calls: obj.method()
		for _, m := range pyMethodCall.FindAllStringSubmatch(line, -1) {
			obj := m[1]
			method := m[2]
			if obj == "self" || obj == "cls" {
				// Find the enclosing class
				targetQN := resolveTarget(filePath, method, definedNames)
				edges = append(edges, graph.Edge{
					Kind:            graph.EdgeCalls,
					SourceQualified: callerQN,
					TargetQualified: targetQN,
					FilePath:        filePath,
					Line:            lineNum,
				})
			} else {
				targetQN := resolveTarget(filePath, obj+"."+method, importMap)
				edges = append(edges, graph.Edge{
					Kind:            graph.EdgeCalls,
					SourceQualified: callerQN,
					TargetQualified: targetQN,
					FilePath:        filePath,
					Line:            lineNum,
				})
			}
		}

		// Direct function calls
		for _, m := range pyCallRe.FindAllStringSubmatch(line, -1) {
			funcName := m[1]
			// Skip common builtins and keywords
			if isPythonBuiltin(funcName) {
				continue
			}
			// Skip if already captured as method call
			if pyMethodCall.MatchString(line) && strings.Contains(line, "."+funcName+"(") {
				continue
			}
			targetQN := resolveTarget(filePath, funcName, definedNames)
			if targetQN == funcName {
				targetQN = resolveTarget(filePath, funcName, importMap)
			}
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeCalls,
				SourceQualified: callerQN,
				TargetQualified: targetQN,
				FilePath:        filePath,
				Line:            lineNum,
			})
		}
	}

	// Add TESTED_BY edges for test functions
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			// Check if source is a test function
			for _, n := range nodes {
				if n.QualifiedName == e.SourceQualified && n.IsTest {
					edges = append(edges, graph.Edge{
						Kind:            graph.EdgeTestedBy,
						SourceQualified: e.TargetQualified,
						TargetQualified: e.SourceQualified,
						FilePath:        e.FilePath,
						Line:            e.Line,
					})
					break
				}
			}
		}
	}

	return nodes, edges
}

// parseTypeScript extracts code structure from TypeScript/JavaScript source using regex.
func parseTypeScript(filePath string, source []byte) ([]graph.Node, []graph.Edge) {
	lines := strings.Split(string(source), "\n")
	isTest := isTestFile(filePath, LangTypeScript)

	var nodes []graph.Node
	var edges []graph.Edge

	definedNames := make(map[string]string) // name -> qualifiedName
	importMap := make(map[string]string)    // module alias -> path

	var currentClass string
	var braceDepth int
	var classStartLine int

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Imports
		if m := tsImportRe.FindStringSubmatch(trimmed); m != nil {
			modulePath := m[1]
			importMap[modulePath] = modulePath
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeImportsFrom,
				SourceQualified: filePath,
				TargetQualified: modulePath,
				FilePath:        filePath,
				Line:            lineNum,
			})
			continue
		}

		// Interfaces
		if m := tsInterfaceRe.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			extends := m[2]
			qn := Qualify(filePath, "", name)
			definedNames[name] = qn

			node := graph.Node{
				Kind:          graph.KindType,
				Name:          name,
				QualifiedName: qn,
				FilePath:      filePath,
				LineStart:     lineNum,
				LineEnd:       findBlockEnd(lines, i),
				Language:      string(DetectLanguage(filePath)),
			}
			nodes = append(nodes, node)
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeContains,
				SourceQualified: filePath,
				TargetQualified: qn,
				FilePath:        filePath,
				Line:            lineNum,
			})

			if extends != "" {
				for _, base := range strings.Split(extends, ",") {
					base = strings.TrimSpace(base)
					if base != "" {
						edges = append(edges, graph.Edge{
							Kind:            graph.EdgeInherits,
							SourceQualified: qn,
							TargetQualified: resolveTarget(filePath, base, definedNames),
							FilePath:        filePath,
							Line:            lineNum,
						})
					}
				}
			}
			continue
		}

		// Type aliases
		if m := tsTypeRe.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			qn := Qualify(filePath, "", name)
			definedNames[name] = qn
			node := graph.Node{
				Kind:          graph.KindType,
				Name:          name,
				QualifiedName: qn,
				FilePath:      filePath,
				LineStart:     lineNum,
				LineEnd:       lineNum,
				Language:      string(DetectLanguage(filePath)),
			}
			nodes = append(nodes, node)
			continue
		}

		// Classes
		if m := tsClassRe.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			extends := m[2]
			implements := m[3]

			currentClass = name
			classStartLine = lineNum
			braceDepth = 0

			qn := Qualify(filePath, "", name)
			definedNames[name] = qn

			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeContains,
				SourceQualified: filePath,
				TargetQualified: qn,
				FilePath:        filePath,
				Line:            lineNum,
			})

			if extends != "" {
				edges = append(edges, graph.Edge{
					Kind:            graph.EdgeInherits,
					SourceQualified: qn,
					TargetQualified: resolveTarget(filePath, extends, definedNames),
					FilePath:        filePath,
					Line:            lineNum,
				})
			}
			if implements != "" {
				for _, iface := range strings.Split(implements, ",") {
					iface = strings.TrimSpace(iface)
					if iface != "" {
						edges = append(edges, graph.Edge{
							Kind:            graph.EdgeImplements,
							SourceQualified: qn,
							TargetQualified: resolveTarget(filePath, iface, definedNames),
							FilePath:        filePath,
							Line:            lineNum,
						})
					}
				}
			}
			continue
		}

		// Track class braces to know when class ends
		if currentClass != "" {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 && i > classStartLine-1 {
				qn := Qualify(filePath, "", currentClass)
				node := graph.Node{
					Kind:          graph.KindClass,
					Name:          currentClass,
					QualifiedName: qn,
					FilePath:      filePath,
					LineStart:     classStartLine,
					LineEnd:       lineNum,
					Language:      string(DetectLanguage(filePath)),
				}
				nodes = append(nodes, node)
				currentClass = ""
			}
		}

		// Methods inside class
		if currentClass != "" {
			if m := tsMethodRe.FindStringSubmatch(line); m != nil {
				name := m[1]
				params := m[2]
				retType := m[3]
				if name == "constructor" || name == "get" || name == "set" {
					continue
				}
				qn := Qualify(filePath, currentClass, name)
				definedNames[name] = qn

				funcKind := graph.KindFunction
				if isTest && (strings.HasPrefix(name, "test") || strings.HasPrefix(name, "should")) {
					funcKind = graph.KindTest
				}

				node := graph.Node{
					Kind:          funcKind,
					Name:          name,
					QualifiedName: qn,
					FilePath:      filePath,
					LineStart:     lineNum,
					LineEnd:       findBlockEnd(lines, i),
					Language:      string(DetectLanguage(filePath)),
					ParentName:    currentClass,
					Params:        "(" + params + ")",
					ReturnType:    retType,
					IsTest:        funcKind == graph.KindTest,
				}
				nodes = append(nodes, node)
				edges = append(edges, graph.Edge{
					Kind:            graph.EdgeContains,
					SourceQualified: Qualify(filePath, "", currentClass),
					TargetQualified: qn,
					FilePath:        filePath,
					Line:            lineNum,
				})
				continue
			}
		}

		// Top-level functions
		if m := tsFuncRe.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			params := m[2]
			retType := m[3]

			funcKind := graph.KindFunction
			if isTest && isTestFuncName(name) {
				funcKind = graph.KindTest
			}

			qn := Qualify(filePath, "", name)
			definedNames[name] = qn

			node := graph.Node{
				Kind:          funcKind,
				Name:          name,
				QualifiedName: qn,
				FilePath:      filePath,
				LineStart:     lineNum,
				LineEnd:       findBlockEnd(lines, i),
				Language:      string(DetectLanguage(filePath)),
				Params:        "(" + params + ")",
				ReturnType:    retType,
				IsTest:        funcKind == graph.KindTest,
			}
			nodes = append(nodes, node)
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeContains,
				SourceQualified: filePath,
				TargetQualified: qn,
				FilePath:        filePath,
				Line:            lineNum,
			})
			continue
		}

		// Arrow functions
		if m := tsArrowRe.FindStringSubmatch(trimmed); m != nil {
			name := m[1]

			funcKind := graph.KindFunction
			if isTest && isTestFuncName(name) {
				funcKind = graph.KindTest
			}

			qn := Qualify(filePath, "", name)
			definedNames[name] = qn

			node := graph.Node{
				Kind:          funcKind,
				Name:          name,
				QualifiedName: qn,
				FilePath:      filePath,
				LineStart:     lineNum,
				LineEnd:       findBlockEnd(lines, i),
				Language:      string(DetectLanguage(filePath)),
				IsTest:        funcKind == graph.KindTest,
			}
			nodes = append(nodes, node)
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeContains,
				SourceQualified: filePath,
				TargetQualified: qn,
				FilePath:        filePath,
				Line:            lineNum,
			})
			continue
		}
	}

	// If class wasn't closed (malformed), still add it
	if currentClass != "" {
		qn := Qualify(filePath, "", currentClass)
		node := graph.Node{
			Kind:          graph.KindClass,
			Name:          currentClass,
			QualifiedName: qn,
			FilePath:      filePath,
			LineStart:     classStartLine,
			LineEnd:       len(lines),
			Language:      string(DetectLanguage(filePath)),
		}
		nodes = append(nodes, node)
	}

	// Extract calls (second pass)
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "import") {
			continue
		}

		callerQN := findEnclosingFunc(filePath, nodes, lineNum)
		if callerQN == "" {
			continue
		}

		for _, m := range tsMethodCallRe.FindAllStringSubmatch(line, -1) {
			obj := m[1]
			method := m[2]
			_ = obj
			targetQN := resolveTarget(filePath, method, definedNames)
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeCalls,
				SourceQualified: callerQN,
				TargetQualified: targetQN,
				FilePath:        filePath,
				Line:            lineNum,
			})
		}

		for _, m := range tsCallRe.FindAllStringSubmatch(line, -1) {
			funcName := m[1]
			if isJSBuiltin(funcName) {
				continue
			}
			if tsMethodCallRe.MatchString(line) && strings.Contains(line, "."+funcName+"(") {
				continue
			}
			targetQN := resolveTarget(filePath, funcName, definedNames)
			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeCalls,
				SourceQualified: callerQN,
				TargetQualified: targetQN,
				FilePath:        filePath,
				Line:            lineNum,
			})
		}
	}

	return nodes, edges
}

// Helper functions

func closeScopes(scopes []scopeEntry, indent int, lines []string, currentLine int, nodes *[]graph.Node) []scopeEntry {
	for len(scopes) > 0 {
		top := scopes[len(scopes)-1]
		if indent >= 0 && top.indent < indent {
			break
		}
		scopes = scopes[:len(scopes)-1]

		lineEnd := currentLine
		if lineEnd > len(lines) {
			lineEnd = len(lines)
		}

		// Build qualified name
		var parentName string
		for _, s := range scopes {
			if s.kind == graph.KindClass {
				parentName = s.name
			}
		}

		// closeScopes creates nodes without filePath/qualifiedName;
		// the caller (parsePython) sets these on the returned nodes.
		_ = parentName

		if top.kind == graph.KindFunction || top.kind == graph.KindTest {
			node := graph.Node{
				Kind:       top.kind,
				Name:       top.name,
				LineStart:  top.start,
				LineEnd:    lineEnd,
				ParentName: parentName,
				IsTest:     top.kind == graph.KindTest,
			}
			*nodes = append(*nodes, node)
		} else if top.kind == graph.KindClass {
			node := graph.Node{
				Kind:      top.kind,
				Name:      top.name,
				LineStart: top.start,
				LineEnd:   lineEnd,
			}
			*nodes = append(*nodes, node)
		}

		if indent < 0 {
			continue // close all
		}
	}
	return scopes
}

// scopeEntry is defined as a local struct in parsePython; we use it at package level
// for helper functions.
type scopeEntry struct {
	name   string
	indent int
	kind   graph.NodeKind
	start  int
}

func currentQualifiedName(filePath string, scopes []scopeEntry, name string) string {
	for i := len(scopes) - 1; i >= 0; i-- {
		if scopes[i].kind == graph.KindClass {
			return Qualify(filePath, scopes[i].name, name)
		}
	}
	return Qualify(filePath, "", name)
}

func resolveTarget(filePath, name string, lookupMap map[string]string) string {
	if qn, ok := lookupMap[name]; ok {
		return qn
	}
	return name
}

func findEnclosingFunc(filePath string, nodes []graph.Node, lineNum int) string {
	var best *graph.Node
	for i := range nodes {
		n := &nodes[i]
		if (n.Kind == graph.KindFunction || n.Kind == graph.KindTest) &&
			n.LineStart <= lineNum && n.LineEnd >= lineNum {
			if best == nil || (n.LineEnd-n.LineStart) < (best.LineEnd-best.LineStart) {
				best = n
			}
		}
	}
	if best != nil {
		if best.QualifiedName != "" {
			return best.QualifiedName
		}
		return Qualify(filePath, best.ParentName, best.Name)
	}
	return ""
}

func findBlockEnd(lines []string, startIdx int) int {
	depth := 0
	for i := startIdx; i < len(lines); i++ {
		depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		if depth <= 0 && i > startIdx {
			return i + 1
		}
	}
	return len(lines)
}

func isTestFuncName(name string) bool {
	return strings.HasPrefix(name, "test") || strings.HasPrefix(name, "Test") ||
		strings.HasPrefix(name, "should") || strings.HasSuffix(name, "Test") ||
		strings.HasSuffix(name, "Spec")
}

func isPythonBuiltin(name string) bool {
	builtins := map[string]bool{
		"print": true, "len": true, "range": true, "str": true, "int": true,
		"float": true, "list": true, "dict": true, "set": true, "tuple": true,
		"bool": true, "type": true, "isinstance": true, "issubclass": true,
		"hasattr": true, "getattr": true, "setattr": true, "delattr": true,
		"super": true, "property": true, "staticmethod": true, "classmethod": true,
		"open": true, "enumerate": true, "zip": true, "map": true, "filter": true,
		"sorted": true, "reversed": true, "any": true, "all": true, "min": true,
		"max": true, "sum": true, "abs": true, "round": true, "hash": true,
		"id": true, "input": true, "repr": true, "format": true, "vars": true,
		"dir": true, "globals": true, "locals": true, "iter": true, "next": true,
		"callable": true, "exec": true, "eval": true, "compile": true,
		"ValueError": true, "TypeError": true, "KeyError": true, "Exception": true,
		"RuntimeError": true, "OSError": true, "IOError": true, "IndexError": true,
		"AttributeError": true, "ImportError": true, "NotImplementedError": true,
		"if": true, "else": true, "elif": true, "for": true, "while": true,
		"return": true, "yield": true, "pass": true, "break": true, "continue": true,
		"with": true, "as": true, "try": true, "except": true, "finally": true,
		"raise": true, "assert": true, "not": true, "and": true, "or": true,
		"in": true, "is": true, "lambda": true, "True": true, "False": true,
		"None": true, "bytes": true, "bytearray": true, "memoryview": true,
		"object": true, "complex": true, "frozenset": true, "chr": true, "ord": true,
	}
	return builtins[name]
}

func isJSBuiltin(name string) bool {
	builtins := map[string]bool{
		"console": true, "require": true, "setTimeout": true, "setInterval": true,
		"clearTimeout": true, "clearInterval": true, "Promise": true,
		"Array": true, "Object": true, "String": true, "Number": true,
		"Boolean": true, "Map": true, "Set": true, "Date": true, "Math": true,
		"JSON": true, "Error": true, "TypeError": true, "RegExp": true,
		"parseInt": true, "parseFloat": true, "isNaN": true, "isFinite": true,
		"encodeURI": true, "decodeURI": true, "encodeURIComponent": true,
		"decodeURIComponent": true, "fetch": true, "Response": true,
		"Request": true, "Headers": true, "URL": true, "URLSearchParams": true,
		"Buffer": true, "process": true, "global": true, "window": true,
		"document": true, "undefined": true, "null": true, "NaN": true,
		"Infinity": true, "Symbol": true, "BigInt": true, "WeakMap": true,
		"WeakSet": true, "Proxy": true, "Reflect": true, "Intl": true,
		"if": true, "else": true, "for": true, "while": true, "do": true,
		"switch": true, "case": true, "break": true, "continue": true,
		"return": true, "throw": true, "try": true, "catch": true, "finally": true,
		"new": true, "delete": true, "typeof": true, "instanceof": true,
		"void": true, "in": true, "of": true, "with": true, "yield": true,
		"await": true, "async": true, "this": true, "super": true,
		"describe": true, "it": true, "test": true, "expect": true,
		"beforeEach": true, "afterEach": true, "beforeAll": true, "afterAll": true,
	}
	return builtins[name]
}
