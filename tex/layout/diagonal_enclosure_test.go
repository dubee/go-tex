package layout

import (
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
	"math"
	"testing"
)

// Values are from pinned MathJax 3.2.2 SVG lines, in em and y-down coordinates.
// Rules isolate enclosure geometry from the independently open glyph metrics.
func TestDiagonalEnclosureReferenceGeometry(t *testing.T) {
	for _, tc := range []struct {
		tex        string
		depth      bool
		directions []bool
	}{
		{`\cancel{\rule{1em}{.5em}}`, false, []bool{false}},
		{`\bcancel{\rule{1em}{.5em}}`, false, []bool{true}},
		{`\xcancel{\rule{1em}{.5em}}`, false, []bool{false, true}},
		{`\cancel{\rule[-.3em]{.7em}{.8em}}`, true, []bool{false}},
		{`\bcancel{\rule[-.3em]{.7em}{.8em}}`, true, []bool{true}},
		{`\xcancel{\rule[-.3em]{.7em}{.8em}}`, true, []bool{false, true}},
		{`\enclose{updiagonalstrike}{\rule{1em}{.5em}}`, false, []bool{false}},
		{`\enclose{updiagonalstrike,updiagonalstrike}{\rule{1em}{.5em}}`, false, []bool{false}},
	} {
		t.Run(tc.tex, func(t *testing.T) {
			nodes, err := parser.Parse(tc.tex)
			if err != nil {
				t.Fatal(err)
			}
			box := Layout(nodes, DefaultOptions())
			dl := ToNominalDisplayList(box)
			width, depth, right, bottom := 1.534, .267, 1.5005, .2335
			if tc.depth {
				width, depth, right, bottom = 1.234, .567, 1.2005, .5335
			}
			eq := func(got, want float64) {
				t.Helper()
				if math.Abs(got-want) > 1e-14 {
					t.Fatalf("%.17g != %.17g", got, want)
				}
			}
			eq(dl.Width, width)
			eq(dl.Height, .767)
			eq(dl.Depth, depth)
			strokes := []PathItem{}
			for _, item := range dl.Items {
				if p, ok := item.(PathItem); ok {
					strokes = append(strokes, p)
				}
			}
			if len(strokes) != len(tc.directions) {
				t.Fatalf("%d strokes", len(strokes))
			}
			for i, p := range strokes {
				if p.Fill || len(p.Commands) != 2 || p.Commands[0].Kind != path.KindMoveTo || p.Commands[1].Kind != path.KindLineTo {
					t.Fatal(p)
				}
				eq(p.StrokeWidth, .067)
				eq(p.X, 0)
				// Display-list origins are top-down; commands remain baseline-relative.
				eq(p.Y, dl.Height)
				eq(p.Commands[0].X, .0335)
				eq(p.Commands[1].X, right)
				a, b := bottom, -.7335
				if tc.directions[i] {
					a, b = b, a
				}
				eq(p.Commands[0].Y, a)
				eq(p.Commands[1].Y, b)
			}
		})
	}
}
