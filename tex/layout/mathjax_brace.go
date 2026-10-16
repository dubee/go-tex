package layout

import (
	_ "embed"
	"encoding/json"
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
	"math"
	"strconv"
)

// Pinned MathJax delimiter 23DE/23DF: four-piece horizontal assembly with
// .1em overlap. The extender glyph is a solid horizontal rectangle, so its
// clipped nested SVG uses a bounded rectangle-intersection path with separate
// clip-origin/extent quantization. This is not general SVG clipping.
//
//go:embed mathjax_skew.json
var mathJaxSkewData []byte
var mathJaxSkew = func() map[string]map[string]float64 {
	m := map[string]map[string]float64{}
	if err := json.Unmarshal(mathJaxSkewData, &m); err != nil {
		panic(err)
	}
	return m
}()

func mathJaxBraceDelta(b *Box, over bool) float64 {
	switch c := b.Content.(type) {
	case Glyph:
		v := .75 * mathJaxCharacterItalic(b)
		if over {
			v += mathJaxSkew[c.FontID][strconv.Itoa(int(c.CharCode))]
		}
		return v
	case HBox:
		if len(c.Children) == 1 {
			return mathJaxBraceDelta(c.Children[0], over)
		}
	}
	return 0
}

type mathJaxBrace struct{ Over bool }

func (mathJaxBrace) contentMarker() {}
func layoutMathJaxBrace(n *parser.HorizBrace, o Options) *Box {
	body := layoutNode(n.Base, o)
	width := math.Max(1.8, body.Width)
	brace := &Box{Width: width, Height: .32, Depth: .2, Color: o.Color, Content: mathJaxBrace{n.IsOver}}
	y := body.Depth + .105 + .32
	h, dep := body.Height, y+.2+.1
	if n.IsOver {
		y = -(math.Max(body.Height, .442) + .105 + .2)
		h = -y + .32 + .1
		dep = body.Depth
	}
	delta := mathJaxBraceDelta(body, n.IsOver)
	bx, ax := (width-body.Width)/2, delta
	if !n.IsOver {
		ax = -delta
	}
	if ax < 0 {
		bx -= ax
		ax = 0
	}
	return &Box{Width: math.Max(bx+body.Width, ax+width), Height: h, Depth: dep, Color: o.Color, Content: Positioned{Children: []Placed{{body, bx, 0, 1}, {brace, ax, y, 1}}}}
}
func layoutMathJaxBraceScript(n *parser.HorizBrace, sup, sub parser.Node, o Options) *Box {
	base := layoutMathJaxBrace(n, o)
	target := sub
	st := o.Style.Subscript()
	if n.IsOver {
		target = sup
		st = o.Style.Superscript()
	}
	if target == nil {
		return base
	}
	annotation := layoutNode(target, o.WithStyle(st))
	scales := [3]float64{1, 1 / math.Sqrt(2), .5}
	s := scales[st.SizeIndex()] / scales[o.Style.SizeIndex()]
	width := math.Max(base.Width, annotation.Width*s)
	y := base.Depth + math.Max(.167, .6-annotation.Height*s) - .105 + annotation.Height*s
	h, dep := base.Height, y+annotation.Depth*s+.1
	if n.IsOver {
		y = -(base.Height + math.Max(.111, .2-math.Max(0, annotation.Depth*s)) - .105 + annotation.Depth*s)
		h = -y + annotation.Height*s + .1
		dep = base.Depth
	}
	return &Box{Width: width, Height: h, Depth: dep, Color: o.Color, Content: Positioned{Children: []Placed{{base, (width - base.Width) / 2, 0, 1}, {annotation, (width - annotation.Width*s) / 2, y, s}}}}
}
func emitMathJaxBrace(b *Box, c mathJaxBrace, d *DisplayList, x, y, scale float64, q svgCoordinates) bool {
	left, right, mid := rune(0xE152), rune(0xE153), rune(0xE156)
	if c.Over {
		left, right, mid = 0xE150, 0xE151, 0xE155
	}
	o := DefaultOptions()
	o.MathJaxGlyphs = true
	o.FontOverride = "MathJax-Brace"
	o.Color = b.Color
	for _, p := range []struct {
		cp rune
		x  float64
	}{{left, 0}, {right, b.Width - .45}, {mid, (b.Width - .9) / 2}} {
		glyph := layoutSymbol(string(p.cp), parser.ModeText, o)
		if !emitMathJaxGlyph(glyph, d, x+q.translation(p.x)*scale, y, scale, q) {
			return false
		}
	}
	// addExtH: endpoint width .45 and half-middle width .45, minus HFUZZ .1.
	length := b.Width/2 - .7
	if length > 0 {
		for _, offset := range []float64{.35, b.Width/2 + .35} {
			w, h := q.translation(length)*scale, .12*scale
			d.Items = append(d.Items, PathItem{X: x + q.translation(offset)*scale, Y: y - h, Commands: []path.Command{path.MoveTo(0, 0), path.LineTo(w, 0), path.LineTo(w, h), path.LineTo(0, h), path.Close()}, Fill: true, ClippedExtender: true, Color: b.Color})
		}
	}
	return true
}
