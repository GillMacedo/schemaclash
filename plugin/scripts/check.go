package main

import (
	"sort"
	"strings"
)

// finding is one place that merges resources with different schema URLs.
type finding struct {
	Code       string   `json:"code"`
	Severity   string   `json:"severity"`
	Function   string   `json:"function"`
	Place      place    `json:"place"`
	Dependency bool     `json:"inDependency"`
	SchemaURLs []string `json:"schemaURLs"`
	Inputs     []input  `json:"inputs"`
	Programs   []string `json:"programs"`
	Fix        string   `json:"fix"`
}

type sdkInfo struct {
	Module    string `json:"module"`
	SchemaURL string `json:"defaultSchemaURL"`
}

type report struct {
	Tool        string    `json:"tool"`
	Dir         string    `json:"dir"`
	Programs    []string  `json:"programs"`
	SDK         sdkInfo   `json:"sdk"`
	MergePoints int       `json:"mergePoints"`
	Findings    []finding `json:"findings"`
}

// check finds every function outside sdk/resource that merges or detects
// resources whose schema URLs differ.
func check(dir string, list []*listPkg) (*report, error) {
	a, err := newAnalysis(dir, list)
	if err != nil {
		return nil, err
	}
	var matched, mains []*listPkg
	for _, p := range list {
		if !p.DepOnly {
			matched = append(matched, p)
			if p.Name == "main" {
				mains = append(mains, p)
			}
		}
	}
	roots := mains
	if len(roots) == 0 {
		roots = matched
	}
	closure := map[string]map[string]bool{}
	var linked []string
	for _, r := range roots {
		c := map[string]bool{}
		var walk func(string)
		walk = func(path string) {
			if c[path] {
				return
			}
			c[path] = true
			if p := a.byPath[path]; p != nil {
				for _, imp := range p.Imports {
					walk(imp)
				}
			}
		}
		walk(r.ImportPath)
		if c[resourcePath] {
			closure[r.ImportPath] = c
			linked = append(linked, r.ImportPath)
		}
	}
	sort.Strings(linked)
	if len(linked) == 0 || a.resource == nil {
		what := plural(len(roots), "package", "packages")
		if len(mains) > 0 {
			what = plural(len(roots), "main package", "main packages")
		}
		return nil, refuse("no package links %s (checked %s), so there is nothing to check", resourcePath, what)
	}
	def := a.resource.decls["Default"]
	if def == nil {
		return nil, refuse("can't find resource.Default in %s; this SDK version may be laid out differently", a.resource.lp.Dir)
	}
	defURLs := a.summarize(def)
	if len(defURLs) != 1 {
		return nil, refuse("expected resource.Default in %s to use one schema URL, found %d; this SDK version may be laid out differently", a.resource.lp.Dir, len(defURLs))
	}
	rep := &report{Tool: "schemaclash " + version, Dir: dir, Programs: linked}
	for u := range defURLs {
		rep.SDK = sdkInfo{Module: a.modString(a.resource.lp), SchemaURL: u}
	}

	type cand struct {
		d   *decl
		ins []input
		set []string
	}
	var cands []cand
	conflicting := map[*decl][]string{}
	var paths []string
	for path := range a.pkgs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if path == resourcePath {
			continue
		}
		info := a.pkgs[path]
		var names []string
		for n := range info.decls {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			d := info.decls[n]
			if !a.mergesVia(d, map[*decl]bool{}) {
				continue
			}
			rep.MergePoints++
			ins := a.inputs(d)
			set := urlSet(ins)
			if len(set) >= 2 {
				cands = append(cands, cand{d, ins, set})
				conflicting[d] = set
			}
		}
	}
	for _, c := range cands {
		// A caller of a function that already conflicts, with no new
		// schema URL of its own, is the same problem: report it once.
		dup := false
		for _, in := range c.ins {
			if s, ok := conflicting[in.callee]; ok && in.callee != c.d && strings.Join(s, " ") == strings.Join(c.set, " ") {
				dup = true
			}
		}
		if dup {
			continue
		}
		var progs []string
		for _, r := range linked {
			if closure[r][c.d.pkg.lp.ImportPath] {
				progs = append(progs, r)
			}
		}
		sort.Slice(c.ins, func(i, j int) bool {
			if c.ins[i].URL != c.ins[j].URL {
				return c.ins[i].URL > c.ins[j].URL
			}
			return c.ins[i].At.Line < c.ins[j].At.Line
		})
		rep.Findings = append(rep.Findings, finding{
			Code:       "schema-url-conflict",
			Severity:   "ERROR",
			Function:   c.d.show,
			Place:      c.d.at,
			Dependency: !a.inMain(c.d.pkg.lp),
			SchemaURLs: c.set,
			Inputs:     c.ins,
			Programs:   progs,
			Fix:        fixText(rep.SDK, c.ins),
		})
	}
	return rep, nil
}

func ver(url string) string {
	if strings.HasPrefix(url, schemaPrefix) {
		return strings.TrimPrefix(url, schemaPrefix)
	}
	return url
}

// fixText says how to make every schema URL in a merge the same.
func fixText(sdk sdkInfo, ins []input) string {
	want := ver(sdk.SchemaURL)
	hasSDK := false
	for _, in := range ins {
		if in.URL == sdk.SchemaURL {
			hasSDK = true
		}
	}
	var parts []string
	seen := map[string]bool{}
	for _, in := range ins {
		if in.URL == sdk.SchemaURL && hasSDK {
			continue
		}
		o := in.Origin
		var s string
		switch {
		case !hasSDK:
			continue
		case o.inDep:
			s = o.mod + " uses schema " + ver(o.URL) + " (" + o.Place.String() + "): upgrade or downgrade it to a release built on the same semconv as the SDK"
		case strings.HasSuffix(o.What, ")"):
			s = "in " + o.Place.File + ", import " + semconvPrefix + want + " instead of " + strings.TrimSuffix(o.What[strings.Index(o.What, "(")+1:], ")")
		default:
			s = "in " + o.Place.File + ", change " + o.URL + " to " + sdk.SchemaURL
		}
		if !seen[s] {
			seen[s] = true
			parts = append(parts, s)
		}
	}
	if !hasSDK {
		return "use one semconv version for every schema URL merged here; " + want + " is the one " + sdk.Module + " uses. Or build the extra attributes with resource.NewSchemaless"
	}
	return strings.Join(parts, "; ") + ". Schema " + want + " is the one " + sdk.Module + " uses. Or build the extra attributes with resource.NewSchemaless, which has no schema URL"
}
