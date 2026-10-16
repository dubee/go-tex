package render

import (
	"bytes"
	"github.com/luo-studio/go-tex/tex/layout"
	texpath "github.com/luo-studio/go-tex/tex/path"
	"math"
	"reflect"
	"testing"
)

type curveCapture struct{ points [][2]float64 }

func (s *curveCapture) MoveTo(x, y float32) {
	s.points = append(s.points, [2]float64{float64(x), float64(y)})
}
func (s *curveCapture) LineTo(x, y float32) {
	p := [2]float64{float64(x), float64(y)}
	if len(s.points) == 0 || s.points[len(s.points)-1] != p {
		s.points = append(s.points, p)
	}
}
func (s *curveCapture) ClosePath() {}

// Exact vertices captured with cairo_copy_path_flat from Sharp's pinned Cairo1.18.4.
func TestCairoCurveVertices(t *testing.T) {
	cases := []struct {
		name     string
		controls [4][2]float64
		want     [][2]float64
	}{
		{"arch", [4][2]float64{{0, 0}, {0, 10}, {10, 10}, {10, 0}}, [][2]float64{{0.0, 0.0}, {0.109375, 1.7578125}, {0.4296875, 3.28125}, {0.921875, 4.5703125}, {1.5625, 5.625}, {2.31640625, 6.4453125}, {3.1640625, 7.03125}, {4.06640625, 7.3828125}, {5.0, 7.5}, {5.9296875, 7.3828125}, {6.8359375, 7.03125}, {7.6796875, 6.4453125}, {8.4375, 5.625}, {9.07421875, 4.5703125}, {9.5703125, 3.28125}, {9.88671875, 1.7578125}, {10.0, 0.0}}},
		{"loop", [4][2]float64{{2, 2}, {12, -4}, {-8, -4}, {2, 2}}, [][2]float64{{2.0, 2.0}, {3.53515625, 0.9453125}, {4.4609375, 0.03125}, {4.85546875, -0.7421875}, {4.8125, -1.375}, {4.4140625, -1.8671875}, {3.7578125, -2.21875}, {2.921875, -2.4296875}, {2.0, -2.5}, {1.07421875, -2.4296875}, {0.2421875, -2.21875}, {-0.41796875, -1.8671875}, {-0.8125, -1.375}, {-0.859375, -0.7421875}, {-0.4609375, 0.03125}, {0.4609375, 0.9453125}, {2.0, 2.0}}},
		{"fractional", [4][2]float64{{-0.125, 0.0625}, {0.13, 9.003}, {10.001, 7.999}, {12.0625, 0.125}}, [][2]float64{{-0.125, 0.0625}, {0.02734375, 1.62109375}, {0.3828125, 2.953125}, {0.9140625, 4.05859375}, {1.59375, 4.94921875}, {2.39453125, 5.6171875}, {3.29296875, 6.08203125}, {4.26171875, 6.33984375}, {5.2890625, 6.3984375}, {6.32421875, 6.25}, {7.36328125, 5.91796875}, {8.3671875, 5.39453125}, {9.3203125, 4.69140625}, {10.1875, 3.8046875}, {10.953125, 2.74609375}, {11.5859375, 1.515625}, {12.0625, 0.125}}},
		{"flat", [4][2]float64{{1, 1}, {2, 1}, {3, 1}, {4, 1}}, [][2]float64{{1.0, 1.0}, {4.0, 1.0}}},
		{"tiny", [4][2]float64{{0, 0}, {0.01, 0.02}, {0.02, 0.01}, {0.03, 0}}, [][2]float64{{0.0, 0.0}, {0.03125, 0.0}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &curveCapture{}
			p := c.controls
			err := cairoFill(s, []texpath.Command{texpath.MoveTo(p[0][0], p[0][1]), texpath.CubicTo(p[1][0], p[1][1], p[2][0], p[2][1], p[3][0], p[3][1])}, 0, 0, 1)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.points, c.want) {
				t.Fatalf("got %v; want %v", s.points, c.want)
			}
			reference := []texpath.Command{texpath.MoveTo(c.want[0][0], c.want[0][1])}
			for _, p := range c.want[1:] {
				reference = append(reference, texpath.LineTo(p[0], p[1]))
			}
			raster := func(commands []texpath.Command) []byte {
				t.Helper()
				im, err := Image(layout.DisplayList{Width: 32, Height: 32, Items: []layout.DisplayItem{layout.PathItem{Commands: commands, Fill: true, Color: layout.Color{A: 255}}}}, WithFontSize(1), WithPadding(0), WithMathJaxViewport())
				if err != nil {
					t.Fatal(err)
				}
				return im.Pix
			}
			curve := []texpath.Command{texpath.MoveTo(p[0][0], p[0][1]), texpath.CubicTo(p[1][0], p[1][1], p[2][0], p[2][1], p[3][0], p[3][1])}
			if !bytes.Equal(raster(curve), raster(reference)) {
				t.Fatal("renderer did not use captured Cairo vertices")
			}

		})
	}
}
func TestCairoCurveLimits(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), 8388608} {
		if err := cairoFill(&curveCapture{}, []texpath.Command{texpath.MoveTo(v, 0)}, 0, 0, 1); err == nil {
			t.Fatal("invalid coordinate accepted", v)
		}
	}
	budget := 0
	if err := cairoSpline([4]cairoPoint{{0, 0}, {0, 2560}, {2560, 2560}, {2560, 0}}, func(cairoPoint) {}, &budget); err == nil {
		t.Fatal("budget not enforced")
	}
}
