package render

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/luo-studio/go-tex/tex/layout"
	"math"
	"testing"
)

// Exact alpha masks from the Cairo1.18.4 library bundled with pinned Sharp.
func TestCairoRectangleCoverage(t *testing.T) {
	for _, tc := range []struct {
		name string
		box  [4]float64
		hash string
	}{
		{"integer", [4]float64{2, 3, 18, 4}, "66113a81dbce19bf448ad7fde8a109f28f0f7dc45a20623676c7203fab08a8af"},
		{"fractional", [4]float64{2.125, 3.25, 18.5, 4.125}, "95cc966f63fba8ccb5d9634ddd09649aa3b563b8a18651ebe912ecf82a99f175"},
		{"thin", [4]float64{2.25, 3.125, 0.125, 10.5}, "2cabaf51e0ce3993b483986c97ccf07760efc22eb88584b2d0b733617f4517f2"},
		{"short", [4]float64{2.25, 3.125, 10.5, 0.125}, "c6ef0adc9ef576b0b1909efc3b90dc9d488f44e210333924689def65f2ff0406"},
		{"tiny", [4]float64{2.25, 3.125, 0.125, 0.25}, "2c68d8c181c3a4c12851b47a3ef7d2ef622ff59fbcc64ea2135c99528df5b951"},
		{"edge", [4]float64{2.5, 3.125, 18.50390625, 4.75}, "d2326dd05f745eb8fdd5efd2e4bdeed437f62592cb7c3931440fd50a15fd3a34"},
		{"half-ties", [4]float64{2.001953125, 3.005859375, 18.25, 4.25}, "858ab81d6ccbf53e65527067b041d8c20ad64c4d530367cea7bafc2976d725ad"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := cairoRectangleMask(32, 32, tc.box[0], tc.box[1], tc.box[2], tc.box[3])
			if m == nil {
				t.Fatal("valid box rejected")
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(m.Pix)); got != tc.hash {
				t.Fatalf("got %s want %s", got, tc.hash)
			}
			// Invoke the production renderer with a viewBox that is exactly32x32.
			dl := layout.DisplayList{Width: 32, Height: 32, Items: []layout.DisplayItem{layout.Rect{X: tc.box[0], Y: tc.box[1], Width: tc.box[2], Height: tc.box[3], Color: layout.Color{A: 255}}}}
			im, err := Image(dl, WithMathJaxViewport(), WithFontSize(1), WithPadding(0))
			if err != nil {
				t.Fatal(err)
			}
			var alpha []byte
			for j := 3; j < len(im.Pix); j += 4 {
				alpha = append(alpha, im.Pix[j])
			}
			if !bytes.Equal(alpha, m.Pix) {
				t.Fatal("rectangle coverage not used by renderer")
			}
		})
	}
}
func TestCairoRectangleFallback(t *testing.T) {
	for _, box := range [][4]float64{{-1, 1, 4, 4}, {1, 1, 32, 32}, {1, 1, 0, 4}, {math.NaN(), 1, 4, 4}, {1, 1, math.Inf(1), 4}} {
		if cairoRectangleMask(32, 32, box[0], box[1], box[2], box[3]) != nil {
			t.Fatalf("unsupported box accepted: %v", box)
		}
	}
}
