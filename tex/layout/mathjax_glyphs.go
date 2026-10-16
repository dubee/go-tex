package layout

import (
	_ "embed"
	"encoding/json"
	"github.com/luo-studio/go-tex/tex/parser"
	"strconv"
	"sync"

	"github.com/luo-studio/go-tex/tex/path"
)

// Pinned MathJax 3.2.2 Roman, italic, bold, Greek and mapped-relation glyphs.
// See the root LICENSE for MathJax provenance and attribution.
//
//go:embed mathjax_glyphs.json
var mathJaxGlyphData []byte

type mathJaxGlyph struct {
	Height, Depth, Width, Italic float64
	Path                         string
	Chars                        string `json:"chars"`
	commands                     []path.Command
}

var mathJaxGlyphOnce sync.Once
var mathJaxGlyphs map[string]map[string]mathJaxGlyph

func pinnedMathJaxGlyph(font string, cp rune) (mathJaxGlyph, bool) {
	mathJaxGlyphOnce.Do(func() {
		if err := json.Unmarshal(mathJaxGlyphData, &mathJaxGlyphs); err != nil {
			panic(err)
		}
		for _, table := range mathJaxGlyphs {
			for key, g := range table {
				g.commands = scaleSVGPathXY(parseSVGPath(g.Path), .001, -.001)
				table[key] = g
			}
		}
	})
	g, ok := mathJaxGlyphs[font][strconv.Itoa(int(cp))]
	return g, ok
}

// SupportsMathJaxGlyphs rejects mixed font/layout pipelines. Supported
// constructors include bounded text/font runs, mapped negated relations, groups,
// fractions, side scripts, choices, phantoms and diagonal cancellation. Every
// glyph and child must be supported; system-font text, composite script bases,
// stretchy accents and other constructors retain the whole-expression fallback.
func SupportsMathJaxGlyphs(nodes []parser.Node) bool {
	return SupportsMathJaxGlyphsWithSystem(nodes, nil)
}

// SupportsMathJaxGlyphsWithSystem permits caller-resolved outlines for Unicode
// scalars absent from the embedded font. Resolution is local to one layout.
func SupportsMathJaxGlyphsWithSystem(nodes []parser.Node, resolve func(rune) bool) bool {
	var visit func(parser.Node, string) bool
	var visitList func([]parser.Node, string) bool
	// MathJax groups adjacent digits into one mn base. The native parser
	// attaches a script to the last digit; keep those composite bases out.
	digit := func(n parser.Node) bool {
		text := ""
		switch n := n.(type) {
		case *parser.TextOrd:
			text = n.Text
		case *parser.MathOrd:
			text = n.Text
		}
		return len(text) == 1 && text[0] >= '0' && text[0] <= '9'
	}
	visitList = func(nodes []parser.Node, font string) bool {
		nodes = mathJaxNegations(nodes)
		for i, n := range nodes {
			if s, ok := n.(*parser.SupSub); ok && i > 0 && digit(nodes[i-1]) && digit(s.Base) {
				return false
			}
			if !visit(n, font) {
				return false
			}
		}
		return true
	}
	symbol := func(text string, mode parser.Mode, font string) bool {
		ch, _ := decodeFirstRune(text)
		cp := resolveCodepoint(text, ch, mode)
		if mathFont, _ := mathAlphanumericFont(ch); mathFont != "" {
			return false
		}
		if font == "" {
			font = selectFont(text, cp, mode)
		}
		if font == "AMS-Regular" {
			font = "Main-Regular"
		}
		_, ok := pinnedMathJaxGlyph(font, cp)
		return ok
	}
	visit = func(node parser.Node, font string) bool {
		switch n := node.(type) {
		case *parser.Unicode:
			if font == "" {
				font = "Main-Regular"
			}
			g, ok := pinnedMathJaxGlyph("Unicode-"+font, n.Value)
			return ok && g.Chars == "" || (!ok && resolve != nil && resolve(n.Value))
		case *parser.MathOrd:
			return symbol(n.Text, parser.ModeMath, font)
		case *parser.TextOrd:
			return symbol(n.Text, n.Mode, font)
		case *parser.Atom:
			return symbol(n.Text, n.Mode, font)
		case *parser.OrdGroup:
			return visitList(n.Body, font)
		case *parser.Color:
			return visitList(n.Body, font)
		case *parser.Font:
			next := mathFontToKaTeX(n.Font)
			if next != "Main-Regular" && next != "Math-Italic" && next != "Main-Italic" && next != "Main-Bold" && next != "Math-BoldItalic" {
				return false
			}
			return visit(n.Body, next)
		case *parser.MClass:
			return visitList(n.Body, font)
		case *parser.Text:
			next := "Main-Regular"
			if n.Font != nil && *n.Font != "\\text" {
				next = mathFontToKaTeX(*n.Font)
				if next == "" {
					return false
				}
			}
			for _, child := range n.Body {
				switch c := child.(type) {
				case *parser.Spacing:
					if c.Text != " " {
						return false
					}
				case *parser.TextOrd:
					if !symbol(c.Text, parser.ModeText, next) {
						return false
					}
				case *parser.MathOrd:
					if !symbol(c.Text, parser.ModeText, next) {
						return false
					}
				default:
					return false
				}
			}
			return true
		case *parser.Array:
			if !supportedMathJaxTable(n) {
				return false
			}
			for _, row := range n.Body {
				for _, cell := range row {
					if style, ok := cell.(*parser.Styling); ok {
						if !visitList(style.Body, font) {
							return false
						}
					} else if !visit(cell, font) {
						return false
					}
				}
			}
			return true
		case *parser.Multline:
			for _, row := range n.Rows {
				if style, ok := row.(*parser.Styling); ok {
					if !visitList(style.Body, font) {
						return false
					}
					continue
				}
				if !visit(row, font) {
					return false
				}
			}
			return len(n.Rows) > 0
		case *parser.XArrow:
			return (n.Label == `\xrightarrow` || n.Label == `\xleftarrow` || n.Label == `\xleftrightarrow`) && visit(n.Body, font) && (n.Below == nil || visit(n.Below, font))
		case *parser.Op:
			return n.Symbol && n.Name != nil && (*n.Name == `\sum` || *n.Name == `\prod`)
		case *parser.HorizBrace:
			return (n.Label == `\overbrace` || n.Label == `\underbrace`) && visit(n.Base, font)
		case *parser.SupSub:
			if hb, ok := n.Base.(*parser.HorizBrace); ok {
				return ((hb.IsOver && n.Sub == nil) || (!hb.IsOver && n.Sup == nil)) && visit(hb, font) && (n.Sup == nil || visit(n.Sup, font)) && (n.Sub == nil || visit(n.Sub, font))
			}
			_, op := n.Base.(*parser.Op)
			return (isCharacterBox(n.Base) || op) && visit(n.Base, font) &&
				(n.Sup == nil || visit(n.Sup, font)) && (n.Sub == nil || visit(n.Sub, font))
		case *parser.MathChoice:
			// Validate every branch, not just the current display/text choice:
			// fractions may select script styles later during layout.
			for _, branch := range [][]parser.Node{n.Display, n.Text, n.Script, n.ScriptScript} {
				if !visitList(branch, font) {
					return false
				}
			}
			return true
		case *parser.Styling:
			// The parser wraps cfrac in display style for its KaTeX path.
			// MathJax CFrac itself inherits the surrounding fraction style.
			if len(n.Body) == 1 {
				if f, ok := n.Body[0].(*parser.GenFrac); ok && f.Continued {
					return visit(f, font)
				}
			}
			return (n.Style == "text" || n.Style == "display") && visitList(n.Body, font)
		case *parser.VPhantom:
			return visit(n.Body, font)
		case *parser.Phantom:
			// Invisible contents still contribute their glyph metrics.
			return visitList(n.Body, font)
		case *parser.MultiScripts:
			if !visit(n.Base, font) {
				return false
			}
			for _, child := range []parser.Node{n.LeftSup, n.LeftSub, n.RightSup, n.RightSub} {
				if child != nil && !visit(child, font) {
					return false
				}
			}
			return true
		case *parser.CancelTo:
			return visit(n.Body, font) && visit(n.Target, font)
		case *parser.Decorated:
			return (!n.Enclosure || n.Notation == "circle") && visit(n.Body, font)
		case *parser.Enclose:
			if n.Label == `\colorbox` || n.Label == `\fcolorbox` || n.Label == `\fbox` {
				return visit(n.Body, font)
			}
			return (n.Label == `\cancel` || n.Label == `\bcancel` || n.Label == `\xcancel`) && visit(n.Body, font)
		case *parser.GenFrac:
			return ((n.HasBarLine && n.LeftDelim == nil && n.RightDelim == nil && n.BarSize == nil) || mathJaxBinomial(n)) && visit(n.Numer, font) && visit(n.Denom, font)
		default:
			return false
		}
	}
	return len(nodes) > 0 && visitList(nodes, "")
}
