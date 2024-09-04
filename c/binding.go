package c

//#include "parser.h"
//TSLanguage *tree_sitter_c();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_c())
	return sitter.NewLanguage(ptr)
}
