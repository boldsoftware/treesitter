package golang

//#include "parser.h"
//TSLanguage *tree_sitter_go();
import "C"
import (
	"unsafe"

	treesitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *treesitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_go())
	return treesitter.NewLanguage(ptr)
}
