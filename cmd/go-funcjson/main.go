package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"os"
	"path"
	"path/filepath"

	"github.com/podhmo/go-structjson"
)

var target = flag.String("target", "", "target")

type FuncDefinition struct {
	Name    string  `json:"name"`
	Params  []Value `json:"params"`
	Returns []Value `json:"returns"`
}
type Value struct {
	Name string          `json:"name"`
	Type structjson.Type `json:"type"`
}
type World struct {
	Modules map[string]*Module `json:"module"`
}
type Module struct {
	Name     string           `json:"name"`
	FullName string           `json:"fullname"`
	Files    map[string]*File `json:"file"`
}

type File struct {
	Name       string                                  `json:"name"`
	FuncMap    map[string]*FuncDefinition              `json:"function"`
	ImportsMap map[string]*structjson.ImportDefinition `json:"import,omitempty"`
}

func NewFuncDefinition(name string) *FuncDefinition {
	return &FuncDefinition{
		Name:    name,
		Params:  []Value{},
		Returns: []Value{},
	}
}

func NewFile(name string) *File {
	return &File{
		Name:       name,
		FuncMap:    make(map[string]*FuncDefinition),
		ImportsMap: make(map[string]*structjson.ImportDefinition),
	}
}

func NewWorld() *World {
	return &World{Modules: make(map[string]*Module)}
}
func NewModule(name string) *Module {
	return &Module{Name: name, Files: make(map[string]*File)}
}

func parse(world *World, fpath string, used map[string]struct{}) error {
	_, exists := used[fpath]
	if exists {
		return nil
	}
	used[fpath] = struct{}{}

	pkgs, err := structjson.CollectPackageMap(fpath)
	if err != nil {
		return err
	}
	gosrc := path.Join(os.Getenv("GOPATH"), "src")
	r := structjson.NewResult("")
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		module := NewModule(pkg.Name)
		world.Modules[pkg.Name] = module
		for fname, f := range pkg.Files {
			if f == nil {
				continue
			}
			if module.FullName == "" {
				if stat, err := os.Stat(fname); err == nil {
					if stat.IsDir() {
						module.FullName = fname[len(gosrc)+1:]
					} else {
						module.FullName = filepath.Dir(fname)[len(gosrc)+1:]
					}
				}
			}
			file := NewFile(fname)
			module.Files[file.Name] = file
			file.ImportsMap = structjson.CollectImports(f.Imports)
			for _, node := range f.Decls {
				switch node := node.(type) {
				case *ast.FuncDecl:
					fdef := NewFuncDefinition(node.Name.Name)
					if node.Type.Results != nil {
						for _, p := range node.Type.Params.List {
							fdef.Params = append(fdef.Params, Value{Name: p.Names[0].Name, Type: structjson.FindType(r, p.Type)})
						}
					}
					if node.Type.Results != nil {
						for _, p := range node.Type.Results.List {
							if p.Names != nil {
								fdef.Returns = append(fdef.Returns, Value{Name: p.Names[0].Name, Type: structjson.FindType(r, p.Type)})
							} else {
								fdef.Returns = append(fdef.Returns, Value{Type: structjson.FindType(r, p.Type)})
							}
						}
					}
					file.FuncMap[fdef.Name] = fdef
					// spew.Dump(node)
				}
			}
		}
	}
	return nil
}

func main() {
	flag.Parse()
	args := flag.Args()
	_ = args
	if *target == "" {
		fmt.Fprintf(os.Stderr, "go-structjson --target [target]\n")
		os.Exit(1)
	}
	world := NewWorld()
	used := map[string]struct{}{} // fpath
	fpath, err := filepath.Abs(*target)
	if err != nil {
		panic(err)
	}
	if err := parse(world, fpath, used); err != nil {
		panic(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(world); err != nil {
		panic(err)
	}
}
