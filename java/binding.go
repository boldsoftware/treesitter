package java

//#include "parser.h"
//TSLanguage *tree_sitter_java();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_java())
	return sitter.NewLanguage(ptr)
}
