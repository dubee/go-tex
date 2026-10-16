package layout

import (
	"github.com/luo-studio/go-tex/tex/parser"
	"reflect"
	"testing"
)

func TestLibrsvgParsedNumbers(t *testing.T) {
	// SVG tokens captured from unchanged MathJax output; librsvg's CSS parser
	// represents these numbers as f32, unlike its f64 path-data parser.
	c := svgCoordinates{parsed: true}
	for _, v := range []struct{ input, want float64 }{{.7512, .7512000122070313}, {1.7514, 1.7514000244140625}, {.2225, .2225}} {
		if got := c.translation(v.input); got != v.want {
			t.Fatalf("%g -> %.17g want %.17g", v.input, got, v.want)
		}
	}
	if got := c.scale(.707); got != .7070000171661377 {
		t.Fatal(got)
	}
	n, e := parser.Parse(`\frac{a+b}{c+d}`)
	if e != nil {
		t.Fatal(e)
	}
	o := DefaultOptions()
	o.MathJaxGlyphs = true
	o.MathJaxFractionMetrics = true
	o.FractionRulePadding = .1
	b := Layout(n, o)
	raw, parsed := ToMathJaxGlyphDisplayList(b), ToLibrsvgGlyphDisplayList(b)
	if raw.Width != parsed.Width || raw.Height != parsed.Height || raw.Depth != parsed.Depth {
		t.Fatal("CSS parsing changed nominal layout")
	}
	if reflect.DeepEqual(raw.Items, parsed.Items) {
		t.Fatal("parsed coordinates were not applied")
	}
}
func TestLibrsvgUnsupportedFallback(t *testing.T) {
	n, e := parser.Parse(`x^2`)
	if e != nil {
		t.Fatal(e)
	}
	b := Layout(n, DefaultOptions())
	if !reflect.DeepEqual(ToLibrsvgGlyphDisplayList(b), ToNominalDisplayList(b)) {
		t.Fatal("unsupported expression fallback changed")
	}
}
