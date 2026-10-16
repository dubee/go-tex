package layout

import (
	"github.com/luo-studio/go-tex/tex/fontmetrics"
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
	"math"
	"strings"
)

// Positioned stores child coordinates relative to the parent's baseline.
type Positioned struct{ Children []Placed }
type Placed struct {
	Box         *Box
	X, Y, Scale float64
}

func (Positioned) contentMarker() {}
func placedBox(items []Placed, color Color) *Box {
	b := &Box{Color: color, Content: Positioned{Children: items}}
	for _, p := range items {
		b.Width = math.Max(b.Width, p.X+p.Box.Width*p.Scale)
		b.Height = math.Max(b.Height, p.Box.Height*p.Scale-p.Y)
		b.Depth = math.Max(b.Depth, p.Y+p.Box.Depth*p.Scale)
	}
	return b
}
func layoutMultiScripts(n *parser.MultiScripts, o Options) *Box {
	if o.MathJaxGlyphs {
		return layoutMathJaxMultiScripts(n, o)
	}
	base := layoutNode(n.Base, o)
	// Use the normal TeX side-script algorithm for vertical shifts and sizes.
	side := func(sup, sub parser.Node) *Box {
		return layoutSupSubNode(&parser.OrdGroup{Mode: parser.ModeMath, Body: []parser.Node{n.Base}}, sup, sub, o)
	}
	left, right := side(n.LeftSup, n.LeftSub), side(n.RightSup, n.RightSub)
	l, ok := left.Content.(SupSub)
	if !ok {
		l = SupSub{Base: base}
	}
	r, ok := right.Content.(SupSub)
	if !ok {
		r = SupSub{Base: base}
	}
	lw := 0.
	if l.Sup != nil {
		lw = math.Max(lw, l.Sup.Width*l.SupScale)
	}
	if l.Sub != nil {
		lw = math.Max(lw, l.Sub.Width*l.SubScale)
	}
	gap := 0.
	if lw > 0 {
		gap = .05
	}
	items := []Placed{{base, lw + gap, 0, 1}}
	if l.Sup != nil {
		items = append(items, Placed{l.Sup, lw - l.Sup.Width*l.SupScale, -l.SupShift, l.SupScale})
	}
	if l.Sub != nil {
		items = append(items, Placed{l.Sub, lw - l.Sub.Width*l.SubScale, l.SubShift, l.SubScale})
	}
	rx := lw + gap + base.Width
	if r.Sup != nil {
		items = append(items, Placed{r.Sup, rx + r.ItalicCorrection, -r.SupShift, r.SupScale})
	}
	if r.Sub != nil {
		items = append(items, Placed{r.Sub, rx + r.SubHKern, r.SubShift, r.SubScale})
	}
	return placedBox(items, o.Color)
}
func layoutCancelTo(n *parser.CancelTo, o Options) *Box {
	if o.MathJaxGlyphs {
		return layoutMathJaxCancelTo(n, o)
	}
	body := layoutNode(n.Body, o)
	ss := o.Style.Superscript()
	ratio := ss.SizeMultiplier() / o.Style.SizeMultiplier()
	target := layoutNode(n.Target, o.WithStyle(ss))
	x1, y1 := -.12, body.Depth+.12
	x2, y2 := body.Width+.2, -body.Height-.18
	// Stroke a diagonal with a real arrowhead; keep the target above/right.
	dx, dy := x2-x1, y2-y1
	length := math.Hypot(dx, dy)
	ux, uy := dx/length, dy/length
	cmds := []path.Command{path.MoveTo(0, y1), path.LineTo(x2-x1, y2), path.MoveTo(x2-x1-.18*ux+.07*uy, y2-.18*uy-.07*ux), path.LineTo(x2-x1, y2), path.LineTo(x2-x1-.18*ux-.07*uy, y2-.18*uy+.07*ux)}
	// The arrow is visible layout content. Its lower endpoint extends below
	// the body; retaining zero depth relied on canvas expansion to hide the
	// missing box extent and clips when a renderer honors nominal dimensions.
	arrow := &Box{Width: x2 - x1, Height: -y2, Depth: y1, Color: o.Color, Content: SvgPath{Commands: cmds, Fill: false}}
	return placedBox([]Placed{{body, .12, 0, 1}, {arrow, 0, 0, 1}, {target, x2 + .2, y2 - .05, ratio}}, o.Color)
}

func layoutMultline(n *parser.Multline, o Options) *Box {
	rows := make([]*Box, len(n.Rows))
	width := 35.36
	for i, node := range n.Rows {
		rows[i] = layoutNode(node, o)
		width = math.Max(width, rows[i].Width)
	}
	if len(rows) == 0 {
		return NewEmpty()
	}
	items := make([]Placed, len(rows))
	y := 0.
	for i, row := range rows {
		x := (width - row.Width) / 2
		if i == 0 {
			x = 0
		} else if i == len(rows)-1 {
			x = width - row.Width
		}
		if i > 0 {
			y += math.Max(1.5, rows[i-1].Depth+row.Height+.3)
		}
		items[i] = Placed{row, x, y, 1}
	}
	shift := (y+rows[len(rows)-1].Depth-rows[0].Height)/2 + o.Metrics().AxisHeight
	for i := range items {
		items[i].Y -= shift
	}
	b := placedBox(items, o.Color)
	b.Width = width
	return b
}

func layoutUnicode(n *parser.Unicode, o Options) *Box {
	// MathJax UnicodeConfiguration emits mtext with the current mathvariant.
	// Known scalars use that variant's embedded SVG glyph, not system text or
	// TeX symbol-name remapping. Unknown/explicit-font forms remain outside the
	// bounded MathJax glyph pipeline.
	if o.MathJaxGlyphs {
		font := o.FontOverride
		if font == "" {
			font = "Main-Regular"
		}
		if outline, ok := o.SystemUnicode[n.Value]; ok {
			// MathJax unknown mtext uses .6/.75/.2 placeholder metrics;
			// the SVG text outline may extend outside that nominal box.
			return &Box{Width: .6, Height: .75, Depth: .2, Color: o.Color, Content: Glyph{CharCode: n.Value, Outline: outline}}
		}
		if g, ok := pinnedMathJaxGlyph("Unicode-"+font, n.Value); ok && g.Chars == "" {
			return &Box{Width: g.Width, Height: g.Height, Depth: g.Depth, Color: o.Color,
				Content: Glyph{FontID: "Unicode-" + font, CharCode: n.Value, Outline: g.commands}}
		}
	}
	for _, font := range []string{"Main-Regular", "AMS-Regular", "Math-Italic"} {
		if m, ok := fontmetrics.Lookup(font, n.Value); ok {
			return &Box{Width: m.Width, Height: m.Height, Depth: m.Depth, Color: o.Color, Content: Glyph{FontID: font, CharCode: n.Value}}
		}
	}
	// The rasterizer rejects absent glyphs explicitly, with their codepoint.
	return layoutSymbol(string(n.Value), parser.ModeText, o)
}
func layoutDecorated(n *parser.Decorated, o Options) *Box {
	if o.MathJaxGlyphs && n.Enclosure && n.Notation == "circle" {
		return layoutMathJaxCircle(n, o)
	}
	// MathJax CommonDiagonalStrike uses fullBBox: padding + thickness on
	// all sides. Its SVG lineData insets the stroke endpoints by thickness/2.
	// Keep other menclose notations and CSS bbox separate until reconciled.
	diagonal := n.Enclosure && len(strings.Fields(n.Notation)) > 0
	for _, notation := range strings.Fields(n.Notation) {
		diagonal = diagonal && (notation == "updiagonalstrike" || notation == "downdiagonalstrike")
	}
	if diagonal {
		return layoutDiagonalEnclosure(n, o)
	}
	body := layoutNode(n.Body, o)
	pad := n.Padding
	w, h, d := body.Width+2*pad, body.Height+pad, body.Depth+pad
	items := []Placed{}
	if n.Background != "" {
		bg, ok := parseColor(n.Background)
		if !ok {
			bg = o.Color
		}
		rect := &Box{Width: w, Height: h, Depth: d, Color: bg, Content: SvgPath{Fill: true, Commands: []path.Command{path.MoveTo(0, -h), path.LineTo(w, -h), path.LineTo(w, d), path.LineTo(0, d), path.Close()}}}
		items = append(items, Placed{rect, 0, 0, 1})
	}
	items = append(items, Placed{body, pad, 0, 1})
	col, ok := parseColor(n.BorderColor)
	if !ok {
		col = o.Color
	}
	for _, notation := range strings.Fields(n.Notation) {
		var cmds []path.Command
		switch notation {
		case "circle":
			const k = .5522847498307936
			rx, ry := w/2, (h+d)/2
			cy := (d - h) / 2
			cmds = []path.Command{path.MoveTo(w, cy), path.CubicTo(w, cy+k*ry, rx+k*rx, d, rx, d), path.CubicTo(rx-k*rx, d, 0, cy+k*ry, 0, cy), path.CubicTo(0, cy-k*ry, rx-k*rx, -h, rx, -h), path.CubicTo(rx+k*rx, -h, w, cy-k*ry, w, cy), path.Close()}
		case "roundedbox":
			r := math.Min(.2, math.Min(w, h+d)/4)
			cmds = []path.Command{path.MoveTo(r, -h), path.LineTo(w-r, -h), path.QuadTo(w, -h, w, -h+r), path.LineTo(w, d-r), path.QuadTo(w, d, w-r, d), path.LineTo(r, d), path.QuadTo(0, d, 0, d-r), path.LineTo(0, -h+r), path.QuadTo(0, -h, r, -h), path.Close()}

		case "box":
			cmds = []path.Command{path.MoveTo(0, -h), path.LineTo(w, -h), path.LineTo(w, d), path.LineTo(0, d), path.Close()}
		case "updiagonalstrike", "updiagonalarrow":
			cmds = []path.Command{path.MoveTo(0, d), path.LineTo(w, -h)}
			if notation == "updiagonalarrow" {
				cmds = append(cmds, path.MoveTo(w-.18, -h), path.LineTo(w, -h), path.LineTo(w, -h+.18))
			}
		case "downdiagonalstrike":
			cmds = []path.Command{path.MoveTo(0, -h), path.LineTo(w, d)}
		case "horizontalstrike":
			cmds = []path.Command{path.MoveTo(0, (d-h)/2), path.LineTo(w, (d-h)/2)}
		case "verticalstrike":
			cmds = []path.Command{path.MoveTo(w/2, -h), path.LineTo(w/2, d)}
		case "top":
			cmds = []path.Command{path.MoveTo(0, -h), path.LineTo(w, -h)}
		case "bottom":
			cmds = []path.Command{path.MoveTo(0, d), path.LineTo(w, d)}
		case "left":
			cmds = []path.Command{path.MoveTo(0, -h), path.LineTo(0, d)}
		case "right":
			cmds = []path.Command{path.MoveTo(w, -h), path.LineTo(w, d)}
		}
		shape := &Box{Width: w, Height: 0, Depth: 0, Color: col, Content: SvgPath{Commands: cmds, Fill: false, StrokeWidth: n.Border}}
		items = append(items, Placed{shape, 0, 0, 1})
	}
	b := placedBox(items, o.Color)
	b.Width = w
	b.Height = h
	b.Depth = d
	return b
}

// layoutLabeledArrow uses padded script boxes, including the asymmetric label
// inset on one-headed arrows. The saved MathJax reference SVGs establish the
// 5/10mu and 6/6mu insets, struts and baseline offsets. A plain OpLimits box
// cannot represent these: centering its unpadded labels loses the inset.
func layoutLabeledArrow(n *parser.XArrow, o Options) *Box {
	if o.MathJaxGlyphs {
		return layoutMathJaxLabeledArrow(n, o)
	}
	supStyle, subStyle := o.Style.Superscript(), o.Style.Subscript()
	supRatio := supStyle.SizeMultiplier() / o.Style.SizeMultiplier()
	subRatio := subStyle.SizeMultiplier() / o.Style.SizeMultiplier()
	// MathJax's script multiplier is sqrt(1/2), versus TeX's rounded .7.
	if !o.Style.IsTight() {
		supRatio, subRatio = math.Sqrt(.5), math.Sqrt(.5)
	}
	above := layoutNode(n.Body, o.WithStyle(supStyle))
	var below *Box
	if n.Below != nil {
		below = layoutNode(n.Below, o.WithStyle(subStyle))
	}
	left, right := 5./18, 10./18
	if n.Label == `\xleftarrow` {
		left, right = right, left
	}
	if n.Label == `\xleftrightarrow` {
		left, right = 6./18, 6./18
	}
	upperW := (above.Width + left + right) * supRatio
	width := math.Max(1, upperW)
	lowerW := 0.
	if below != nil {
		lowerW = (below.Width + left + right) * subRatio
		width = math.Max(width, lowerW)
	}
	commands, height, _ := katexStretchyPath(n.Label, width)
	arrow := &Box{Width: width, Height: height / 2, Depth: height / 2, Color: o.Color, Content: SvgPath{Commands: commands, Fill: true}}
	items := []Placed{{arrow, 0, -o.Metrics().AxisHeight, 1}}
	upperDepth := math.Max(.25, above.Depth) * supRatio
	upperY := -(.583 + math.Max(.111, .2-upperDepth) + upperDepth - .2*supRatio)
	items = append(items, Placed{above, (width-upperW)/2 + left*supRatio, upperY, supRatio})
	if below != nil {
		lowerHeight := math.Max(.75, below.Height) * subRatio
		lowerY := .082 + math.Max(.167, .6-lowerHeight) + lowerHeight - .15*subRatio
		items = append(items, Placed{below, (width-lowerW)/2 + left*subRatio, lowerY, subRatio})
	}
	return placedBox(items, o.Color)
}

// layoutDiagonalEnclosure reproduces the pinned menclose diagonal geometry.
func layoutDiagonalEnclosure(n *parser.Decorated, o Options) *Box {
	body := layoutNode(n.Body, o)
	const padding, thickness = .2, .067
	pad, inset := padding+thickness, thickness/2
	w, h, d := body.Width+2*pad, body.Height+pad, body.Depth+pad
	items := []Placed{{body, pad, 0, 1}}
	seen := map[string]bool{}
	for _, notation := range strings.Fields(n.Notation) {
		if seen[notation] {
			continue
		}
		seen[notation] = true
		y1, y2 := d-inset, -h+inset
		if notation == "downdiagonalstrike" {
			y1, y2 = y2, y1
		}
		stroke := &Box{Width: w, Color: o.Color, Content: SvgPath{Commands: []path.Command{path.MoveTo(inset, y1), path.LineTo(w-inset, y2)}, StrokeWidth: thickness}}
		items = append(items, Placed{stroke, 0, 0, 1})
	}
	b := placedBox(items, o.Color)
	b.Width, b.Height, b.Depth = w, h, d
	return b
}
