// Command rpcgen generates a typed JSON-RPC client from a Go interface.
//
// The interface describes the methods a remote service exposes; every method
// must return (T, error) or error. The generated struct implements the
// interface by forwarding each call through jsonrpc4go.Call, so call sites are
// ordinary typed Go calls checked at compile time.
//
// Usage:
//
//	//go:generate go run github.com/sunquakes/jsonrpc4go/v2/cmd/rpcgen -iface IntRpcApi -service IntRpc -out intrpc_rpc.go
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// options configures one rpcgen run.
type options struct {
	dir     string
	file    string
	iface   string
	service string
	name    string
	pkg     string
	out     string
}

// param is one generated method parameter.
type param struct {
	name string
	typ  string
}

// methodModel is one generated method.
type methodModel struct {
	Name   string
	Params []param
	Args   []string // argument expressions passed to the client
	Result string   // result type; empty when the method only returns error
}

// ifaceModel is the parsed interface to generate a client for.
type ifaceModel struct {
	Name    string
	Methods []methodModel
}

func main() {
	o := options{}
	flag.StringVar(&o.dir, "dir", ".", "directory scanned for the interface when -file is empty")
	flag.StringVar(&o.file, "file", "", "single source file that declares the interface")
	flag.StringVar(&o.iface, "iface", "", "interface name to generate a client for (required)")
	flag.StringVar(&o.service, "service", "", "JSON-RPC service prefix (default: the interface name)")
	flag.StringVar(&o.name, "name", "", "generated struct name (default: <iface>Client)")
	flag.StringVar(&o.pkg, "pkg", "", "package name of the generated file (default: the source package)")
	flag.StringVar(&o.out, "out", "", "output file (default: stdout)")
	flag.Parse()
	if err := run(o, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rpcgen:", err)
		os.Exit(1)
	}
}

// run parses the interface and writes the generated source.
func run(o options, stdout io.Writer) error {
	if o.iface == "" {
		return errors.New("-iface is required")
	}
	pkg, model, err := parseInterface(o)
	if err != nil {
		return err
	}
	service := o.service
	if service == "" {
		service = o.iface
	}
	name := o.name
	if name == "" {
		name = o.iface + "Client"
	}
	outPkg := o.pkg
	if outPkg == "" {
		outPkg = pkg
	}
	src, err := render(outPkg, name, service, model)
	if err != nil {
		return err
	}
	src, err = format.Source(src)
	if err != nil {
		return fmt.Errorf("format generated source: %w", err)
	}
	if o.out == "" {
		_, err = stdout.Write(src)
		return err
	}
	return os.WriteFile(o.out, src, 0o644)
}

// parseInterface locates the interface and converts its methods to a model.
func parseInterface(o options) (string, *ifaceModel, error) {
	fset := token.NewFileSet()
	var (
		files []*ast.File
		pkg   string
	)
	if o.file != "" {
		f, perr := parser.ParseFile(fset, o.file, nil, parser.ParseComments)
		if perr != nil {
			return "", nil, perr
		}
		files, pkg = []*ast.File{f}, f.Name.Name
	} else {
		entries, derr := os.ReadDir(o.dir)
		if derr != nil {
			return "", nil, derr
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, perr := parser.ParseFile(fset, filepath.Join(o.dir, name), nil, parser.ParseComments)
			if perr != nil {
				return "", nil, perr
			}
			if pkg == "" {
				pkg = f.Name.Name
			}
			files = append(files, f)
		}
	}
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != o.iface {
					continue
				}
				it, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					return "", nil, fmt.Errorf("%s is not an interface", o.iface)
				}
				model, err := buildModel(o.iface, it)
				if err != nil {
					return "", nil, err
				}
				return pkg, model, nil
			}
		}
	}
	return "", nil, fmt.Errorf("interface %q not found", o.iface)
}

// buildModel converts an interface type into the generation model.
func buildModel(name string, it *ast.InterfaceType) (*ifaceModel, error) {
	model := &ifaceModel{Name: name}
	if it.Methods == nil {
		return model, nil
	}
	for _, field := range it.Methods.List {
		ft, ok := field.Type.(*ast.FuncType)
		if !ok {
			return nil, fmt.Errorf("embedded interfaces are not supported: interface %s embeds %s",
				name, types.ExprString(field.Type))
		}
		if ft.TypeParams != nil && len(ft.TypeParams.List) > 0 {
			return nil, errors.New("generic methods are not supported")
		}
		for _, ident := range field.Names {
			m, err := buildMethod(ident.Name, ft)
			if err != nil {
				return nil, err
			}
			model.Methods = append(model.Methods, m)
		}
	}
	return model, nil
}

// buildMethod converts one interface method into the generation model.
func buildMethod(name string, ft *ast.FuncType) (methodModel, error) {
	m := methodModel{Name: name}
	auto := 0
	if ft.Params != nil {
		for _, field := range ft.Params.List {
			if _, ok := field.Type.(*ast.Ellipsis); ok {
				return m, fmt.Errorf("method %q is variadic; variadic methods are not supported", name)
			}
			typ := types.ExprString(field.Type)
			if len(field.Names) == 0 {
				p := fmt.Sprintf("a%d", auto)
				auto++
				m.Params = append(m.Params, param{name: p, typ: typ})
				m.Args = append(m.Args, p)
				continue
			}
			for _, ident := range field.Names {
				p := ident.Name
				if p == "_" {
					p = fmt.Sprintf("a%d", auto)
					auto++
				}
				m.Params = append(m.Params, param{name: p, typ: typ})
				m.Args = append(m.Args, p)
			}
		}
	}
	var results []string
	if ft.Results != nil {
		for _, field := range ft.Results.List {
			typ := types.ExprString(field.Type)
			n := len(field.Names)
			if n == 0 {
				n = 1
			}
			for i := 0; i < n; i++ {
				results = append(results, typ)
			}
		}
	}
	switch len(results) {
	case 0:
		return m, fmt.Errorf("method %q must return (value, error) or error", name)
	case 1:
		if results[0] != "error" {
			return m, fmt.Errorf("method %q must return (value, error) or error, not %s", name, results[0])
		}
	case 2:
		if results[1] != "error" {
			return m, fmt.Errorf("method %q: second result must be error, got %s", name, results[1])
		}
		if results[0] == "error" {
			return m, fmt.Errorf("method %q: first result must not be error", name)
		}
		m.Result = results[0]
	default:
		return m, fmt.Errorf("method %q has %d results; needs at most two", name, len(results))
	}
	return m, nil
}

// render produces the generated source for the model.
func render(pkg, name, service string, model *ifaceModel) ([]byte, error) {
	callPrefix, clientPrefix := "jsonrpc4go.", "client."
	var imports []string
	switch pkg {
	case "jsonrpc4go":
		callPrefix, clientPrefix = "", "client."
		imports = []string{`"github.com/sunquakes/jsonrpc4go/v2/client"`}
	case "client":
		callPrefix, clientPrefix = "jsonrpc4go.", ""
		imports = []string{`"github.com/sunquakes/jsonrpc4go/v2"`}
	default:
		imports = []string{
			`"github.com/sunquakes/jsonrpc4go/v2"`,
			`"github.com/sunquakes/jsonrpc4go/v2/client"`,
		}
	}
	var b bytes.Buffer
	b.WriteString("// Code generated by rpcgen. DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import (\n")
	for _, imp := range imports {
		fmt.Fprintf(&b, "\t%s\n", imp)
	}
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "// %s is a typed JSON-RPC client for the %q service.\n", name, service)
	fmt.Fprintf(&b, "type %s struct {\n\tc %sClient\n}\n\n", name, clientPrefix)
	fmt.Fprintf(&b, "// New%s wraps a JSON-RPC client with typed methods for the %q service.\n", name, service)
	fmt.Fprintf(&b, "func New%s(c %sClient) *%s {\n\treturn &%s{c: c}\n}\n\n", name, clientPrefix, name, name)
	fmt.Fprintf(&b, "// Client returns the underlying JSON-RPC client.\n")
	fmt.Fprintf(&b, "func (p *%s) Client() %sClient {\n\treturn p.c\n}\n\n", name, clientPrefix)
	for _, m := range model.Methods {
		args := ""
		if len(m.Args) > 0 {
			args = ", " + strings.Join(m.Args, ", ")
		}
		fmt.Fprintf(&b, "// %s calls %q.\n", m.Name, service+"/"+m.Name)
		if m.Result == "" {
			fmt.Fprintf(&b, "func (p *%s) %s(%s) error {\n", name, m.Name, paramsString(m.Params))
			fmt.Fprintf(&b, "\t_, err := %sCall[any](p.c, %q%s)\n\treturn err\n}\n\n", callPrefix, m.Name, args)
			continue
		}
		fmt.Fprintf(&b, "func (p *%s) %s(%s) (%s, error) {\n", name, m.Name, paramsString(m.Params), m.Result)
		fmt.Fprintf(&b, "\treturn %sCall[%s](p.c, %q%s)\n}\n\n", callPrefix, m.Result, m.Name, args)
	}
	fmt.Fprintf(&b, "var _ %s = (*%s)(nil)\n", model.Name, name)
	return b.Bytes(), nil
}

// paramsString renders a parameter list, grouping adjacent parameters that
// share a type ("a, b int").
func paramsString(ps []param) string {
	var parts []string
	for i := 0; i < len(ps); {
		j := i + 1
		names := []string{ps[i].name}
		for j < len(ps) && ps[j].typ == ps[i].typ {
			names = append(names, ps[j].name)
			j++
		}
		parts = append(parts, strings.Join(names, ", ")+" "+ps[i].typ)
		i = j
	}
	return strings.Join(parts, ", ")
}
