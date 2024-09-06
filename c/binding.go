package c

//#include "parser.h"
//TSLanguage *tree_sitter_c();
import "C"
import (
	"unsafe"

	"github.com/boldsoftware/treesitter"
)

func init() {
	ptr := unsafe.Pointer(C.tree_sitter_c())
	treesitter.RegisterLanguage("c", treesitter.NewLanguage(ptr))
}
