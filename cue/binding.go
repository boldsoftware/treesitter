package cue

//#include "parser.h"
//TSLanguage *tree_sitter_cue();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_cue())
	return sitter.NewLanguage(ptr)
}
