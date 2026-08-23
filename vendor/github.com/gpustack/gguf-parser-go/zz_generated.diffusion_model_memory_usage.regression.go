package gguf_parser

import "math"

// GuessSD1DiffusionModelMemoryUsage returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func GuessSD1DiffusionModelMemoryUsage(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{7876368.5672, 161.4230198633, 0.0078124893}
	degree := 2
	x := float64(width * height)

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}

// GuessSD2DiffusionModelMemoryUsage returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func GuessSD2DiffusionModelMemoryUsage(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{-355043979.0562, -1193.3271458642, 0.0054023818}
	degree := 2
	x := float64(width * height)

	if flashAttention {
		coefficients = []float64{3780681.28078, 513.2102510935}
		degree = 1
	}

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}

// GuessSDXLDiffusionModelMemoryUsage returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func GuessSDXLDiffusionModelMemoryUsage(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{55541290.3893, 138.3196116655, 0.0006109455}
	degree := 2
	x := float64(width * height)

	if flashAttention {
		coefficients = []float64{-5958802.78052, 500.0687898915}
		degree = 1
	}

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}

// GuessSDXLRefinerDiffusionModelMemoryUsage returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func GuessSDXLRefinerDiffusionModelMemoryUsage(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{49395992.3449, 155.2477810191, 0.0007351736}
	degree := 2
	x := float64(width * height)

	if flashAttention {
		coefficients = []float64{7031343.31998, 599.4137437227}
		degree = 1
	}

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}

// GuessSD3MediumDiffusionModelMemoryUsage returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func GuessSD3MediumDiffusionModelMemoryUsage(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{16529921.3700, 234.6656247718, 0.0014648995}
	degree := 2
	x := float64(width * height)

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}

// GuessSD35MediumDiffusionModelMemoryUsage returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func GuessSD35MediumDiffusionModelMemoryUsage(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{17441103.4726, 281.6956819806, 0.0014651233}
	degree := 2
	x := float64(width * height)

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}

// GuessSD35LargeDiffusionModelMemoryUsage returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func GuessSD35LargeDiffusionModelMemoryUsage(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{23204369.2029, 410.3731196298, 0.0023195947}
	degree := 2
	x := float64(width * height)

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}

// GuessFLUXDiffusionModelMemoryUsage returns the memory usage in bytes for the given width and height,
// which is calculated by linear regression or polynomial regression.
func GuessFLUXDiffusionModelMemoryUsage(width, height uint32, flashAttention bool) uint64 {
	coefficients := []float64{46511668.6742, 997.7758807792, 0.0014573393}
	degree := 2
	x := float64(width * height)

	y := float64(0)
	for i := 0; i <= degree; i++ {
		y += coefficients[i] * math.Pow(x, float64(i))
	}
	return uint64(y)
}
