package stats

import (
	"math"
	"testing"
)

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.6f, want ~%.6f (tol %.g)", name, got, want, tol)
	}
}

func TestDescribe(t *testing.T) {
	d, err := Describe([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if err != nil {
		t.Fatal(err)
	}
	if d.N != 8 {
		t.Errorf("N = %d, want 8", d.N)
	}
	approx(t, "mean", d.Mean, 5, 1e-9)
	approx(t, "sd", d.SD, 2.138, 1e-3) // sample sd
	approx(t, "median", d.Median, 4.5, 1e-9)
	if d.Min != 2 || d.Max != 9 {
		t.Errorf("min/max = %v/%v", d.Min, d.Max)
	}
}

func TestDescribe_TooSmall(t *testing.T) {
	if _, err := Describe([]float64{1}); err != ErrInsufficientData {
		t.Errorf("err = %v, want ErrInsufficientData", err)
	}
}

func TestIndependentTTest(t *testing.T) {
	// Welch: A mean 7 var 2.5, B mean 5 var 2.5, n=5 each => t=2, df=8.
	res, err := IndependentTTest([]float64{5, 6, 7, 8, 9}, []float64{3, 4, 5, 6, 7})
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "t", res.T, 2.0, 1e-9)
	approx(t, "df", res.DF, 8.0, 1e-9)
	// Known two-tailed p for t=2.0, df=8 is ~0.0805.
	approx(t, "p", res.P, 0.0805, 5e-4)
}

func TestPearson_Perfect(t *testing.T) {
	res, err := Pearson([]float64{1, 2, 3, 4, 5}, []float64{2, 4, 6, 8, 10})
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "r", res.R, 1.0, 1e-9)
	approx(t, "p", res.P, 0.0, 1e-9)
}

func TestPearson_Known(t *testing.T) {
	// Textbook example: r ≈ 0.9746.
	res, err := Pearson([]float64{1, 2, 3, 4, 5}, []float64{2, 4, 5, 4, 5})
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "r", res.R, 0.7746, 1e-3)
}

func TestPearson_LengthMismatch(t *testing.T) {
	if _, err := Pearson([]float64{1, 2}, []float64{1}); err != ErrLengthMismatch {
		t.Errorf("err = %v, want ErrLengthMismatch", err)
	}
}

func TestOneWayANOVA(t *testing.T) {
	// Groups with means 2,5,8; SSB=54, SSW=6 => F=27, df 2/6.
	res, err := OneWayANOVA([][]float64{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}})
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "F", res.F, 27.0, 1e-9)
	approx(t, "dfB", res.DFBetween, 2, 1e-9)
	approx(t, "dfW", res.DFWithin, 6, 1e-9)
	// p should be small and well-defined.
	if !(res.P > 0 && res.P < 0.01) {
		t.Errorf("p = %v, want a small positive value", res.P)
	}
}

func TestCronbachAlpha_IdenticalItems(t *testing.T) {
	// Two identical items => alpha = 1.
	a, err := CronbachAlpha([][]float64{{1, 2, 3}, {1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "alpha", a, 1.0, 1e-9)
}

func TestCronbachAlpha_Known(t *testing.T) {
	// 3 items × 4 respondents, a moderately reliable set.
	items := [][]float64{
		{2, 3, 3, 4},
		{3, 3, 4, 5},
		{2, 4, 4, 5},
	}
	a, err := CronbachAlpha(items)
	if err != nil {
		t.Fatal(err)
	}
	if a < 0 || a > 1 {
		t.Errorf("alpha = %v, expected within (0,1) for this set", a)
	}
}

func TestRegIncBeta_Symmetry(t *testing.T) {
	// I_x(a,b) = 1 - I_{1-x}(b,a)
	x := regIncBeta(2, 3, 0.4)
	y := 1 - regIncBeta(3, 2, 0.6)
	approx(t, "betai symmetry", x, y, 1e-10)
}
