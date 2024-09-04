package bash

//#include "parser.h"
//TSLanguage *tree_sitter_bash();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_bash())
	return sitter.NewLanguage(ptr)
}
