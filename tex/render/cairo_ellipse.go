/* cairo - a vector graphics library with display and print output
 *
 * Copyright © 2002 University of Southern California
 * Copyright © 2011 Intel Corporation
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
 *	Chris Wilson <chris@chris-wilson.co.uk>
 */

package render

import (
	"fmt"
	texpath "github.com/luo-studio/go-tex/tex/path"
	"math"
)

// Bounded smooth ellipse stroke using Cairo's spline tangents, compute_face and inner joins.
// This is not a general stroker: no cusps, corners, caps or arbitrary transforms.
func cairoEllipseStroke(r cairoPathSink, commands []texpath.Command, x, y, em, width float64) error {
	if len(commands) != 6 || commands[0].Kind != texpath.KindMoveTo || commands[5].Kind != texpath.KindClose || !finite(width) || width <= 0 {
		return fmt.Errorf("render: invalid ellipse stroke")
	}
	for _, c := range commands {
		for _, pair := range [][2]float64{{c.X, c.Y}, {c.X1, c.Y1}, {c.X2, c.Y2}} {
			a, b := x+pair[0]*em, y+pair[1]*em
			if !finite(a) || !finite(b) || math.Abs(a) > 1e6 || math.Abs(b) > 1e6 {
				return fmt.Errorf("render: ellipse coordinates exceed bounds")
			}
		}
	}
	point := func(a, b float64) cairoPoint {
		return cairoPoint{int64(math.RoundToEven((x + a*em) * 256)), int64(math.RoundToEven((y + b*em) * 256))}
	}
	var outer, inner []cairoPoint
	var prev cairoPoint
	have := false
	face := func(p, t cairoPoint) {
		if have && p == prev {
			return
		}
		have = true
		prev = p
		dx, dy := float64(t.x), float64(t.y)
		n := math.Hypot(dx, dy)
		if n == 0 {
			return
		}
		ox, oy := int64(math.RoundToEven(-dy/n*width*128)), int64(math.RoundToEven(dx/n*width*128))
		outer = append(outer, cairoPoint{p.x + ox, p.y + oy})
		inner = append(inner, cairoPoint{p.x - ox, p.y - oy})
	}
	budget := 100000
	var split func([4]cairoPoint) error
	split = func(p [4]cairoPoint) error {
		budget--
		if budget < 0 {
			return fmt.Errorf("render: ellipse work budget exceeded")
		}
		if cairoError(p) < .01 {
			face(p[0], cairoPoint{p[1].x - p[0].x, p[1].y - p[0].y})
			return nil
		}
		ab, bc, cd := cairoMid(p[0], p[1]), cairoMid(p[1], p[2]), cairoMid(p[2], p[3])
		abc, bcd := cairoMid(ab, bc), cairoMid(bc, cd)
		mid := cairoMid(abc, bcd)
		if err := split([4]cairoPoint{p[0], ab, abc, mid}); err != nil {
			return err
		}
		return split([4]cairoPoint{mid, bcd, cd, p[3]})
	}
	p := point(commands[0].X, commands[0].Y)
	for i, c := range commands[1:5] {
		if i > 0 {
			if len(inner) == 0 {
				return fmt.Errorf("render: degenerate ellipse")
			}
			// Cairo inner_join retains this out-and-back edge even for equal
			// tangents. Its zero winding still affects TOR subrow scheduling.
			inner = append(inner, p, inner[len(inner)-1])
		}
		if c.Kind != texpath.KindCubicTo {
			return fmt.Errorf("render: invalid ellipse segment")
		}
		b, d, end := point(c.X1, c.Y1), point(c.X2, c.Y2), point(c.X, c.Y)
		face(p, cairoPoint{b.x - p.x, b.y - p.y})
		if err := split([4]cairoPoint{p, b, d, end}); err != nil {
			return err
		}
		face(end, cairoPoint{end.x - d.x, end.y - d.y})
		p = end
	}
	if len(inner) == 0 {
		return fmt.Errorf("render: degenerate ellipse")
	}
	// inner_close retains the corresponding join at the start of the path.
	inner = append(inner, p, inner[0])
	for j, ps := range [][]cairoPoint{outer, inner} {
		for i := range ps {
			ix := i
			if j == 1 {
				ix = len(ps) - 1 - i
			}
			p := ps[ix]
			if i == 0 {
				r.MoveTo(float32(p.x)/256, float32(p.y)/256)
			} else {
				r.LineTo(float32(p.x)/256, float32(p.y)/256)
			}
		}
		r.ClosePath()
	}
	return nil
}
