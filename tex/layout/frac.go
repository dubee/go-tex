package layout

import (
	"github.com/luo-studio/go-tex/tex/mathstyle"
	"github.com/luo-studio/go-tex/tex/parser"
	"math"
)

// layoutFraction implements TeXbook Rule 15d. Mirrors upstream's
// layout_fraction in engine.rs.
func layoutFraction(numer parser.Node, denom parser.Node, barThickness float64, continued bool, opts Options) *Box {
	numerStyle := opts.Style.Numerator()
	denomStyle := opts.Style.Denominator()
	numerOpts := opts.WithStyle(numerStyle)
	denomOpts := opts.WithStyle(denomStyle)

	var numerBox, denomBox *Box
	if continued && opts.MathJaxGlyphs {
		// MathJax AmsMethods.CFrac constructs each child as
		// \strut\textstyle{body}. The strut inherits the child's script
		// level; the content resets to text style, with a separate SVG scale.
		child := func(node parser.Node, childOpts Options) *Box {
			body := layoutNode(node, childOpts.WithStyle(mathstyle.Text))
			s := 1 / math.Sqrt(2)
			scales := [3]float64{1, s, s * s}
			ratio := 1 / scales[childOpts.Style.SizeIndex()]
			if ratio != 1 {
				body = &Box{Width: body.Width * ratio, Height: body.Height * ratio, Depth: body.Depth * ratio, Color: opts.Color, Content: Scaled{Body: body, ChildScale: ratio}}
			}
			return makeHBox([]*Box{{Height: .86, Depth: .3, Color: opts.Color, Content: Empty{}}, body}, opts.Color)
		}
		numerBox, denomBox = child(numer, numerOpts), child(denom, denomOpts)
	} else {
		numerBox = layoutNode(numer, numerOpts)
		if continued && !opts.MathJaxGlyphs {
			pt := opts.Metrics().PtPerEm
			hMin := 8.5 / pt
			dMin := 3.5 / pt
			if numerBox.Height < hMin {
				numerBox.Height = hMin
			}
			if numerBox.Depth < dMin {
				numerBox.Depth = dMin
			}
		}
		denomBox = layoutNode(denom, denomOpts)

	}

	numerRatio := numerStyle.SizeMultiplier() / opts.Style.SizeMultiplier()
	denomRatio := denomStyle.SizeMultiplier() / opts.Style.SizeMultiplier()
	if opts.FractionScriptScale > 0 {
		// Pinned MathJax CommonWrapper.getScale caps scriptlevel at two,
		// then divides the absolute child scale by the parent scale.
		// Restrict this override to fraction boundaries; other script
		// constructors retain their upstream behavior.
		s := opts.FractionScriptScale
		scales := [3]float64{1, s, s * s}
		parentScale := scales[opts.Style.SizeIndex()]
		numerRatio = scales[numerStyle.SizeIndex()] / parentScale
		denomRatio = scales[denomStyle.SizeIndex()] / parentScale
	}

	if opts.MathJaxSizes && opts.FractionScriptScale > 0 && opts.Style.SizeIndex() == 0 {
		// A direct mathsize declaration belongs to the fraction child itself.
		// Build that child in its declared size context and compute its single
		// absolute-child/parent ratio, rather than multiply rounded ratios.
		// Nested styling/group contents retain their normal layout paths.
		// Script-style parents retain their existing path: their wrapper
		// composition has a separate unresolved absolute/product boundary.
		if !continued {
			numerBox, numerRatio = fractionSizedChild(numer, numerOpts, opts, numerBox, numerRatio)
		}
		denomBox, denomRatio = fractionSizedChild(denom, denomOpts, opts, denomBox, denomRatio)
	}

	numerHeight := numerBox.Height * numerRatio
	numerDepth := numerBox.Depth * numerRatio
	denomHeight := denomBox.Height * denomRatio
	denomDepth := denomBox.Depth * denomRatio
	numerWidth := numerBox.Width * numerRatio
	denomWidth := denomBox.Width * denomRatio

	metrics := opts.Metrics()
	axis := metrics.AxisHeight
	rule := barThickness

	var numShift, denShift float64
	if opts.Style.IsDisplay() {
		numShift = metrics.Num1
		denShift = metrics.Denom1
	} else if barThickness > 0 {
		numShift = metrics.Num2
		denShift = metrics.Denom2
	} else {
		numShift = metrics.Num3
		denShift = metrics.Denom2
	}

	if barThickness > 0 && opts.MathJaxFractionMetrics {
		// CommonMfracMixin.getTUV/getFractionBBox; preserve operation order.
		axis = .25
		numBase, denBase, factor := .394, .345, 1.5
		if opts.Style.IsDisplay() {
			numBase, denBase, factor = .676, .686, 3.5
		}
		clearance := factor * rule
		u, v := numBase-axis-clearance, denBase+axis-clearance
		numShift = axis + clearance + max64(numerDepth, u)
		denShift = -(axis - clearance - max64(denomHeight, v))
	} else if barThickness > 0 {
		var minClearance float64
		if opts.Style.IsDisplay() {
			minClearance = 3 * rule
		} else {
			minClearance = rule
		}
		numClearance := (numShift - numerDepth) - (axis + rule/2)
		if numClearance < minClearance {
			numShift += minClearance - numClearance
		}
		denClearance := (axis - rule/2) + (denShift - denomHeight)
		if denClearance < minClearance {
			denShift += minClearance - denClearance
		}
	} else if opts.MathJaxFractionMetrics {
		numShift, denShift = .444, .345
		p := 3 * .06
		if opts.Style.IsDisplay() {
			numShift, denShift, p = .676, .686, 7*.06
		}
		// CommonMfrac.getUVQ uses absolute child scale for the gap test,
		// while combining children at their relative scale into the bbox.
		absolute := [3]float64{1, 1 / math.Sqrt(2), .5}[opts.Style.SizeIndex()]
		q := (numShift - numerDepth*absolute) - (denomHeight*absolute - denShift)
		if q < p {
			numShift += (p - q) / 2
			denShift += (p - q) / 2
		}
	} else {
		var minGap float64
		if opts.Style.IsDisplay() {
			minGap = 7 * metrics.DefaultRuleThickness
		} else {
			minGap = 3 * metrics.DefaultRuleThickness
		}
		gap := (numShift - numerDepth) - (denomHeight - denShift)
		if gap < minGap {
			adjust := (minGap - gap) / 2
			numShift += adjust
			denShift += adjust
		}
	}

	totalWidth := max64(numerWidth, denomWidth)
	if barThickness > 0 {
		totalWidth += 2 * opts.FractionRulePadding
	}
	height := numerHeight + numShift
	depth := denomDepth + denShift
	return &Box{
		Width:  totalWidth,
		Height: height,
		Depth:  depth,
		Color:  opts.Color,
		Content: Fraction{
			Numer:        numerBox,
			Denom:        denomBox,
			NumerShift:   numShift,
			DenomShift:   denShift,
			BarThickness: rule,
			NumerScale:   numerRatio,
			DenomScale:   denomRatio,
		},
	}
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func fractionSizedChild(node parser.Node, child, parent Options, fallback *Box, ratio float64) (*Box, float64) {
	for {
		group, ok := node.(*parser.OrdGroup)
		if !ok || len(group.Body) != 1 {
			break
		}
		node = group.Body[0]
	}
	size, ok := node.(*parser.Sizing)
	if !ok || int(size.Size) <= 0 || int(size.Size) >= len(sizingMultipliers) {
		return fallback, ratio
	}
	mult := sizingMultiplier(int(size.Size), true)
	child.mathJaxSize = mult
	body := layoutExpression(size.Body, child, true)
	s := parent.FractionScriptScale
	scales := [3]float64{1, s, s * s}
	parentSize := parent.mathJaxSize
	if parentSize == 0 {
		parentSize = 1
	}
	return body, (scales[child.Style.SizeIndex()] * mult) / (scales[parent.Style.SizeIndex()] * parentSize)
}
