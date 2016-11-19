package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	structjson "github.com/podhmo/go-structjson"
)

// TODO: support iota
// TODO: comment extraction

var target = flag.String("target", "", "target")
var verbose = flag.Bool("verbose", false, "verbose")
var exclude = flag.String("exclude", "fmt,log,reflect,go/ast,unsafe,html/template,text/template,encoding/xml,syscall,windows,encoding/binary,sync,os,flag,net/http,go/format,encoding/json,sys,bufio,bytes/buffer,unicode,sync/atomic", "")

type App struct {
	gopath     string
	goroot     string
	verbose    bool
	excludeMap map[string]struct{}
	used       map[string]struct{}
}

func (app *App) parse(world *structjson.World, fpath string, pkgName string, depth int) error {
	_, exists := app.used[fpath]
	if exists {
		return nil
	}
	app.used[fpath] = struct{}{}

	if _, exists := app.excludeMap[pkgName]; exists {
		if app.verbose {
			fmt.Fprintf(os.Stderr, "%sparse: skip %q\n", strings.Repeat(" ", depth), pkgName)
		}
		return nil
	}
	if app.verbose {
		fmt.Fprintf(os.Stderr, "%sparse: %q\n", strings.Repeat(" ", depth), fpath)
	}

	pkgs, err := structjson.CollectPackageMap(fpath)
	if err != nil {
		return err
	}

	pkgNameList := make([]string, len(pkgs))
	i := 0
	for _, pkg := range pkgs {
		pkgNameList[i] = pkg.Name
		i++
	}
	sort.Sort(sort.StringSlice(pkgNameList))

	gosrc := path.Join(app.gopath, "src")
	for _, pkgName := range pkgNameList {
		pkg := pkgs[pkgName]

		// skip main
		if pkg.Name == "main" {
			continue
		}

		module := structjson.NewModule(pkg.Name)
		world.Modules[pkg.Name] = module

		fileNameList := make([]string, len(pkg.Files))
		i := 0
		for fname := range pkg.Files {
			fileNameList[i] = fname
			i++
		}
		sort.Sort(sort.StringSlice(fileNameList))

		for _, fname := range fileNameList {
			f := pkg.Files[fname]
			if module.FullName == "" {
				if strings.HasPrefix(fname, app.goroot) {
					module.FullName = module.Name
				} else {
					if stat, err := os.Stat(fname); err == nil {
						if stat.IsDir() {
							module.FullName = fname[len(gosrc)+1:]
						} else {
							module.FullName = filepath.Dir(fname)[len(gosrc)+1:]
						}
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
							if err := app.parse(world, dpath, im.FullName, depth+1); err != nil {
								return err
							}
						}
					}
				} else {
					{
						dpath := path.Join(app.goroot, "src", im.FullName)
						if _, err := os.Stat(dpath); err == nil {
							if err := app.parse(world, dpath, im.FullName, depth+1); err != nil {
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
	excludeMap := map[string]struct{}{}
	for _, name := range strings.Split(*exclude, ",") {
		excludeMap[strings.TrimSpace(name)] = struct{}{}
	}
	app := App{
		gopath:     os.Getenv("GOPATH"),
		goroot:     runtime.GOROOT(),
		verbose:    *verbose,
		used:       map[string]struct{}{},
		excludeMap: excludeMap,
	}
	if err := app.parse(world, fpath, "", 0); err != nil {
		panic(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(world); err != nil {
		panic(err)
	}
}
