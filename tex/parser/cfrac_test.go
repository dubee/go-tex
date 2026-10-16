package parser

import "testing"

func TestContinuedFractionAlignment(t *testing.T) {
	for _, tc := range []struct{ tex, align string }{
		{`\cfrac{a}{b}`, ""}, {`\cfrac[]{a}{b}`, ""}, {`\cfrac[l]{a}{b}`, "l"}, {`\cfrac[ r ]{a}{b}`, "r"},
	} {
		nodes, err := Parse(tc.tex)
		if err != nil {
			t.Fatal(err)
		}
		style, ok := nodes[0].(*Styling)
		if !ok || len(nodes) != 1 || len(style.Body) != 1 {
			t.Fatalf("unexpected nodes %v", nodes)
		}
		f, ok := style.Body[0].(*GenFrac)
		if !ok || !f.Continued || f.Align != tc.align {
			t.Fatalf("%s: %+v", tc.tex, f)
		}
	}
	for _, tex := range []string{`\cfrac[c]{a}{b}`, `\cfrac[lr]{a}{b}`, `\cfrac[L]{a}{b}`, `\cfrac[l{a}{b}`, `\cfrac[l]{a}`} {
		if _, err := Parse(tex); err == nil {
			t.Fatalf("invalid continued fraction accepted: %s", tex)
		}
	}
}
