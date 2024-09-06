package javascript_test

import (
	"context"
	"testing"

	"github.com/boldsoftware/treesitter"
	_ "github.com/boldsoftware/treesitter/javascript"
	"github.com/stretchr/testify/assert"
)

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := treesitter.Parse(context.Background(), []byte("let a = 1"), "javascript")
	assert.NoError(err)
	assert.Equal(
		"(program (lexical_declaration (variable_declarator name: (identifier) value: (number))))",
		n.String(),
	)
}
