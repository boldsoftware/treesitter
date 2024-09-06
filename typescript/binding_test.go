package typescript_test

import (
	"context"
	"testing"

	"github.com/boldsoftware/treesitter"
	_ "github.com/boldsoftware/treesitter/typescript"
	"github.com/stretchr/testify/assert"
)

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := treesitter.Parse(context.Background(), []byte("let a : number = 1;"), "typescript")
	assert.NoError(err)
	assert.Equal(
		"(program (lexical_declaration (variable_declarator name: (identifier) type: (type_annotation (predefined_type)) value: (number))))",
		n.String(),
	)
}
