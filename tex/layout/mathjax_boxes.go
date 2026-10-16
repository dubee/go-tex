package layout

import (
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
	"strconv"
)

// MathJax 3.2.2 SVGWrapper.handleBorder/handleColor and CommonMpadded.
// CSS pixels use the lite adaptor's 8px ex and TeX font's .442em ex.
// Preserve wrapper and mpadded transforms separately for SVG serialization.
type mathJaxCSSBox struct {
	Body        *Box
	Pad, Border float64
	Background  *Color
	BorderColor Color
	ColorBox    bool
}

func (mathJaxCSSBox) contentMarker() {}
func mathJaxCSSLength(s string) float64 {
	if len(s) < 3 {
		return 0
	}
	v, _ := strconv.ParseFloat(s[:len(s)-2], 64)
	switch s[len(s)-2:] {
	case "px":
		return v / (8 / .442)
	case "pt":
		return v / 10
	default:
		return v
	}
}
func layoutMathJaxCSSBox(n *parser.Decorated, o Options) *Box {
	body := layoutNode(n.Body, o)
	pad := mathJaxCSSLength(n.PaddingSpec)
	border := mathJaxCSSLength(n.BorderSpec)
	col := o.Color
	if c, ok := parseColor(n.BorderColor); ok {
		col = c
	}
	var bg *Color
	if n.Background != "" {
		c, _ := parseColor(n.Background)
		bg = &c
	}
	b := mathJaxBox(body, pad, border, bg, col, false, o)
	// BBox uses parseInt for width but the complete decimal for lspace/height.
	if n.PaddingSpec != "" {
		raw := n.PaddingSpec[:len(n.PaddingSpec)-2]
		digits := 0
		for digits < len(raw) && raw[digits] >= '0' && raw[digits] <= '9' {
			digits++
		}
		if digits == 0 {
			b.Width = 2*body.Width + 2*border
		} else {
			number, _ := strconv.Atoi(raw[:digits])
			extra := mathJaxCSSLength(strconv.Itoa(2*number) + n.PaddingSpec[len(n.PaddingSpec)-2:])
			b.Width = body.Width + extra + 2*border
		}
	}
	return b
}
func layoutMathJaxColorBox(n *parser.Enclose, o Options) *Box {
	body := layoutNode(mathJaxInternalMath(n.Body), o)
	if n.Label == `\fbox` {
		return &Box{Width: body.Width + .534, Height: body.Height + .267, Depth: body.Depth + .267, Color: o.Color, Content: mathJaxFrame{body}}
	}
	border := 0.
	if n.Label == `\fcolorbox` {
		border = mathJaxCSSLength("2px")
	}
	col := o.Color
	if n.BorderColor != nil {
		if c, ok := parseColor(*n.BorderColor); ok {
			col = c
		}
	}
	return mathJaxBox(body, mathJaxCSSLength("5px"), border, parseColorPtr(n.BackgroundColor), col, true, o)
}
func mathJaxBox(body *Box, pad, border float64, bg *Color, col Color, colorbox bool, o Options) *Box {
	return &Box{Width: body.Width + 2*pad + 2*border, Height: body.Height + pad + border, Depth: body.Depth + pad + border, Color: o.Color, Content: mathJaxCSSBox{body, pad, border, bg, col, colorbox}}
}
func emitMathJaxCSSBox(b *Box, c mathJaxCSSBox, d *DisplayList, x, y, scale float64, q svgCoordinates) bool {
	dx := c.Border
	gx := x + q.translation(dx)*scale
	// SVGWrapper paints the background before inserting the four border polygons.
	if c.Background != nil {
		bottom := q.translation(-b.Depth)
		height := q.translation(b.Height + b.Depth)
		d.Items = append(d.Items, Rect{X: gx + q.translation(-dx)*scale, Y: y - (bottom+height)*scale, Width: q.translation(b.Width) * scale, Height: height * scale, Color: *c.Background})
	}
	if c.Border > 0 {
		const fuzz = .005
		h, dep, w := b.Height+fuzz, b.Depth+fuzz, b.Width+fuzz
		t := c.Border
		rt, lt, rb, lb := [2]float64{w, h}, [2]float64{-fuzz, h}, [2]float64{w, -dep}, [2]float64{-fuzz, -dep}
		irt, ilt, irb, ilb := [2]float64{w - t, h - t}, [2]float64{-fuzz + t, h - t}, [2]float64{w - t, -dep + t}, [2]float64{-fuzz + t, -dep + t}
		for _, points := range [][4][2]float64{{lt, rt, irt, ilt}, {rb, rt, irt, irb}, {lb, rb, irb, ilb}, {lb, lt, ilt, ilb}} {
			cmds := []path.Command{}
			for i, p := range points {
				px, py := q.translation(p[0]-dx)*scale, -q.translation(p[1])*scale
				if i == 0 {
					cmds = append(cmds, path.MoveTo(px, py))
				} else {
					cmds = append(cmds, path.LineTo(px, py))
				}
			}
			cmds = append(cmds, path.Close())
			d.Items = append(d.Items, PathItem{X: gx, Y: y, Commands: cmds, Fill: true, Color: c.BorderColor})
		}
	}
	pad := c.Pad
	if c.ColorBox {
		pad += dx
	}
	return emitMathJaxGlyph(c.Body, d, gx+q.translation(pad)*scale, y, scale, q)
}

type mathJaxFrame struct{ Body *Box }

func (mathJaxFrame) contentMarker() {}
func emitMathJaxFrame(b *Box, c mathJaxFrame, d *DisplayList, x, y, scale float64, q svgCoordinates) bool {
	if !emitMathJaxGlyph(c.Body, d, x+q.translation(.267)*scale, y, scale, q) {
		return false
	}
	inset := q.translation(.0335)
	bottom := q.translation(-b.Depth + .0335)
	w := q.translation(b.Width - .067)
	h := q.translation(b.Height + b.Depth - .067)
	d.Items = append(d.Items, PathItem{X: x + inset*scale, Y: y - (bottom+h)*scale, RectStroke: true, Commands: []path.Command{path.MoveTo(0, 0), path.LineTo(w*scale, 0), path.LineTo(w*scale, h*scale), path.LineTo(0, h*scale), path.Close()}, StrokeWidth: q.translation(.067) * scale, Color: b.Color})
	return true
}

// internalMath's $...$ inherits the surrounding MathJax style; the parser's
// synthetic text-mode Styling nodes are KaTeX wrappers, not explicit TeX styles.
func mathJaxInternalMath(n parser.Node) parser.Node {
	switch v := n.(type) {
	case *parser.Styling:
		if v.Mode == parser.ModeText {
			children := make([]parser.Node, len(v.Body))
			for i, c := range v.Body {
				children[i] = mathJaxInternalMath(c)
			}
			return &parser.OrdGroup{Mode: v.Mode, Body: children}
		}
	case *parser.OrdGroup:
		c := *v
		c.Body = make([]parser.Node, len(v.Body))
		for i, n := range v.Body {
			c.Body[i] = mathJaxInternalMath(n)
		}
		return &c
	}
	return n
}
