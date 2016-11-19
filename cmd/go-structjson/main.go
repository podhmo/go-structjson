package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	structjson "github.com/podhmo/go-structjson"
)

// TODO: support iota
// TODO: tags extraction
// TODO: comment extraction

var target = flag.String("target", "", "target")

type App struct {
	gopath string
	goroot string
	used   map[string]struct{}
}

func (app *App) parse(world *structjson.World, fpath string) error {
	_, exists := app.used[fpath]
	if exists {
		return nil
	}
	app.used[fpath] = struct{}{}

	pkgs, err := structjson.CollectPackageMap(fpath)
	if err != nil {
		return err
	}

	gosrc := path.Join(app.gopath, "src")
	for _, pkg := range pkgs {
		module := structjson.NewModule(pkg.Name)
		world.Modules[pkg.Name] = module
		for fname, f := range pkg.Files {
			if module.FullName == "" && strings.HasPrefix(fname, gosrc) {
				if stat, err := os.Stat(fname); err == nil {
					if stat.IsDir() {
						module.FullName = fname[len(gosrc)+1:]
					} else {
						module.FullName = filepath.Dir(fname)[len(gosrc)+1:]
					}
				}

			}
			// skip test code
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}

			result, err := structjson.CollectResult(fname, f.Scope, f.Imports)
			if err != nil {
				return err
			}
			// skip no contents
			if len(result.AliasMap) == 0 && len(result.StructMap) == 0 && len(result.InterfaceMap) == 0 {
				continue
			}
			module.Files[fname] = result
			for _, im := range result.ImportsMap {
				if im.NeedParse && strings.Contains(im.FullName, "/") {
					// pseudo imports
					{
						dpath := path.Join(gosrc, im.FullName)
						if _, err := os.Stat(dpath); err == nil {
							if err := app.parse(world, dpath); err != nil {
								return err
							}
						}
					}
				} else {
					{
						dpath := path.Join(app.goroot, "src", im.FullName)
						if _, err := os.Stat(dpath); err == nil {
							if err := app.parse(world, dpath); err != nil {
								return err
							}
						}
					}
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

	world := structjson.NewWorld()
	fpath, err := filepath.Abs(*target)
	if err != nil {
		panic(err)
	}
	app := App{
		gopath: os.Getenv("GOPATH"),
		goroot: runtime.GOROOT(),
		used:   map[string]struct{}{},
	}
	if err := app.parse(world, fpath); err != nil {
		panic(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(world); err != nil {
		panic(err)
	}
}
