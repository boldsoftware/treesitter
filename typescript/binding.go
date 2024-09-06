package typescript

//#include "parser.h"
//TSLanguage *tree_sitter_typescript();
import "C"
import (
	"unsafe"

	"github.com/boldsoftware/treesitter"
)

func init() {
	ptr := unsafe.Pointer(C.tree_sitter_typescript())
	treesitter.RegisterLanguage("typescript", treesitter.NewLanguage(ptr))
}
