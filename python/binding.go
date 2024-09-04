package python

//#include "parser.h"
//TSLanguage *tree_sitter_python();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_python())
	return sitter.NewLanguage(ptr)
}
