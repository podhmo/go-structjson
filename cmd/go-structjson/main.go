package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"path/filepath"

	structjson "github.com/podhmo/go-structjson"
)

// TODO: support iota
// TODO: tags extraction
// TODO: comment extraction

var target = flag.String("target", "", "target")

func parse(world *structjson.World, fpath string, used map[string]struct{}) error {
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
			result, err := structjson.CollectResult(fname, f.Scope, f.Imports)
			if err != nil {
				return err
			}
			module.Files[fname] = result
			for _, im := range result.ImportsMap {
				if im.NeedParse && strings.Contains(im.FullName, "/") {
					// TODO: standard library
					// pseudo imports
					dpath := path.Join(gosrc, im.FullName)
					if _, err := os.Stat(dpath); err == nil {
						if err := parse(world, dpath, used); err != nil {
							return err
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
