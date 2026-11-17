package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	resourcePath  = "go.opentelemetry.io/otel/sdk/resource"
	semconvPrefix = "go.opentelemetry.io/otel/semconv/v"
	schemaPrefix  = "https://opentelemetry.io/schemas/"
)

// mergeFuncs are the sdk/resource functions that combine resources and
// return ErrSchemaURLConflict when their schema URLs differ.
var mergeFuncs = map[string]bool{"Merge": true, "New": true, "Detect": true}

// resourceTypes are the sdk/resource types whose values carry a schema URL
// into a resource: a function that returns one of them passes its schema
// URLs on to the caller.
var resourceTypes = map[string]bool{"Resource": true, "Option": true, "Detector": true}

// place is a file and line, with the file shown relative to --dir when it
// is inside it, and as module@version/path otherwise.
type place struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

func (p place) String() string { return p.File + ":" + strconv.Itoa(p.Line) }

// origin is where a schema URL is written in source.
type origin struct {
	URL   string `json:"schemaURL"`
	What  string `json:"what"`
	Place place  `json:"place"`
	inDep bool
	inArg bool // an argument of a call into a package that reaches sdk/resource
	mod   string
}

type refKind int

const (
	refName   refKind = iota // a package-level function or variable
	refType                  // a composite literal of a type: its Detect method
	refMethod                // x.M() on a value: every method M of the package
)

type ref struct {
	pkg  string
	name string
	kind refKind
	at   place
	show string
}

// decl is a package-level function, method or variable.
type decl struct {
	pkg     *pkgInfo
	name    string
	show    string
	at      place
	isFunc  bool
	resish  bool // a function that returns a resource, option or detector
	touches bool // refers to the sdk/resource package
	merges  bool // calls resource.Merge, New or Detect
	direct  []origin
	refs    []ref

	resultTypes []ast.Expr
	body        ast.Node
	file        *fileInfo

	done     bool
	visiting bool
	summary  map[string]origin
}

type fileInfo struct {
	path    string
	imports map[string]string // local name to import path
	dots    []string          // dot-imported paths
}

type pkgInfo struct {
	lp      *listPkg
	decls   map[string]*decl // functions and vars by name, methods as (T).M
	detects map[string]bool  // types with a Detect method
	methods map[string][]*decl
}

// analysis holds every package that can reach sdk/resource.
type analysis struct {
	fset     *token.FileSet
	dir      string
	byPath   map[string]*listPkg
	pkgs     map[string]*pkgInfo
	semconv  map[string]string // semconv import path to its SchemaURL
	resource *pkgInfo
}

// reaches reports, for every package, whether it imports sdk/resource
// directly or through other packages.
func reaches(byPath map[string]*listPkg) map[string]bool {
	memo := map[string]bool{}
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(path string) bool {
		if v, ok := memo[path]; ok {
			return v
		}
		if path == resourcePath {
			memo[path] = true
			return true
		}
		if seen[path] {
			return false
		}
		seen[path] = true
		p := byPath[path]
		found := false
		if p != nil {
			for _, imp := range p.Imports {
				if walk(imp) {
					found = true
				}
			}
		}
		memo[path] = found
		return found
	}
	for path := range byPath {
		walk(path)
	}
	return memo
}

func newAnalysis(dir string, list []*listPkg) (*analysis, error) {
	a := &analysis{
		fset:    token.NewFileSet(),
		dir:     dir,
		byPath:  map[string]*listPkg{},
		pkgs:    map[string]*pkgInfo{},
		semconv: map[string]string{},
	}
	for _, p := range list {
		a.byPath[p.ImportPath] = p
	}
	reach := reaches(a.byPath)
	var paths []string
	for path, p := range a.byPath {
		if p.Standard {
			continue
		}
		if strings.HasPrefix(path, semconvPrefix) && !strings.Contains(path[len(semconvPrefix):], "/") {
			url, err := a.readSchemaURL(p)
			if err != nil {
				return nil, err
			}
			a.semconv[path] = url
		}
		if reach[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		info, err := a.parsePackage(a.byPath[path])
		if err != nil {
			return nil, err
		}
		a.pkgs[path] = info
	}
	a.resource = a.pkgs[resourcePath]
	for _, info := range a.pkgs {
		for _, d := range info.decls {
			a.scan(d)
		}
	}
	return a, nil
}

// readSchemaURL finds the SchemaURL constant of a semconv package.
func (a *analysis) readSchemaURL(p *listPkg) (string, error) {
	for _, name := range append(append([]string{}, p.GoFiles...), p.CgoFiles...) {
		path := filepath.Join(p.Dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return "", refuse("can't read %s: %v", path, err)
		}
		if !bytes.Contains(src, []byte("SchemaURL")) {
			continue
		}
		f, err := parser.ParseFile(a.fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return "", refuse("can't parse %s: %v", path, err)
		}
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, s := range g.Specs {
				vs := s.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if n.Name != "SchemaURL" || i >= len(vs.Values) {
						continue
					}
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if v, err := strconv.Unquote(lit.Value); err == nil && v != "" {
							return v, nil
						}
					}
				}
			}
		}
	}
	return "", refuse("can't find the SchemaURL constant of %s in %s", p.ImportPath, p.Dir)
}

func (a *analysis) parsePackage(p *listPkg) (*pkgInfo, error) {
	info := &pkgInfo{lp: p, decls: map[string]*decl{}, detects: map[string]bool{}, methods: map[string][]*decl{}}
	files := append(append([]string{}, p.GoFiles...), p.CgoFiles...)
	if len(files) == 0 {
		return nil, refuse("package %s has no Go files in %s", p.ImportPath, p.Dir)
	}
	for _, name := range files {
		path := filepath.Join(p.Dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, refuse("can't read %s: %v", path, err)
		}
		f, err := parser.ParseFile(a.fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, refuse("can't parse %s: %v", path, err)
		}
		fi := &fileInfo{path: path, imports: map[string]string{}}
		for _, spec := range f.Imports {
			ipath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, refuse("bad import in %s: %v", path, err)
			}
			local := ""
			if spec.Name != nil {
				local = spec.Name.Name
			} else if ip := a.byPath[ipath]; ip != nil && ip.Name != "" {
				local = ip.Name
			} else {
				local = ipath[strings.LastIndex(ipath, "/")+1:]
			}
			switch local {
			case "_":
			case ".":
				fi.dots = append(fi.dots, ipath)
			default:
				fi.imports[local] = ipath
			}
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				key, show := d.Name.Name, p.ImportPath+"."+d.Name.Name
				if d.Recv != nil && len(d.Recv.List) == 1 {
					t := recvType(d.Recv.List[0].Type)
					key = "(" + t + ")." + d.Name.Name
					show = p.ImportPath + ".(" + t + ")." + d.Name.Name
					if d.Name.Name == "Detect" {
						info.detects[t] = true
					}
				}
				var results []ast.Expr
				if d.Type.Results != nil {
					for _, r := range d.Type.Results.List {
						results = append(results, r.Type)
					}
				}
				dd := &decl{
					pkg: info, name: key, show: show, at: a.place(p, d.Pos()), isFunc: true,
					resultTypes: results, body: d.Body, file: fi,
				}
				info.decls[key] = dd
				if d.Recv != nil {
					info.methods[d.Name.Name] = append(info.methods[d.Name.Name], dd)
				}
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, s := range d.Specs {
					vs := s.(*ast.ValueSpec)
					for i, n := range vs.Names {
						var val ast.Expr
						switch {
						case i < len(vs.Values):
							val = vs.Values[i]
						case len(vs.Values) == 1:
							val = vs.Values[0]
						}
						if val == nil || n.Name == "_" {
							continue
						}
						info.decls[n.Name] = &decl{
							pkg: info, name: n.Name, show: p.ImportPath + "." + n.Name, at: a.place(p, n.Pos()),
							body: val, file: fi,
						}
					}
				}
			}
		}
	}
	return info, nil
}

func recvType(e ast.Expr) string {
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		default:
			return "?"
		}
	}
}

// place shows a position relative to --dir when the file is inside it,
// and as module@version/path when it is in the module cache.
func (a *analysis) place(p *listPkg, pos token.Pos) place {
	ps := a.fset.Position(pos)
	file := ps.Filename
	if rel, err := filepath.Rel(a.dir, file); err == nil && !strings.HasPrefix(rel, "..") {
		file = filepath.ToSlash(rel)
	} else if m := p.Module; m != nil && m.Dir != "" {
		if rel, err := filepath.Rel(m.Dir, file); err == nil && !strings.HasPrefix(rel, "..") {
			file = m.Path + "@" + m.Version + "/" + filepath.ToSlash(rel)
		}
	}
	return place{File: file, Line: ps.Line}
}

func (a *analysis) inMain(p *listPkg) bool {
	return p.Module != nil && p.Module.Main
}

func (a *analysis) modString(p *listPkg) string {
	m := p.Module
	if m == nil {
		return p.ImportPath
	}
	if m.Replace != nil {
		if m.Replace.Version != "" {
			return m.Replace.Path + " " + m.Replace.Version
		}
		return m.Path + " (replaced by " + m.Replace.Path + ")"
	}
	if m.Version == "" {
		return m.Path
	}
	return m.Path + " " + m.Version
}

// scan walks one declaration and records what it refers to.
func (a *analysis) scan(d *decl) {
	p := d.pkg
	inResource := p.lp.ImportPath == resourcePath
	if inResource {
		d.touches = true
	}
	for _, rt := range d.resultTypes {
		if a.isResourceType(d.file, p, rt) {
			d.resish = true
		}
	}
	argDepth, otherDepth := 0, 0
	addOrigin := func(url, what string, pos token.Pos) {
		if otherDepth > 0 {
			return
		}
		d.direct = append(d.direct, origin{
			URL: url, What: what, Place: a.place(p.lp, pos),
			inDep: !a.inMain(p.lp), mod: a.modString(p.lp), inArg: argDepth > 0,
		})
	}
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			// A schema URL passed to a function of a package that
			// reaches sdk/resource counts even when this function
			// doesn't use sdk/resource itself.
			if a.callsOther(d.file, n.Fun) {
				// Such as trace.WithSchemaURL: a tracer's schema URL,
				// which never meets a resource's.
				ast.Inspect(n.Fun, visit)
				otherDepth++
				for _, arg := range n.Args {
					ast.Inspect(arg, visit)
				}
				otherDepth--
				return false
			}
			if !a.callsRelated(d.file, p, n.Fun) {
				return true
			}
			ast.Inspect(n.Fun, visit)
			argDepth++
			for _, arg := range n.Args {
				ast.Inspect(arg, visit)
			}
			argDepth--
			return false
		case *ast.SelectorExpr:
			x, ok := n.X.(*ast.Ident)
			ipath, isImport := "", false
			if ok {
				ipath, isImport = d.file.imports[x.Name]
			}
			if !isImport {
				// A method call on a value. Without types, follow every
				// method of this package with that name. Detect is left
				// out: detectors are followed from the values passed in.
				if n.Sel.Name != "Detect" && len(p.methods[n.Sel.Name]) > 0 {
					d.refs = append(d.refs, ref{pkg: p.lp.ImportPath, name: n.Sel.Name, kind: refMethod, at: a.place(p.lp, n.Pos()), show: "." + n.Sel.Name + "()"})
				}
				if !ok {
					ast.Inspect(n.X, visit)
				}
				return false
			}
			switch {
			case ipath == resourcePath:
				d.touches = true
				if mergeFuncs[n.Sel.Name] {
					d.merges = true
				}
				d.refs = append(d.refs, ref{pkg: ipath, name: n.Sel.Name, at: a.place(p.lp, n.Pos()), show: x.Name + "." + n.Sel.Name})
			case strings.HasPrefix(ipath, semconvPrefix) && n.Sel.Name == "SchemaURL":
				if url := a.semconv[ipath]; url != "" {
					addOrigin(url, x.Name+".SchemaURL ("+ipath+")", n.Pos())
				}
			case a.pkgs[ipath] != nil:
				d.refs = append(d.refs, ref{pkg: ipath, name: n.Sel.Name, at: a.place(p.lp, n.Pos()), show: x.Name + "." + n.Sel.Name})
			}
			return false
		case *ast.Ident:
			if n.Name == "SchemaURL" {
				for _, dot := range d.file.dots {
					if url := a.semconv[dot]; url != "" {
						addOrigin(url, "SchemaURL ("+dot+")", n.Pos())
					}
				}
			}
			for _, dot := range d.file.dots {
				if dot == resourcePath {
					if mergeFuncs[n.Name] {
						d.merges = true
					}
					d.touches = true
					d.refs = append(d.refs, ref{pkg: dot, name: n.Name, at: a.place(p.lp, n.Pos()), show: n.Name})
				}
			}
			if n.Name != d.name && p.decls[n.Name] != nil {
				if inResource && mergeFuncs[n.Name] {
					d.merges = true
				}
				d.refs = append(d.refs, ref{pkg: p.lp.ImportPath, name: n.Name, at: a.place(p.lp, n.Pos()), show: n.Name})
			}
		case *ast.KeyValueExpr:
			if _, ok := n.Key.(*ast.Ident); ok {
				ast.Inspect(n.Value, visit)
				return false
			}
		case *ast.CompositeLit:
			switch t := n.Type.(type) {
			case *ast.Ident:
				if p.detects[t.Name] {
					d.refs = append(d.refs, ref{pkg: p.lp.ImportPath, name: "(" + t.Name + ").Detect", kind: refType, at: a.place(p.lp, n.Pos()), show: t.Name + "{}"})
				}
			case *ast.SelectorExpr:
				if x, ok := t.X.(*ast.Ident); ok {
					if ipath, ok := d.file.imports[x.Name]; ok {
						if q := a.pkgs[ipath]; q != nil && q.detects[t.Sel.Name] {
							d.refs = append(d.refs, ref{pkg: ipath, name: "(" + t.Sel.Name + ").Detect", kind: refType, at: a.place(p.lp, n.Pos()), show: x.Name + "." + t.Sel.Name + "{}"})
						}
					}
				}
			}
			for _, e := range n.Elts {
				ast.Inspect(e, visit)
			}
			return false
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				if v, err := strconv.Unquote(n.Value); err == nil && strings.HasPrefix(v, schemaPrefix) {
					addOrigin(v, "schema URL string", n.Pos())
				}
			}
		}
		return true
	}
	ast.Inspect(d.body, visit)
	if !d.touches {
		kept := d.direct[:0]
		for _, o := range d.direct {
			if o.inArg {
				kept = append(kept, o)
			}
		}
		d.direct = kept
	}
}

// callsRelated reports whether fun names a package-level function of a
// package that reaches sdk/resource.
func (a *analysis) callsRelated(fi *fileInfo, p *pkgInfo, fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.Ident:
		d := p.decls[f.Name]
		return d != nil && d.isFunc
	case *ast.SelectorExpr:
		x, ok := f.X.(*ast.Ident)
		if !ok {
			return false
		}
		ipath, ok := fi.imports[x.Name]
		if !ok {
			return false
		}
		q := a.pkgs[ipath]
		if q == nil {
			return false
		}
		d := q.decls[f.Sel.Name]
		return d != nil && d.isFunc
	}
	return false
}

// callsOther reports whether fun is a function of an imported package
// that can't reach sdk/resource, so a schema URL passed to it never ends up
// in a resource.
func (a *analysis) callsOther(fi *fileInfo, fun ast.Expr) bool {
	f, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := f.X.(*ast.Ident)
	if !ok {
		return false
	}
	ipath, ok := fi.imports[x.Name]
	return ok && a.pkgs[ipath] == nil && !strings.HasPrefix(ipath, semconvPrefix)
}

// mergesVia reports whether d merges resources itself or calls a function
// that does and returns the result.
func (a *analysis) mergesVia(d *decl, seen map[*decl]bool) bool {
	if d.merges {
		return true
	}
	if seen[d] {
		return false
	}
	seen[d] = true
	for _, r := range d.refs {
		for _, t := range a.lookup(r) {
			if a.mergesVia(t, seen) {
				return true
			}
		}
	}
	return false
}

// isResourceType reports whether a result type carries a resource: a
// *resource.Resource, a resource.Option or a resource.Detector, or a type
// with a Detect method.
func (a *analysis) isResourceType(fi *fileInfo, p *pkgInfo, e ast.Expr) bool {
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
			continue
		case *ast.ArrayType:
			e = t.Elt
			continue
		case *ast.Ellipsis:
			e = t.Elt
			continue
		case *ast.Ident:
			if p.lp.ImportPath == resourcePath && resourceTypes[t.Name] {
				return true
			}
			return p.detects[t.Name]
		case *ast.SelectorExpr:
			x, ok := t.X.(*ast.Ident)
			if !ok {
				return false
			}
			ipath := fi.imports[x.Name]
			if ipath == resourcePath && resourceTypes[t.Sel.Name] {
				return true
			}
			if q := a.pkgs[ipath]; q != nil {
				return q.detects[t.Sel.Name]
			}
		}
		return false
	}
}

func (a *analysis) lookup(r ref) []*decl {
	p := a.pkgs[r.pkg]
	if p == nil {
		return nil
	}
	if r.kind == refMethod {
		var out []*decl
		for _, m := range p.methods[r.name] {
			if m.resish {
				out = append(out, m)
			}
		}
		return out
	}
	d := p.decls[r.name]
	if d == nil {
		return nil
	}
	if r.kind == refName && d.isFunc && !d.resish {
		return nil
	}
	return []*decl{d}
}

// summarize returns the schema URLs a declaration can put into a resource,
// with where each one is written.
func (a *analysis) summarize(d *decl) map[string]origin {
	if d.done {
		return d.summary
	}
	if d.visiting {
		return nil
	}
	d.visiting = true
	s := map[string]origin{}
	for _, o := range d.direct {
		if _, ok := s[o.URL]; !ok {
			s[o.URL] = o
		}
	}
	for _, r := range d.refs {
		for _, t := range a.lookup(r) {
			for url, o := range a.summarize(t) {
				if _, ok := s[url]; !ok {
					s[url] = o
				}
			}
		}
	}
	d.visiting = false
	d.done = true
	d.summary = s
	return s
}

// input is one schema URL that reaches a declaration, and where.
type input struct {
	URL    string `json:"schemaURL"`
	At     place  `json:"at"`
	Via    string `json:"via,omitempty"`
	Origin origin `json:"origin"`
	callee *decl
}

func (a *analysis) inputs(d *decl) []input {
	var out []input
	seen := map[string]bool{}
	add := func(in input) {
		k := in.URL + "|" + in.At.String() + "|" + in.Via
		if !seen[k] {
			seen[k] = true
			out = append(out, in)
		}
	}
	for _, o := range d.direct {
		add(input{URL: o.URL, At: o.Place, Origin: o})
	}
	for _, r := range d.refs {
		for _, t := range a.lookup(r) {
			s := a.summarize(t)
			urls := make([]string, 0, len(s))
			for url := range s {
				urls = append(urls, url)
			}
			sort.Strings(urls)
			for _, url := range urls {
				add(input{URL: url, At: r.at, Via: r.show, Origin: s[url], callee: t})
			}
		}
	}
	return out
}

func urlSet(ins []input) []string {
	m := map[string]bool{}
	for _, in := range ins {
		m[in.URL] = true
	}
	var out []string
	for u := range m {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}
