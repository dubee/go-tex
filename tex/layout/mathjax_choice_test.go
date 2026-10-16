package layout

import (
	"github.com/luo-studio/go-tex/tex/parser"
	"testing"
)

func TestMathJaxChoiceAndPhantomScope(t *testing.T) {
	for _, tc := range []struct {
		tex  string
		want bool
	}{
		{`\mathchoice{A}{B}{C}{D}`, true},
		{`\frac{\mathchoice{A}{B}{C}{D}}{x}`, true},
		{`\mathchoice{A}{B}{C}{\alpha}`, true},
		{`\mathchoice{A}{\text{B}}{C}{D}`, true},
		{`\phantom{x}+y`, true},
		{`\phantom{\frac{a+b}{c+d}}+x`, true},
		{`\phantom{\alpha}+x`, true},
		{`\phantom{x^2}+x`, true}, {`\phantom{x^{\alpha}}+x`, true},
		{`\mathrm{\phantom{ab}+c}`, true},
		{`\mathchoice{A}{B}{C}{\text{中文}}`, false},
		{`\phantom{\text{中文}}+x`, false},
	} {
		n, err := parser.Parse(tc.tex)
		if err != nil {
			t.Fatal(err)
		}
		if got := SupportsMathJaxGlyphs(n); got != tc.want {
			t.Fatalf("%s supported=%v, want %v", tc.tex, got, tc.want)
		}
	}
}
