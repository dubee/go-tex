package layout

import (
	"github.com/luo-studio/go-tex/tex/path"
	"math"
	"math/big"
)

// ToMathJaxGlyphDisplayList reproduces SVG wrapper transform serialization for
// the bounded ordinary-glyph/fraction pipeline. Layout dimensions stay at full
// precision; translations are serialized in 1000/em units to one decimal and
// relative scales to three decimals before composing parent transforms.
// Unsupported box constructors retain the existing nominal display list.
func ToMathJaxGlyphDisplayList(b *Box) DisplayList {
	return mathJaxGlyphDisplayList(b, svgCoordinates{})
}

// ToLibrsvgGlyphDisplayList includes librsvg 2.61.2's CSS number parsing:
// serialized SVG transform/length numbers pass through float32 before composing.
// Glyph path data itself uses the separate float64 path parser.
func ToLibrsvgGlyphDisplayList(b *Box) DisplayList {
	return mathJaxGlyphDisplayList(b, svgCoordinates{parsed: true})
}

func mathJaxGlyphDisplayList(b *Box, coordinates svgCoordinates) DisplayList {
	d := DisplayList{Width: b.Width, Height: b.Height, Depth: b.Depth}
	if !emitMathJaxGlyph(b, &d, 0, b.Height, 1, coordinates) {
		return ToNominalDisplayList(b)
	}
	return d
}

type svgCoordinates struct{ parsed bool }

func (c svgCoordinates) number(v float64) float64 {
	if c.parsed {
		return float64(float32(v))
	}
	return v
}
func (c svgCoordinates) translation(v float64) float64 { return c.number(svgFixed(v*1000, 1)) / 1000 }
func (c svgCoordinates) scale(v float64) float64       { return c.number(svgFixed(v, 3)) }

// CommonOutputJax.fixed uses Number.toFixed: round the exact binary float to
// decimal, with ties away from zero (not a rounded intermediate v*10^n).
func svgFixed(v float64, places int) float64 {
	if math.Abs(v) < .0006 {
		return 0
	}
	factor := int64(1)
	for i := 0; i < places; i++ {
		factor *= 10
	}
	r := new(big.Rat).SetFloat64(math.Abs(v))
	if r == nil {
		return v
	}
	n := new(big.Int).Mul(r.Num(), big.NewInt(factor))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(n, r.Denom(), rem)
	if rem.Lsh(rem, 1).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	f, _ := new(big.Rat).SetFrac(q, big.NewInt(factor)).Float64()
	return math.Copysign(f, v)
}

func emitMathJaxGlyph(b *Box, d *DisplayList, x, baseline, scale float64, coordinates svgCoordinates) bool {
	switch c := b.Content.(type) {
	case Empty, Kern:
		return true
	case Glyph:
		if len(c.Outline) == 0 {
			if c.CharCode == 32 || c.CharCode == 160 {
				return true
			}
			return false
		}
		commands := make([]path.Command, len(c.Outline))
		for i, cmd := range c.Outline {
			commands[i] = scalePathCommand(cmd, scale)
		}
		d.Items = append(d.Items, PathItem{X: x, Y: baseline, Commands: commands, Fill: true, Color: b.Color})
		return true
	case HBox:
		// Bare fractions are represented by an engine HBox with null-delimiter
		// kerns. MathJax emits those kerns inside mfrac's child translations.
		if len(c.Children) == 3 {
			if _, ok := c.Children[0].Content.(Kern); ok {
				if f, ok := c.Children[1].Content.(Fraction); ok {
					if _, ok := c.Children[2].Content.(Kern); ok {
						return emitMathJaxFraction(c.Children[1], f, d, x, baseline, scale, c.Children[0].Width, coordinates)
					}
				}
			}
		}
		offset := 0.0
		for _, child := range c.Children {
			if !emitMathJaxGlyph(child, d, x+coordinates.translation(offset)*scale, baseline, scale, coordinates) {
				return false
			}
			offset += child.Width
		}
		return true
	case Scaled:
		return emitMathJaxGlyph(c.Body, d, x, baseline, scale*coordinates.scale(c.ChildScale), coordinates)
	case SupSub:
		if c.CenterScripts || !emitMathJaxGlyph(c.Base, d, x, baseline, scale, coordinates) {
			return false
		}
		if c.Sup != nil && !emitMathJaxGlyph(c.Sup, d, x+coordinates.translation(c.Base.Width+c.SupHKern)*scale, baseline-coordinates.translation(c.SupShift)*scale, scale*coordinates.scale(c.SupScale), coordinates) {
			return false
		}
		if c.Sub != nil && !emitMathJaxGlyph(c.Sub, d, x+coordinates.translation(c.Base.Width+c.SubHKern)*scale, baseline+coordinates.translation(c.SubShift)*scale, scale*coordinates.scale(c.SubScale), coordinates) {
			return false
		}
		return true
	case mathJaxFrame:
		return emitMathJaxFrame(b, c, d, x, baseline, scale, coordinates)
	case mathJaxCircle:
		return emitMathJaxCircle(b, d, x, baseline, scale, coordinates)
	case mathJaxBrace:
		return emitMathJaxBrace(b, c, d, x, baseline, scale, coordinates)
	case mathJaxLabeledArrow:
		return emitMathJaxLabeledArrow(b, c, d, x, baseline, scale, coordinates)
	case mathJaxArrow:
		return emitMathJaxArrow(b, c, d, x, baseline, scale, coordinates)
	case mathJaxCSSBox:
		return emitMathJaxCSSBox(b, c, d, x, baseline, scale, coordinates)
	case mathJaxTable:
		for _, row := range c.Rows {
			if !emitMathJaxGlyph(row.Box, d, x, baseline+coordinates.translation(row.Y)*scale, scale*coordinates.scale(row.Scale), coordinates) {
				return false
			}
		}
		// equation.cjs serializes only the SVG element, excluding MathJax's
		// external .mjx-solid stylesheet. Table rule lines inherit stroke-width
		// zero. They contribute geometry but no painted pixels in that reference.

		return true
	case Positioned:
		// Composite glyphs and diagonal enclosures preserve each child path
		// and its SVG transform/compositing boundary.
		for _, child := range c.Children {
			if !emitMathJaxGlyph(child.Box, d, x+coordinates.translation(child.X)*scale, baseline+coordinates.translation(child.Y)*scale, scale*coordinates.scale(child.Scale), coordinates) {
				return false
			}
		}
		return true
	case SvgPath:
		// MathJax's diagonal menclose uses an SVG line with the default butt
		// caps. Represent the stroke as a filled quadrilateral for Cairo fill.
		if c.Fill || len(c.Commands) != 2 || c.Commands[0].Kind != path.KindMoveTo || c.Commands[1].Kind != path.KindLineTo || c.StrokeWidth <= 0 {
			return false
		}
		a, z := c.Commands[0], c.Commands[1]
		x1, y1 := coordinates.translation(a.X), coordinates.translation(a.Y)
		x2, y2 := coordinates.translation(z.X), coordinates.translation(z.Y)
		d.Items = append(d.Items, PathItem{X: x, Y: baseline, Commands: []path.Command{path.MoveTo(x1*scale, y1*scale), path.LineTo(x2*scale, y2*scale)}, StrokeWidth: coordinates.translation(c.StrokeWidth) * scale, Butt: true, Color: b.Color})
		return true
	case Fraction:
		return emitMathJaxFraction(b, c, d, x, baseline, scale, 0, coordinates)
	default:
		return false
	}
}

func emitMathJaxFraction(b *Box, c Fraction, d *DisplayList, x, baseline, scale, pad float64, coordinates svgCoordinates) bool {

	nx := pad + (b.Width-c.Numer.Width*c.NumerScale)/2
	dx := pad + (b.Width-c.Denom.Width*c.DenomScale)/2
	if c.Align == "l" {
		nx, dx = pad+c.RulePadding, pad+c.RulePadding
	} else if c.Align == "r" {
		nx = pad + b.Width - c.RulePadding - c.Numer.Width*c.NumerScale
		dx = pad + b.Width - c.RulePadding - c.Denom.Width*c.DenomScale
	}
	if !emitMathJaxGlyph(c.Numer, d, x+coordinates.translation(nx)*scale, baseline-coordinates.translation(c.NumerShift)*scale, scale*coordinates.scale(c.NumerScale), coordinates) {
		return false
	}
	if !emitMathJaxGlyph(c.Denom, d, x+coordinates.translation(dx)*scale, baseline+coordinates.translation(c.DenomShift)*scale, scale*coordinates.scale(c.DenomScale), coordinates) {
		return false
	}
	if c.BarThickness <= 0 {
		return true
	}
	// SVG rect y denotes its bottom in MathJax's y-up group.
	h := coordinates.translation(c.BarThickness)
	y := coordinates.translation(.25 - c.BarThickness/2)
	d.Items = append(d.Items, Rect{X: x + coordinates.translation(pad)*scale, Y: baseline - (y+h)*scale, Width: coordinates.translation(b.Width) * scale, Height: h * scale, Color: b.Color})
	return true
}
