package markdown

import (
	"context"

	"github.com/boldsoftware/treesitter"
	tree_sitter_markdown "github.com/boldsoftware/treesitter/markdown/tree-sitter-markdown"
	tree_sitter_markdown_inline "github.com/boldsoftware/treesitter/markdown/tree-sitter-markdown-inline"
)

type MarkdownTree struct {
	blockTree     *treesitter.Tree
	inlineTrees   []*treesitter.Tree
	inlineIndices map[uintptr]int
}

func (t *MarkdownTree) Edit(edit treesitter.EditInput) {
	t.blockTree.Edit(edit)
	for _, tree := range t.inlineTrees {
		tree.Edit(edit)
	}
}

func (t *MarkdownTree) BlockTree() *treesitter.Tree {
	return t.blockTree
}

func (t *MarkdownTree) InlineTree(parent treesitter.Node) *treesitter.Tree {
	if parent == (treesitter.Node{}) {
		return nil
	}

	index, ok := t.inlineIndices[parent.ID()]
	if ok {
		return t.inlineTrees[index]
	}

	return nil
}

func (t *MarkdownTree) InlineRootNode(parent treesitter.Node) treesitter.Node {
	tree := t.InlineTree(parent)
	if tree == nil {
		return treesitter.Node{}
	}

	return tree.RootNode()
}

func (t *MarkdownTree) InlineTrees() []*treesitter.Tree {
	return t.inlineTrees
}

func (t *MarkdownTree) Iter(f func(node *Node) bool) {
	root := t.blockTree.RootNode()
	t.iter(&Node{root, t.InlineRootNode(root)}, f)
}

func (t *MarkdownTree) iter(node *Node, f func(node *Node) bool) (goNext bool) {
	goNext = f(node)
	if !goNext {
		return goNext
	}

	childCount := node.NamedChildCount()
	for i := 0; i < int(childCount); i++ {
		child := node.NamedChild(i)

		goNext = t.iter(&Node{Node: child, Inline: t.InlineRootNode(child)}, f)
		if !goNext {
			return goNext
		}
	}

	return true
}

type Node struct {
	treesitter.Node
	Inline treesitter.Node
}

func ParseCtx(ctx context.Context, oldTree *MarkdownTree, content []byte) (*MarkdownTree, error) {
	p := treesitter.NewParser(tree_sitter_markdown.GetLanguage())

	var old *treesitter.Tree
	if oldTree != nil {
		old = oldTree.blockTree
	}
	tree, err := p.Parse(ctx, old, content)
	if err != nil {
		return nil, err
	}

	res := &MarkdownTree{
		blockTree:     tree,
		inlineTrees:   []*treesitter.Tree{},
		inlineIndices: map[uintptr]int{},
	}
	p.Close()

	p = treesitter.NewParser(tree_sitter_markdown_inline.GetLanguage())
	defer p.Close()

	q, err := treesitter.NewQuery([]byte(`(inline) @inline`), tree_sitter_markdown.GetLanguage())
	if err != nil {
		return nil, err
	}

	qc := treesitter.NewQueryCursor()
	qc.Exec(q, tree.RootNode())

	idx := int(0)
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}

		for _, capture := range match.Captures {
			r := capture.Node.Range()
			ranges := []treesitter.Range{}
			for i := 0; i < int(capture.Node.NamedChildCount()); i++ {
				child := capture.Node.NamedChild(i)
				childRange := child.Range()
				ranges = append(ranges, treesitter.Range{
					StartPoint: r.StartPoint,
					StartByte:  r.StartByte,
					EndPoint:   childRange.EndPoint,
					EndByte:    childRange.EndByte,
				})

				r.StartPoint = childRange.EndPoint
				r.StartByte = childRange.EndByte
			}

			ranges = append(ranges, r)
			p.SetIncludedRanges(ranges)
			var old *treesitter.Tree
			if oldTree != nil && idx < len(oldTree.inlineTrees) {
				old = oldTree.inlineTrees[idx]
			}

			inlineTree, err := p.Parse(ctx, old, content)
			if err != nil {
				return nil, err
			}

			res.inlineTrees = append(res.inlineTrees, inlineTree)
			res.inlineIndices[capture.Node.ID()] = idx
			idx++
		}
	}
	qc.Close()

	return res, nil
}
