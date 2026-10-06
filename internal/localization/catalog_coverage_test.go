package localization

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const coverageModule = "github.com/PHPCraftdream/HotamSpec"
const localizationPackage = coverageModule + "/internal/localization"

type coverageFile struct {
	path, pkg, locAlias string
	fset                *token.FileSet
	file                *ast.File
}
type coverageFunc struct {
	params map[string]int
	body   ast.Node
	file   *coverageFile
}
type coverageKey struct{ pkg, name string }

// Checks direct and transitively disclosed wrapper templates with package-level const resolution. Plain variables and fields are not tracked.
func TestCatalogCoversAllTextLookupTemplates(t *testing.T) {
	_, own, _, _ := runtime.Caller(0)
	root := filepath.Dir(own)
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("repository root not found")
		}
		root = parent
	}
	files := []*coverageFile{}
	consts := map[string]map[string]ast.Expr{}
	funcs := map[coverageKey]coverageFunc{}
	candidateTypes := map[coverageKey]bool{}
	parseErrors, walkErrors, filesWalked, importing := 0, 0, 0, 0
	var issues []string
	for _, tree := range []string{"internal", "cmd"} {
		base := filepath.Join(root, tree)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				walkErrors++
				issues = append(issues, "walk "+filepath.ToSlash(path)+": "+err.Error())
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			filesWalked++
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				parseErrors++
				issues = append(issues, "parse "+filepath.ToSlash(path)+": "+err.Error())
				return nil
			}
			rel, _ := filepath.Rel(root, filepath.Dir(path))
			pkg := coverageModule
			if rel != "." {
				pkg += "/" + filepath.ToSlash(rel)
			}
			cf := &coverageFile{path: filepath.ToSlash(path), pkg: pkg, fset: fset, file: file}
			for _, im := range file.Imports {
				ip, e := strconv.Unquote(im.Path.Value)
				if e != nil {
					continue
				}
				alias := pathBaseImport(ip)
				if im.Name != nil {
					alias = im.Name.Name
				}
				if ip == localizationPackage {
					cf.locAlias = alias
				}
			}
			if cf.locAlias != "" {
				importing++
			}
			if consts[pkg] == nil {
				consts[pkg] = map[string]ast.Expr{}
			}
			for _, d := range file.Decls {
				switch x := d.(type) {
				case *ast.GenDecl:
					if x.Tok == token.TYPE {
						for _, spec := range x.Specs {
							ts := spec.(*ast.TypeSpec)
							if _, ok := ts.Type.(*ast.FuncType); ok {
								candidateTypes[coverageKey{pkg, ts.Name.Name}] = true
							}
						}
					}
					if x.Tok == token.CONST {
						for _, sp := range x.Specs {
							vs := sp.(*ast.ValueSpec)
							for i, n := range vs.Names {
								if i < len(vs.Values) {
									consts[pkg][n.Name] = vs.Values[i]
								}
							}
						}
					}
				case *ast.FuncDecl:
					if x.Recv == nil && x.Body != nil {
						funcs[coverageKey{pkg, x.Name.Name}] = coverageFunc{paramPositions(x.Type), x.Body, cf}
					}
				}
			}
			files = append(files, cf)
			return nil
		})
		if err != nil {
			walkErrors++
			issues = append(issues, "walk "+filepath.ToSlash(base)+": "+err.Error())
		}
	}
	wrappers := map[coverageKey]map[int]bool{}
	wrapperTypes := map[coverageKey]map[int]bool{}
	resolveCallee := func(c *ast.CallExpr, f *coverageFile) (coverageKey, bool) {
		switch fn := c.Fun.(type) {
		case *ast.SelectorExpr:
			id, ok := fn.X.(*ast.Ident)
			if !ok {
				return coverageKey{}, false
			}
			if id.Name == f.locAlias && f.locAlias != "" {
				return coverageKey{"localization", fn.Sel.Name}, true
			}
			for _, im := range f.file.Imports {
				ip, e := strconv.Unquote(im.Path.Value)
				if e != nil {
					continue
				}
				a := pathBaseImport(ip)
				if im.Name != nil {
					a = im.Name.Name
				}
				if a == id.Name {
					return coverageKey{ip, fn.Sel.Name}, true
				}
			}
		case *ast.Ident:
			return coverageKey{f.pkg, fn.Name}, true
		}
		return coverageKey{}, false
	}
	mark := func(k coverageKey, idx int) bool {
		if wrappers[k] == nil {
			wrappers[k] = map[int]bool{}
		}
		if wrappers[k][idx] {
			return false
		}
		wrappers[k][idx] = true
		return true
	}
	markType := func(k coverageKey, idx int) bool {
		if wrapperTypes[k] == nil {
			wrapperTypes[k] = map[int]bool{}
		}
		if wrapperTypes[k][idx] {
			return false
		}
		wrapperTypes[k][idx] = true
		return true
	}
	changed := true
	for changed {
		changed = false
		for k, fn := range funcs {
			paramIndex := fn.params
			ast.Inspect(fn.body, func(n ast.Node) bool {
				// A closure parameter shadowing an outer one hides it.
				if lit, ok := n.(*ast.FuncLit); ok {
					for name := range paramPositions(lit.Type) {
						if _, shadow := paramIndex[name]; shadow {
							return false
						}
					}
				}
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if typeName, ok := c.Fun.(*ast.Ident); ok && candidateTypes[coverageKey{k.pkg, typeName.Name}] && len(c.Args) >= 1 {
					if lit, ok := c.Args[0].(*ast.FuncLit); ok {
						litParams := paramPositions(lit.Type)
						ast.Inspect(lit.Body, func(nn ast.Node) bool {
							call, ok := nn.(*ast.CallExpr)
							if !ok {
								return true
							}
							callee, ok := resolveCallee(call, fn.file)
							if !ok {
								return true
							}
							indexes := []int{}
							if callee.pkg == "localization" && (callee.name == "Text" || callee.name == "Lookup") {
								indexes = append(indexes, 1)
							} else {
								for i := range wrappers[callee] {
									indexes = append(indexes, i)
								}
								for i := range wrapperTypes[callee] {
									indexes = append(indexes, i)
								}
							}
							for _, i := range indexes {
								if i < len(call.Args) {
									if id, ok := call.Args[i].(*ast.Ident); ok {
										if pi, yes := litParams[id.Name]; yes && markType(coverageKey{k.pkg, typeName.Name}, pi) {
											changed = true
										}
									}
								}
							}
							return true
						})
					}
				}
				callee, ok := resolveCallee(c, fn.file)
				if !ok {
					return true
				}
				indexes := []int{}
				if callee.pkg == "localization" && (callee.name == "Text" || callee.name == "Lookup") {
					if len(c.Args) > 1 {
						indexes = append(indexes, 1)
					}
				} else {
					for i := range wrappers[callee] {
						indexes = append(indexes, i)
					}
				}
				for _, i := range indexes {
					if i < len(c.Args) {
						if id, ok := c.Args[i].(*ast.Ident); ok {
							if pi, yes := paramIndex[id.Name]; yes && mark(k, pi) {
								changed = true
							}
						}
					}
				}
				return true
			})
		}
	}
	var missing []string
	resolved, skipped, directSites, wrapperSites, wrapperTypeSites := 0, 0, 0, 0, 0
	for _, f := range files {
		resolve := func(e ast.Expr) (string, bool) {
			for hops := 0; hops <= 2; hops++ {
				switch x := e.(type) {
				case *ast.BasicLit:
					if x.Kind == token.STRING {
						v, err := strconv.Unquote(x.Value)
						return v, err == nil
					}
					return "", false
				case *ast.Ident:
					if hops == 2 {
						return "", false
					}
					v, ok := consts[f.pkg][x.Name]
					if !ok {
						return "", false
					}
					e = v
				default:
					return "", false
				}
			}
			return "", false
		}
		// Function-typed parameters are scoped to the declaration that owns them.
		var scope map[string]coverageKey
		ast.Inspect(f.file, func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok {
				scope = map[string]coverageKey{}
				ast.Inspect(fd, func(m ast.Node) bool {
					ft, ok := m.(*ast.FuncType)
					if !ok || ft.Params == nil {
						return true
					}
					for _, field := range ft.Params.List {
						id, ok := field.Type.(*ast.Ident)
						if !ok {
							continue
						}
						key := coverageKey{f.pkg, id.Name}
						if len(wrapperTypes[key]) == 0 {
							continue
						}
						for _, name := range field.Names {
							scope[name.Name] = key
						}
					}
					return true
				})
				return true
			}
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fileScoped := scope
			k, ok := resolveCallee(c, f)
			if !ok {
				return true
			}
			typeSite := false
			if id, yes := c.Fun.(*ast.Ident); yes && len(wrappers[k]) == 0 {
				if typeKey, exists := fileScoped[id.Name]; exists {
					k = typeKey
					typeSite = true
				}
			}
			indexes := []int{}
			direct := k.pkg == "localization" && (k.name == "Text" || k.name == "Lookup")
			if direct {
				indexes = []int{1}
			} else if typeSite {
				for i := range wrapperTypes[k] {
					indexes = append(indexes, i)
				}
			} else {
				for i := range wrappers[k] {
					indexes = append(indexes, i)
				}
			}
			if len(indexes) == 0 {
				return true
			}
			if direct {
				directSites++
			} else if typeSite {
				wrapperTypeSites++
			} else {
				wrapperSites++
			}
			for _, p := range indexes {
				if p >= len(c.Args) {
					continue
				}
				template, ok := resolve(c.Args[p])
				if !ok {
					skipped++
					continue
				}
				resolved++
				for _, lang := range []string{"ru", "zh"} {
					if _, exists := catalogs[lang][template]; !exists {
						pos := f.fset.Position(c.Pos())
						missing = append(missing, template+" ["+lang+"] at "+f.path+":"+strconv.Itoa(pos.Line))
					}
				}
			}
			return true
		})
	}
	formatNames := func(m map[coverageKey]map[int]bool) []string {
		names := []string{}
		for k, idxs := range m {
			if len(idxs) == 0 {
				continue
			}
			is := []int{}
			for i := range idxs {
				is = append(is, i)
			}
			sort.Ints(is)
			parts := []string{}
			for _, i := range is {
				parts = append(parts, strconv.Itoa(i))
			}
			names = append(names, k.pkg+"."+k.name+" templateIdx=["+strings.Join(parts, ",")+"]")
		}
		sort.Strings(names)
		return names
	}
	wrapperNames := formatNames(wrappers)
	wrapperTypeNames := formatNames(wrapperTypes)
	t.Logf("filesWalked=%d parseErrors=%d walkErrors=%d localizationImportingFiles=%d", filesWalked, parseErrors, walkErrors, importing)
	t.Logf("wrappers=%d: %s", len(wrapperNames), strings.Join(wrapperNames, "; "))
	t.Logf("wrapperTypes=%d: %s", len(wrapperTypeNames), strings.Join(wrapperTypeNames, "; "))
	t.Logf("directSites=%d wrapperSites=%d wrapperTypeSites=%d resolvedTemplates=%d skippedDynamic=%d", directSites, wrapperSites, wrapperTypeSites, resolved, skipped)
	if resolved == 0 {
		t.Fatal("no statically resolved Text/Lookup templates found")
	}
	for _, issue := range issues {
		t.Errorf("%s", issue)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("missing catalog translations:\n%s", strings.Join(missing, "\n"))
	}
}

// paramPositions maps named parameters to their full signature position, counting unnamed ones.
func paramPositions(ft *ast.FuncType) map[string]int {
	out := map[string]int{}
	if ft.Params == nil {
		return out
	}
	idx := 0
	for _, field := range ft.Params.List {
		if len(field.Names) == 0 {
			idx++
			continue
		}
		for _, n := range field.Names {
			out[n.Name] = idx
			idx++
		}
	}
	return out
}

func pathBaseImport(ip string) string {
	if i := strings.LastIndex(ip, "/"); i >= 0 {
		return ip[i+1:]
	}
	return ip
}
