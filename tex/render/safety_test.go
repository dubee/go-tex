package render

import (
	"context"
	"github.com/luo-studio/go-tex/tex/layout"
	"math"
	"testing"
)

func TestMissingGlyphNotSilentlyOmitted(t *testing.T) {
	dl := layout.DisplayList{Width: 1, Height: 1, Items: []layout.DisplayItem{layout.GlyphPath{Font: "Main-Regular", CharCode: '😀', Scale: 1, Color: layout.Black}}}
	if _, e := PNG(dl); e == nil {
		t.Fatal("missing glyph accepted")
	}
	dl.Items[0] = layout.GlyphPath{Font: "missing", CharCode: 'x', Scale: 1, Color: layout.Black}
	if _, e := PNG(dl); e == nil {
		t.Fatal("missing font accepted")
	}
}
func TestRenderBoundsAndCancellation(t *testing.T) {
	dl := layout.DisplayList{Width: 1, Height: 1}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PNG(dl, WithContext(ctx)); e != context.Canceled {
		t.Fatal(e)
	}
	for _, o := range []Option{WithScale(math.NaN()), WithScale(0), WithPadding(-1), WithFontSize(1e10)} {
		if _, e := PNG(dl, o); e == nil {
			t.Fatal("unsafe options accepted")
		}
	}
}
