package tree_sitter_markdown_inline

//#include "parser.h"
//TSLanguage *tree_sitter_markdown_inline();
import "C"
import (
	"unsafe"

	sitter "github.com/boldsoftware/treesitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_markdown_inline())
	return sitter.NewLanguage(ptr)
}
