package typescript

//#include "parser.h"
//TSLanguage *tree_sitter_typescript();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_typescript())
	return sitter.NewLanguage(ptr)
}
