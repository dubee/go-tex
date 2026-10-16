package render

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"
)

func TestCairoCoverageMasks(t *testing.T) {
	t.Run("triangle", func(t *testing.T) {
		p := &coveragePath{}
		p.MoveTo(2.125, 1.25)
		p.LineTo(28.75, 4.625)
		p.LineTo(13.125, 29.875)
		p.ClosePath()
		if !p.supported(32, 32) {
			t.Fatal("supported polygon rejected")
		}
		im, e := p.mask(32, 32)
		if e != nil {
			t.Fatal(e)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(im.Pix)); got != "f5f47f28545c9ff91156e7976847f57ae7bbc05e141c7af0b2f7f2bb1903afbf" {
			t.Fatalf("got %s", got)
		}
	})
	t.Run("diamond", func(t *testing.T) {
		p := &coveragePath{}
		p.MoveTo(16.0625, 1.25)
		p.LineTo(30.25, 16.125)
		p.LineTo(16.125, 30.75)
		p.LineTo(1.25, 16.125)
		p.ClosePath()
		if !p.supported(32, 32) {
			t.Fatal("supported polygon rejected")
		}
		im, e := p.mask(32, 32)
		if e != nil {
			t.Fatal(e)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(im.Pix)); got != "e748193566d281c5aeef9a56f90d523a6cef54f7af715db8476ea8fecfe39b05" {
			t.Fatalf("got %s", got)
		}
	})
	t.Run("concave", func(t *testing.T) {
		p := &coveragePath{}
		p.MoveTo(1.0625, 1.125)
		p.LineTo(30.25, 1.5)
		p.LineTo(14.25, 15.0625)
		p.LineTo(29.5, 30.25)
		p.LineTo(2.25, 30.125)
		p.LineTo(16.5, 15.5)
		p.ClosePath()
		if !p.supported(32, 32) {
			t.Fatal("supported polygon rejected")
		}
		im, e := p.mask(32, 32)
		if e != nil {
			t.Fatal(e)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(im.Pix)); got != "bd20b5301322bad937ef6ca3510f7094e81728a47376bd776ec35ea6f12aa014" {
			t.Fatalf("got %s", got)
		}
	})
	t.Run("clipped", func(t *testing.T) {
		p := &coveragePath{}
		p.MoveTo(-5.25, -2.125)
		p.LineTo(31.25, 4.5)
		p.LineTo(18.75, 37.5)
		p.ClosePath()
		if p.supported(32, 32) {
			t.Fatal("unverified raster path accepted")
		}
	})
	t.Run("hole", func(t *testing.T) {
		p := &coveragePath{}
		p.MoveTo(2.125, 2.25)
		p.LineTo(29.5, 2.25)
		p.LineTo(29.5, 29.75)
		p.LineTo(2.125, 29.75)
		p.ClosePath()
		p.MoveTo(8.25, 8.25)
		p.LineTo(8.25, 23.5)
		p.LineTo(23.25, 23.5)
		p.LineTo(23.25, 8.25)
		p.ClosePath()
		if p.supported(32, 32) {
			t.Fatal("unverified raster path accepted")
		}
	})
}

func TestCairoCoverageBounds(t *testing.T) {
	p := &coveragePath{}
	p.MoveTo(1, 1)
	p.LineTo(4097, 2)
	p.LineTo(3, 4)
	p.ClosePath()
	if p.supported(8192, 8192) {
		t.Fatal("large fixed-point products not bounded")
	}
	if _, err := p.mask(32, 32); err == nil {
		t.Fatal("invalid path accepted")
	}
}

func TestCairoCoverageCloseContinuation(t *testing.T) {
	// SVG's current point after Z remains the subpath start. A subsequent L
	// starts a new subpath there, whose last edge must also close on fill.
	a, b := &coveragePath{}, &coveragePath{}
	for _, p := range []*coveragePath{a, b} {
		p.MoveTo(2, 2)
		p.LineTo(20, 3)
		p.LineTo(10, 20)
		p.ClosePath()
	}
	a.LineTo(4, 25)
	a.LineTo(15, 13)
	a.ClosePath()
	b.MoveTo(2, 2)
	b.LineTo(4, 25)
	b.LineTo(15, 13)
	b.ClosePath()
	if !reflect.DeepEqual(a.edges, b.edges) {
		t.Fatal("post-close current point or closing edge lost")
	}
}
