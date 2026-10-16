package layout

import (
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
)

// MathJax fullBBox and SVGmenclose.ellipse: padding .2, stroke .067.
type mathJaxCircle struct{}

func (mathJaxCircle) contentMarker() {}
func layoutMathJaxCircle(n *parser.Decorated, o Options) *Box {
	body := layoutNode(n.Body, o)
	w, h, d := body.Width+.534, body.Height+.267, body.Depth+.267
	circle := &Box{Width: w, Height: h, Depth: d, Color: o.Color, Content: mathJaxCircle{}}
	return &Box{Width: w, Height: h, Depth: d, Color: o.Color, Content: Positioned{Children: []Placed{{body, .267, 0, 1}, {circle, 0, 0, 1}}}}
}
func emitMathJaxCircle(b *Box, d *DisplayList, x, y, scale float64, q svgCoordinates) bool {
	const k = .5522847498
	rx, ry := q.translation((b.Width-.067)/2)*scale, -q.translation((b.Height+b.Depth-.067)/2)*scale
	cx, cy := q.translation(b.Width/2)*scale, -q.translation((b.Height-b.Depth)/2)*scale
	cmds := []path.Command{path.MoveTo(cx+rx, cy), path.CubicTo(cx+rx, cy+k*ry, cx+k*rx, cy+ry, cx, cy+ry), path.CubicTo(cx-k*rx, cy+ry, cx-rx, cy+k*ry, cx-rx, cy), path.CubicTo(cx-rx, cy-k*ry, cx-k*rx, cy-ry, cx, cy-ry), path.CubicTo(cx+k*rx, cy-ry, cx+rx, cy-k*ry, cx+rx, cy), path.Close()}
	d.Items = append(d.Items, PathItem{X: x, Y: y, Commands: cmds, EllipseStroke: true, StrokeWidth: q.translation(.067) * scale, Color: b.Color})
	return true
}
