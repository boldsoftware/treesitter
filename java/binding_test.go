package java_test

import (
	"context"
	"testing"

	sitter "github.com/boldsoftware/treesitter"
	"github.com/boldsoftware/treesitter/java"
	"github.com/stretchr/testify/assert"
)

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := sitter.Parse(context.Background(), []byte("import java.io.*;"), java.GetLanguage())
	assert.NoError(err)
	assert.Equal(
		"(program (import_declaration (scoped_identifier scope: (identifier) name: (identifier)) (asterisk)))",
		n.String(),
	)
}
