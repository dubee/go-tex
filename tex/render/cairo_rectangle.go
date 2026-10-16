/* cairo - a vector graphics library with display and print output
 *
 * Copyright © 2009 Intel Corporation
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
 * Contributor(s):
 *	Chris Wilson <chris@chris-wilson.co.uk>
 */

// Bounded Go adaptation of Cairo 1.18.4 single-rectangle coverage.
package render

import (
	"image"
	"math"
)

// cairoRectangleMask retains the existing renderer for clipped/nonfinite boxes.
// Coverage is an exact 24.8 fixed-point overlap, not a comparison tolerance.
func cairoRectangleMask(width, height int, x, y, w, h float64) *image.Alpha {
	if !finite(x) || !finite(y) || !finite(w) || !finite(h) || w <= 0 || h <= 0 || x < 0 || y < 0 || x+w > float64(width) || y+h > float64(height) {
		return nil
	}
	left, top := int64(math.RoundToEven(x*256)), int64(math.RoundToEven(y*256))
	right, bottom := int64(math.RoundToEven((x+w)*256)), int64(math.RoundToEven((y+h)*256))
	m := image.NewAlpha(image.Rect(0, 0, width, height))
	for row := top / 256; row < (bottom+255)/256; row++ {
		dy := min(bottom, (row+1)*256) - max(top, row*256)
		for col := left / 256; col < (right+255)/256; col++ {
			dx := min(right, (col+1)*256) - max(left, col*256)
			alpha := min(int64(255), (dx*dy)>>8)
			m.Pix[int(row)*m.Stride+int(col)] = uint8(alpha)
		}
	}
	return m
}

// Rectilinear stroke coverage: quantize the centerline and offset separately,
// then sum disjoint box areas before converting the coverage to an alpha byte.
func cairoFrameMask(width, height int, x, y, w, h, t float64) *image.Alpha {
	if !finite(x+y+w+h+t) || t <= 0 || w <= 0 || h <= 0 {
		return nil
	}
	l, u, r, b := int64(math.RoundToEven(x*256)), int64(math.RoundToEven(y*256)), int64(math.RoundToEven((x+w)*256)), int64(math.RoundToEven((y+h)*256))
	half := int64(math.RoundToEven(t * 128))
	l0, u0, r0, b0 := l-half, u-half, r+half, b+half
	l1, u1, r1, b1 := l+half, u+half, r-half, b-half
	if l0 < 0 || u0 < 0 || r0 > int64(width)*256 || b0 > int64(height)*256 || l1 > r1 || u1 > b1 {
		return nil
	}
	mask := image.NewAlpha(image.Rect(0, 0, width, height))
	for row := u0 / 256; row < (b0+255)/256; row++ {
		for col := l0 / 256; col < (r0+255)/256; col++ {
			area := func(a, c, e, f int64) int64 {
				return max(int64(0), min(e, (col+1)*256)-max(a, col*256)) * max(int64(0), min(f, (row+1)*256)-max(c, row*256))
			}
			alpha := min(int64(255), (area(l0, u0, r0, b0)-area(l1, u1, r1, b1))>>8)
			mask.Pix[int(row)*mask.Stride+int(col)] = uint8(alpha)
		}
	}
	return mask
}
