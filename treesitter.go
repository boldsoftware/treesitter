package treesitter

// #include "bindings.h"
import "C"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"math"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Parse is a shortcut for parsing bytes of source code,
// returns root node
func Parse(ctx context.Context, content []byte, lang string) (Node, error) {
	p := NewParser(lang)
	tree, err := p.Parse(ctx, nil, content)
	if err != nil {
		return Node{}, err
	}

	return tree.RootNode(), nil
}

// Parser produces concrete syntax tree based on source code using Language
type Parser struct {
	c      *C.TSParser
	cancel *uintptr
	lang   *Language
}

// NewParser creates new Parser.
func NewParser(language string) *Parser {
	lang := languages[language]
	if lang == nil {
		panic(fmt.Sprintf("language %s not found; missing import _ statement", language))
	}
	cancel := uintptr(0)
	p := &Parser{c: C.ts_parser_new(), cancel: &cancel, lang: lang}
	C.ts_parser_set_cancellation_flag(p.c, (*C.size_t)(unsafe.Pointer(p.cancel)))
	C.ts_parser_set_language(p.c, (*C.struct_TSLanguage)(lang.ptr))
	runtime.SetFinalizer(p, (*Parser).Close)
	return p
}

// maintain a map of read functions that can be called from C
var readFuncs = &readFuncsMap{funcs: make(map[int]ReadFunc)}

// ReadFunc is a function to retrieve a chunk of text at a given byte offset and (row, column) position
// it should return nil to indicate the end of the document
type ReadFunc func(offset uint32, position Point) []byte

// InputEncoding is a encoding of the text to parse
type InputEncoding int

const (
	InputEncodingUTF8 InputEncoding = iota
	InputEncodingUTF16
)

// Input defines parameters for parse method
type Input struct {
	Read     ReadFunc
	Encoding InputEncoding
}

var (
	ErrOperationLimit = errors.New("operation limit was hit")
	ErrNoLanguage     = errors.New("cannot parse without language")
)

// Parse produces new Tree from content using old tree
func (p *Parser) Parse(ctx context.Context, oldTree *Tree, content []byte) (*Tree, error) {
	var cTree *C.TSTree
	if oldTree != nil {
		cTree = oldTree.c
	}

	parseComplete := make(chan struct{})

	// run goroutine only if context is cancelable to avoid performance impact
	if ctx.Done() != nil {
		go func() {
			select {
			case <-ctx.Done():
				atomic.StoreUintptr(p.cancel, 1)
			case <-parseComplete:
				return
			}
		}()
	}

	input := C.CBytes(content)
	cTree = C.ts_parser_parse_string(p.c, cTree, (*C.char)(input), C.uint32_t(len(content)))
	close(parseComplete)
	C.free(input)

	return p.convertTSTree(ctx, cTree)
}

// ParseInput produces new Tree by reading from a callback defined in input
// it is useful if your data is stored in specialized data structure
// as it will avoid copying the data into []bytes
// and faster access to edited part of the data
func (p *Parser) ParseInput(ctx context.Context, oldTree *Tree, input Input) (*Tree, error) {
	var cTree *C.TSTree
	if oldTree != nil {
		cTree = oldTree.c
	}

	funcID := readFuncs.register(input.Read)
	cTree = C.call_ts_parser_parse(p.c, cTree, C.int(funcID), C.TSInputEncoding(input.Encoding))
	readFuncs.unregister(funcID)

	return p.convertTSTree(ctx, cTree)
}

// convertTSTree converts the tree-sitter response into a *Tree or an error.
//
// tree-sitter can fail for 3 reasons:
// - cancelation
// - operation limit hit
// - no language set
//
// We check for all those conditions if ther return value is nil.
// see: https://github.com/tree-sitter/tree-sitter/blob/7890a29db0b186b7b21a0a95d99fa6c562b8316b/lib/include/tree_sitter/api.h#L209-L246
func (p *Parser) convertTSTree(ctx context.Context, tsTree *C.TSTree) (*Tree, error) {
	if tsTree == nil {
		if ctx.Err() != nil {
			// reset cancellation flag so the parse can be re-used
			atomic.StoreUintptr(p.cancel, 0)
			// context cancellation caused a timeout, return that error
			return nil, ctx.Err()
		}

		if C.ts_parser_language(p.c) == nil {
			return nil, ErrNoLanguage
		}

		return nil, ErrOperationLimit
	}

	return p.newTree(tsTree), nil
}

// OperationLimit returns the duration in microseconds that parsing is allowed to take
func (p *Parser) OperationLimit() int {
	return int(C.ts_parser_timeout_micros(p.c))
}

// SetOperationLimit limits the maximum duration in microseconds that parsing should be allowed to take before halting
func (p *Parser) SetOperationLimit(limit int) {
	C.ts_parser_set_timeout_micros(p.c, C.uint64_t(limit))
}

// Reset causes the parser to parse from scratch on the next call to parse, instead of resuming
// so that it sees the changes to the beginning of the source code.
func (p *Parser) Reset() {
	C.ts_parser_reset(p.c)
}

// SetIncludedRanges sets text ranges of a file
func (p *Parser) SetIncludedRanges(ranges []Range) {
	cRanges := make([]C.TSRange, len(ranges))
	for i, r := range ranges {
		cRanges[i] = C.TSRange{
			start_point: C.TSPoint{
				row:    C.uint32_t(r.StartPoint.Row),
				column: C.uint32_t(r.StartPoint.Column),
			},
			end_point: C.TSPoint{
				row:    C.uint32_t(r.EndPoint.Row),
				column: C.uint32_t(r.EndPoint.Column),
			},
			start_byte: C.uint32_t(r.StartByte),
			end_byte:   C.uint32_t(r.EndByte),
		}
	}
	C.ts_parser_set_included_ranges(p.c, (*C.TSRange)(unsafe.Pointer(&cRanges[0])), C.uint(len(ranges)))
}

// Debug enables debug output to stderr
func (p *Parser) Debug() {
	logger := C.stderr_logger_new(true)
	C.ts_parser_set_logger(p.c, logger)
}

// Close should be called to ensure that all the memory used by the parse is freed.
//
// As the constructor in go-tree-sitter would set this func call through runtime.SetFinalizer,
// parser.Close() will be called by Go's garbage collector and users would not have to call this manually.
func (p *Parser) Close() {
	if p.c != nil {
		C.ts_parser_delete(p.c)
		p.c = nil
	}
}

type Point struct {
	Row    int
	Column int
}

type Range struct {
	StartPoint Point
	EndPoint   Point
	StartByte  int
	EndByte    int
}

// we use cache for nodes on normal tree object
// it prevent run of SetFinalizer as it introduces cycle
// we can workaround it using separate object
// for details see: https://github.com/golang/go/issues/7358#issuecomment-66091558
type baseTree struct {
	c *C.TSTree
}

// newTree creates a new tree object from a C pointer. The function will set a finalizer for the object,
// thus no free is needed for it.
func (p *Parser) newTree(c *C.TSTree) *Tree {
	base := &baseTree{c: c}
	runtime.SetFinalizer(base, (*baseTree).Close)

	newTree := &Tree{p: p, baseTree: base}
	return newTree
}

// Tree represents the syntax tree of an entire source code file
// Note: Tree instances are not thread safe;
// you must copy a tree if you want to use it on multiple threads simultaneously.
type Tree struct {
	*baseTree

	// p is a pointer to a Parser that produced the Tree. Only used to keep Parser alive.
	// Otherwise Parser may be GC'ed (and deleted by the finalizer) while some Tree objects are still in use.
	p *Parser
}

// Copy returns a new copy of a tree
func (t *Tree) Copy() *Tree {
	return t.p.newTree(C.ts_tree_copy(t.c))
}

// RootNode returns root node of a tree
func (t *Tree) RootNode() Node {
	n := C.ts_tree_root_node(t.c)
	return Node{c: (C.TSNode)(n), t: t}
}

func (t *Tree) goString(ptr *C.char) string {
	return t.p.lang.goString(ptr)
}

// Close should be called to ensure that all the memory used by the tree is freed.
//
// As the constructor in go-tree-sitter would set this func call through runtime.SetFinalizer,
// parser.Close() will be called by Go's garbage collector and users would not have to call this manually.
func (t *baseTree) Close() {
	if t.c != nil {
		C.ts_tree_delete(t.c)
		t.c = nil
	}
}

type EditInput struct {
	StartIndex  int
	OldEndIndex int
	NewEndIndex int
	StartPoint  Point
	OldEndPoint Point
	NewEndPoint Point
}

func (i EditInput) c() *C.TSInputEdit {
	return &C.TSInputEdit{
		start_byte:   C.uint32_t(i.StartIndex),
		old_end_byte: C.uint32_t(i.OldEndIndex),
		new_end_byte: C.uint32_t(i.NewEndIndex),
		start_point: C.TSPoint{
			row:    C.uint32_t(i.StartPoint.Row),
			column: C.uint32_t(i.StartPoint.Column),
		},
		old_end_point: C.TSPoint{
			row:    C.uint32_t(i.OldEndPoint.Row),
			column: C.uint32_t(i.OldEndPoint.Column),
		},
		new_end_point: C.TSPoint{
			row:    C.uint32_t(i.OldEndPoint.Row),
			column: C.uint32_t(i.OldEndPoint.Column),
		},
	}
}

// Edit the syntax tree to keep it in sync with source code that has been edited.
func (t *Tree) Edit(i EditInput) {
	if t.c == nil {
		panic("tree is closed")
	}
	C.ts_tree_edit(t.c, i.c())
}

var languages = map[string]*Language{}

// RegisterLanguage registers a language with the parser.
// It is called on init from packages that contain a language parser. E.g.
//
//	import _ "github.com/boldsoftware/treesitter/golang"
//
// calls RegisterLanguage("go", l) allowing go to be used as a language.
func RegisterLanguage(langName string, l *Language) {
	if languages[langName] != nil {
		panic("language " + langName + " already registered")
	}
	languages[langName] = l
}

// Language defines how to parse a particular programming language
type Language struct {
	ptr      unsafe.Pointer
	cstrings map[*C.char]string // unchanged after NewLanguage
}

// NewLanguage creates new Language from c pointer
func NewLanguage(ptr unsafe.Pointer) *Language {
	l := &Language{ptr: ptr, cstrings: make(map[*C.char]string)}
	// load up cstrings
	for i := 0; i < l.FieldCount(); i++ {
		ptr := l.cFieldName(i)
		l.cstrings[ptr] = C.GoString(ptr)
	}
	for i := 0; i < l.SymbolCount(); i++ {
		ptr := l.cSymbolName(Symbol(i))
		l.cstrings[ptr] = C.GoString(ptr)
	}
	return l
}

func (l *Language) goString(ptr *C.char) string {
	if s, found := l.cstrings[ptr]; found {
		return s
	}
	return C.GoString(ptr)
}

func (l *Language) cSymbolName(s Symbol) *C.char {
	return C.ts_language_symbol_name((*C.TSLanguage)(l.ptr), s)
}

// SymbolName returns a node type string for the given Symbol.
func (l *Language) SymbolName(s Symbol) string { return l.goString(l.cSymbolName(s)) }

// SymbolType returns named, anonymous, or a hidden type for a Symbol.
func (l *Language) SymbolType(s Symbol) SymbolType {
	return SymbolType(C.ts_language_symbol_type((*C.TSLanguage)(l.ptr), s))
}

// SymbolCount returns the number of distinct field names in the language.
func (l *Language) SymbolCount() int {
	return int(C.ts_language_symbol_count((*C.TSLanguage)(l.ptr)))
}

func (l *Language) cFieldName(idx int) *C.char {
	return C.ts_language_field_name_for_id((*C.TSLanguage)(l.ptr), C.ushort(idx))
}

func (l *Language) FieldName(idx int) string { return l.goString(l.cFieldName(idx)) }

func (l *Language) FieldCount() int {
	return int(C.ts_language_field_count((*C.TSLanguage)(l.ptr)))
}

// Node represents a single node in the syntax tree.
//
// It tracks its start and end positions in the source code,
// as well as its relation to other nodes like its parent, siblings and children.
type Node struct {
	c C.TSNode
	t *Tree
}

// TODO: consider unexporting this function
func (n Node) ID() uintptr {
	return uintptr(n.c.id)
}

// StartByte returns the node's start byte.
func (n Node) StartByte() int {
	return int(C.ts_node_start_byte(n.c))
}

// EndByte returns the node's end byte.
func (n Node) EndByte() int {
	return int(C.ts_node_end_byte(n.c))
}

// StartPoint returns the node's start position in terms of rows and columns.
func (n Node) StartPoint() Point {
	p := C.ts_node_start_point(n.c)
	return Point{
		Row:    int(p.row),
		Column: int(p.column),
	}
}

// EndPoint returns the node's end position in terms of rows and columns.
func (n Node) EndPoint() Point {
	p := C.ts_node_end_point(n.c)
	return Point{
		Row:    int(p.row),
		Column: int(p.column),
	}
}

func (n Node) Range() Range {
	return Range{
		StartByte:  n.StartByte(),
		EndByte:    n.EndByte(),
		StartPoint: n.StartPoint(),
		EndPoint:   n.EndPoint(),
	}
}

// Symbol returns the node's type as a Symbol.
func (n Node) Symbol() Symbol {
	return C.ts_node_symbol(n.c)
}

// Type returns the node's type as a string.
func (n Node) Type() string {
	return n.t.goString(C.ts_node_type(n.c))
}

// String returns an S-expression representing the node as a string.
func (n Node) String() string {
	if n == (Node{}) {
		return "(nil)"
	}
	ptr := C.ts_node_string(n.c)
	defer C.free(unsafe.Pointer(ptr))
	return C.GoString(ptr)
}

// Equal checks if two nodes are identical.
func (n Node) Equal(other Node) bool {
	defer runtime.KeepAlive(n.t)
	return bool(C.ts_node_eq(n.c, other.c))
}

// IsNull checks if the node is null.
func (n Node) IsNull() bool {
	defer runtime.KeepAlive(n.t)
	return bool(C.ts_node_is_null(n.c))
}

// IsNamed checks if the node is *named*.
// Named nodes correspond to named rules in the grammar,
// whereas *anonymous* nodes correspond to string literals in the grammar.
func (n Node) IsNamed() bool {
	defer runtime.KeepAlive(n.t)
	return bool(C.ts_node_is_named(n.c))
}

// IsMissing checks if the node is *missing*.
// Missing nodes are inserted by the parser in order to recover from certain kinds of syntax errors.
func (n Node) IsMissing() bool {
	defer runtime.KeepAlive(n.t)
	return bool(C.ts_node_is_missing(n.c))
}

// IsExtra checks if the node is *extra*.
// Extra nodes represent things like comments, which are not required the grammar, but can appear anywhere.
func (n Node) IsExtra() bool {
	defer runtime.KeepAlive(n.t)
	return bool(C.ts_node_is_extra(n.c))
}

// IsError checks if the node is a syntax error.
// Syntax errors represent parts of the code that could not be incorporated into a valid syntax tree.
func (n Node) IsError() bool {
	return n.Symbol() == math.MaxUint16
}

// HasChanges checks if a syntax node has been edited.
func (n Node) HasChanges() bool {
	defer runtime.KeepAlive(n.t)
	return bool(C.ts_node_has_changes(n.c))
}

// HasError check if the node is a syntax error or contains any syntax errors.
func (n Node) HasError() bool {
	defer runtime.KeepAlive(n.t)
	return bool(C.ts_node_has_error(n.c))
}

// Parent returns the node's immediate parent.
func (n Node) Parent() Node {
	nn := C.ts_node_parent(n.c)
	return Node{c: (C.TSNode)(nn), t: n.t}
}

// Child returns the node's child at the given index, where zero represents the first child.
func (n Node) Child(idx int) Node {
	nn := C.ts_node_child(n.c, C.uint32_t(idx))
	return Node{c: (C.TSNode)(nn), t: n.t}
}

// NamedChild returns the node's *named* child at the given index.
func (n Node) NamedChild(idx int) Node {
	nn := C.ts_node_named_child(n.c, C.uint32_t(idx))
	return Node{c: (C.TSNode)(nn), t: n.t}
}

func (n Node) ChildCount() int {
	defer runtime.KeepAlive(n.t)
	return int(C.ts_node_child_count(n.c))
}

// NamedChildCount returns the node's number of *named* children.
func (n Node) NamedChildCount() int {
	defer runtime.KeepAlive(n.t)
	return int(C.ts_node_named_child_count(n.c))
}

// Children returns an iterator over n's children.
func (n Node) Children() iter.Seq2[int, Node] {
	return func(yield func(int, Node) bool) {
		for i := range n.ChildCount() {
			if !yield(i, n.Child(i)) {
				return
			}
		}
	}
}

// NamedChildren returns an iterator over n's named children.
func (n Node) NamedChildren() iter.Seq2[int, Node] {
	return func(yield func(int, Node) bool) {
		for i := range n.NamedChildCount() {
			if !yield(i, n.NamedChild(i)) {
				return
			}
		}
	}
}

// ChildByFieldName returns the node's child with the given field name.
func (n Node) ChildByFieldName(name string) Node {
	str := C.CString(name)
	defer C.free(unsafe.Pointer(str))
	nn := C.ts_node_child_by_field_name(n.c, str, C.uint32_t(len(name)))
	return Node{c: (C.TSNode)(nn), t: n.t}
}

// FieldNameForChild returns the field name of the child at the given index, or "" if not named.
func (n Node) FieldNameForChild(idx int) string {
	return n.t.goString(C.ts_node_field_name_for_child(n.c, C.uint32_t(idx)))
}

// NextSibling returns the node's next sibling.
func (n Node) NextSibling() Node {
	nn := C.ts_node_next_sibling(n.c)
	return Node{c: (C.TSNode)(nn), t: n.t}
}

// NextNamedSibling returns the node's next *named* sibling.
func (n Node) NextNamedSibling() Node {
	nn := C.ts_node_next_named_sibling(n.c)
	return Node{c: (C.TSNode)(nn), t: n.t}
}

// PrevSibling returns the node's previous sibling.
func (n Node) PrevSibling() Node {
	nn := C.ts_node_prev_sibling(n.c)
	return Node{c: (C.TSNode)(nn), t: n.t}
}

// PrevNamedSibling returns the node's previous *named* sibling.
func (n Node) PrevNamedSibling() Node {
	nn := C.ts_node_prev_named_sibling(n.c)
	return Node{c: (C.TSNode)(nn), t: n.t}
}

// Edit the node to keep it in-sync with source code that has been edited.
func (n Node) Edit(i EditInput) {
	C.ts_node_edit(&n.c, i.c())
}

func (n Node) NamedDescendantForPointRange(start Point, end Point) Node {
	cStartPoint := C.TSPoint{
		row:    C.uint32_t(start.Row),
		column: C.uint32_t(start.Column),
	}
	cEndPoint := C.TSPoint{
		row:    C.uint32_t(end.Row),
		column: C.uint32_t(end.Column),
	}
	nn := C.ts_node_named_descendant_for_point_range(n.c, cStartPoint, cEndPoint)
	return Node{c: (C.TSNode)(nn), t: n.t}
}

type Symbol = C.TSSymbol

type SymbolType int

const (
	SymbolTypeRegular SymbolType = iota
	SymbolTypeAnonymous
	SymbolTypeAuxiliary
)

var symbolTypeNames = []string{
	"Regular",
	"Anonymous",
	"Auxiliary",
}

func (t SymbolType) String() string {
	return symbolTypeNames[t]
}

// TreeCursor allows you to walk a syntax tree more efficiently than is
// possible using the `Node` functions. It is a mutable object that is always
// on a certain syntax node, and can be moved imperatively to different nodes.
type TreeCursor struct {
	c *C.TSTreeCursor
	t *Tree
}

// NewTreeCursor creates a new tree cursor starting from the given node.
func NewTreeCursor(n Node) *TreeCursor {
	cc := C.ts_tree_cursor_new(n.c)
	c := &TreeCursor{
		c: &cc,
		t: n.t,
	}
	runtime.SetFinalizer(c, (*TreeCursor).Close)
	return c
}

// Close should be called to ensure that all the memory used by the tree cursor
// is freed.
//
// As the constructor in go-tree-sitter would set this func call through runtime.SetFinalizer,
// parser.Close() will be called by Go's garbage collector and users would not have to call this manually.
func (c *TreeCursor) Close() {
	if c.c != nil {
		C.ts_tree_cursor_delete(c.c)
		c.c = nil
	}
}

// Reset re-initializes a tree cursor to start at a different node.
func (c *TreeCursor) Reset(n Node) {
	C.ts_tree_cursor_reset(c.c, n.c)
}

// CurrentNode of the tree cursor.
func (c *TreeCursor) CurrentNode() Node {
	n := C.ts_tree_cursor_current_node(c.c)
	return Node{c: (C.TSNode)(n), t: c.t}
}

// CurrentFieldName gets the field name of the tree cursor's current node.
//
// This returns empty string if the current node doesn't have a field.
func (c *TreeCursor) CurrentFieldName() string {
	defer runtime.KeepAlive(c.t)
	return c.t.goString(C.ts_tree_cursor_current_field_name(c.c))
}

// GoToParent moves the cursor to the parent of its current node.
//
// This returns `true` if the cursor successfully moved, and returns `false`
// if there was no parent node (the cursor was already on the root node).
func (c *TreeCursor) GoToParent() bool {
	defer runtime.KeepAlive(c.t)
	return bool(C.ts_tree_cursor_goto_parent(c.c))
}

// GoToNextSibling moves the cursor to the next sibling of its current node.
//
// This returns `true` if the cursor successfully moved, and returns `false`
// if there was no next sibling node.
func (c *TreeCursor) GoToNextSibling() bool {
	defer runtime.KeepAlive(c.t)
	return bool(C.ts_tree_cursor_goto_next_sibling(c.c))
}

// GoToFirstChild moves the cursor to the first child of its current node.
//
// This returns `true` if the cursor successfully moved, and returns `false`
// if there were no children.
func (c *TreeCursor) GoToFirstChild() bool {
	defer runtime.KeepAlive(c.t)
	return bool(C.ts_tree_cursor_goto_first_child(c.c))
}

// GoToFirstChildForByte moves the cursor to the first child of its current node
// that extends beyond the given byte offset.
//
// This returns the index of the child node if one was found, and returns -1
// if no such child was found.
func (c *TreeCursor) GoToFirstChildForByte(b uint32) int64 {
	defer runtime.KeepAlive(c.t)
	return int64(C.ts_tree_cursor_goto_first_child_for_byte(c.c, C.uint32_t(b)))
}

// QueryErrorType - value that indicates the type of QueryError.
type QueryErrorType int

const (
	QueryErrorNone QueryErrorType = iota
	QueryErrorSyntax
	QueryErrorNodeType
	QueryErrorField
	QueryErrorCapture
	QueryErrorStructure
	QueryErrorLanguage
)

func QueryErrorTypeToString(errorType QueryErrorType) string {
	switch errorType {
	case QueryErrorNone:
		return "none"
	case QueryErrorNodeType:
		return "node type"
	case QueryErrorField:
		return "field"
	case QueryErrorCapture:
		return "capture"
	case QueryErrorSyntax:
		return "syntax"
	default:
		return "unknown"
	}
}

// QueryError - if there is an error in the query,
// then the Offset argument will be set to the byte offset of the error,
// and the Type argument will be set to a value that indicates the type of error.
type QueryError struct {
	Offset  uint32
	Type    QueryErrorType
	Message string
}

func (qe *QueryError) Error() string {
	return qe.Message
}

// Query API
type Query struct {
	c *C.TSQuery
}

// NewQuery creates a query by specifying a string containing one or more patterns.
// In case of error returns QueryError.
func NewQuery(pattern []byte, language string) (*Query, error) {
	var (
		erroff  C.uint32_t
		errtype C.TSQueryError
	)
	lang := languages[language]
	if lang == nil {
		return nil, fmt.Errorf("unknown language %s; missing import _ statement", language)
	}

	input := C.CBytes(pattern)
	c := C.ts_query_new(
		(*C.struct_TSLanguage)(lang.ptr),
		(*C.char)(input),
		C.uint32_t(len(pattern)),
		&erroff,
		&errtype,
	)
	C.free(input)
	if errtype != C.TSQueryError(QueryErrorNone) {
		errorOffset := uint32(erroff)
		// search for the line containing the offset
		line := 1
		line_start := 0
		for i, c := range pattern {
			line_start = i
			if uint32(i) >= errorOffset {
				break
			}
			if c == '\n' {
				line++
			}
		}
		column := int(errorOffset) - line_start
		errorType := QueryErrorType(errtype)
		errorTypeToString := QueryErrorTypeToString(errorType)

		var message string
		switch errorType {
		// errors that apply to a single identifier
		case QueryErrorNodeType:
			fallthrough
		case QueryErrorField:
			fallthrough
		case QueryErrorCapture:
			// find identifier at input[errorOffset]
			// and report it in the error message
			s := string(pattern[errorOffset:])
			identifierRegexp := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*`)
			m := identifierRegexp.FindStringSubmatch(s)
			if len(m) > 0 {
				message = fmt.Sprintf("invalid %s '%s' at line %d column %d",
					errorTypeToString, m[0], line, column)
			} else {
				message = fmt.Sprintf("invalid %s at line %d column %d",
					errorTypeToString, line, column)
			}

		// errors the report position
		case QueryErrorSyntax:
			fallthrough
		case QueryErrorStructure:
			fallthrough
		case QueryErrorLanguage:
			fallthrough
		default:
			s := string(pattern[errorOffset:])
			lines := strings.Split(s, "\n")
			whitespace := strings.Repeat(" ", column)
			message = fmt.Sprintf("invalid %s at line %d column %d\n%s\n%s^",
				errorTypeToString, line, column,
				lines[0], whitespace)
		}

		return nil, &QueryError{
			Offset:  errorOffset,
			Type:    errorType,
			Message: message,
		}
	}

	q := &Query{c: c}

	// Copied from: https://github.com/klothoplatform/go-tree-sitter/commit/e351b20167b26d515627a4a1a884528ede5fef79
	// this is just used for syntax validation - it does not actually filter anything
	for i := uint32(0); i < q.PatternCount(); i++ {
		predicates := q.PredicatesForPattern(i)
		for _, steps := range predicates {
			if len(steps) == 0 {
				continue
			}

			if steps[0].Type != QueryPredicateStepTypeString {
				return nil, errors.New("predicate must begin with a literal value")
			}

			operator := q.StringValueForId(steps[0].ValueId)
			switch operator {
			case "eq?", "not-eq?":
				if len(steps) != 4 {
					return nil, fmt.Errorf("wrong number of arguments to `#%s` predicate. Expected 2, got %d", operator, len(steps)-2)
				}
				if steps[1].Type != QueryPredicateStepTypeCapture {
					return nil, fmt.Errorf("first argument of `#%s` predicate must be a capture. Got %s", operator, q.StringValueForId(steps[1].ValueId))
				}
			case "match?", "not-match?":
				if len(steps) != 4 {
					return nil, fmt.Errorf("wrong number of arguments to `#%s` predicate. Expected 2, got %d", operator, len(steps)-2)
				}
				if steps[1].Type != QueryPredicateStepTypeCapture {
					return nil, fmt.Errorf("first argument of `#%s` predicate must be a capture. Got %s", operator, q.StringValueForId(steps[1].ValueId))
				}
				if steps[2].Type != QueryPredicateStepTypeString {
					return nil, fmt.Errorf("second argument of `#%s` predicate must be a string. Got %s", operator, q.StringValueForId(steps[2].ValueId))
				}
			case "set!", "is?", "is-not?":
				if len(steps) < 3 || len(steps) > 4 {
					return nil, fmt.Errorf("wrong number of arguments to `#%s` predicate. Expected 1 or 2, got %d", operator, len(steps)-2)
				}
				if steps[1].Type != QueryPredicateStepTypeString {
					return nil, fmt.Errorf("first argument of `#%s` predicate must be a string. Got %s", operator, q.StringValueForId(steps[1].ValueId))
				}
				if len(steps) > 2 && steps[2].Type != QueryPredicateStepTypeString {
					return nil, fmt.Errorf("second argument of `#%s` predicate must be a string. Got %s", operator, q.StringValueForId(steps[2].ValueId))
				}
			}
		}
	}

	runtime.SetFinalizer(q, (*Query).Close)

	return q, nil
}

// Close should be called to ensure that all the memory used by the query is freed.
//
// As the constructor in go-tree-sitter would set this func call through runtime.SetFinalizer,
// parser.Close() will be called by Go's garbage collector and users would not have to call this manually.
func (q *Query) Close() {
	if q.c != nil {
		C.ts_query_delete(q.c)
		q.c = nil
	}
}

func (q *Query) PatternCount() uint32 {
	return uint32(C.ts_query_pattern_count(q.c))
}

func (q *Query) CaptureCount() uint32 {
	return uint32(C.ts_query_capture_count(q.c))
}

func (q *Query) StringCount() uint32 {
	return uint32(C.ts_query_string_count(q.c))
}

type QueryPredicateStepType int

const (
	QueryPredicateStepTypeDone QueryPredicateStepType = iota
	QueryPredicateStepTypeCapture
	QueryPredicateStepTypeString
)

type QueryPredicateStep struct {
	Type    QueryPredicateStepType
	ValueId int
}

func (q *Query) PredicatesForPattern(patternIndex uint32) [][]QueryPredicateStep {
	var (
		length          C.uint32_t
		cPredicateSteps []C.TSQueryPredicateStep
		predicateSteps  []QueryPredicateStep
	)

	cPredicateStep := C.ts_query_predicates_for_pattern(q.c, C.uint32_t(patternIndex), &length)

	count := int(length)
	slice := (*reflect.SliceHeader)((unsafe.Pointer(&cPredicateSteps)))
	slice.Cap = count
	slice.Len = count
	slice.Data = uintptr(unsafe.Pointer(cPredicateStep))
	for _, s := range cPredicateSteps {
		stepType := QueryPredicateStepType(s._type)
		valueID := int(s.value_id)
		predicateSteps = append(predicateSteps, QueryPredicateStep{stepType, valueID})
	}

	return splitPredicates(predicateSteps)
}

func (q *Query) CaptureNameForId(id int) string {
	var length C.uint32_t
	name := C.ts_query_capture_name_for_id(q.c, C.uint32_t(id), &length)
	return C.GoStringN(name, C.int(length))
}

func (q *Query) StringValueForId(id int) string {
	var length C.uint32_t
	value := C.ts_query_string_value_for_id(q.c, C.uint32_t(id), &length)
	return C.GoStringN(value, C.int(length))
}

type Quantifier int

const (
	QuantifierZero = iota
	QuantifierZeroOrOne
	QuantifierZeroOrMore
	QuantifierOne
	QuantifierOneOrMore
)

func (q *Query) CaptureQuantifierForId(id uint32, captureId uint32) Quantifier {
	return Quantifier(C.ts_query_capture_quantifier_for_id(q.c, C.uint32_t(id), C.uint32_t(captureId)))
}

// QueryCursor carries the state needed for processing the queries.
type QueryCursor struct {
	c *C.TSQueryCursor
	// keep a pointer to the query to avoid garbage collection
	q *Query
	t *Tree
}

// NewQueryCursor creates a query cursor.
func NewQueryCursor() *QueryCursor {
	qc := &QueryCursor{c: C.ts_query_cursor_new()}
	runtime.SetFinalizer(qc, (*QueryCursor).Close)
	return qc
}

// Exec executes the query on a given syntax node.
func (qc *QueryCursor) Exec(q *Query, n Node) {
	qc.q = q
	qc.t = n.t
	C.ts_query_cursor_exec(qc.c, q.c, n.c)
}

func (qc *QueryCursor) SetPointRange(startPoint Point, endPoint Point) {
	cStartPoint := C.TSPoint{
		row:    C.uint32_t(startPoint.Row),
		column: C.uint32_t(startPoint.Column),
	}
	cEndPoint := C.TSPoint{
		row:    C.uint32_t(endPoint.Row),
		column: C.uint32_t(endPoint.Column),
	}
	C.ts_query_cursor_set_point_range(qc.c, cStartPoint, cEndPoint)
}

// Close should be called to ensure that all the memory used by the query cursor is freed.
//
// As the constructor in go-tree-sitter would set this func call through runtime.SetFinalizer,
// parser.Close() will be called by Go's garbage collector and users would not have to call this manually.
func (qc *QueryCursor) Close() {
	if qc.c != nil {
		C.ts_query_cursor_delete(qc.c)
		qc.c = nil
	}
}

// QueryCapture is a captured node by a query with an index
type QueryCapture struct {
	Index int
	Node  Node
}

// QueryMatch - you can then iterate over the matches.
type QueryMatch struct {
	ID           int
	PatternIndex uint16
	Captures     []QueryCapture
}

// NextMatch iterates over matches.
// This function will return (nil, false) when there are no more matches.
// Otherwise, it will populate the QueryMatch with data
// about which pattern matched and which nodes were captured.
func (qc *QueryCursor) NextMatch() (*QueryMatch, bool) {
	var cqm C.TSQueryMatch
	if ok := C.ts_query_cursor_next_match(qc.c, &cqm); !bool(ok) {
		return nil, false
	}

	qm := &QueryMatch{
		ID:           int(cqm.id),
		PatternIndex: uint16(cqm.pattern_index),
	}

	cqc := unsafe.Slice((*C.TSQueryCapture)(cqm.captures), int(cqm.capture_count))
	for _, c := range cqc {
		idx := int(c.index)
		qm.Captures = append(qm.Captures, QueryCapture{idx, Node{c: c.node, t: qc.t}})
	}

	return qm, true
}

func (qc *QueryCursor) NextCapture() (*QueryMatch, int, bool) {
	var (
		cqm          C.TSQueryMatch
		captureIndex C.uint32_t
	)

	if ok := C.ts_query_cursor_next_capture(qc.c, &cqm, &captureIndex); !bool(ok) {
		return nil, 0, false
	}

	qm := &QueryMatch{
		ID:           int(cqm.id),
		PatternIndex: uint16(cqm.pattern_index),
	}

	cqc := unsafe.Slice((*C.TSQueryCapture)(cqm.captures), int(cqm.capture_count))
	for _, c := range cqc {
		idx := int(c.index)
		qm.Captures = append(qm.Captures, QueryCapture{idx, Node{c: c.node, t: qc.t}})
	}

	return qm, int(captureIndex), true
}

// Copied From: https://github.com/klothoplatform/go-tree-sitter/commit/e351b20167b26d515627a4a1a884528ede5fef79

func splitPredicates(steps []QueryPredicateStep) [][]QueryPredicateStep {
	var predicateSteps [][]QueryPredicateStep
	var currentSteps []QueryPredicateStep
	for _, step := range steps {
		currentSteps = append(currentSteps, step)
		if step.Type == QueryPredicateStepTypeDone {
			predicateSteps = append(predicateSteps, currentSteps)
			currentSteps = []QueryPredicateStep{}
		}
	}
	return predicateSteps
}

func (qc *QueryCursor) FilterPredicates(m *QueryMatch, input []byte) *QueryMatch {
	qm := &QueryMatch{
		ID:           m.ID,
		PatternIndex: m.PatternIndex,
	}

	q := qc.q

	predicates := q.PredicatesForPattern(uint32(qm.PatternIndex))
	if len(predicates) == 0 {
		qm.Captures = m.Captures
		return qm
	}

	// track if we matched all predicates globally
	matchedAll := true

	// check each predicate against the match
	for _, steps := range predicates {
		operator := q.StringValueForId(steps[0].ValueId)

		switch operator {
		case "eq?", "not-eq?":
			isPositive := operator == "eq?"

			expectedCaptureNameLeft := q.CaptureNameForId(steps[1].ValueId)

			if steps[2].Type == QueryPredicateStepTypeCapture {
				expectedCaptureNameRight := q.CaptureNameForId(steps[2].ValueId)

				var nodeLeft, nodeRight Node

				for _, c := range m.Captures {
					captureName := q.CaptureNameForId(c.Index)

					if captureName == expectedCaptureNameLeft {
						nodeLeft = c.Node
					}
					if captureName == expectedCaptureNameRight {
						nodeRight = c.Node
					}

					if nodeLeft != (Node{}) && nodeRight != (Node{}) {
						eq := bytes.Equal(nodeContent(nodeLeft, input), nodeContent(nodeRight, input))
						if eq != isPositive {
							matchedAll = false
						}
						break
					}
				}
			} else {
				expectedValueRight := q.StringValueForId(steps[2].ValueId)

				for _, c := range m.Captures {
					captureName := q.CaptureNameForId(c.Index)

					if expectedCaptureNameLeft != captureName {
						continue
					}

					// TODO: make a version of StringValueForId that doesn't allocate
					if (string(nodeContent(c.Node, input)) == expectedValueRight) != isPositive {
						matchedAll = false
						break
					}
				}
			}

			if matchedAll == false {
				break
			}

		case "match?", "not-match?":
			isPositive := operator == "match?"

			expectedCaptureName := q.CaptureNameForId(steps[1].ValueId)
			regex := regexp.MustCompile(q.StringValueForId(steps[2].ValueId))

			for _, c := range m.Captures {
				captureName := q.CaptureNameForId(c.Index)
				if expectedCaptureName != captureName {
					continue
				}

				if regex.Match(nodeContent(c.Node, input)) != isPositive {
					matchedAll = false
					break
				}
			}
		}
	}

	if matchedAll {
		qm.Captures = append(qm.Captures, m.Captures...)
	}

	return qm
}

func nodeContent(n Node, b []byte) []byte { return b[n.StartByte():n.EndByte()] }

// keeps callbacks for parser.parse method
type readFuncsMap struct {
	sync.Mutex

	funcs map[int]ReadFunc
	count int
}

func (m *readFuncsMap) register(f ReadFunc) int {
	m.Lock()
	defer m.Unlock()

	m.count++
	m.funcs[m.count] = f
	return m.count
}

func (m *readFuncsMap) unregister(id int) {
	m.Lock()
	defer m.Unlock()

	delete(m.funcs, id)
}

func (m *readFuncsMap) get(id int) ReadFunc {
	m.Lock()
	defer m.Unlock()

	return m.funcs[id]
}

//export callReadFunc
func callReadFunc(id C.int, byteIndex C.uint32_t, position C.TSPoint, bytesRead *C.uint32_t) *C.char {
	readFunc := readFuncs.get(int(id))
	content := readFunc(uint32(byteIndex), Point{
		Row:    int(position.row),
		Column: int(position.column),
	})
	*bytesRead = C.uint32_t(len(content))

	// Note: This memory is freed inside the C code; see bindings.c
	input := C.CBytes(content)
	return (*C.char)(input)
}
