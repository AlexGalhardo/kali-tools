package bootstrap_test

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCompositionRootsUseBootstrap(t *testing.T) {
	for _, root := range []struct {
		name  string
		files []string
	}{
		{name: "desktop", files: []string{"main.go"}},
		{name: "cli", files: []string{"cmd/kali-tools/main.go"}},
	} {
		t.Run(root.name, func(t *testing.T) {
			var usesBootstrap, hasRun bool
			for _, path := range root.files {
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, filepath.Join("../..", path), nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				imports := importNames(t, file)
				for _, name := range imports {
					if name == "github.com/AlexGalhardo/kali-tools/internal/bootstrap" {
						usesBootstrap = true
					}
					if name == "github.com/AlexGalhardo/kali-tools/internal/adapter/probe" {
						t.Errorf("%s imports probe directly", path)
					}
				}
				ast.Inspect(file, func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok {
						if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
							if pkg, ok := selector.X.(*ast.Ident); ok && imports[pkg.Name] == "github.com/AlexGalhardo/kali-tools/internal/adapter/storage" {
								if selector.Sel.Name == "Open" || selector.Sel.Name == "DefaultPath" {
									t.Errorf("%s calls storage.%s", path, selector.Sel.Name)
								}
							}
						}
					}
					return true
				})
				for _, declaration := range file.Decls {
					fn, ok := declaration.(*ast.FuncDecl)
					if !ok || fn.Recv != nil || fn.Name.Name != "run" {
						continue
					}
					hasRun = true
					var parameters []string
					for _, field := range fn.Type.Params.List {
						var output bytes.Buffer
						if err := format.Node(&output, fset, field.Type); err != nil {
							t.Fatal(err)
						}
						for range field.Names {
							parameters = append(parameters, output.String())
						}
					}
					if got := strings.Join(parameters, ", "); got != "context.Context, []string, func(string) string, io.Writer, io.Writer" {
						t.Errorf("run parameters = %s", got)
					}
					var result bytes.Buffer
					if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
						t.Fatal("run must return int")
					}
					if err := format.Node(&result, fset, fn.Type.Results.List[0].Type); err != nil {
						t.Fatal(err)
					}
					if result.String() != "int" {
						t.Errorf("run result = %s, want int", result.String())
					}
				}
			}
			if !usesBootstrap || !hasRun {
				t.Errorf("bootstrap import = %v, run = %v; want both", usesBootstrap, hasRun)
			}
		})
	}
}

func TestNoEnvOrFlagReadsBelowCompositionRoots(t *testing.T) {
	err := filepath.WalkDir("../../internal", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		imports := importNames(t, file)
		for _, name := range imports {
			if name == "flag" && !strings.Contains(filepath.ToSlash(path), "/adapter/cli/") {
				t.Errorf("%s imports flag", path)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || imports[pkg.Name] != "os" {
				return true
			}
			switch selector.Sel.Name {
			case "Getenv", "LookupEnv", "Environ", "ExpandEnv":
				t.Errorf("%s calls os.%s", path, selector.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func importNames(t *testing.T, file *ast.File) map[string]string {
	t.Helper()
	imports := make(map[string]string)
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}
	return imports
}

func TestCoreDoesNotImportAdapters(t *testing.T) {
	for _, name := range []string{"app", "job", "network", "capture", "wifi"} {
		err := filepath.WalkDir(filepath.Join("..", name), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range importNames(t, file) {
				if strings.HasPrefix(imported, "github.com/AlexGalhardo/kali-tools/internal/adapter/") {
					t.Errorf("%s imports %s", path, imported)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDrivingRootsImportOnlyApplication(t *testing.T) {
	for _, root := range []struct {
		path    string
		allowed []string
	}{
		{"../../main.go", []string{"bootstrap", "adapter/wailsui"}},
		{"../../cmd/kali-tools/main.go", []string{"bootstrap", "app", "adapter/cli", "adapter/httpapi"}},
		{"../../cmd/kali-tools/serve.go", []string{"bootstrap", "app", "domain", "adapter/cli", "adapter/httpapi", "adapter/contract"}},
		{"../adapter/cli/cli.go", []string{"app", "domain", "network", "capture", "adapter/contract"}},
		{"../adapter/cli/capture.go", []string{"app", "domain", "job", "capture", "adapter/contract"}},
		{"../adapter/wailsui/bindings.go", []string{"app", "domain", "job", "adapter/contract"}},
		{"../adapter/wailsui/events.go", []string{"job", "adapter/contract"}},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), root.path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range importNames(t, file) {
			const prefix = "github.com/AlexGalhardo/kali-tools/"
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			allowed := false
			// W01 shares static assets through a pure package outside internal/.
			if (root.path == "../../main.go" || root.path == "../../cmd/kali-tools/serve.go") && name == prefix+"frontend" {
				allowed = true
			}
			for _, pkg := range root.allowed {
				if name == prefix+"internal/"+pkg {
					allowed = true
				}
			}
			if !allowed {
				t.Errorf("%s imports %s", root.path, name)
			}
		}
	}
}
