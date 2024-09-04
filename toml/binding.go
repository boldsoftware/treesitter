package toml

//#include "parser.h"
//TSLanguage *tree_sitter_toml();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_toml())
	return sitter.NewLanguage(ptr)
}
