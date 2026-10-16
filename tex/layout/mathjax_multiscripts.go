package layout

import (
	"github.com/luo-studio/go-tex/tex/parser"
	"math"
)

// One pre/post pair follows CommonMmultiscripts' combined script bounding boxes.
// Nested Positioned boxes retain SVGmmultiscripts' row then child translations.
func layoutMathJaxMultiScripts(n *parser.MultiScripts, o Options) *Box {
	base := layoutNode(n.Base, o)
	s := 1 / math.Sqrt(2)
	scales := [3]float64{1, s, s * s}
	ratio := scales[o.Style.Superscript().SizeIndex()] / scales[o.Style.SizeIndex()]
	child := func(node parser.Node, sup bool) *Box {
		if node == nil {
			return NewEmpty()
		}
		style := o.Style.Subscript()
		if sup {
			style = o.Style.Superscript()
		}
		return layoutNode(node, o.WithStyle(style))
	}
	ls, lb, rs, rb := child(n.LeftSup, true), child(n.LeftSub, false), child(n.RightSup, true), child(n.RightSub, false)
	lh, rh := math.Max(ls.Width, lb.Width)*ratio, math.Max(rs.Width, rb.Width)*ratio
	sh, sd := math.Max(ls.Height, rs.Height)*ratio, math.Max(ls.Depth, rs.Depth)*ratio
	bh, bd := math.Max(lb.Height, rb.Height)*ratio, math.Max(lb.Depth, rb.Depth)*ratio
	// getU uses the first post-subscript child (empty for a prescript-only node).
	u := .363
	if o.Style.IsCramped() {
		u = .289
	} else if o.Style.IsDisplay() {
		u = .413
	}
	u = math.Max(u, rb.Depth*ratio+.442/4)
	v := .247
	node := parser.NormalizeArgument(n.Base)
	_, large := node.(*parser.Op)
	if !isCharacterBox(node) || large {
		u = math.Max(u, base.Height-.386*ratio)
		v = math.Max(v, base.Depth+.05) // combined script bbox has rscale=1
	}
	if bh == 0 && bd == 0 {
		v = 0
	} else if sh == 0 && sd == 0 {
		// CommonMmultiscripts places sub-only pairs at getUVQ's returned u.
		subshift := math.Max(.15, rb.Height*ratio-.8*.442)
		if !isCharacterBox(node) || large {
			subshift = math.Max(subshift, base.Depth+.05*ratio)
		}
		u = -subshift
		v = 0
	} else if gap := (u - sd) - (bh - v); gap < .18 {
		v += .18 - gap
		if p := .8*.442 - (u - sd); p > 0 {
			u += p
			v -= p
		}
	}
	items := []Placed{}
	add := func(b *Box, rowX, rowY, dx float64) {
		nested := placedBox([]Placed{{b, dx, 0, ratio}}, o.Color)
		items = append(items, Placed{nested, rowX, rowY, 1})
	}
	x := 0.
	if n.LeftSup != nil || n.LeftSub != nil {
		sx, bx := 0., 0.
		if !n.LeftAligned {
			sx = lh - ls.Width*ratio
			bx = lh - lb.Width*ratio
		}
		add(ls, .05, -u, sx)
		add(lb, .05, v, bx)
		x = .05 + lh
	}
	items = append(items, Placed{base, x, 0, 1})
	x += base.Width
	if n.RightSup != nil || n.RightSub != nil {
		add(rs, x, -u, 0)
		add(rb, x, v, 0)
		x += rh + .05
	}
	out := placedBox(items, o.Color)
	out.Width = x
	return out
}
