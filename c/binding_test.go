package c_test

import (
	"context"
	"testing"

	"github.com/boldsoftware/treesitter"
	_ "github.com/boldsoftware/treesitter/c"
	"github.com/stretchr/testify/assert"
)

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := treesitter.Parse(context.Background(), []byte("int a = 2;"), "c")
	assert.NoError(err)
	assert.Equal(
		"(translation_unit (declaration type: (primitive_type) declarator: (init_declarator declarator: (identifier) value: (number_literal))))",
		n.String(),
	)
}
