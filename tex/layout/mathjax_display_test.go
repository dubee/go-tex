package layout

import (
	"github.com/luo-studio/go-tex/tex/mathstyle"
	"github.com/luo-studio/go-tex/tex/parser"
	"math"
	"reflect"
	"testing"
)

func TestMathJaxSerializedFractionPlacement(t *testing.T) {
	nodes, e := parser.Parse(`\frac{a+b}{c+d}`)
	if e != nil {
		t.Fatal(e)
	}
	for _, inline := range []bool{false, true} {
		o := DefaultOptions()
		o.MathJaxGlyphs = true
		o.MathJaxFractionMetrics = true
		o.FractionRulePadding = .1
		o.FractionScriptScale = 1 / math.Sqrt(2)
		if inline {
			o.Style = mathstyle.Text
		}
		b := Layout(nodes, o)
		d := ToMathJaxGlyphDisplayList(b)
		// Translation and scale tokens from fresh unchanged MathJax3.2.2 SVG.
		xs := []float64{.22, .22 + .7512, .22 + 1.7514, .2225, .2225 + .6552, .2225 + 1.6554}
		ys := []float64{-.676, -.676, -.676, .686, .686, .686}
		if inline {
			xs = []float64{.22, .22 + .529*.707, .22 + 1.307*.707, .2218, .2218 + .433*.707, .2218 + 1.211*.707}
			ys = []float64{-.398, -.398, -.398, .345, .345, .345}
		}
		n := 0
		for _, item := range d.Items {
			if p, ok := item.(PathItem); ok {
				if n >= 6 || math.Abs(p.X-xs[n]) > 1e-14 || math.Abs(p.Y-d.Height-ys[n]) > 1e-14 {
					t.Fatalf("inline=%v glyph%d at %.17g %.17g", inline, n, p.X, p.Y-d.Height)
				}
				n++
			}
		}
		if n != 6 {
			t.Fatalf("%d glyphs", n)
		}
		if d.Width != b.Width || d.Height != b.Height || d.Depth != b.Depth {
			t.Fatal("serialization changed layout bounds")
		}
	}
}

func TestMathJaxSerializationFallback(t *testing.T) {
	n, e := parser.Parse(`x^2`)
	if e != nil {
		t.Fatal(e)
	}
	b := Layout(n, DefaultOptions())
	if !reflect.DeepEqual(ToNominalDisplayList(b), ToMathJaxGlyphDisplayList(b)) {
		t.Fatal("unsupported tree did not retain nominal path")
	}
}

func TestMathJaxFixedDecimal(t *testing.T) {
	// Original CommonOutputJax.fixed inputs; retain binary-to-decimal semantics.
	for _, c := range []struct {
		v    float64
		n    int
		want float64
	}{
		{1.25, 1, 1.3}, {-1.25, 1, -1.3}, {2.55, 1, 2.5}, {-2.55, 1, -2.5}, {.00059, 3, 0}, {.0006, 3, .001}, {1 / math.Sqrt(2), 3, .707},
	} {
		if got := svgFixed(c.v, c.n); got != c.want {
			t.Fatalf("%.17g,%d=%g want %g", c.v, c.n, got, c.want)
		}
	}
}
