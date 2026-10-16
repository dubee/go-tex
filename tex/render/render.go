// Package render rasterises embedded font outlines and layout paths in pure Go.
// No SVG interpreter, native library, or external executable is used.
package render

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"sync"

	"github.com/luo-studio/go-tex/tex/fonts"
	"github.com/luo-studio/go-tex/tex/layout"
	texpath "github.com/luo-studio/go-tex/tex/path"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

type config struct {
	em, padding, stroke, scale float64
	ctx                        context.Context
	mathJaxViewport            bool
}
type Option func(*config)

func WithFontSize(v float64) Option          { return func(c *config) { c.em = v } }
func WithPadding(v float64) Option           { return func(c *config) { c.padding = v } }
func WithStrokeWidth(v float64) Option       { return func(c *config) { c.stroke = v } }
func WithScale(v float64) Option             { return func(c *config) { c.scale = v } }
func WithContext(ctx context.Context) Option { return func(c *config) { c.ctx = ctx } }

// WithMathJaxViewport uses the pinned SVG root's one-decimal
// SVG-unit precision (1000 units/em). Sharp rounds the resulting pixel size.
// It does not replace glyph metrics or the vector rasterizer with MathJax's.
func WithMathJaxViewport() Option { return func(c *config) { c.mathJaxViewport = true } }
func PNG(dl layout.DisplayList, opts ...Option) ([]byte, error) {
	var b bytes.Buffer
	err := PNGTo(&b, dl, opts...)
	return b.Bytes(), err
}
func PNGTo(w io.Writer, dl layout.DisplayList, opts ...Option) error {
	img, err := Image(dl, opts...)
	if err != nil {
		return err
	}
	return png.Encode(w, img)
}

var fontCache sync.Map

func loadFont(name string) (*sfnt.Font, error) {
	if f, ok := fontCache.Load(name); ok {
		return f.(*sfnt.Font), nil
	}
	data, err := fonts.TTF(name)
	if err != nil {
		return nil, err
	}
	f, err := sfnt.Parse(data)
	if err != nil {
		return nil, err
	}
	fontCache.Store(name, f)
	return f, nil
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// Image bounds allocations and rejects missing glyphs instead of silently omitting them.
func Image(dl layout.DisplayList, opts ...Option) (*image.RGBA, error) {
	c := config{em: 40, padding: 10, stroke: 1.5, scale: 1, ctx: context.Background()}
	for _, o := range opts {
		o(&c)
	}
	for _, v := range []float64{c.em, c.padding, c.stroke, c.scale, dl.Width, dl.Height, dl.Depth} {
		if !finite(v) {
			return nil, fmt.Errorf("render: nonfinite dimensions")
		}
	}
	if c.em <= 0 || c.scale <= 0 || c.padding < 0 || c.stroke <= 0 || c.ctx == nil {
		return nil, fmt.Errorf("render: invalid options")
	}
	em, pad := c.em*c.scale, c.padding*c.scale
	width, height := math.Ceil(dl.Width*em+2*pad), math.Ceil((dl.Height+dl.Depth)*em+2*pad)
	padY := pad
	if c.mathJaxViewport {
		// MathJax serializes the viewBox in SVG units (1000/em), then
		// equation.cjs converts the padded box to CSS pixels. Keep that
		// arithmetic order: half-pixel cases depend on float64 rounding.
		w := math.Round(math.Max(dl.Width, .016)*10000) / 10
		h := math.Round(math.Max(dl.Height+dl.Depth, .016)*10000) / 10
		top := math.Round(dl.Height*10000) / 10
		paddingUnits := c.padding / c.em * 1000
		boxW, boxH := (w+2*paddingUnits)/1000, (h+2*paddingUnits)/1000
		// librsvg CssLength parses SVG width/height through finite_f32,
		// then promotes to f64 before libvips applies density and rounds.
		width = math.Floor(float64(float32(boxW*c.em))*c.scale + .5)
		height = math.Floor(float64(float32(boxH*c.em))*c.scale + .5)
		// libvips 8.17.3 passes the rounded image size to librsvg's
		// render_document. SVG's default xMidYMid meet fits the viewBox
		// uniformly and centers any fractional slack in that viewport.
		// ViewBox uses the same CSS number parser, independently of the
		// physical width/height above. Parse after adding SVG padding.
		boxW = float64(float32(w+2*paddingUnits)) / 1000
		boxH = float64(float32(h+2*paddingUnits)) / 1000
		originX := float64(float32(-paddingUnits)) / 1000
		originY := float64(float32(-top-paddingUnits)) / 1000
		em = math.Min(width/boxW, height/boxH)
		pad = (width-boxW*em)/2 - originX*em
		padY = (height-boxH*em)/2 + (-originY-dl.Height)*em
	}
	if width < 1 || height < 1 || width > 32768 || height > 32768 || width*height > 64000000 || len(dl.Items) > 100000 {
		return nil, fmt.Errorf("render: dimensions or item count exceed limit")
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	if width*height*float64(len(dl.Items)) > 2e9 {
		return nil, fmt.Errorf("render: raster work budget exceeded")
	}
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	r := vector.NewRasterizer(img.Bounds().Dx(), img.Bounds().Dy())
	paint := func(col layout.Color) {
		r.DrawOp = draw.Over
		r.Draw(img, img.Bounds(), image.NewUniform(color.NRGBA{R: col.R, G: col.G, B: col.B, A: col.A}), image.Point{})
	}
	reset := func() { r.Reset(img.Bounds().Dx(), img.Bounds().Dy()) }
	rectangle := func(x, y, w, h float64, col layout.Color) {
		if w <= 0 || h <= 0 {
			return
		}
		if c.mathJaxViewport {
			if mask := cairoRectangleMask(img.Bounds().Dx(), img.Bounds().Dy(), x, y, w, h); mask != nil {
				draw.DrawMask(img, img.Bounds(), image.NewUniform(color.NRGBA{R: col.R, G: col.G, B: col.B, A: col.A}), image.Point{}, mask, image.Point{}, draw.Over)
				return
			}
		}
		reset()
		r.MoveTo(float32(x), float32(y))
		r.LineTo(float32(x+w), float32(y))
		r.LineTo(float32(x+w), float32(y+h))
		r.LineTo(float32(x), float32(y+h))
		r.ClosePath()
		paint(col)
	}
	var buf sfnt.Buffer
	for _, item := range dl.Items {
		if err := c.ctx.Err(); err != nil {
			return nil, err
		}
		switch v := item.(type) {
		case layout.GlyphPath:
			f, err := loadFont(v.Font)
			if err != nil {
				return nil, err
			}
			idx, err := f.GlyphIndex(&buf, v.CharCode)
			if err != nil {
				return nil, err
			}
			if idx == 0 {
				return nil, fmt.Errorf("render: missing glyph U+%04X in %s", v.CharCode, v.Font)
			}
			size := em * v.Scale
			if !finite(size) || size <= 0 || size > 32768 {
				return nil, fmt.Errorf("render: invalid glyph size")
			}
			segs, err := f.LoadGlyph(&buf, idx, fixed.Int26_6(math.Round(size*64)), nil)
			if err != nil {
				return nil, err
			}
			x, y := float32(v.X*em+pad), float32(v.Y*em+padY)
			reset()
			point := func(p fixed.Point26_6) (float32, float32) { return x + float32(p.X)/64, y + float32(p.Y)/64 }
			for _, s := range segs {
				switch s.Op {
				case sfnt.SegmentOpMoveTo:
					r.ClosePath()
					a, b := point(s.Args[0])
					r.MoveTo(a, b)
				case sfnt.SegmentOpLineTo:
					a, b := point(s.Args[0])
					r.LineTo(a, b)
				case sfnt.SegmentOpQuadTo:
					a, b := point(s.Args[0])
					d, e := point(s.Args[1])
					r.QuadTo(a, b, d, e)
				case sfnt.SegmentOpCubeTo:
					a, b := point(s.Args[0])
					d, e := point(s.Args[1])
					g, h := point(s.Args[2])
					r.CubeTo(a, b, d, e, g, h)
				}
			}
			r.ClosePath()
			paint(v.Color)
		case layout.Rect:
			rectangle(v.X*em+pad, v.Y*em+padY, v.Width*em, v.Height*em, v.Color)
		case layout.Line:
			x, y, t, w := v.X*em+pad, v.Y*em+padY, v.Thickness*em, v.Width*em
			if v.Dashed && t > 0 {
				for off := 0.; off < w; off += 6 * t {
					rectangle(x+off, y-t/2, math.Min(3*t, w-off), t, v.Color)
				}
			} else {
				rectangle(x, y-t/2, w, t, v.Color)
			}
		case layout.PathItem:
			reset()
			x, y := v.X*em+pad, v.Y*em+padY
			if v.EllipseStroke && c.mathJaxViewport {
				coverage := &coveragePath{}
				if err := cairoEllipseStroke(coverage, v.Commands, x, y, em, v.StrokeWidth*em); err != nil {
					return nil, err
				}
				mask, err := coverage.mask(img.Bounds().Dx(), img.Bounds().Dy())
				if err != nil {
					return nil, err
				}
				cairoOver(img, mask, v.Color)
				continue
			}
			if v.RectStroke && c.mathJaxViewport {
				mask := cairoFrameMask(img.Bounds().Dx(), img.Bounds().Dy(), x, y, v.Commands[2].X*em, v.Commands[2].Y*em, v.StrokeWidth*em)
				if mask != nil {
					cairoOver(img, mask, v.Color)
					continue
				}
			}
			if v.Butt && c.mathJaxViewport {
				coverage := &coveragePath{}
				if err := cairoButtLine(coverage, v.Commands, x, y, em, v.StrokeWidth*em); err != nil {
					return nil, err
				}
				if !coverage.supported(img.Bounds().Dx(), img.Bounds().Dy()) {
					if err := cairoButtLine(r, v.Commands, x, y, em, v.StrokeWidth*em); err != nil {
						return nil, err
					}
					paint(v.Color)
					continue
				}
				mask, err := coverage.mask(img.Bounds().Dx(), img.Bounds().Dy())
				if err != nil {
					return nil, err
				}
				cairoOver(img, mask, v.Color)
				continue
			}
			if v.Fill {
				if v.SystemOutline && c.mathJaxViewport {
					commands, err := cairoSystemOutline(v.Commands, 1000)
					if err != nil {
						return nil, err
					}
					coverage := &coveragePath{}
					// librsvg constructs the text path on an identity recording surface
					// in the 1000-unit SVG coordinate space, then applies the viewport.
					if err := cairoFill(coverage, commands, x, y, em/1000); err != nil {
						return nil, err
					}
					if coverage.supported(img.Bounds().Dx(), img.Bounds().Dy()) {
						mask, err := coverage.mask(img.Bounds().Dx(), img.Bounds().Dy())
						if err != nil {
							return nil, err
						}
						cairoOver(img, mask, v.Color)
					} else {
						if err := cairoFill(r, commands, x, y, em/1000); err != nil {
							return nil, err
						}
						paint(v.Color)
					}
					continue
				}
				if c.mathJaxViewport {
					coverage := &coveragePath{}
					if v.ClippedExtender {
						// cairo_rectangle constructs the nested SVG clip with relative
						// lines: quantize its origin and width separately. The glyph's
						// top/bottom remain absolute path coordinates.
						left := math.RoundToEven(x*256) / 256
						right := left + math.RoundToEven(v.Commands[1].X*em*256)/256
						top := math.RoundToEven(y*256) / 256
						bottom := math.RoundToEven((y+v.Commands[2].Y*em)*256) / 256
						// An integer horizontal clip becomes a region. Cairo then
						// retains the rectilinear fill's rectangle coverage fast path.
						if left == math.Trunc(left) && right == math.Trunc(right) {
							if mask := cairoRectangleMask(img.Bounds().Dx(), img.Bounds().Dy(), left, top, right-left, bottom-top); mask != nil {
								cairoOver(img, mask, v.Color)
								continue
							}
						}
						coverage.MoveTo(float32(left), float32(top))
						coverage.LineTo(float32(right), float32(top))
						coverage.LineTo(float32(right), float32(bottom))
						coverage.LineTo(float32(left), float32(bottom))
						coverage.ClosePath()
					} else if err := cairoFill(coverage, v.Commands, x, y, em); err != nil {
						return nil, err
					}
					if coverage.supportedPath(img.Bounds().Dx(), img.Bounds().Dy(), v.ClippedExtender) {
						mask, err := coverage.mask(img.Bounds().Dx(), img.Bounds().Dy())
						if err != nil {
							return nil, err
						}
						cairoOver(img, mask, v.Color)
						continue
					}
					if err := cairoFill(r, v.Commands, x, y, em); err != nil {
						return nil, err
					}
					paint(v.Color)
					continue
				}
				for _, s := range v.Commands {
					a, b := float32(x+s.X*em), float32(y+s.Y*em)
					switch s.Kind {
					case texpath.KindMoveTo:
						r.MoveTo(a, b)
					case texpath.KindLineTo:
						r.LineTo(a, b)
					case texpath.KindQuadTo:
						r.QuadTo(float32(x+s.X1*em), float32(y+s.Y1*em), a, b)
					case texpath.KindCubicTo:
						r.CubeTo(float32(x+s.X1*em), float32(y+s.Y1*em), float32(x+s.X2*em), float32(y+s.Y2*em), a, b)
					case texpath.KindClose:
						r.ClosePath()
					}
				}
				paint(v.Color)
			} else {
				width := c.stroke * c.scale
				if v.StrokeWidth > 0 {
					width = v.StrokeWidth * em
				}
				strokePath(r, v.Commands, x, y, em, width)
				paint(v.Color)
			}
		default:
			return nil, fmt.Errorf("render: unknown display item %T", item)
		}
	}
	return img, nil
}

// Stroke paths as joined round capsules. Curves are flattened at a bounded,
// subpixel step size; fills retain the original quadratic/cubic outlines.
func strokePath(r *vector.Rasterizer, commands []texpath.Command, x, y, em, width float64) {
	radius := width / 2
	disk := func(px, py float64) {
		const k = .5522847498307936
		r.MoveTo(float32(px+radius), float32(py))
		r.CubeTo(float32(px+radius), float32(py+k*radius), float32(px+k*radius), float32(py+radius), float32(px), float32(py+radius))
		r.CubeTo(float32(px-k*radius), float32(py+radius), float32(px-radius), float32(py+k*radius), float32(px-radius), float32(py))
		r.CubeTo(float32(px-radius), float32(py-k*radius), float32(px-k*radius), float32(py-radius), float32(px), float32(py-radius))
		r.CubeTo(float32(px+k*radius), float32(py-radius), float32(px+radius), float32(py-k*radius), float32(px+radius), float32(py))
		r.ClosePath()
	}
	line := func(ax, ay, bx, by float64) {
		dx, dy := bx-ax, by-ay
		l := math.Hypot(dx, dy)
		if l == 0 {
			return
		}
		nx, ny := -dy/l*radius, dx/l*radius
		r.MoveTo(float32(ax-nx), float32(ay-ny))
		r.LineTo(float32(bx-nx), float32(by-ny))
		r.LineTo(float32(bx+nx), float32(by+ny))
		r.LineTo(float32(ax+nx), float32(ay+ny))
		r.ClosePath()
		disk(ax, ay)
		disk(bx, by)
	}
	var px, py, sx, sy float64
	for _, s := range commands {
		ex, ey := x+s.X*em, y+s.Y*em
		switch s.Kind {
		case texpath.KindMoveTo:
			px, py, sx, sy = ex, ey, ex, ey
		case texpath.KindLineTo:
			line(px, py, ex, ey)
			px, py = ex, ey
		case texpath.KindClose:
			line(px, py, sx, sy)
			px, py = sx, sy
		case texpath.KindQuadTo, texpath.KindCubicTo:
			ax, ay := px, py
			cx, cy := x+s.X1*em, y+s.Y1*em
			dx, dy := x+s.X2*em, y+s.Y2*em
			length := math.Hypot(cx-ax, cy-ay) + math.Hypot(ex-cx, ey-cy)
			if s.Kind == texpath.KindCubicTo {
				length += math.Hypot(dx-cx, dy-cy) + math.Hypot(ex-dx, ey-dy)
			}
			steps := int(math.Min(8192, math.Max(4, math.Ceil(length*2))))
			for i := 1; i <= steps; i++ {
				t := float64(i) / float64(steps)
				u := 1 - t
				var bx, by float64
				if s.Kind == texpath.KindQuadTo {
					bx = u*u*ax + 2*u*t*cx + t*t*ex
					by = u*u*ay + 2*u*t*cy + t*t*ey
				} else {
					bx = u*u*u*ax + 3*u*u*t*cx + 3*u*t*t*dx + t*t*t*ex
					by = u*u*u*ay + 3*u*u*t*cy + 3*u*t*t*dy + t*t*t*ey
				}
				line(px, py, bx, by)
				px, py = bx, by
			}
		}
	}
}

// cairoOver uses Cairo/Pixman byte-rounded source-over for supported glyph
// coverage and diagonal strokes in the MathJax viewport.
func cairoOver(dst *image.RGBA, mask *image.Alpha, c layout.Color) {
	mul := func(a, b uint8) uint8 { v := uint32(a)*uint32(b) + 128; return uint8((v + (v >> 8)) >> 8) }
	for y := 0; y < dst.Rect.Dy(); y++ {
		for x := 0; x < dst.Rect.Dx(); x++ {
			m := mask.AlphaAt(x, y).A
			if m == 0 {
				continue
			}
			a := mul(c.A, m)
			i := dst.PixOffset(x, y)
			for k, s := range []uint8{mul(mul(c.R, c.A), m), mul(mul(c.G, c.A), m), mul(mul(c.B, c.A), m), a} {
				dst.Pix[i+k] = s + mul(dst.Pix[i+k], 255-a)
			}
		}
	}
}
