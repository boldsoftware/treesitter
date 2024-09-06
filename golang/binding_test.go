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
