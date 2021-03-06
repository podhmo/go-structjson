package structjson

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

type World struct {
	Modules map[string]*Module `json:"module"`
}
type Module struct {
	Name     string             `json:"name"`
	FullName string             `json:"fullname"`
	Files    map[string]*Result `json:"file"`
}

func NewWorld() *World {
	return &World{Modules: make(map[string]*Module)}
}
func NewModule(name string) *Module {
	return &Module{Name: name, Files: make(map[string]*Result)}
}

type indexAliasValues []*AliasValue

func (p indexAliasValues) Len() int           { return len(p) }
func (p indexAliasValues) Less(i, j int) bool { return p[i].i < p[j].i }
func (p indexAliasValues) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type nameAliasValues []*AliasValue

func (p nameAliasValues) Len() int           { return len(p) }
func (p nameAliasValues) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p nameAliasValues) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type Result struct {
	Name          string                          `json:"name"`
	AliasMap      map[string]*AliasDefinition     `json:"alias,omitempty"`
	StructMap     map[string]*StructDefinition    `json:"struct,omitempty"`
	InterfaceMap  map[string]*InterfaceDefinition `json:"interface,omitempty"`
	MaybeAliasses []*AliasValue                   `json:"-"`
	ImportsMap    map[string]*ImportDefinition    `json:"import,omitempty"`
	i             int
}

func NewResult(name string) *Result {
	return &Result{
		Name:          name,
		StructMap:     make(map[string]*StructDefinition),
		InterfaceMap:  make(map[string]*InterfaceDefinition),
		AliasMap:      make(map[string]*AliasDefinition),
		MaybeAliasses: []*AliasValue{},
		ImportsMap:    make(map[string]*ImportDefinition),
	}
}

func (r *Result) AddStruct(ob *ast.Object) (*StructDefinition, error) {
	item, exists := r.StructMap[ob.Name]
	if !exists {
		item = &StructDefinition{}
	}
	item.rawDef = ob
	item.Name = ob.Name
	r.StructMap[ob.Name] = item
	fields, err := findFields(r, ob.Decl.(ast.Node))
	item.Fields = fields
	return item, err
}

func (r *Result) AddInterface(ob *ast.Object) (*InterfaceDefinition, error) {
	// FIXME: not support seriously
	item, exists := r.InterfaceMap[ob.Name]
	if !exists {
		item = &InterfaceDefinition{}
	}
	item.rawDef = ob
	item.Name = ob.Name
	r.InterfaceMap[ob.Name] = item
	return item, nil
}

type nameVisitor struct{ Found string }

func (v *nameVisitor) Visit(node ast.Node) ast.Visitor {
	switch node := node.(type) {
	case *ast.Ident:
		v.Found = node.Name
	}
	return v
}

func findName(node ast.Node) string {
	namevisitor := &nameVisitor{Found: ""}
	ast.Walk(namevisitor, node)
	return namevisitor.Found
}

type Field struct {
	Name  string              `json:"name"`
	Tags  map[string][]string `json:"tags"`
	Type  Type                `json:"type"`
	Embed bool                `json:"embed"`
}

type Type interface{}
type fieldsVisitor struct {
	Found  map[string]*Field
	Result *Result
}

func (v *fieldsVisitor) Visit(node ast.Node) ast.Visitor {
	structNode := node.(*ast.TypeSpec).Type.(*ast.StructType)
	if structNode.Incomplete {
		panic(fmt.Sprintf("%s is incomplete struct definition", node.(*ast.TypeSpec).Name.Name))
	}
	for _, field := range structNode.Fields.List {
		if err := v.visitField(field); err != nil {
			panic(err)
		}
	}
	return nil
}

func parseTags(node *ast.Field) map[string][]string {
	tags := map[string][]string{}
	if node.Tag != nil {
		unquoted, err := strconv.Unquote(node.Tag.Value)
		if err != nil {
			panic(err)
		}
		for _, section := range strings.Split(unquoted, " ") {
			nameAndargs := strings.SplitN(section, ":", 2)
			if len(nameAndargs) == 2 {
				unquotedArgs, err := strconv.Unquote(nameAndargs[1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "scan tags failure %q\n", unquoted)
				}
				for _, arg := range strings.Split(unquotedArgs, ",") {
					trimmed := strings.Trim(arg, " ")
					tags[nameAndargs[0]] = append(tags[nameAndargs[0]], trimmed)
				}
			}
		}
	}
	return tags
}

func (v *fieldsVisitor) visitField(node *ast.Field) error {
	typ := FindType(v.Result, node.Type)
	if len(node.Names) == 0 {
		name := findName(node.Type)
		v.Found[name] = &Field{
			Name:  name,
			Embed: true,
			Tags:  parseTags(node),
			Type:  typ,
		}
		return nil
	}
	for _, nameNode := range node.Names {
		tags := []string{}
		if node.Tag != nil {
			tags = append(tags, node.Tag.Value)
		}
		embed := false
		name := nameNode.Name
		v.Found[name] = &Field{
			Name:  name,
			Embed: embed,
			Tags:  parseTags(node),
			Type:  typ,
		}

	}
	return nil
}

func FindType(r *Result, node ast.Node) Type {
	m := make(map[string]Type)
	switch node := node.(type) {
	case *ast.Ident:
		m["kind"] = "primitive"
		m["value"] = node.Name
	case *ast.ArrayType:
		m["kind"] = "array"
		m["value"] = FindType(r, node.Elt)
	case *ast.MapType:
		m["kind"] = "map"
		m["key"] = FindType(r, node.Key)
		m["value"] = FindType(r, node.Value)
	case *ast.StructType:
		m["kind"] = "struct"
		m["fields"] = FindType(r, node.Fields)
	case *ast.InterfaceType:
		m["kind"] = "interface"
		m["methods"] = FindType(r, node.Methods)
	case *ast.StarExpr:
		m["kind"] = "pointer"
		m["value"] = FindType(r, node.X)
	case *ast.SelectorExpr:
		m["kind"] = "selector"
		m["prefix"] = node.X.(*ast.Ident).Name
		m["value"] = node.Sel.Name
		if def, exists := r.ImportsMap[node.X.(*ast.Ident).Name]; exists {
			def.NeedParse = true
		}
	case *ast.FuncType:
		m["kind"] = "func"
		m["args"] = FindType(r, node.Params)
		m["results"] = FindType(r, node.Results)
	case *ast.TypeSpec:
		return FindType(r, node.Type)
	case *ast.ChanType:
		m["kind"] = "channel"
		m["value"] = FindType(r, node.Value)
		m["dir"] = node.Dir
		// xxx
	case *ast.FieldList:
		if node == nil {
			return []Type{}
		}
		args := make([]Type, len(node.List))
		for i, arg := range node.List {
			args[i] = FindType(r, arg.Type)
		}
		return args
	case *ast.Ellipsis:
		m["kind"] = "ellipsis"
		m["value"] = FindType(r, node.Elt)
	default:
		spew.Dump(node)
		panic(node)
	}
	return m
}

func findFields(r *Result, val ast.Node) (map[string]*Field, error) {
	v := &fieldsVisitor{Result: r, Found: make(map[string]*Field)}
	ast.Walk(v, val)
	if v.Found == nil {
		return nil, fmt.Errorf("fields is not found")
	}
	return v.Found, nil
}

func (r *Result) AddAlias(ob *ast.Object) (*AliasDefinition, error) {
	item, exists := r.AliasMap[ob.Name]
	if !exists {
		item = &AliasDefinition{}
	}
	item.rawDef = ob
	item.Name = ob.Name
	if ob.Decl != nil {
		item.Original = FindType(r, ob.Decl.(*ast.TypeSpec))
	}
	r.AliasMap[ob.Name] = item
	if !exists {
		newGuesses := make([]*AliasValue, 0, len(r.MaybeAliasses))
		for _, value := range r.MaybeAliasses {
			// xxx:
			if ob.Name == value.TypeName {
				r.AddAliasValue(value.rawDef)
			} else {
				newGuesses = append(newGuesses, value)
			}
		}
		r.MaybeAliasses = newGuesses
	}
	return item, nil
}

type callexprVisitor struct {
	Found *ast.CallExpr
}

func (v *callexprVisitor) Visit(node ast.Node) ast.Visitor {
	if v.Found == nil && node != nil {
		if callexpr, ok := node.(*ast.CallExpr); ok {
			v.Found = callexpr
		}
	}
	return v
}

func findCallExpr(val ast.Node) (*ast.CallExpr, error) {
	v := &callexprVisitor{}
	ast.Walk(v, val)
	return v.Found, nil
}

type basicLitVisitor struct {
	Found *ast.BasicLit
}

func (v *basicLitVisitor) Visit(node ast.Node) ast.Visitor {
	if v.Found == nil && node != nil {
		if basicLit, ok := node.(*ast.BasicLit); ok {
			v.Found = basicLit
		}
	}
	return v
}

func findBasicLit(val ast.Node) (*ast.BasicLit, error) {
	v := &basicLitVisitor{}
	ast.Walk(v, val)
	if v.Found == nil {
		return nil, fmt.Errorf("basicLit is not found")
	}
	return v.Found, nil
}

func findValueType(val ast.Node) (*ast.CallExpr, error) {
	v := &callexprVisitor{}
	ast.Walk(v, val)
	if v.Found == nil {
		return nil, fmt.Errorf("callExpr is not found")
	}
	return v.Found, nil
}

func (r *Result) AddAliasValue(ob *ast.Object) (*AliasDefinition, error) {
	spec := ob.Decl.(*ast.ValueSpec)
	values := spec.Values
	if len(values) != 1 {
		// iota
		return nil, nil
		// return nil, fmt.Errorf("hmm:%v", values) // xxx;
	}
	var value *AliasValue
	switch node := spec.Type.(type) {
	case *ast.Ident:
		typeName := node.Name
		for _, v := range values {
			lit, err := findBasicLit(v)
			if err != nil {
				if ident, ok := v.(*ast.Ident); ok && ident.Name == "iota" {
					// iota
					return nil, nil
				}
				fmt.Fprintf(os.Stderr, "const %s is complex definition. skip..\n", ob.Name)
				return nil, nil
			}
			if lit == nil {
				return nil, fmt.Errorf("not found: %s", ob.Name)
			}
			r.i++
			value = &AliasValue{TypeName: typeName, Name: ob.Name, Value: lit.Value, rawDef: ob, i: r.i}
			break
		}
	case nil:
		// finding type name
		for _, v := range values {
			f, err := findCallExpr(v)
			if err != nil {
				return nil, err // xxx;
			}
			if f == nil {
				if _, ok := v.(*ast.BasicLit); ok {
					return nil, nil
				}
				fmt.Fprintf(os.Stderr, "const %s is complex definition. skip..\n", ob.Name)
				return nil, nil
			}
			lit, err := findBasicLit(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "const %s is complex definition. skip..\n", ob.Name)
				return nil, nil
			}
			if lit == nil {
				return nil, fmt.Errorf("not found@@: %s", ob.Name)
			}
			fident, ok := f.Fun.(*ast.Ident)
			if !ok {
				fmt.Fprintf(os.Stderr, "const %s is complex definition. skip..\n", ob.Name)
				return nil, nil
			}
			r.i++
			value = &AliasValue{TypeName: fident.Name, Name: ob.Name, Value: lit.Value, rawDef: ob, i: r.i}
			break
		}
	}
	// fident.Obj is same as type def?
	if value == nil {
		fmt.Fprintf(os.Stderr, "const %s is complex definition. skip..\n", ob.Name)
		return nil, nil
	}
	if _, ok := r.AliasMap[value.TypeName]; !ok {
		r.MaybeAliasses = append(r.MaybeAliasses, value)
		return nil, nil
	}

	item, exists := r.AliasMap[value.TypeName]
	if !exists {
		item = &AliasDefinition{}
		r.AliasMap[ob.Name] = item
	}
	item.Candidates = append(item.Candidates, value)
	item.rawCandidates = append(item.rawCandidates, ob)
	return item, nil
}

type StructDefinition struct {
	Name   string `json:"name"`
	rawDef *ast.Object
	Fields map[string]*Field `json:"fields"`
}

type InterfaceDefinition struct {
	Name   string `json:"name"`
	rawDef *ast.Object
	// TODO: methods
}

type AliasDefinition struct {
	Name          string        `json:"name"`
	Original      Type          `json:"original"`
	Candidates    []*AliasValue `json:"candidates"`
	rawDef        *ast.Object
	rawCandidates []*ast.Object
}

type AliasValue struct {
	TypeName string `json:"-"`
	i        int
	Name     string      `json:"name"`
	Value    interface{} `json:"value"`
	rawDef   *ast.Object
}

type ImportDefinition struct {
	Name      string `json:"name"`
	FullName  string `json:"fullname"`
	NeedParse bool   `json:"needparse"`
}

func CollectImports(imports []*ast.ImportSpec) map[string]*ImportDefinition {
	m := map[string]*ImportDefinition{}
	for _, im := range imports {
		fullname, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			continue
		}
		var name string
		if im.Name == nil {
			name = path.Base(fullname)
		} else {
			name = im.Name.Name
		}
		m[name] = &ImportDefinition{
			Name:      name,
			FullName:  fullname,
			NeedParse: false,
		}
	}
	return m
}

func CollectResult(name string, scope *ast.Scope, imports []*ast.ImportSpec) (*Result, error) {
	r := NewResult(name)
	r.ImportsMap = CollectImports(imports)
	for _, ob := range scope.Objects {
		anyFound := false
		if isStructDefinition(ob) {
			anyFound = true
			_, err := r.AddStruct(ob)
			if err != nil {
				return r, err
			}
		}
		if isInterfaceDefinition(ob) {
			// work-around
			anyFound = true
			_, err := r.AddInterface(ob)
			if err != nil {
				return r, err
			}
		}
		if isAliasDefinition(ob) {
			anyFound = true
			_, err := r.AddAlias(ob)
			if err != nil {
				return r, err
			}

		}
		if isAliasValueDefinition(ob) {
			anyFound = true
			_, err := r.AddAliasValue(ob)
			if err != nil {
				return r, err
			}
		}
		if !anyFound {
			// fmt.Println(ob.Name)
		}
	}
	for _, alias := range r.AliasMap {
		sort.Sort(nameAliasValues(alias.Candidates))
	}
	return r, nil
}

func isInterfaceDefinition(ob *ast.Object) bool {
	if ob.Kind != ast.Typ {
		return false
	}

	node, ok := ob.Decl.(*ast.TypeSpec)
	if !ok {
		return false
	}

	// skip if unexported struct.
	if strings.Title(node.Name.Name) != node.Name.Name {
		return false
	}

	_, ok = node.Type.(*ast.InterfaceType)
	if !ok {
		return false
	}
	return true
}

func isStructDefinition(ob *ast.Object) bool {
	if ob.Kind != ast.Typ {
		return false
	}

	node, ok := ob.Decl.(*ast.TypeSpec)
	if !ok {
		return false
	}

	// skip if unexported struct.
	if strings.Title(node.Name.Name) != node.Name.Name {
		return false
	}

	_, ok = node.Type.(*ast.StructType)
	if !ok {
		return false
	}
	return true
}

func isAliasDefinition(ob *ast.Object) bool {
	if ob.Kind != ast.Typ {
		return false
	}
	node, ok := ob.Decl.(*ast.TypeSpec)
	if !ok {
		return false
	}
	// skip if unexported alias.
	if strings.Title(node.Name.Name) != node.Name.Name {
		return false
	}

	switch node.Type.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.ArrayType, *ast.MapType, *ast.StarExpr:
		return true
	default:
		return false
	}
}

func isAliasValueDefinition(ob *ast.Object) bool {
	if ob.Kind != ast.Con {
		return false
	}
	_, ok := ob.Decl.(*ast.ValueSpec)
	if !ok {
		return false
	}
	return true
}

func CollectPackageMap(fpath string) (map[string]*ast.Package, error) {
	stat, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	if stat.IsDir() {
		return parser.ParseDir(fset, fpath, nil, 0)
	}
	f, err := parser.ParseFile(fset, fpath, nil, 0)
	if err != nil {
		return nil, err
	}
	pkg := &ast.Package{
		Name:  f.Name.Name,
		Files: make(map[string]*ast.File),
	}
	pkg.Files[fpath] = f

	m := make(map[string]*ast.Package)
	m[fpath] = pkg
	return m, nil
}
