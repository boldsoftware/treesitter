package rust_test

import (
	"context"
	"testing"

	sitter "github.com/boldsoftware/treesitter"
	"github.com/boldsoftware/treesitter/rust"
	"github.com/stretchr/testify/assert"
)

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := sitter.ParseCtx(context.Background(), []byte("mod one;"), rust.GetLanguage())
	assert.NoError(err)
	assert.Equal(
		"(source_file (mod_item name: (identifier)))",
		n.String(),
	)
}
