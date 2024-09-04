package php

//#include "parser.h"
//TSLanguage *tree_sitter_php();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_php())
	return sitter.NewLanguage(ptr)
}
