package layout

import (
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
	"math"
)

// MathJax 3.2.2 CommonMenclose/Notation and CancelConfiguration (Apache-2.0).
// Keep the mpadded target, menclose child and rotated arrow as separate SVG
// transform boundaries. Path coordinates use the SVG path parser, while rotate
// attributes use librsvg's CSS number parser.
type mathJaxArrow struct {
	Body *Box
	Left float64
}

func (mathJaxArrow) contentMarker() {}
func layoutMathJaxCancelTo(n *parser.CancelTo, o Options) *Box {
	body := layoutNode(n.Body, o)
	H, w := body.Height+body.Depth, body.Width
	R := math.Sqrt(H*H + w*w)
	px, py := .2, .2
	if R > 0 {
		px = math.Max(px, .335*w/R)
		py = math.Max(py, .335*H/R)
	}
	a := math.Atan2(H+2*py, w+2*px)
	b := math.Atan2(2, 5)
	if b > a {
		py += .067 * math.Sqrt(29) * math.Sin(b-a)
	}
	if b > math.Pi/2-a {
		px += .067 * math.Sqrt(29) * math.Sin(b+a-math.Pi/2)
	}
	base := &Box{Width: w + 2*px, Height: body.Height + py, Depth: body.Depth + py, Color: o.Color, Content: mathJaxArrow{body, px}}
	target := layoutNode(n.Target, o.WithStyle(o.Style.Superscript()))
	padded := &Box{Width: target.Width, Height: target.Height + .1, Depth: math.Max(0, target.Depth-.1), Color: o.Color, Content: Positioned{Children: []Placed{{target, 0, -.1, 1}}}}
	scales := [3]float64{1, 1 / math.Sqrt(2), .5}
	s := scales[o.Style.Superscript().SizeIndex()] / scales[o.Style.SizeIndex()]
	u := .363
	if o.Style.IsCramped() {
		u = .289
	} else if o.Style.IsDisplay() {
		u = .413
	}
	u = math.Max(u, math.Max(base.Height-.386*s, padded.Depth*s+.442/4))
	c := SupSub{Base: base, Sup: padded, SupScale: s, SupShift: u, SupHKern: .033}
	return &Box{Width: base.Width + .033 + padded.Width*s + .05, Height: math.Max(base.Height, u+padded.Height*s), Depth: math.Max(base.Depth, padded.Depth*s-u), Color: o.Color, Content: c}
}
func emitMathJaxArrow(b *Box, c mathJaxArrow, d *DisplayList, x, y, scale float64, q svgCoordinates) bool {
	if !emitMathJaxGlyph(c.Body, d, x+q.translation(c.Left)*scale, y, scale, q) {
		return false
	}
	w, H := b.Width, b.Height+b.Depth
	W := math.Sqrt(w*w + H*H)
	dw := (W - w) / 2
	m := (b.Height - b.Depth) / 2
	// Preserve rounding of each authored relative segment before accumulating it.
	fixed := func(v float64) float64 { return svgFixed(v*1000, 1) / 1000 }
	px, py := fixed(w+dw), fixed(m)
	points := [][2]float64{{px, py}}
	rel := func(a, b float64) { px += fixed(a); py += fixed(b); points = append(points, [2]float64{px, py}) }
	abs := func(a, b float64) { px = fixed(a); py = fixed(b); points = append(points, [2]float64{px, py}) }
	rel(-.335, .134)
	rel(.067, .0335-.134)
	abs(-dw, m+.0335)
	rel(0, -.067)
	abs(w+dw-.268, m-.0335)
	rel(-.067, .0335-.134)
	angle := q.number(svgFixed(math.Atan2(H, w)*180/math.Pi, 3)) * math.Pi / 180
	co, si := math.Cos(angle), math.Sin(angle)
	cx, cy := q.translation(w/2), q.translation(m)
	commands := []path.Command{}
	for i, p := range points {
		a, z := p[0]-cx, p[1]-cy
		xx, yy := (cx+a*co-z*si)*scale, -(cy+a*si+z*co)*scale
		if i == 0 {
			commands = append(commands, path.MoveTo(xx, yy))
		} else {
			commands = append(commands, path.LineTo(xx, yy))
		}
	}
	commands = append(commands, path.Close())
	d.Items = append(d.Items, PathItem{X: x, Y: y, Commands: commands, Fill: true, Color: b.Color})
	return true
}
