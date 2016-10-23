package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	structjson "github.com/podhmo/go-structjson"
)

// TODO: extract struct information
// TODO: support iota
// TODO: tags extraction
// TODO: comment extraction

var target = flag.String("target", "", "target")

func main() {
	flag.Parse()
	args := flag.Args()
	_ = args
	if *target == "" {
		fmt.Fprintf(os.Stderr, "go-structjson --target [target]\n")
		os.Exit(1)
	}

	fpath := *target
	pkgs, e := structjson.CollectPackageMap(fpath)
	if e != nil {
		log.Fatal(e)
		return
	}

	world := structjson.NewWorld()
	for _, pkg := range pkgs {
		module := structjson.NewModule(pkg.Name)
		world.Modules[pkg.Name] = module
		for fname, f := range pkg.Files {
			result, err := structjson.CollectResult(fname, f.Scope)
			if err != nil {
				panic(err)
			}
			module.Files[fname] = result
		}
	}
	b, err := json.Marshal(world)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
