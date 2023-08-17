// Copyright (c) 2023, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// adapted from: https://github.com/material-foundation/material-color-utilities
// Copyright 2022 Google LLC
// Licensed under the Apache License, Version 2.0 (the "License")

package hct

import (
	"github.com/goki/cam/cam16"
	"github.com/goki/cam/cie"
	"github.com/goki/mat32"
)

// SolveToRGBLin Finds an sRGB linear color (represented by mat32.Vec3, 0-100 range)
// with the given hue, chroma, and tone, if possible.
// if not possible to represent the target values, the hue and tone will be
// sufficiently close, and chroma will be maximized.
func SolveToRGBLin(hue, chroma, tone float32) mat32.Vec3 {
	if chroma < 0.0001 || tone < 0.0001 || tone > 99.9999 {
		y := cie.SRGBFmLinearComp(tone)
		return mat32.Vec3{y, y, y}
	}
	hue_deg := cam16.SanitizeDeg(hue)
	hue_rad := mat32.DegToRad(hue_deg)
	y := cie.LToY(tone)
	exact := FindResultByJ(hue_rad, chroma, y)
	if exact != nil {
		return *exact
	}
	return BisectToLimit(y, hue_rad)
}

// SolveToRGB Finds an sRGB (gamma corrected, 0-1 range) color
// with the given hue, chroma, and tone, if possible.
// if not possible to represent the target values, the hue and tone will be
// sufficiently close, and chroma will be maximized.
func SolveToRGB(hue, chroma, tone float32) (r, g, b float32) {
	lin := SolveToRGBLin(hue, chroma, tone)
	r, g, b = cie.SRGBFmLinear100(lin.X, lin.Y, lin.Z)
	return
}

// Finds a color with the given hue, chroma, and Y.
// @param hue_radians The desired hue in radians.
// @param chroma The desired chroma.
// @param y The desired Y.
// @return The desired color as linear sRGB values.
func FindResultByJ(hue_rad, chroma, y float32) *mat32.Vec3 {
	// Initial estimate of j.
	j := mat32.Sqrt(y) * 11

	// ===========================================================
	// Operations inlined from Cam16 to avoid repeated calculation
	// ===========================================================
	vw := cam16.NewStdView()
	t_inner_coeff := 1 / mat32.Pow(1.64-mat32.Pow(0.29, vw.BgYToWhiteY), 0.73)
	e_hue := 0.25 * (mat32.Cos(hue_rad+2) + 3.8)
	p1 := e_hue * (50000 / 13) * vw.NC * vw.NCB
	h_sin := mat32.Sin(hue_rad)
	h_cos := mat32.Cos(hue_rad)
	for itr := 0; itr < 5; itr++ {
		j_norm := j / 100
		alpha := float32(0)
		if !(chroma == 0 || j == 0) {
			alpha = chroma / mat32.Sqrt(j_norm)
		}
		t := mat32.Pow(alpha*t_inner_coeff, 1/0.9)
		ac := vw.AW * mat32.Pow(j_norm, 1/vw.C/vw.Z)
		p2 := ac / vw.NBB
		gamma := 23 * (p2 + 0.305) * t / (23*p1 + 11*t*h_cos + 108*t*h_sin)
		a := gamma * h_cos
		b := gamma * h_sin
		r_a := (460*p2 + 451*a + 288*b) / 1403
		g_a := (460*p2 - 891*a - 261*b) / 1403
		b_a := (460*p2 - 220*a - 6300*b) / 1403
		r_c_scaled := cam16.InverseChromaticAdapt(r_a)
		g_c_scaled := cam16.InverseChromaticAdapt(g_a)
		b_c_scaled := cam16.InverseChromaticAdapt(b_a)
		scaled := mat32.Vec3{r_c_scaled, g_c_scaled, b_c_scaled}
		linrgb := MatMul(scaled, kLinrgbFromScaledDiscount)

		if linrgb.X < 0 || linrgb.Y < 0 || linrgb.Z < 0 {
			return nil
		}
		k_r := kYFromLinrgb[0]
		k_g := kYFromLinrgb[1]
		k_b := kYFromLinrgb[2]
		fnj := k_r*linrgb.X + k_g*linrgb.Y + k_b*linrgb.Z
		if fnj <= 0 {
			return nil
		}
		if itr == 4 || mat32.Abs(fnj-y) < 0.002 {
			if linrgb.X > 100.01 || linrgb.Y > 100.01 || linrgb.Z > 100.01 {
				return nil
			}
			return &linrgb
		}
		// Iterates with Newton method,
		// Using 2 * fn(j) / j as the approximation of fn'(j)
		j = j - (fnj-y)*j/(2*fnj)
	}
	return nil
}