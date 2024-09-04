package kotlin

//#include "parser.h"
//TSLanguage *tree_sitter_kotlin();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_kotlin())
	return sitter.NewLanguage(ptr)
}
