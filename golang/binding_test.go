package golang_test

import (
	"context"
	"testing"

	"github.com/boldsoftware/treesitter"
	_ "github.com/boldsoftware/treesitter/golang"
	"github.com/stretchr/testify/assert"
)

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := treesitter.Parse(context.Background(), []byte("package main"), "go")
	assert.NoError(err)
	assert.Equal(
		"(source_file (package_clause (package_identifier)))",
		n.String(),
	)
}

// TestStringAllocs tests that cstrings map loaded up in NewLanguage
// means that string methods on nodes to do not allocate.
func TestStringAllocs(t *testing.T) {
	p := treesitter.NewParser("go")
	defer p.Close()
	data := []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, " + "playground", "!") // print
}
`)

	parseAllocs := testing.AllocsPerRun(1000, func() {
		p.Parse(context.Background(), nil, data)
	})

	var walkFn func(n treesitter.Node)
	walkFn = func(n treesitter.Node) {
		for i := 0; i < int(n.ChildCount()); i++ {
			_ = n.Type()
			_ = n.FieldNameForChild(i)
			walkFn(n.Child(i))
		}
	}

	allocs := testing.AllocsPerRun(1000, func() {
		tree, err := p.Parse(context.Background(), nil, data)
		if err != nil {
			t.Fatal(err)
		}
		walkFn(tree.RootNode())
	})

	nodeAllocs := allocs - parseAllocs
	t.Logf("parseAllocs=%v, nodeAllocs=%v", parseAllocs, nodeAllocs)

	const wantNodeAllocs = 0 // without cstrings this is 55
	if nodeAllocs != wantNodeAllocs {
		t.Errorf("AllocsPerRun=%v, want %v", nodeAllocs, wantNodeAllocs)
	}
}
