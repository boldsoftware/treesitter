package golang

//#include "parser.h"
//TSLanguage *tree_sitter_go();
import "C"
import (
	"unsafe"

	"github.com/boldsoftware/treesitter"
)

func init() {
	ptr := unsafe.Pointer(C.tree_sitter_go())
	treesitter.RegisterLanguage("go", treesitter.NewLanguage(ptr))
}
