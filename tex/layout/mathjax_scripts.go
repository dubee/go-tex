package layout

import (
	"math"

	"github.com/luo-studio/go-tex/tex/parser"
)

// layoutMathJaxCharacterScripts follows MathJax 3.2.2 CommonScriptbase and
// CommonMsubsup for the single-character bases admitted by the glyph gate.
// Operators, accents and composite bases retain the existing layout pipeline.
func layoutMathJaxCharacterScripts(base *Box, sup, sub parser.Node, opts Options) *Box {
	s := 1 / math.Sqrt(2)
	scales := [3]float64{1, s, s * s}
	ratio := func(index int) float64 { return scales[index] / scales[opts.Style.SizeIndex()] }
	c := SupSub{Base: base, SupScale: ratio(opts.Style.Superscript().SizeIndex()), SubScale: ratio(opts.Style.Subscript().SizeIndex())}
	if sup != nil {
		c.Sup = layoutNode(sup, opts.WithStyle(opts.Style.Superscript()))
	}
	if sub != nil {
		c.Sub = layoutNode(sub, opts.WithStyle(opts.Style.Subscript()))
	}
	ic := mathJaxCharacterItalic(base)
	adjustedIC := 0.0
	if ic != 0 {
		adjustedIC = 1.05*ic + .05
	}
	c.SupHKern = .033 + adjustedIC - ic
	c.SubHKern = .033 - ic
	u := .363
	if opts.Style.IsCramped() {
		u = .289
	} else if opts.Style.IsDisplay() {
		u = .413
	}
	if c.Sup != nil {
		// In msubsup, inherited scriptChild is child 1 (the subscript).
		// getUVQ calls getU() before computing the actual inter-script gap.
		child, scale := c.Sup, c.SupScale
		if c.Sub != nil {
			child, scale = c.Sub, c.SubScale
		}
		c.SupShift = math.Max(u, child.Depth*scale+.442/4)
	}
	if c.Sub != nil {
		if c.Sup == nil {
			c.SubShift = math.Max(.15, c.Sub.Height*c.SubScale-.8*.442)
		} else {
			c.SubShift = .247
			gap := (c.SupShift - c.Sup.Depth*c.SupScale) - (c.Sub.Height*c.SubScale - c.SubShift)
			if gap < 3*.06 {
				c.SubShift += 3*.06 - gap
				p := .8*.442 - (c.SupShift - c.Sup.Depth*c.SupScale)
				if p > 0 {
					c.SupShift += p
					c.SubShift -= p
				}
			}
		}
	}
	b := &Box{Width: base.Width, Height: base.Height, Depth: base.Depth, Color: opts.Color, Content: c}
	if c.Sup != nil {
		b.Width = math.Max(b.Width, base.Width+c.SupHKern+c.Sup.Width*c.SupScale)
		b.Height = math.Max(b.Height, c.SupShift+c.Sup.Height*c.SupScale)
		b.Depth = math.Max(b.Depth, c.Sup.Depth*c.SupScale-c.SupShift)
	}
	if c.Sub != nil {
		b.Width = math.Max(b.Width, base.Width+c.SubHKern+c.Sub.Width*c.SubScale)
		b.Height = math.Max(b.Height, c.Sub.Height*c.SubScale-c.SubShift)
		b.Depth = math.Max(b.Depth, c.SubShift+c.Sub.Depth*c.SubScale)
	}
	b.Width += .05
	return b
}

func mathJaxCharacterItalic(b *Box) float64 {
	switch c := b.Content.(type) {
	case Glyph:
		if g, ok := pinnedMathJaxGlyph(c.FontID, c.CharCode); ok {
			return g.Italic
		}
	case HBox:
		if len(c.Children) == 1 {
			return mathJaxCharacterItalic(c.Children[0])
		}
	}
	return 0
}
