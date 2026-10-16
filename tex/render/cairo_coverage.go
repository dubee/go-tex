/* glitter-paths - polygon scan converter
 *
 * Copyright (c) 2008  M Joonas Pihlaja
 * Copyright (c) 2007  David Turner
 *
 * Permission is hereby granted, free of charge, to any person
 * obtaining a copy of this software and associated documentation
 * files (the "Software"), to deal in the Software without
 * restriction, including without limitation the rights to use,
 * copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following
 * conditions:
 *
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
 * OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
 * HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 */

// Go adaptation of Cairo 1.18.4 coverage arithmetic; see the root LICENSE for attribution.
package render

import (
	"fmt"
	"image"
	"sort"
)

type coverageEdge struct {
	a, b        cairoPoint
	top, bottom int64
	dir         int
}
type coveragePath struct {
	edges          []coverageEdge
	current, start cairoPoint
	open           bool
	invalid        bool
}

func (p *coveragePath) MoveTo(x, y float32) {
	p.ClosePath()
	p.current = cairoPoint{int64(x * 256), int64(y * 256)}
	p.start = p.current
	p.open = true
}
func (p *coveragePath) LineTo(x, y float32) {
	if !p.open {
		p.start = p.current
		p.open = true
	}
	q := cairoPoint{int64(x * 256), int64(y * 256)}
	p.edge(p.current, q)
	p.current = q
}
func (p *coveragePath) ClosePath() {
	if p.open {
		p.edge(p.current, p.start)
		p.current = p.start
		p.open = false
	}
}
func (p *coveragePath) edge(a, b cairoPoint) {
	for _, v := range []int64{a.x, a.y, b.x, b.y} {
		if v < -1048576 || v > 1048576 {
			p.invalid = true
			return
		}
	}
	if a.y == b.y {
		return
	}
	dir := 1
	if a.y > b.y {
		a, b = b, a
		dir = -1
	}
	top, bottom := (15*a.y+128)>>8, (15*b.y+128)>>8
	if top < bottom {
		p.edges = append(p.edges, coverageEdge{a, b, top, bottom, dir})
	}
}

type coverageQ struct{ q, r int64 }

func normQ(q coverageQ, d int64) coverageQ {
	if q.r < 0 {
		q.q--
		q.r += d
	} else if q.r >= d {
		q.q++
		q.r -= d
	}
	return q
}
func (e coverageEdge) at(y int64) (coverageQ, int64, coverageQ) {
	if e.a.x == e.b.x {
		return coverageQ{e.a.x, 0}, 0, coverageQ{}
	}
	ex := (e.b.x - e.a.x) * 256
	d := (e.b.y - e.a.y) * 15 * 512
	t := ((2*y+1)*256 - e.a.y*30) * ex
	x := normQ(coverageQ{e.a.x + t/d, t % d}, d)
	step := coverageQ{ex * 512 / d, ex * 512 % d}
	return x, d, step
}
func (e coverageEdge) cell(y int64) int64 {
	x, d, _ := e.at(y)
	if d != 0 && x.r >= d/2 {
		x.q++
	}
	return x.q
}

type coverageCell struct{ height, area int64 }

func renderCoverageEdge(c map[int64]coverageCell, e coverageEdge, y int64, sign int64) {
	x1, d, step := e.at(y)
	x2, _, _ := e.at(y + 15)
	if d != 0 {
		x1 = normQ(coverageQ{x1.q - step.q/2, x1.r - step.r/2}, d)
		x2 = normQ(coverageQ{x2.q - step.q/2, x2.r - step.r/2}, d)
	}
	i1, i2 := x1.q>>8, x2.q>>8
	f1, f2 := x1.q&255, x2.q&255
	add := func(i, h, a int64) { v := c[i]; v.height += sign * h; v.area += sign * a; c[i] = v }
	if i1 == i2 {
		add(i1, 15, (f1+f2)*15)
		return
	}
	if i2 < i1 {
		i1, i2 = i2, i1
		f1, f2 = f2, f1
		x1, x2 = x2, x1
	}
	dx := (x2.q-x1.q)*d + x2.r - x1.r
	tmp := ((i1+1)*256*d - x1.q*d - x1.r) * 15
	yy := coverageQ{tmp / dx, tmp % dx}
	add(i1, yy.q, yy.q*(256+f1))
	last := yy.q
	inc := coverageQ{15 * 256 * d / dx, 15 * 256 * d % dx}
	for i := i1 + 1; i < i2; i++ {
		yy = normQ(coverageQ{yy.q + inc.q, yy.r + inc.r}, dx)
		add(i, yy.q-last, (yy.q-last)*256)
		last = yy.q
	}
	add(i2, 15-last, (15-last)*f2)
}

// Cairo uses separate clipped-edge and rectilinear fast paths. Retain the
// established rasterizer for these until those paths are independently proven.
func (p *coveragePath) supported(w, h int) bool { return p.supportedPath(w, h, false) }
func (p *coveragePath) supportedPath(w, h int, rectilinear bool) bool {
	diagonal := false
	for _, e := range p.edges {
		for _, q := range []cairoPoint{e.a, e.b} {
			if q.x < 0 || q.y < 0 || q.x > int64(w)*256 || q.y > int64(h)*256 {
				return false
			}
		}
		diagonal = diagonal || e.a.x != e.b.x
	}
	return !p.invalid && (diagonal || rectilinear)
}
func (p *coveragePath) mask(w, h int) (*image.Alpha, error) {
	if p.invalid || len(p.edges) > 100000 {
		return nil, fmt.Errorf("render: coverage path exceeds bounds")
	}
	if int64(h)*int64(len(p.edges))*15 > 100000000 {
		return nil, fmt.Errorf("render: coverage work budget exceeded")
	}
	out := image.NewAlpha(image.Rect(0, 0, w, h))
	active := []coverageEdge{}
	for row := 0; row < h; row++ {
		y := int64(row * 15)
		active = active[:0]
		full := true
		for _, e := range p.edges {
			if e.top < y+15 && e.bottom > y {
				active = append(active, e)
				if e.top > y || e.bottom < y+15 {
					full = false
				}
			}
		}
		if len(active) == 0 {
			continue
		}
		sort.SliceStable(active, func(i, j int) bool { return active[i].cell(y) < active[j].cell(y) })
		if full {
			for i := 1; i < len(active); i++ {
				if active[i].cell(y+15) < active[i-1].cell(y+15) {
					full = false
					break
				}
			}
		}
		cells := map[int64]coverageCell{}
		if full {
			winding := 0
			started := false
			var left coverageEdge
			for i, e := range active {
				if !started {
					started = true
					left = e
				}
				winding += e.dir
				if winding == 0 && (i+1 == len(active) || active[i+1].cell(y) != e.cell(y)) {
					renderCoverageEdge(cells, left, y, 1)
					renderCoverageEdge(cells, e, y, -1)
					started = false
				}
			}
		} else {
			for sub := int64(0); sub < 15; sub++ {
				edges := []coverageEdge{}
				for _, e := range active {
					if e.top <= y+sub && e.bottom > y+sub {
						edges = append(edges, e)
					}
				}
				sort.SliceStable(edges, func(i, j int) bool { return edges[i].cell(y+sub) < edges[j].cell(y+sub) })
				winding := 0
				var start int64
				started := false
				for i, e := range edges {
					x := e.cell(y + sub)
					winding += e.dir
					if winding != 0 && !started {
						start = x
						started = true
					}
					if winding == 0 && started && (i+1 == len(edges) || edges[i+1].cell(y+sub) != x) {
						a, b := start>>8, x>>8
						v := cells[a]
						v.area += 2 * (start & 255)
						v.height++
						cells[a] = v
						v = cells[b]
						v.area -= 2 * (x & 255)
						v.height--
						cells[b] = v
						started = false
					}
				}
			}
		}
		cover := int64(0)
		for x, v := range cells {
			if x < 0 {
				cover += v.height * 512
			}
		}
		for x := 0; x < w; x++ {
			v := cells[int64(x)]
			cover += v.height * 512
			area := cover - v.area
			alpha := (area*17 + 256) >> 9
			if alpha < 0 || alpha > 255 {
				return nil, fmt.Errorf("render: invalid coverage %d", alpha)
			}
			out.Pix[row*out.Stride+x] = uint8(alpha)
		}
	}
	return out, nil
}
