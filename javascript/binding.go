package javascript

//#include "parser.h"
//TSLanguage *tree_sitter_javascript();
import "C"
import (
	"unsafe"

	"github.com/boldsoftware/treesitter"
)

func init() {
	ptr := unsafe.Pointer(C.tree_sitter_javascript())
	treesitter.RegisterLanguage("javascript", treesitter.NewLanguage(ptr))
}
