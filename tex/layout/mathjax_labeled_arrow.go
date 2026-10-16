package layout

import (
	"github.com/luo-studio/go-tex/tex/mathstyle"
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
	"math"
)

// AMS xArrow builds padded labels around a scriptlevel-zero horizontal mo.
// Preserve the mpadded inner transform separately from the munderover placement.
func layoutMathJaxLabeledArrow(n *parser.XArrow, o Options) *Box {
	b, _ := (mathJaxArrowLayoutCache{}).layout(n, o, 0)
	return b
}

// Stretch measurement revisits nested labels before resizing their core mo.
// Cache those measurements per layout call, avoiding exponential nested work.
type mathJaxArrowLayoutKey struct {
	node  *parser.XArrow
	style mathstyle.Style
	width float64
}
type mathJaxArrowLayoutResult struct {
	box     *Box
	natural float64
}
type mathJaxArrowLayoutCache map[mathJaxArrowLayoutKey]mathJaxArrowLayoutResult

func (cache mathJaxArrowLayoutCache) layout(n *parser.XArrow, o Options, forced float64) (box *Box, naturalWidth float64) {
	key := mathJaxArrowLayoutKey{n, o.Style, forced}
	if saved, ok := cache[key]; ok {
		return saved.box, saved.natural
	}
	defer func() { cache[key] = mathJaxArrowLayoutResult{box, naturalWidth} }()
	scales := [3]float64{1, 1 / math.Sqrt(2), .5}
	s := scales[o.Style.Superscript().SizeIndex()] / scales[o.Style.SizeIndex()]
	left := .278 // ParseUtil.Em rounds mu lengths to three decimals.
	cp := rune(0x2192)
	if n.Label == `\xleftarrow` {
		left, cp = .556, 0x2190
	}
	if n.Label == `\xleftrightarrow` {
		left, cp = .333, 0x2194
	}
	padWidth := .833
	if cp == 0x2194 {
		padWidth = .667
	}
	nested := func(node parser.Node) *parser.XArrow {
		n, _ := parser.NormalizeArgument(node).(*parser.XArrow)
		return n
	}
	childForced := 0.
	padded := func(node parser.Node, above bool) *Box {
		style, y := o.Style.Subscript(), -.15
		if above {
			style, y = o.Style.Superscript(), .2
		}
		var child *Box
		if inner := nested(node); inner != nil {
			child, _ = cache.layout(inner, o.WithStyle(mathstyle.Text), childForced)
		} else {
			child = layoutNode(node, o.WithStyle(style))
		}
		b := placedBox([]Placed{{child, left, y, 1}}, o.Color)
		b.Width = child.Width + padWidth
		if above {
			b.Height = math.Max(0, child.Height-.2)
			b.Depth = math.Max(.25, child.Depth)
		} else {
			b.Height = math.Max(.75, child.Height)
			b.Depth = math.Max(0, child.Depth-.15)
		}
		return b
	}
	above := padded(n.Body, true)
	as, bs := s, s
	if nested(n.Body) != nil {
		as = 1
	}
	if n.Below != nil && nested(n.Below) != nil {
		bs = 1
	}
	var below *Box
	w := above.Width * as
	if n.Below != nil {
		below = padded(n.Below, false)
		w = math.Max(w, below.Width*bs)
	}
	natural := w
	// When every child is embellished, stretchChildren measures all children
	// before changing any core mo. A nested arrow's labels supply that measure.
	if inner := nested(n.Body); inner != nil && n.Below == nil {
		_, nw := cache.layout(inner, o.WithStyle(mathstyle.Text), 0)
		w = nw + padWidth
		childForced = w
		above = padded(n.Body, true)
	}
	if forced > 0 {
		w = forced
	}
	// CommonMo.getStretchedVariant's delimiterfactor/shortfall selection.
	fixed := math.Max(w*.901, w-.3) <= 1
	aw, ah, ad := w, .583, .082
	if fixed {
		g, _ := pinnedMathJaxGlyph("MathJax-LabeledArrow", cp)
		aw, ah, ad = g.Width, g.Height, g.Depth
	}
	arrow := &Box{Width: aw, Height: ah, Depth: ad, Color: o.Color, Content: mathJaxLabeledArrow{cp, fixed}}
	w = math.Max(math.Max(above.Width*as, w), aw)
	if below != nil {
		w = math.Max(w, below.Width*bs)
	}
	gap := math.Max(.111, .2-math.Max(0, above.Depth*as))
	// Embellished arrow labels inherit the core operator accent attribute.
	if nested(n.Body) != nil {
		gap = .06 * 1.75
	}
	u := ah + gap + above.Depth*as
	items := []Placed{{arrow, (w - aw) / 2, 0, 1}}
	h, d := math.Max(ah, u+above.Height*as)+.1, ad
	if below != nil {
		v := ad + math.Max(.167, .6-below.Height*bs) + below.Height*bs
		items = append(items, Placed{below, (w - below.Width*bs) / 2, v, bs})
		d = math.Max(d, v+below.Depth*bs) + .1
	}
	items = append(items, Placed{above, (w - above.Width*as) / 2, -u, as})
	return &Box{Width: w, Height: h, Depth: d, Color: o.Color, Content: Positioned{Children: items}}, natural
}

type mathJaxLabeledArrow struct {
	Code  rune
	Fixed bool
}

func (mathJaxLabeledArrow) contentMarker() {}
func emitMathJaxLabeledArrow(b *Box, c mathJaxLabeledArrow, d *DisplayList, x, y, scale float64, q svgCoordinates) bool {
	o := DefaultOptions()
	o.MathJaxGlyphs = true
	o.FontOverride = "MathJax-LabeledArrow"
	o.Color = b.Color
	glyph := func(cp rune, dx float64) bool {
		return emitMathJaxGlyph(layoutSymbol(string(cp), parser.ModeText, o), d, x+q.translation(dx)*scale, y, scale, q)
	}
	if c.Fixed {
		return glyph(c.Code, 0)
	}
	l, r := 0., 0.
	if c.Code != 0x2192 {
		if !glyph(0x2190, 0) {
			return false
		}
		l = .9
	}
	if c.Code != 0x2190 {
		if !glyph(0x2192, b.Width-1) {
			return false
		}
		r = .9
	}
	// SVGmo.addExtH scales the minus glyph 1.5x wider than its viewport.
	// Its curved ends lie outside the clip; the visible shaft is a rectangle.
	length := b.Width - l - r
	if length > 0 {
		w := q.translation(length) * scale
		h := .04 * scale
		d.Items = append(d.Items, PathItem{X: x + q.translation(l)*scale, Y: y - .27*scale, Commands: []path.Command{path.MoveTo(0, 0), path.LineTo(w, 0), path.LineTo(w, h), path.LineTo(0, h), path.Close()}, Fill: true, ClippedExtender: true, Color: b.Color})
	}
	return true
}
