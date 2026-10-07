// Report CRAP per authored function from Go AST complexity and statement coverage.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type block struct{ start, end, statements, count int }
type result struct {
	File       string  `json:"file"`
	Function   string  `json:"function"`
	Line       int     `json:"line"`
	Complexity int     `json:"complexity"`
	Coverage   float64 `json:"coverage"`
	CRAP       float64 `json:"crap"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func readCoverage() (map[string][]block, error) {
	f, err := os.Open("coverage.out")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string][]block{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) != 3 {
			continue
		}
		path, loc, ok := strings.Cut(fields[0], ":")
		if !ok {
			continue
		}
		span := strings.Split(loc, ",")
		if len(span) != 2 {
			continue
		}
		start, _ := strconv.Atoi(strings.Split(span[0], ".")[0])
		end, _ := strconv.Atoi(strings.Split(span[1], ".")[0])
		n, _ := strconv.Atoi(fields[1])
		count, _ := strconv.Atoi(fields[2])
		path = strings.TrimPrefix(path, "swimlane/backend/")
		out[path] = append(out[path], block{start, end, n, count})
	}
	return out, s.Err()
}
func complexity(body *ast.BlockStmt) int {
	n := 1
	ast.Inspect(body, func(node ast.Node) bool {
		switch v := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			n++
		case *ast.CaseClause:
			if v.List != nil {
				n++
			}
		case *ast.CommClause:
			if v.Comm != nil {
				n++
			}
		case *ast.BinaryExpr:
			if v.Op == token.LAND || v.Op == token.LOR {
				n++
			}
		}
		return true
	})
	return n
}
func score(file string, fn *ast.FuncDecl, fset *token.FileSet, blocks []block) result {
	start, end := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
	total, covered := 0, 0
	for _, b := range blocks {
		if b.start < start || b.end > end {
			continue
		}
		total += b.statements
		if b.count > 0 {
			covered += b.statements
		}
	}
	rate := 1.0
	if total > 0 {
		rate = float64(covered) / float64(total)
	}
	cc := complexity(fn.Body)
	miss := 1 - rate
	crap := float64(cc*cc)*miss*miss*miss + float64(cc)
	return result{file, fn.Name.Name, start, cc, rate, crap}
}
func run() error {
	coverage, err := readCoverage()
	if err != nil {
		return err
	}
	rows := []result{}
	fset := token.NewFileSet()
	files, err := filepath.Glob("internal/studio/*.go")
	if err != nil {
		return err
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range parsed.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				rows = append(rows, score(file, fn, fset, coverage[file]))
			}
		}
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile("../docs/go-quality.json", append(data, '\n'), 0644); err != nil {
		return err
	}
	failed := false
	maximum := 0.0
	for _, r := range rows {
		maximum = max(maximum, r.CRAP)
		if r.CRAP >= 10 {
			fmt.Printf("%s:%d %s complexity=%d coverage=%.1f%% CRAP=%.3f\n", r.File, r.Line, r.Function, r.Complexity, 100*r.Coverage, r.CRAP)
			failed = true
		}
	}
	fmt.Printf("%d Go API functions; maximum CRAP %.3f\n", len(rows), maximum)
	if failed {
		return fmt.Errorf("CRAP must be under 10")
	}
	return nil
}
