package markdown

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	htmltoken "golang.org/x/net/html"
)

type Heading struct {
	Level    int
	ID, Text string
}
type Result struct {
	HTML, Summary string
	TOC           []Heading
	Mermaid       bool
}

func Convert(body, explicitSummary string, maxChars int) (Result, error) {
	if maxChars <= 0 {
		return Result{}, fmt.Errorf("summary limit must be positive")
	}
	md := goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Footnote), goldmark.WithParserOptions(parser.WithAutoHeadingID(), parser.WithInlineParsers(util.Prioritized(&mathInlineParser{}, 150)), parser.WithBlockParsers(util.Prioritized(&mathBlockParser{}, 50))), goldmark.WithRendererOptions(gmhtml.WithUnsafe(), renderer.WithNodeRenderers(util.Prioritized(&mathRenderer{}, 100), util.Prioritized(&codeRenderer{}, 100))))
	source := []byte(body)
	doc := md.Parser().Parse(text.NewReader(source))
	var result Result
	_ = ast.Walk(doc, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok {
			id, _ := h.AttributeString("id")
			result.TOC = append(result.TOC, Heading{h.Level, string(id.([]byte)), plainInline(h, source, false)})
		}
		if fence, ok := n.(*ast.FencedCodeBlock); ok && string(fence.Language(source)) == "mermaid" {
			result.Mermaid = true
		}
		if (n.Kind() == ast.KindParagraph || n.Kind() == ast.KindTextBlock) && result.Summary == "" {
			result.Summary = plainInline(n, source, true)
		}
		return ast.WalkContinue, nil
	})
	if explicitSummary != "" && strings.TrimSpace(explicitSummary) != "" {
		result.Summary = normalize(explicitSummary)
	} else {
		r := []rune(result.Summary)
		if len(r) > maxChars {
			result.Summary = string(r[:maxChars])
		}
	}
	var out bytes.Buffer
	if err := md.Renderer().Render(&out, source, doc); err != nil {
		return Result{}, err
	}
	result.HTML = out.String()
	return result, nil
}

func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

func plainInline(root ast.Node, source []byte, summary bool) string {
	var b strings.Builder
	hidden := 0
	_ = ast.Walk(root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		if n.Kind() == kindMathInline || n.Kind() == kindMathBlock {
			return ast.WalkSkipChildren, nil
		}
		if summary && n.Kind() == ast.KindCodeSpan {
			return ast.WalkSkipChildren, nil
		}
		if summary && n.Kind() == ast.KindImage {
			return ast.WalkSkipChildren, nil
		}
		if raw, ok := n.(*ast.RawHTML); ok {
			z := htmltoken.NewTokenizer(bytes.NewReader(raw.Segments.Value(source)))
			for {
				typ := z.Next()
				if typ == htmltoken.ErrorToken {
					break
				}
				token := z.Token()
				if token.Data == "script" || token.Data == "style" {
					if typ == htmltoken.StartTagToken {
						hidden++
					} else if typ == htmltoken.EndTagToken && hidden > 0 {
						hidden--
					}
				}
			}
			return ast.WalkSkipChildren, nil
		}
		if hidden > 0 {
			return ast.WalkSkipChildren, nil
		}
		switch v := n.(type) {
		case *ast.Text:
			b.Write(v.Segment.Value(source))
			if v.SoftLineBreak() || v.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(v.Value)
		}
		return ast.WalkContinue, nil
	})
	return normalize(stdhtml.UnescapeString(b.String()))
}

type codeRenderer struct{}

func (*codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, renderCode)
	reg.Register(ast.KindCodeBlock, renderCode)
}
func renderCode(w util.BufWriter, source []byte, n ast.Node, enter bool) (ast.WalkStatus, error) {
	if !enter {
		return ast.WalkContinue, nil
	}
	if fence, ok := n.(*ast.FencedCodeBlock); ok && string(fence.Language(source)) == "mermaid" {
		_, err := fmt.Fprintf(w, `<pre class="mermaid">%s</pre>`, stdhtml.EscapeString(string(n.Lines().Value(source))))
		return ast.WalkSkipChildren, err
	}
	lexer := lexers.Fallback
	if fenced, ok := n.(*ast.FencedCodeBlock); ok {
		if found := lexers.Get(string(fenced.Language(source))); found != nil {
			lexer = found
		}
	}
	tokens, err := lexer.Tokenise(nil, string(n.Lines().Value(source)))
	if err != nil {
		return ast.WalkStop, err
	}
	err = html.New(html.WithClasses(true)).Format(w, styles.Get("github"), tokens)
	return ast.WalkSkipChildren, err
}

func HighlightCSS() string {
	var b bytes.Buffer
	_ = html.New(html.WithClasses(true)).WriteCSS(&b, styles.Get("github"))
	var dark bytes.Buffer
	_ = html.New(html.WithClasses(true)).WriteCSS(&dark, styles.Get("github-dark"))
	scope := func(prefix string) string {
		s := strings.ReplaceAll(dark.String(), ".chroma", prefix+" .chroma")
		// Dark styles omit tokens that inherit their base color. Reset the light
		// theme's explicit token colors before applying those dark overrides.
		return prefix + " .chroma span{color:inherit;font-style:inherit;font-weight:inherit;text-decoration:inherit;background-color:transparent}" + strings.ReplaceAll(s, ".bg", prefix+" .bg")
	}
	return b.String() + scope(`[data-theme="dark"]`) + "@media(prefers-color-scheme:dark){" + scope(`:root:not([data-theme="light"])`) + "}"
}
