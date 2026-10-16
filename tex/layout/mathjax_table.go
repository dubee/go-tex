package layout

import (
	"math"

	"github.com/luo-studio/go-tex/tex/parser"
)

// mathJaxTable follows the bounded, unframed CommonMtable/SVGmtable model.
// Rows and cells retain distinct SVG translation boundaries.
type mathJaxTable struct {
	Rows []Placed
}

func (mathJaxTable) contentMarker() {}

// Unsupported tags, external rules, custom stretches and row skips keep the
// original whole-expression pipeline until their table contracts are implemented.
func supportedMathJaxTable(a *parser.Array) bool {
	if len(a.Body) == 0 || len(a.Tags) > 0 || (a.IsCD != nil && *a.IsCD) {
		return false
	}
	if a.ArrayStretch != 0 && a.ArrayStretch != 1 && a.ArrayStretch != .5 {
		return false
	}
	for _, gap := range a.RowGaps {
		if gap != nil {
			return false
		}
	}
	for i, rules := range a.HLinesBeforeRow {
		if len(rules) > 1 || (len(rules) > 0 && (i == 0 || i == len(a.Body) || rules[0])) {
			return false
		}
	}
	lastRule := false
	for _, col := range a.Cols {
		if col.Type == parser.AlignTypeSeparator {
			if lastRule || col.Align == nil || *col.Align != "|" {
				return false
			}
			lastRule = true
		} else {
			lastRule = false
		}
	}
	if len(a.Cols) > 0 && (a.Cols[0].Type == parser.AlignTypeSeparator || lastRule) {
		return false
	}
	if a.ColSeparationType != nil {
		switch *a.ColSeparationType {
		case "align", "alignat", "gather":
		default:
			return false
		}
	}
	return true
}

func layoutMathJaxTable(a *parser.Array, opts Options) *Box {
	rowScale := 1.
	if a.ArrayStretch == .5 && opts.Style.SizeIndex() == 0 {
		rowScale = 1 / math.Sqrt(2)
	}

	n := len(a.Body)
	m := 0
	for _, row := range a.Body {
		m = max(m, len(row))
	}
	small := a.ArrayStretch == .5
	sep := ""
	if a.ColSeparationType != nil {
		sep = *a.ColSeparationType
	}
	gap := .4
	if sep == "align" || sep == "alignat" || sep == "gather" {
		gap = .3
	}
	if small {
		gap = .1
	}
	widths := make([]float64, m)
	heights := make([]float64, n)
	depths := make([]float64, n)
	cells := make([][]*Box, n)
	align := make([]byte, m)
	spaces := make([]float64, m)
	lines := make([]float64, m)
	for j := range align {
		align[j] = 'c'
		if j > 0 {
			spaces[j] = 1
		}
		if small {
			spaces[j] = 0
		}
		if sep == "align" || sep == "alignat" {
			if j%2 == 0 {
				align[j] = 'r'
				spaces[j] = 2
			} else {
				align[j] = 'l'
				spaces[j] = 0
			}
			if sep == "alignat" {
				spaces[j] = 0
			}
		}
	}
	col := 0
	for _, spec := range a.Cols {
		if spec.Type == parser.AlignTypeSeparator {
			if col < m {
				lines[col] = .07
			}
			continue
		}
		if col < m && spec.Align != nil && len(*spec.Align) > 0 {
			align[col] = (*spec.Align)[0]
		}
		col++
	}
	for i, row := range a.Body {
		if !small {
			heights[i] = .75
			depths[i] = .25
		}
		cells[i] = make([]*Box, len(row))
		for j, node := range row {
			// Parser-authored cell styles do not create a scaled mstyle wrapper in
			// MathJax; script level belongs to the table and inherited cell options.
			if style, ok := node.(*parser.Styling); ok {
				node = &parser.OrdGroup{Mode: style.Mode, Body: style.Body}
			}
			childOpts := opts
			if small && rowScale != 1 {
				childOpts = opts.WithStyle(4)
			}
			if !small && opts.Style.SizeIndex() == 0 {
				if sep == "align" || sep == "alignat" || sep == "gather" {
					childOpts.Style = 0
				} else {
					childOpts.Style = 2
				}
			}
			cells[i][j] = layoutNode(node, childOpts)
			c := cells[i][j]
			widths[j] = math.Max(widths[j], c.Width)
			heights[i] = math.Max(heights[i], c.Height*rowScale)
			depths[i] = math.Max(depths[i], c.Depth*rowScale)
		}
	}
	width := 0.
	for j, w := range widths {
		width += w * rowScale
		if j > 0 {
			width += spaces[j] + lines[j]
		}
	}
	total := 0.
	for i := range heights {
		total += heights[i] + depths[i]
		if i > 0 {
			total += gap
			if i < len(a.HLinesBeforeRow) && len(a.HLinesBeforeRow[i]) > 0 {
				total += .07
			}
		}
	}
	h := total/2 + .25
	c := mathJaxTable{}
	y := -h
	for i, row := range cells {
		y += heights[i]
		items := []Placed{}
		x := 0.
		for j, cell := range row {
			if j > 0 {
				x += spaces[j] + lines[j]
			}
			dx := 0.
			switch align[j] {
			case 'c':
				dx = (widths[j] - cell.Width) / 2
			case 'r':
				dx = widths[j] - cell.Width
			}
			items = append(items, Placed{Box: cell, X: x + dx, Scale: 1})
			x += widths[j]
		}
		c.Rows = append(c.Rows, Placed{Box: &Box{Content: Positioned{Children: items}}, Y: y, Scale: rowScale})
		y += depths[i]
		if i < n-1 {
			y += gap / 2
			if i+1 < len(a.HLinesBeforeRow) && len(a.HLinesBeforeRow[i+1]) > 0 {
				y += .07
			}
			y += gap / 2
		}
	}
	return &Box{Width: width, Height: h, Depth: total - h, Color: opts.Color, Content: c}
}

func layoutMathJaxMultline(n *parser.Multline, opts Options) *Box {
	rows := make([]*Box, len(n.Rows))
	h := 0.
	w := 35.36
	hs := make([]float64, len(rows))
	ds := make([]float64, len(rows))
	for i, node := range n.Rows {
		// AMS Multline inserts an empty ordinary atom before a leading
		// relation on subsequent rows, preserving its left relation spacing.
		if i > 0 {
			if st, ok := node.(*parser.Styling); ok && len(st.Body) == 1 {
				if group, ok := st.Body[0].(*parser.OrdGroup); ok && len(group.Body) > 0 {
					if cl, ok := nodeMathClass(group.Body[0]); ok && cl == ClassRel {
						copy := *group
						copy.Body = append([]parser.Node{&parser.OrdGroup{Mode: parser.ModeMath}}, group.Body...)
						wrap := *st
						wrap.Body = []parser.Node{&copy}
						node = &wrap
					}
				}
			}
		}
		rows[i] = layoutNode(node, opts)
		hs[i] = math.Max(.75, rows[i].Height)
		ds[i] = math.Max(.25, rows[i].Depth)
		h += hs[i] + ds[i]
		w = math.Max(w, rows[i].Width+2)
		if i > 0 {
			h += .5
		}
	}
	top := h/2 + .25
	y := -top
	c := mathJaxTable{}
	for i, row := range rows {
		y += hs[i]
		x := (w - row.Width) / 2
		if i == 0 {
			x = 1
		} else if i == len(rows)-1 {
			x = w - 1 - row.Width
		}
		c.Rows = append(c.Rows, Placed{Box: &Box{Content: Positioned{Children: []Placed{{Box: row, X: x, Scale: 1}}}}, Y: y, Scale: 1})
		y += ds[i] + .5
	}
	return &Box{Width: w, Height: top, Depth: h - top, Color: opts.Color, Content: c}
}

// CommonMunder/Mover/Munderover with non-accent sum/product bases. Spacing is
// MathJax's font parameter set, independently of the fallback TeX metrics.
func layoutMathJaxLimits(base, sup, sub parser.Node, opts Options) *Box {
	b := layoutNode(base, opts)
	s := 1 / math.Sqrt(2)
	width := b.Width
	var over, under *Box
	if sup != nil {
		over = layoutNode(sup, opts.WithStyle(opts.Style.Superscript()))
		width = math.Max(width, over.Width*s)
	}
	if sub != nil {
		under = layoutNode(sub, opts.WithStyle(opts.Style.Subscript()))
		width = math.Max(width, under.Width*s)
	}
	items := []Placed{{Box: b, X: (width - b.Width) / 2, Scale: 1}}
	h, d := b.Height, b.Depth
	if over != nil {
		u := b.Height + math.Max(.111, .2-math.Max(0, over.Depth*s)) + over.Depth*s
		items = append(items, Placed{Box: over, X: (width - over.Width*s) / 2, Y: -u, Scale: s})
		h = math.Max(h, u+over.Height*s) + .1
	}
	if under != nil {
		v := b.Depth + math.Max(.167, .6-under.Height*s) + under.Height*s
		items = append(items, Placed{Box: under, X: (width - under.Width*s) / 2, Y: v, Scale: s})
		d = math.Max(d, v+under.Depth*s) + .1
	}
	return &Box{Width: width, Height: h, Depth: d, Color: opts.Color, Content: Positioned{Children: items}}
}
