package golang

//#include "parser.h"
//TSLanguage *tree_sitter_go();
import "C"
import (
	"sync"
	"unsafe"

	"github.com/boldsoftware/treesitter"
)

var get = sync.OnceValue(func() *treesitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_go())
	return treesitter.NewLanguage(ptr)
})

func GetLanguage() *treesitter.Language {
	return get()
}
