package ruby

//#include "parser.h"
//TSLanguage *tree_sitter_ruby();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_ruby())
	return sitter.NewLanguage(ptr)
}
