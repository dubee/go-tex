package render

import (
	texpath "github.com/luo-studio/go-tex/tex/path"
	"reflect"
	"testing"
)

// Vertices captured from the pinned Cairo1.18.4 cairo_copy_path_flat.
func TestCairoAuthoredLineCanonicalization(t *testing.T) {
	for _, tc := range []struct {
		name        string
		input, want [][2]float64
	}{
		{"vertical", [][2]float64{{2, 2}, {2, 4.5}, {2, 8}, {8, 9}}, [][2]float64{{2.0, 2.0}, {2.0, 8.0}, {8.0, 9.0}}},
		{"diagonal", [][2]float64{{2, 2}, {4, 4}, {6, 6}, {6, 9}}, [][2]float64{{2.0, 2.0}, {6.0, 6.0}, {6.0, 9.0}}},
		{"reverse", [][2]float64{{2, 2}, {4, 4}, {3, 3}, {6, 9}}, [][2]float64{{2.0, 2.0}, {4.0, 4.0}, {3.0, 3.0}, {6.0, 9.0}}},
		{"duplicate", [][2]float64{{2, 2}, {2, 2}, {4, 4}, {4, 4}, {6, 6}, {6, 9}}, [][2]float64{{2.0, 2.0}, {6.0, 6.0}, {6.0, 9.0}}},
		{"turn", [][2]float64{{2, 2}, {4, 4}, {6, 4}, {6, 9}}, [][2]float64{{2.0, 2.0}, {4.0, 4.0}, {6.0, 4.0}, {6.0, 9.0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := []texpath.Command{texpath.MoveTo(tc.input[0][0], tc.input[0][1])}
			for _, p := range tc.input[1:] {
				cmd = append(cmd, texpath.LineTo(p[0], p[1]))
			}
			sink := &curveCapture{}
			if e := cairoFill(sink, cmd, 0, 0, 1); e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(sink.points, tc.want) {
				t.Fatalf("got %v want %v", sink.points, tc.want)
			}
		})
	}
}
