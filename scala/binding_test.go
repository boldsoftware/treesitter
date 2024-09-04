package scala_test

import (
	"context"
	"testing"

	sitter "github.com/boldsoftware/treesitter"
	"github.com/boldsoftware/treesitter/scala"
	"github.com/stretchr/testify/assert"
)

const code = `package com.foo.bar`

const expected = `(compilation_unit (package_clause name: (package_identifier (identifier) (identifier) (identifier))))`

func TestGrammar(t *testing.T) {
	assert := assert.New(t)

	n, err := sitter.Parse(context.Background(), []byte(code), scala.GetLanguage())
	assert.NoError(err)
	assert.Equal(
		expected,
		n.String(),
	)
}
