package elm

//#cgo CXXFLAGS: -std=gnu++11
//#include "parser.h"
//TSLanguage *tree_sitter_elm();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_elm())
	return sitter.NewLanguage(ptr)
}
