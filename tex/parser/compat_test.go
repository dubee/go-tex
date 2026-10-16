package parser

import (
	"context"
	"strings"
	"testing"
)

func TestCompatibilityParser(t *testing.T) {
	for _, s := range []string{`\xrightarrow[below]{above}`, `\prescript{14}{6}{C}`, `\sideset{_a^b}{_c^d}\sum`, `\cancelto{0}{x}`, `\unicode{x2605}`, `\bbox[5px,border:2px solid red]{x+1}`, `\enclose{circle}{x}`, `\begin{multline}a+b\\=c\end{multline}`, `\newenvironment{foo}{\begin{matrix}}{\end{matrix}}\begin{foo}a&b\end{foo}`, `\newenvironment{foo}[1]{\left(#1+}{\right)}\begin{foo}{2}x\end{foo}`} {
		if _, e := Parse(s); e != nil {
			t.Errorf("%s: %v", s, e)
		}
	}
}
func TestRejectPartialExpansionAndResourceExhaustion(t *testing.T) {
	for _, s := range []string{`x\newcommand{\foo}{\foo}\foo`, `x\newcommand{\foo}[1]{#1}\foo{`, `\newenvironment{foo}{x}{y}\begin{foo}z`, strings.Repeat("{", 300) + "x" + strings.Repeat("}", 300), `\begin{matrix}` + strings.Repeat("x&", 200) + "x" + strings.Repeat(`\\x`, 200) + `\end{matrix}`, `\unicode{xD800}`, `\unicode{1114112}`, `\bbox[-2px]{x}`} {
		if _, e := Parse(s); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ParseContext(ctx, "x"); e != context.Canceled {
		t.Fatal(e)
	}
}
