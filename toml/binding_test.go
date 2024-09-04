package toml_test

import (
	"context"
	"testing"

	sitter "github.com/boldsoftware/treesitter"
	"github.com/boldsoftware/treesitter/toml"
	"github.com/stretchr/testify/assert"
)

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := sitter.Parse(context.Background(), []byte(`key = "value"`), toml.GetLanguage())
	assert.NoError(err)
	assert.Equal(
		"(document (pair (bare_key) (string)))",
		n.String(),
	)
}
