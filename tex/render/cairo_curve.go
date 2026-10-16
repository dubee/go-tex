/* cairo - a vector graphics library with display and print output
 *
 * Copyright © 2002 University of Southern California
 * Copyright © 2005 Red Hat, Inc.
 * Copyright © 2008 Mozilla Corporation (Quartz path conversion)
 *
 * This library is free software; you can redistribute it and/or
 * modify it either under the terms of the GNU Lesser General Public
 * License version 2.1 as published by the Free Software Foundation
 * (the "LGPL") or, at your option, under the terms of the Mozilla
 * Public License Version 1.1 (the "MPL"). If you do not alter this
 * notice, a recipient may use your version of this file under either
 * the MPL or the LGPL.
 *
 * You should have received a copy of the LGPL along with this library
 * in the root LICENSE (LGPL-2.1 section); if not, write to the Free Software
 * Foundation, Inc., 51 Franklin Street, Suite 500, Boston, MA 02110-1335, USA
 * You should have received a copy of the MPL along with this library
 * in the root LICENSE (MPL-1.1 section)
 *
 * The contents of this file are subject to the Mozilla Public License
 * Version 1.1 (the "License"); you may not use this file except in
 * compliance with the License. You may obtain a copy of the License at
 * http://www.mozilla.org/MPL/
 *
 * This software is distributed on an "AS IS" basis, WITHOUT WARRANTY
 * OF ANY KIND, either express or implied. See the LGPL or the MPL for
 * the specific language governing rights and limitations.
 *
 * The Original Code is the cairo graphics library.
 *
 * The Initial Developer of the Original Code is University of Southern
 * California.
 *
 * Contributor(s):
 *	Carl D. Worth <cworth@cworth.org>
 *	Vladimir Vukicevic <vladimir@mozilla.com> (Quartz path conversion)
 */

// Go adaptation of path-line canonicalization and spline subdivision in
// Cairo 1.18.4. Coverage is handled separately by the bounded coverage path.
package render

import (
	"fmt"
	texpath "github.com/luo-studio/go-tex/tex/path"
	"math"
	"math/big"
)

type cairoPoint struct{ x, y int64 }

func cairoMid(a, b cairoPoint) cairoPoint {
	return cairoPoint{a.x + ((b.x - a.x) >> 1), a.y + ((b.y - a.y) >> 1)}
}
func cairoError(p [4]cairoPoint) float64 {
	dx, dy := float64(p[3].x-p[0].x)/256, float64(p[3].y-p[0].y)/256
	v := dx*dx + dy*dy
	worst := 0.0
	for _, q := range p[1:3] {
		bx, by := float64(q.x-p[0].x)/256, float64(q.y-p[0].y)/256
		if v != 0 {
			u := bx*dx + by*dy
			if u >= v {
				bx -= dx
				by -= dy
			} else if u > 0 {
				bx -= u / v * dx
				by -= u / v * dy
			}
		}
		worst = math.Max(worst, bx*bx+by*by)
	}
	return worst
}

// Emits fixed-point vertices at Cairo's default .1 device-pixel tolerance.
// The bounded work counter fails rather than silently truncating a path.
func cairoSpline(p [4]cairoPoint, emit func(cairoPoint), budget *int) error {
	*budget--
	if *budget < 0 {
		return fmt.Errorf("render: curve subdivision budget exceeded")
	}
	if cairoError(p) < .1*.1 {
		emit(p[0])
		return nil
	}
	ab, bc, cd := cairoMid(p[0], p[1]), cairoMid(p[1], p[2]), cairoMid(p[2], p[3])
	abc, bcd := cairoMid(ab, bc), cairoMid(bc, cd)
	mid := cairoMid(abc, bcd)
	if err := cairoSpline([4]cairoPoint{p[0], ab, abc, mid}, emit, budget); err != nil {
		return err
	}
	return cairoSpline([4]cairoPoint{mid, bcd, cd, p[3]}, emit, budget)
}

type cairoPathSink interface {
	MoveTo(float32, float32)
	LineTo(float32, float32)
	ClosePath()
}

func cairoFill(r cairoPathSink, commands []texpath.Command, x, y, em float64) error {
	point := func(a, b float64) (cairoPoint, error) {
		a, b = x+a*em, y+b*em
		if !finite(a) || !finite(b) || math.Abs(a) > 8388607 || math.Abs(b) > 8388607 {
			return cairoPoint{}, fmt.Errorf("render: invalid fixed-point path coordinate")
		}
		return cairoPoint{int64(math.RoundToEven(a * 256)), int64(math.RoundToEven(b * 256))}, nil
	}
	// Cairo collapses consecutive authored line segments of the same slope
	// before flattening curves. Keeping an extra vertex can force the TOR
	// converter to use subrows instead of analytical full-row coverage.
	var lineStart, lineEnd cairoPoint
	pendingLine := false
	flushLine := func() {
		if pendingLine {
			r.LineTo(float32(lineEnd.x)/256, float32(lineEnd.y)/256)
			pendingLine = false
		}
	}
	line := func(p cairoPoint) { r.LineTo(float32(p.x)/256, float32(p.y)/256) }
	var current, start cairoPoint
	var px, py, sx, sy float64
	budget := 100000
	for _, c := range commands {
		if c.Kind == texpath.KindClose {
			flushLine()
			r.ClosePath()
			current = start
			px, py = sx, sy
			continue
		}
		end, err := point(c.X, c.Y)
		if err != nil {
			return err
		}
		switch c.Kind {
		case texpath.KindMoveTo:
			flushLine()
			r.ClosePath()
			r.MoveTo(float32(end.x)/256, float32(end.y)/256)
			start = end
			sx, sy = c.X, c.Y
		case texpath.KindLineTo:
			if end != current {
				if pendingLine && !cairoSameSlope(lineStart, lineEnd, end) {
					flushLine()
				}
				if !pendingLine {
					lineStart = current
				}
				lineEnd, pendingLine = end, true
			}
		case texpath.KindQuadTo, texpath.KindCubicTo:
			flushLine()
			bx, by, cx, cy := c.X1, c.Y1, c.X2, c.Y2
			if c.Kind == texpath.KindQuadTo {
				bx, by = (px+2*c.X1)/3, (py+2*c.Y1)/3
				cx, cy = (c.X+2*c.X1)/3, (c.Y+2*c.Y1)/3
			}
			b, err := point(bx, by)
			if err != nil {
				return err
			}
			d, err := point(cx, cy)
			if err != nil {
				return err
			}
			if err = cairoSpline([4]cairoPoint{current, b, d, end}, line, &budget); err != nil {
				return err
			}
			line(end)
		default:
			return fmt.Errorf("render: unsupported filled path command")
		}
		current = end
		px, py = c.X, c.Y
	}
	flushLine()
	r.ClosePath()
	return nil
}

// Exact slope comparison without overflow at the fixed-point coordinate limits.
// Opposite directions must remain separate, as in Cairo's path canonicalizer.
func cairoSameSlope(a, b, c cairoPoint) bool {
	dx, dy, ex, ey := b.x-a.x, b.y-a.y, c.x-b.x, c.y-b.y
	if (dx < 0 && ex > 0) || (dx > 0 && ex < 0) || (dy < 0 && ey > 0) || (dy > 0 && ey < 0) {
		return false
	}
	var left, right big.Int
	left.Mul(big.NewInt(dx), big.NewInt(ey))
	right.Mul(big.NewInt(dy), big.NewInt(ex))
	return left.Cmp(&right) == 0
}

// cairoButtLine adapts Cairo 1.18.4 _compute_face/_add_sub_edge for a single
// straight SVG line under the uniform MathJax viewport transform. Endpoints
// and the perpendicular offset are quantized separately in device 24.8 units.
// Curves, joins, dashes and nonuniform transforms are deliberately not handled.
func cairoButtLine(r cairoPathSink, commands []texpath.Command, x, y, em, width float64) error {
	if len(commands) != 2 || commands[0].Kind != texpath.KindMoveTo || commands[1].Kind != texpath.KindLineTo || !finite(width) || width <= 0 {
		return fmt.Errorf("render: unsupported butt stroke")
	}
	points := [2]cairoPoint{}
	for i, c := range commands {
		a, b := x+c.X*em, y+c.Y*em
		if !finite(a) || !finite(b) || math.Abs(a) > 1e6 || math.Abs(b) > 1e6 {
			return fmt.Errorf("render: invalid stroke coordinate")
		}
		points[i] = cairoPoint{int64(math.RoundToEven(a * 256)), int64(math.RoundToEven(b * 256))}
	}
	dx, dy := float64(points[1].x-points[0].x), float64(points[1].y-points[0].y)
	length := math.Hypot(dx, dy)
	if length == 0 {
		return nil
	}
	nx, ny := int64(math.RoundToEven(-dy/length*width/2*256)), int64(math.RoundToEven(dx/length*width/2*256))
	p, q := points[0], points[1]
	r.MoveTo(float32(p.x-nx)/256, float32(p.y-ny)/256)
	r.LineTo(float32(q.x-nx)/256, float32(q.y-ny)/256)
	r.LineTo(float32(q.x+nx)/256, float32(q.y+ny)/256)
	r.LineTo(float32(p.x+nx)/256, float32(p.y+ny)/256)
	r.ClosePath()
	return nil
}

// cairoSystemOutline follows cairo-quartz-font.c's path callback: quadratic
// starts come from the already fixed current point. Quantize the glyph locally
// before the separate positioned-glyph translation. SVG paths keep cairoFill.
func cairoSystemOutline(commands []texpath.Command, em float64) ([]texpath.Command, error) {
	if !finite(em) || em <= 0 || len(commands) > 100000 {
		return nil, fmt.Errorf("render: invalid system outline")
	}
	out := make([]texpath.Command, 0, len(commands))
	var px, py, sx, sy float64
	q := func(v float64) float64 { return math.RoundToEven(v*256) / 256 }
	for _, c := range commands {
		c.X *= em
		c.Y *= em
		c.X1 *= em
		c.Y1 *= em
		c.X2 *= em
		c.Y2 *= em
		for _, v := range []float64{c.X, c.Y, c.X1, c.Y1, c.X2, c.Y2} {
			if !finite(v) || math.Abs(v) > 8388607 {
				return nil, fmt.Errorf("render: invalid system outline coordinate")
			}
		}
		if c.Kind == texpath.KindQuadTo {
			c.X2 = (2*c.X1 + c.X) / 3
			c.Y2 = (2*c.Y1 + c.Y) / 3
			c.X1 = (px + 2*c.X1) / 3
			c.Y1 = (py + 2*c.Y1) / 3
			c.Kind = texpath.KindCubicTo
		}
		c.X = q(c.X)
		c.Y = q(c.Y)
		c.X1 = q(c.X1)
		c.Y1 = q(c.Y1)
		c.X2 = q(c.X2)
		c.Y2 = q(c.Y2)
		switch c.Kind {
		case texpath.KindMoveTo:
			sx, sy = c.X, c.Y
		case texpath.KindLineTo, texpath.KindCubicTo:
		case texpath.KindClose:
			c.X, c.Y = sx, sy
		default:
			return nil, fmt.Errorf("render: unsupported system outline command")
		}
		px, py = c.X, c.Y
		out = append(out, c)
	}
	return out, nil
}
