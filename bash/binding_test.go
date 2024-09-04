package bash_test

import (
	"context"
	"testing"

	sitter "github.com/boldsoftware/treesitter"
	"github.com/boldsoftware/treesitter/bash"
	"github.com/stretchr/testify/assert"
)

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := sitter.Parse(context.Background(), []byte("echo 1"), bash.GetLanguage())
	assert.NoError(err)
	assert.Equal(
		"(program (command name: (command_name (word)) argument: (number)))",
		n.String(),
	)
}
