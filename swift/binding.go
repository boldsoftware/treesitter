package swift

//#include "parser.h"
//TSLanguage *tree_sitter_swift();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_swift())
	return sitter.NewLanguage(ptr)
}
