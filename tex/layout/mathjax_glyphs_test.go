package layout

import (
	"github.com/luo-studio/go-tex/tex/mathstyle"
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/path"
	"math"
	"testing"
)

func TestMathJaxGlyphScope(t *testing.T) {
	for _, tc := range []struct {
		tex  string
		want bool
	}{{`\frac{a+b}{c+d}`, true}, {`\frac{1}{\frac{2}{3}}`, true}, {`\mathrm{abc123}`, true}, {`\color{red}{x}`, true}, {`\overbrace{a+b}^{\text{two terms}}`, true}, {`\overbracket{x}`, false}, {`\overbrace{x}_y`, false}, {`\underbrace{x}^y`, false}, {`x^2`, true}, {`123^2`, false}, {`x^{123^2}`, false}, {`1x^2`, true}, {`{ab}^2`, false}, {`x^{\alpha}`, true}, {`\alpha`, true}, {`\mathbf{x}`, true}, {`\text{net profit}`, true}, {`\binom{n}{k}`, true}, {`{n\brack k}`, false}, {`\unicode{x2605}`, true}, {`\unicode{x2264}`, true}, {`\mathit{\unicode{65}}`, true}, {`\unicode{x03B1}`, false}, {`\unicode{x1F600}`, false}, {`\text{中文}`, false}, {`\text{a\quad b}`, false}, {`\not\equiv`, true}, {`\not{ab}`, false}} {
		n, e := parser.Parse(tc.tex)
		if e != nil {
			t.Fatal(e)
		}
		if got := SupportsMathJaxGlyphs(n); got != tc.want {
			t.Fatalf("%s: %v", tc.tex, got)
		}
	}
}
func TestMathJaxFractionGlyphs(t *testing.T) {
	n, e := parser.Parse(`\frac{a+b}{c+d}`)
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
		d := ToNominalDisplayList(Layout(n, o))
		glyphs := 0
		for _, item := range d.Items {
			if p, ok := item.(PathItem); ok {
				glyphs++
				if !p.Fill || len(p.Commands) == 0 || p.Commands[len(p.Commands)-1].Kind != path.KindClose {
					t.Fatal("glyph contours not closed")
				}
			}
		}
		if glyphs != 6 {
			t.Fatalf("%d glyphs", glyphs)
		}
		// MathJax SVG viewBox dimensions excluding 0.15em outer padding.
		wantW, wantH := 2.6204, 2.138
		if inline {
			wantW, wantH = 1.6675, 1.2917
		}
		if math.Abs(math.Round(d.Width*10000)/10000-wantW) > 1e-9 || math.Abs(math.Round((d.Height+d.Depth)*10000)/10000-wantH) > 1e-9 {
			t.Fatalf("inline=%v width %.9g height %.9g", inline, d.Width, d.Height+d.Depth)
		}
	}
}
