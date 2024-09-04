package scala

//#include "parser.h"
//TSLanguage *tree_sitter_scala();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_scala())
	return sitter.NewLanguage(ptr)
}
