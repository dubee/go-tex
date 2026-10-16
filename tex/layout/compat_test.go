package layout

import (
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
	"math"
	"testing"
)

func TestClippedStretchyPolygonsAreClosedAndBounded(t *testing.T) {
	for _, name := range []string{`\overbrace`, `\underbrace`, `\xrightarrow`, `\xleftrightarrow`} {
		cmds, _, ok := katexStretchyPath(name, 3)
		if !ok {
			t.Fatal(name)
		}
		closed := true
		for _, c := range cmds {
			switch c.Kind {
			case path.KindMoveTo:
				if !closed {
					t.Fatalf("%s unclosed subpath", name)
				}
				closed = false
			case path.KindClose:
				closed = true
			}
			if c.Kind != path.KindClose && (math.IsNaN(c.X) || math.Abs(c.X) > 3.001) {
				t.Fatalf("%s out-of-bounds path: %+v", name, c)
			}
		}
		if !closed {
			t.Fatalf("%s unclosed final path", name)
		}
	}
}
func TestPrescriptsPositionAndTextSpacing(t *testing.T) {
	n, e := parser.Parse(`\prescript{14}{6}{C}`)
	if e != nil {
		t.Fatal(e)
	}
	dl := ToDisplayList(Layout(n, DefaultOptions()))
	chars := map[rune]GlyphPath{}
	for _, i := range dl.Items {
		if g, ok := i.(GlyphPath); ok {
			chars[g.CharCode] = g
		}
	}
	if chars['1'].X >= chars['C'].X || chars['6'].X >= chars['C'].X || chars['1'].Y >= chars['C'].Y || chars['6'].Y <= chars['C'].Y {
		t.Fatal(chars)
	}
	n, e = parser.Parse(`\text{net profit}`)
	if e != nil {
		t.Fatal(e)
	}
	b := Layout(n, DefaultOptions())
	if b.Width < 4 || b.Width > 4.1 {
		t.Fatalf("text spacing: %.4f", b.Width)
	}
}

func TestLabeledArrowInsetsAndVerticalClearance(t *testing.T) {
	for _, label := range []string{`\xrightarrow`, `\xleftarrow`, `\xleftrightarrow`} {
		n := &parser.XArrow{Label: label, Body: &parser.MathOrd{Mode: parser.ModeMath, Text: "x"}, Below: &parser.MathOrd{Mode: parser.ModeMath, Text: "y"}}
		box := layoutLabeledArrow(n, DefaultOptions())
		items := box.Content.(Positioned).Children
		if len(items) != 3 {
			t.Fatal("missing arrow or label")
		}
		arrow, above, below := items[0], items[1], items[2]
		if above.Y+above.Box.Depth*above.Scale >= arrow.Y-arrow.Box.Height {
			t.Fatalf("%s: upper annotation touches arrow", label)
		}
		if below.Y-below.Box.Height*below.Scale <= arrow.Y+arrow.Box.Depth {
			t.Fatalf("%s: lower annotation touches arrow", label)
		}
		center := above.X + above.Box.Width*above.Scale/2
		if label == `\xrightarrow` && center >= box.Width/2 || label == `\xleftarrow` && center <= box.Width/2 || label == `\xleftrightarrow` && math.Abs(center-box.Width/2) > 1e-10 {
			t.Fatalf("%s: incorrect label inset", label)
		}
	}
}
