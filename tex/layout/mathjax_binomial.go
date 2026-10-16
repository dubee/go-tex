package layout

import "github.com/luo-studio/go-tex/tex/parser"

// ParseUtil.fixedFence/mathPalette selects bigg for display and big for text
// and script styles. These fences do not grow with tall binomial contents.
func mathJaxBinomial(n *parser.GenFrac) bool {
	return !n.HasBarLine && n.BarSize == nil && n.LeftDelim != nil && n.RightDelim != nil && *n.LeftDelim == "(" && *n.RightDelim == ")"
}
func layoutMathJaxBinomial(n *parser.GenFrac, o Options) *Box {
	f := layoutFraction(n.Numer, n.Denom, 0, false, o)
	font := "MathJax-Binom1"
	if o.Style.IsDisplay() {
		font = "MathJax-Binom3"
	}
	fence := func(cp rune) *Box {
		g, _ := pinnedMathJaxGlyph(font, cp)
		b := &Box{Width: g.Width, Height: g.Height, Depth: g.Depth, Color: o.Color, Content: Glyph{Outline: g.commands}}
		// SVGmo.getCenterOffset aligns the font's actual center with the math axis.
		return placedBox([]Placed{{b, 0, (g.Height-g.Depth)/2 - .25, 1}}, o.Color)
	}
	return makeHBox([]*Box{fence('('), f, fence(')')}, o.Color)
}
