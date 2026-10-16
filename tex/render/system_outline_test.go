package render

import (
	p "github.com/luo-studio/go-tex/tex/path"
	"math"
	"reflect"
	"testing"
)

// Captured from pinned Cairo's fixed path via its Quartz quadratic callback.
// Unlike SVG conversion, the first cubic control uses a quantized start.
func TestSystemOutlineFixedCoordinates(t *testing.T) {
	in := []p.Command{p.MoveTo(.0137, -.0241), p.QuadTo(.2211, -.3377, .519, .006), p.CubicTo(.61, .58, .72, -.13, .81, .09), p.LineTo(.125, .07), p.Close()}
	got, err := cairoSystemOutline(in, 88.4)
	if err != nil {
		t.Fatal(err)
	}
	want := []p.Command{p.MoveTo(1.2109375, -2.12890625), p.CubicTo(13.43359375, -20.61328125, 28.32421875, -19.7265625, 45.87890625, .53125), p.CubicTo(53.92578125, 51.2734375, 63.6484375, -11.4921875, 71.60546875, 7.95703125), p.LineTo(11.05078125, 6.1875), p.Close()}
	// Close records its current point for the next contour, not another segment.
	want[4].X, want[4].Y = want[0].X, want[0].Y
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fixed path differs: %#v", got)
	}
	if in[0].X != .0137 {
		t.Fatal("mutated input")
	}
	for _, scale := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, e := cairoSystemOutline(in, scale); e == nil {
			t.Fatal("accepted invalid scale")
		}
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), 1e9} {
		if _, e := cairoSystemOutline([]p.Command{p.MoveTo(v, 0)}, 1); e == nil {
			t.Fatal("accepted invalid coordinate")
		}
	}
}
