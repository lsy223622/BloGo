package markdown

import (
	"bytes"
	"html"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var kindMathInline = ast.NewNodeKind("MathInline")
var kindMathBlock = ast.NewNodeKind("MathBlock")

type mathInline struct {
	ast.BaseInline
	value []byte
}

func (n *mathInline) Kind() ast.NodeKind            { return kindMathInline }
func (n *mathInline) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

type mathBlock struct{ ast.BaseBlock }

func (n *mathBlock) Kind() ast.NodeKind            { return kindMathBlock }
func (n *mathBlock) IsRaw() bool                   { return true }
func (n *mathBlock) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

type mathInlineParser struct{}

func (*mathInlineParser) Trigger() []byte { return []byte{'$'} }
func (*mathInlineParser) Parse(parent ast.Node, r text.Reader, pc parser.Context) ast.Node {
	line, _ := r.PeekLine()
	if len(line) < 3 || line[1] == '$' {
		return nil
	}
	for i := 1; i < len(line); i++ {
		if line[i] == '$' && line[i-1] != '\\' {
			if len(bytes.TrimSpace(line[1:i])) == 0 {
				return nil
			}
			r.Advance(i + 1)
			return &mathInline{value: append([]byte(nil), line[1:i]...)}
		}
	}
	return nil
}

type mathBlockParser struct{}

func (*mathBlockParser) Trigger() []byte { return []byte{'$'} }
func (*mathBlockParser) Open(parent ast.Node, r text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := r.PeekLine()
	if strings.TrimSpace(string(line)) != "$$" {
		return nil, parser.NoChildren
	}
	return &mathBlock{}, parser.NoChildren
}
func (*mathBlockParser) Continue(n ast.Node, r text.Reader, pc parser.Context) parser.State {
	line, segment := r.PeekLine()
	if strings.TrimSpace(string(line)) == "$$" {
		r.AdvanceToEOL()
		return parser.Close
	}
	n.Lines().Append(segment)
	r.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}
func (*mathBlockParser) Close(ast.Node, text.Reader, parser.Context) {}
func (*mathBlockParser) CanInterruptParagraph() bool                 { return true }
func (*mathBlockParser) CanAcceptIndentedLine() bool                 { return false }

type mathRenderer struct{}

func (*mathRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindMathInline, func(w util.BufWriter, source []byte, n ast.Node, enter bool) (ast.WalkStatus, error) {
		if enter {
			_, err := w.WriteString(`<span class="math-inline">$` + html.EscapeString(string(n.(*mathInline).value)) + `$</span>`)
			return ast.WalkSkipChildren, err
		}
		return ast.WalkContinue, nil
	})
	reg.Register(kindMathBlock, func(w util.BufWriter, source []byte, n ast.Node, enter bool) (ast.WalkStatus, error) {
		if enter {
			_, err := w.WriteString("<div class=\"math-block\">$$\n" + html.EscapeString(string(n.Lines().Value(source))) + "$$</div>\n")
			return ast.WalkSkipChildren, err
		}
		return ast.WalkContinue, nil
	})
}
