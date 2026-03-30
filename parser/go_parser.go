package parser

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strings"

	"github.com/JSLEEKR/codegraph-go/graph"
)

// parseGo uses go/ast to extract structure from Go source files.
func parseGo(filePath string, source []byte) ([]graph.Node, []graph.Edge, error) {
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, filePath, source, goparser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	var nodes []graph.Node
	var edges []graph.Edge

	isTest := isTestFile(filePath, LangGo)

	// Track imports for call resolution
	importMap := make(map[string]string) // alias/pkgName -> import path
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			alias = parts[len(parts)-1]
		}
		importMap[alias] = path

		// Add import edge
		edges = append(edges, graph.Edge{
			Kind:            graph.EdgeImportsFrom,
			SourceQualified: filePath,
			TargetQualified: path,
			FilePath:        filePath,
			Line:            fset.Position(imp.Pos()).Line,
		})
	}

	// Track defined names for call resolution
	definedFuncs := make(map[string]string) // funcName -> qualifiedName

	// Extract type declarations (structs, interfaces)
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			kind := graph.KindType
			switch typeSpec.Type.(type) {
			case *ast.StructType:
				kind = graph.KindClass
			case *ast.InterfaceType:
				kind = graph.KindType
			}

			qn := Qualify(filePath, "", typeSpec.Name.Name)
			node := graph.Node{
				Kind:          kind,
				Name:          typeSpec.Name.Name,
				QualifiedName: qn,
				FilePath:      filePath,
				LineStart:     fset.Position(typeSpec.Pos()).Line,
				LineEnd:       fset.Position(typeSpec.End()).Line,
				Language:      "go",
			}
			nodes = append(nodes, node)

			edges = append(edges, graph.Edge{
				Kind:            graph.EdgeContains,
				SourceQualified: filePath,
				TargetQualified: qn,
				FilePath:        filePath,
				Line:            node.LineStart,
			})

			// Extract interface embedding / struct embedding
			if st, ok := typeSpec.Type.(*ast.StructType); ok {
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 { // embedded field
						if ident, ok := field.Type.(*ast.Ident); ok {
							edges = append(edges, graph.Edge{
								Kind:            graph.EdgeInherits,
								SourceQualified: qn,
								TargetQualified: Qualify(filePath, "", ident.Name),
								FilePath:        filePath,
								Line:            fset.Position(field.Pos()).Line,
							})
						}
					}
				}
			}

			if iface, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				for _, method := range iface.Methods.List {
					if len(method.Names) == 0 { // embedded interface
						if ident, ok := method.Type.(*ast.Ident); ok {
							edges = append(edges, graph.Edge{
								Kind:            graph.EdgeInherits,
								SourceQualified: qn,
								TargetQualified: Qualify(filePath, "", ident.Name),
								FilePath:        filePath,
								Line:            fset.Position(method.Pos()).Line,
							})
						}
					}
				}
			}
		}
	}

	// Extract function declarations
	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		name := funcDecl.Name.Name
		var parentName string
		var receiver string

		// Check for method receiver
		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			recv := funcDecl.Recv.List[0]
			receiver = extractReceiverType(recv.Type)
			parentName = receiver
		}

		funcKind := graph.KindFunction
		if isTest && (strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Example")) {
			funcKind = graph.KindTest
		}

		qn := Qualify(filePath, parentName, name)
		definedFuncs[name] = qn

		params := extractGoParams(fset, funcDecl.Type.Params)
		retType := extractGoResults(fset, funcDecl.Type.Results)

		funcNode := graph.Node{
			Kind:          funcKind,
			Name:          name,
			QualifiedName: qn,
			FilePath:      filePath,
			LineStart:     fset.Position(funcDecl.Pos()).Line,
			LineEnd:       fset.Position(funcDecl.End()).Line,
			Language:      "go",
			ParentName:    parentName,
			Params:        params,
			ReturnType:    retType,
			IsTest:        funcKind == graph.KindTest,
		}
		nodes = append(nodes, funcNode)

		// CONTAINS edge
		containerQN := filePath
		if parentName != "" {
			containerQN = Qualify(filePath, "", parentName)
		}
		edges = append(edges, graph.Edge{
			Kind:            graph.EdgeContains,
			SourceQualified: containerQN,
			TargetQualified: qn,
			FilePath:        filePath,
			Line:            funcNode.LineStart,
		})

		// Extract calls within the function body
		if funcDecl.Body != nil {
			calls := extractGoCalls(fset, funcDecl.Body, filePath, definedFuncs, importMap)
			for i := range calls {
				calls[i].SourceQualified = qn
			}
			edges = append(edges, calls...)

			// TESTED_BY edges for test functions
			if funcNode.IsTest {
				for _, call := range calls {
					if call.Kind == graph.EdgeCalls {
						edges = append(edges, graph.Edge{
							Kind:            graph.EdgeTestedBy,
							SourceQualified: call.TargetQualified,
							TargetQualified: qn,
							FilePath:        filePath,
							Line:            call.Line,
						})
					}
				}
			}
		}
	}

	return nodes, edges, nil
}

func extractReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return extractReceiverType(t.X)
	case *ast.IndexExpr:
		return extractReceiverType(t.X)
	case *ast.IndexListExpr:
		return extractReceiverType(t.X)
	}
	return ""
}

func extractGoParams(fset *token.FileSet, fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return "()"
	}
	var parts []string
	for _, f := range fields.List {
		typeStr := exprToString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typeStr)
		} else {
			for _, name := range f.Names {
				parts = append(parts, name.Name+" "+typeStr)
			}
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func extractGoResults(fset *token.FileSet, fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	var parts []string
	for _, f := range fields.List {
		typeStr := exprToString(f.Type)
		if len(f.Names) > 0 {
			for _, name := range f.Names {
				parts = append(parts, name.Name+" "+typeStr)
			}
		} else {
			parts = append(parts, typeStr)
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func"
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	case *ast.ChanType:
		return "chan " + exprToString(t.Value)
	case *ast.IndexExpr:
		return exprToString(t.X) + "[" + exprToString(t.Index) + "]"
	case *ast.IndexListExpr:
		var indices []string
		for _, idx := range t.Indices {
			indices = append(indices, exprToString(idx))
		}
		return exprToString(t.X) + "[" + strings.Join(indices, ", ") + "]"
	}
	return "any"
}

func extractGoCalls(fset *token.FileSet, body *ast.BlockStmt, filePath string, definedFuncs map[string]string, importMap map[string]string) []graph.Edge {
	var calls []graph.Edge

	ast.Inspect(body, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		line := fset.Position(callExpr.Pos()).Line
		var targetQN string

		switch fn := callExpr.Fun.(type) {
		case *ast.Ident:
			// Direct function call: foo()
			if qn, ok := definedFuncs[fn.Name]; ok {
				targetQN = qn
			} else {
				targetQN = fn.Name
			}
		case *ast.SelectorExpr:
			// Method or package call: pkg.Func() or obj.Method()
			if ident, ok := fn.X.(*ast.Ident); ok {
				if importPath, ok := importMap[ident.Name]; ok {
					// Package function call
					targetQN = importPath + "::" + fn.Sel.Name
				} else {
					// Method call on local variable
					targetQN = Qualify(filePath, ident.Name, fn.Sel.Name)
				}
			}
		}

		if targetQN != "" {
			calls = append(calls, graph.Edge{
				Kind:            graph.EdgeCalls,
				TargetQualified: targetQN,
				FilePath:        filePath,
				Line:            line,
			})
		}

		return true
	})

	return calls
}
