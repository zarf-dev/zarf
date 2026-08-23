//go:build regression

//go:generate go run -tags regression gen.regression.go
package main

import (
	"fmt"
	"strconv"
	"math"
	"os"
	"text/template"
	"bytes"
	"go/format"

	"gonum.org/v1/gonum/mat"
	"golang.org/x/exp/maps"
	"sort"
)

type LinearRegression struct {
	Intercept float64
	Slope     float64
}

func (lr *LinearRegression) Fit(xs, ys []float64) {
	if len(xs) != len(ys) {
		panic("length of xs and ys must be the same")
	}

	var sX, sY, sXY, sXX float64
	for i := 0; i < len(xs); i++ {
		sX += xs[i]
		sY += ys[i]
		sXY += xs[i] * ys[i]
		sXX += xs[i] * xs[i]
	}

	n := float64(len(xs))
	d := n*sXX - sX*sX
	if d == 0 {
		d = 1
	}

	lr.Slope = (n*sXY - sX*sY) / d
	lr.Intercept = (sY*sXX - sX*sXY) / d
}

func (lr *LinearRegression) Predict(x float64) (y float64) {
	return lr.Intercept + lr.Slope*x
}

type PolynomialRegression struct {
	Degree       int
	Coefficients []float64
}

func (pr *PolynomialRegression) Fit(xs, ys []float64) {
	samples := len(xs)
	feats := pr.Degree + 1

	feat := mat.NewDense(samples, feats, nil)
	{
		for i := 0; i < samples; i++ {
			for j := 0; j < feats; j++ {
				feat.Set(i, j, math.Pow(xs[i], float64(j)))
			}
		}
		var qr mat.QR
		qr.Factorize(feat)
	}
	yVec := mat.NewVecDense(samples, ys)

	var coef mat.VecDense
	if err := coef.SolveVec(feat, yVec); err != nil {
		panic("failed to solve")
	}

	pr.Coefficients = coef.RawVector().Data
}

func (pr *PolynomialRegression) Predict(x float64) (y float64) {
	y = 0
	for i := 0; i < pr.Degree+1; i++ {
		y += pr.Coefficients[i] * math.Pow(x, float64(i))
	}
	return
}

func DiffusionModelMemoryUsageRegression(output string) {
	type Regression struct {
		Name                 string
		LinearRegression     *LinearRegression
		PolynomialRegression *PolynomialRegression
	}

	const tmplStr = `
package gguf_parser

import "math"

{{ range . -}}
// {{ .Name }} returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func {{ .Name }}(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{ {{ range $i, $c := .PolynomialRegression.Coefficients }}{{ if eq $i 0 }}{{ printf "%.4f" $c }}{{ else }}{{ printf "%.10f" $c }}{{ end }}, {{ end }} }
	degree := {{ .PolynomialRegression.Degree }}
	x := float64(width * height)
	
	{{ if .LinearRegression -}}
    if flashAttention {
		coefficients = []float64{ {{ printf "%.5f" .LinearRegression.Intercept }}, {{ printf "%.10f" .LinearRegression.Slope }} }
		degree = 1
    }
    {{- end }}

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}

{{ end }}

`
	ts := []struct {
		n     string
		x2y   map[float64]float64
		c     map[float64]float64
		fax2y map[float64]float64
		fac   map[float64]float64
	}{
		{
			n: "GuessSD1DiffusionModelMemoryUsage",
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 49.57 MB(VRAM)     // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 559.90 MB(VRAM)    // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 8360.93 MB(VRAM)   // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 18681.62 MB(VRAM)  // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 25377.96 MB(VRAM)  // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 41842.65 MB(VRAM)  // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 77333.77 MB(VRAM)  // 1792*1792
			x2y: map[float64]float64{
				256 * 256:   49.57,
				512 * 512:   559.90,
				1024 * 1024: 8360.93,
				1024 * 1536: 18681.62,
				1024 * 1792: 25377.96,
				1536 * 1536: 41842.65,
				1792 * 1792: 77333.77,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 56879.17 MB(VRAM)  // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 100924.37 MB(VRAM) // 1792*2048
			c: map[float64]float64{
				1536 * 1792: 56879.17,
				1792 * 2048: 100924.37,
			},
		},
		{
			n: "GuessSD2DiffusionModelMemoryUsage",
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 37.65 MB(VRAM)     // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 367.98 MB(VRAM)    // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 830.86 MB(VRAM)    // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 11769.69 MB(VRAM)  // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 15970.04 MB(VRAM)  // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 26290.73 MB(VRAM)  // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 48521.84 MB(VRAM)  // 1792*1792
			x2y: map[float64]float64{
				256 * 256:   37.65,
				512 * 512:   367.98,
				1024 * 1024: 830.86,
				1024 * 1536: 11769.69,
				1024 * 1792: 15970.04,
				1536 * 1536: 26290.73,
				1792 * 1792: 48521.84,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 35711.24 MB(VRAM) // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 63292.44 MB(VRAM) // 1792*2048
			c: map[float64]float64{
				1536 * 1792: 35711.24,
				1792 * 2048: 63292.44,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 34.52 MB(VRAM)   // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 130.48 MB(VRAM)  // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 519.01 MB(VRAM)  // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 774.69 MB(VRAM)  // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 902.54 MB(VRAM)  // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1158.23 MB(VRAM) // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1573.72 MB(VRAM) // 1792*1792
			fax2y: map[float64]float64{
				256 * 256:   34.52,
				512 * 512:   130.48,
				1024 * 1024: 519.01,
				1024 * 1536: 774.69,
				1024 * 1792: 902.54,
				1536 * 1536: 1158.23,
				1792 * 1792: 1573.72,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1349.99 MB(VRAM) // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1797.44 MB(VRAM) // 1792*2048
			fac: map[float64]float64{
				1536 * 1792: 1349.99,
				1792 * 2048: 1797.44,
			},
		},
		{
			n: "GuessSDXLDiffusionModelMemoryUsage",
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 60.76 MB(VRAM)     // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 132.05 MB(VRAM)    // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 830.86 MB(VRAM)    // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1701.55 MB(VRAM)   // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 2256.90 MB(VRAM)   // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 3607.58 MB(VRAM)   // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 6484.95 MB(VRAM)   // 1792*1792
			x2y: map[float64]float64{
				256 * 256:   60.76,
				512 * 512:   132.05,
				1024 * 1024: 830.86,
				1024 * 1536: 1701.55,
				1024 * 1792: 2256.90,
				1536 * 1536: 3607.58,
				1792 * 1792: 6484.95,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 4830.60 MB(VRAM) // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 8384.30 MB(VRAM) // 1792*2048
			c: map[float64]float64{
				1536 * 1792: 4830.60,
				1792 * 2048: 8384.30,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 60.13 MB(VRAM)   // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 132.05 MB(VRAM)  // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 440.86 MB(VRAM)  // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 726.55 MB(VRAM)  // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 874.40 MB(VRAM)  // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1110.08 MB(VRAM) // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1554.33 MB(VRAM) // 1792*1792
			fax2y: map[float64]float64{
				256 * 256:   60.13,
				512 * 512:   132.05,
				1024 * 1024: 440.86,
				1024 * 1536: 726.55,
				1024 * 1792: 874.40,
				1536 * 1536: 1110.08,
				1792 * 1792: 1554.33,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1339.35 MB(VRAM) // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1769.30 MB(VRAM) // 1792*2048
			fac: map[float64]float64{
				1536 * 1792: 1339.35,
				1792 * 2048: 1769.30,
			},
		},
		{
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 44.57 MB(VRAM)     // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 154.40 MB(VRAM)    // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 968.43 MB(VRAM)    // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 2013.12 MB(VRAM)   // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 2679.46 MB(VRAM)   // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 4300.15 MB(VRAM)   // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 7752.77 MB(VRAM)   // 1792*1792
			n: "GuessSDXLRefinerDiffusionModelMemoryUsage",
			x2y: map[float64]float64{
				256 * 256:   44.57,
				512 * 512:   154.40,
				1024 * 1024: 968.43,
				1024 * 1536: 2013.12,
				1024 * 1792: 2679.46,
				1536 * 1536: 4300.15,
				1792 * 1792: 7752.77,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 5767.67 MB(VRAM)   // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 10031.87 MB(VRAM)  // 1792*2048
			c: map[float64]float64{
				1536 * 1792: 5767.67,
				1792 * 2048: 10031.87,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 44.57 MB(VRAM)   // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 154.40 MB(VRAM)  // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 596.43 MB(VRAM)  // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 915.12 MB(VRAM)  // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1062.46 MB(VRAM) // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1357.15 MB(VRAM) // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1836.02 MB(VRAM) // 1792*1792
			fax2y: map[float64]float64{
				256 * 256:   44.57,
				512 * 512:   154.40,
				1024 * 1024: 596.43,
				1024 * 1536: 915.12,
				1024 * 1792: 1062.46,
				1536 * 1536: 1357.15,
				1792 * 1792: 1836.02,
			},
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 1578.17 MB(VRAM) // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - unet compute buffer size: 2014.02 MB(VRAM) // 1792*2048
			fac: map[float64]float64{
				1536 * 1792: 1578.17,
				1792 * 2048: 2014.02,
			},
		},
		{
			n: "GuessSD3MediumDiffusionModelMemoryUsage",
			// [DEBUG] ggml_extend.hpp:1034 - mmdit compute buffer size: 37.09 MB(VRAM)    // 256*256
			// [DEBUG] ggml_extend.hpp:1034 - mmdit compute buffer size: 169.64 MB(VRAM)   // 512*512
			// [DEBUG] ggml_extend.hpp:1034 - mmdit compute buffer size: 1786.11 MB(VRAM)  // 1024*1024
			// [DEBUG] ggml_extend.hpp:1034 - mmdit compute buffer size: 3824.36 MB(VRAM)  // 1024*1536
			// [DEBUG] ggml_extend.hpp:1034 - mmdit compute buffer size: 5131.48 MB(VRAM)  // 1024*1792
			// [DEBUG] ggml_extend.hpp:1034 - mmdit compute buffer size: 8319.03 MB(VRAM)  // 1536*1536
			// [DEBUG] ggml_extend.hpp:1034 - mmdit compute buffer size: 15141.18 MB(VRAM) // 1792*1792
			x2y: map[float64]float64{
				256 * 256:   37.09,
				512 * 512:   169.64,
				1024 * 1024: 1786.11,
				1024 * 1536: 3824.36,
				1024 * 1792: 5131.48,
				1536 * 1536: 8319.03,
				1792 * 1792: 15141.18,
			},
			// [DEBUG] ggml_extend.hpp:1034 - mmdit compute buffer size: 11215.71 MB(VRAM) // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 19654.65 MB(VRAM) // 1792*2048
			c: map[float64]float64{
				1536 * 1792: 11215.71,
				1792 * 2048: 19654.65,
			},
		},
		{
			n: "GuessSD35MediumDiffusionModelMemoryUsage",
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 41.48 MB(VRAM)     // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 181.64 MB(VRAM)    // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 1834.11 MB(VRAM)   // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 3896.36 MB(VRAM)   // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 5215.48 MB(VRAM)   // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 8427.03 MB(VRAM)   // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 15288.18 MiB(VRAM) // 1792*1792
			x2y: map[float64]float64{
				256 * 256:   41.48,
				512 * 512:   181.64,
				1024 * 1024: 1834.11,
				1024 * 1536: 3896.36,
				1024 * 1792: 5215.48,
				1536 * 1536: 8427.03,
				1792 * 1792: 15288.18,
			},
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 11341.71 MB(VRAM)  // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 19822.65 MB(VRAM)  // 1792*2048
			c: map[float64]float64{
				1536 * 1792: 11341.71,
				1792 * 2048: 19822.65,
			},
		},
		{
			n: "GuessSD35LargeDiffusionModelMemoryUsage",
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 57.27 MB(VRAM)     // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 276.54 MB(VRAM)    // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 2865.44 MB(VRAM)   // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 6109.95 MB(VRAM)   // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 8188.92 MB(VRAM)   // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 13258.86 MB(VRAM)  // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 24091.01 MiB(VRAM) // 1792*1792
			x2y: map[float64]float64{
				256 * 256:   57.27,
				512 * 512:   276.54,
				1024 * 1024: 2865.44,
				1024 * 1536: 6109.95,
				1024 * 1792: 8188.92,
				1536 * 1536: 13258.86,
				1792 * 1792: 24091.01,
			},
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 17859.31 MB(VRAM)  // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - mmdit compute buffer size: 31253.70 MB(VRAM)  // 1792*2048
			c: map[float64]float64{
				1536 * 1792: 17859.31,
				1792 * 2048: 31253.70,
			},
		},
		{
			n: "GuessFLUXDiffusionModelMemoryUsage",
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 103.35 MB(VRAM)     // 256*256
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 398.05 MB(VRAM)     // 512*512
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 2576.18 MB(VRAM)    // 1024*1024
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 4978.31 MB(VRAM)    // 1024*1536
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 6467.37 MB(VRAM)    // 1024*1792
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 10021.49 MB(VRAM)   // 1536*1536
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 17434.95 MB(VRAM)   // 1792*1792
			x2y: map[float64]float64{
				256 * 256:   103.35,
				512 * 512:   398.05,
				1024 * 1024: 2576.18,
				1024 * 1536: 4978.31,
				1024 * 1792: 6467.37,
				1536 * 1536: 10021.49,
				1792 * 1792: 17434.95,
			},
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 13191.09 MB(VRAM)  // 1536*1792
			// [DEBUG] ggml_extend.hpp:1031 - flux compute buffer size: 22266.81 MB(VRAM)  // 1792*2048
			c: map[float64]float64{
				1536 * 1792: 13191.09,
				1792 * 2048: 22266.81,
			},
		},
	}

	rs := make([]Regression, len(ts))
	for i, t := range ts {
		rs[i].Name = t.n
	}

	fmt.Println("Polynomial Regression For None Flash Attention")
	for i, t := range ts {
		pr := PolynomialRegression{
			Degree: 2,
		}

		xs := maps.Keys(t.x2y)
		sort.Float64s(xs)
		ys := make([]float64, len(xs))
		for j, x := range xs {
			ys[j] = t.x2y[x] * 1024 * 1024 // MB to B
		}
		pr.Fit(xs, ys)

		for x, y := range t.c {
			y_ := pr.Predict(x) / 1024 / 1024 // B to MB
			d := (y_ - y) / y * 100
			s := "+"
			if d < 0 {
				s = ""
			}
			c := ""
			if d > 10 {
				c = "?"
			}

			fmt.Printf("%50s: y': %10.2f | y: %10.2f | d: %10s%% %s\n", t.n, y_, y, s+strconv.FormatFloat(d, 'f', 6, 64), c)
		}

		rs[i].PolynomialRegression = &pr
	}

	fmt.Println("Linear Regression For Flash Attention")
	for i, t := range ts {
		if len(t.fax2y) == 0 {
			continue
		}

		lr := LinearRegression{}

		xs := maps.Keys(t.fax2y)
		sort.Float64s(xs)
		ys := make([]float64, len(xs))
		for j, x := range xs {
			ys[j] = t.fax2y[x] * 1024 * 1024 // MB to B
		}
		lr.Fit(xs, ys)

		for x, y := range t.fac {
			y_ := lr.Predict(x) / 1024 / 1024 // B to MB
			d := (y_ - y) / y * 100
			s := "+"
			if d < 0 {
				s = ""
			}
			c := ""
			if d > 10 {
				c = "?"
			}

			fmt.Printf("%50s: y': %10.2f | y: %10.2f | d: %10s%% %s\n", t.n, y_, y, s+strconv.FormatFloat(d, 'f', 6, 64), c)
		}

		rs[i].LinearRegression = &lr
	}

	var code []byte
	{
		var (
			buff bytes.Buffer
			err  error
		)
		tmpl := template.Must(template.New("tmpl").Parse(tmplStr))
		if err = tmpl.Execute(&buff, rs); err != nil {
			panic(fmt.Errorf("failed to execute template: %w", err))
		}
		code, err = format.Source(buff.Bytes())
		if err != nil {
			panic(fmt.Errorf("failed to format source: %w", err))
		}
	}

	if err := os.WriteFile(output, code, 0644); err != nil {
		panic(fmt.Errorf("failed to write file: %w", err))
	}
}

func main() {
	DiffusionModelMemoryUsageRegression("zz_generated.diffusion_model_memory_usage.regression.go")
}
