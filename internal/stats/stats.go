package stats

import (
	"errors"
	"math"
	"sort"
)

// ErrInsufficientData is returned when a sample is too small for a computation.
var ErrInsufficientData = errors.New("stats: insufficient data")

// ErrLengthMismatch is returned when paired inputs differ in length.
var ErrLengthMismatch = errors.New("stats: inputs must have equal length")

// ErrNoVariance is returned when a computation needs non-zero variance.
var ErrNoVariance = errors.New("stats: zero variance")

// Descriptive summarizes a single sample.
type Descriptive struct {
	N      int
	Mean   float64
	SD     float64 // sample standard deviation (n-1)
	Median float64
	Min    float64
	Max    float64
}

func mean(x []float64) float64 {
	s := 0.0
	for _, v := range x {
		s += v
	}
	return s / float64(len(x))
}

// sampleVar is the unbiased (n-1) variance. len(x) must be >= 2.
func sampleVar(x []float64, m float64) float64 {
	s := 0.0
	for _, v := range x {
		d := v - m
		s += d * d
	}
	return s / float64(len(x)-1)
}

// Describe returns descriptive statistics for x.
func Describe(x []float64) (Descriptive, error) {
	if len(x) < 2 {
		return Descriptive{}, ErrInsufficientData
	}
	m := mean(x)
	v := sampleVar(x, m)

	sorted := append([]float64(nil), x...)
	sort.Float64s(sorted)
	n := len(sorted)
	var median float64
	if n%2 == 1 {
		median = sorted[n/2]
	} else {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	}

	return Descriptive{
		N:      n,
		Mean:   m,
		SD:     math.Sqrt(v),
		Median: median,
		Min:    sorted[0],
		Max:    sorted[n-1],
	}, nil
}

// TTest holds the result of an independent-samples (Welch) t-test.
type TTest struct {
	N1, N2       int
	Mean1, Mean2 float64
	T            float64
	DF           float64
	P            float64 // two-tailed
}

// IndependentTTest runs Welch's two-sample t-test (does not assume equal
// variances), which is the robust default.
func IndependentTTest(a, b []float64) (TTest, error) {
	if len(a) < 2 || len(b) < 2 {
		return TTest{}, ErrInsufficientData
	}
	m1, m2 := mean(a), mean(b)
	v1, v2 := sampleVar(a, m1), sampleVar(b, m2)
	n1, n2 := float64(len(a)), float64(len(b))

	se := v1/n1 + v2/n2
	if se == 0 {
		return TTest{}, ErrNoVariance
	}
	t := (m1 - m2) / math.Sqrt(se)
	df := (se * se) / ((v1/n1)*(v1/n1)/(n1-1) + (v2/n2)*(v2/n2)/(n2-1))

	return TTest{
		N1: len(a), N2: len(b),
		Mean1: m1, Mean2: m2,
		T:  t,
		DF: df,
		P:  tDistPTwoTailed(math.Abs(t), df),
	}, nil
}

// Correlation holds a Pearson correlation result.
type Correlation struct {
	N int
	R float64
	T float64
	P float64 // two-tailed
}

// Pearson computes the Pearson correlation coefficient and its significance.
func Pearson(x, y []float64) (Correlation, error) {
	if len(x) != len(y) {
		return Correlation{}, ErrLengthMismatch
	}
	if len(x) < 3 {
		return Correlation{}, ErrInsufficientData
	}
	mx, my := mean(x), mean(y)
	var sxy, sxx, syy float64
	for i := range x {
		dx, dy := x[i]-mx, y[i]-my
		sxy += dx * dy
		sxx += dx * dx
		syy += dy * dy
	}
	if sxx == 0 || syy == 0 {
		return Correlation{}, ErrNoVariance
	}
	r := sxy / math.Sqrt(sxx*syy)
	r = math.Max(-1, math.Min(1, r))
	n := float64(len(x))

	res := Correlation{N: len(x), R: r}
	if r2 := r * r; r2 < 1 {
		res.T = r * math.Sqrt((n-2)/(1-r2))
		res.P = tDistPTwoTailed(math.Abs(res.T), n-2)
	} else {
		res.T = math.Inf(1)
		res.P = 0
	}
	return res, nil
}

// ANOVA holds a one-way ANOVA result.
type ANOVA struct {
	K         int
	F         float64
	DFBetween float64
	DFWithin  float64
	SSBetween float64
	SSWithin  float64
	P         float64
}

// OneWayANOVA runs a one-way analysis of variance across groups.
func OneWayANOVA(groups [][]float64) (ANOVA, error) {
	if len(groups) < 2 {
		return ANOVA{}, ErrInsufficientData
	}
	var grandSum float64
	var total int
	for _, g := range groups {
		if len(g) < 1 {
			return ANOVA{}, ErrInsufficientData
		}
		for _, v := range g {
			grandSum += v
			total++
		}
	}
	if total <= len(groups) {
		return ANOVA{}, ErrInsufficientData
	}
	grand := grandSum / float64(total)

	var ssb, ssw float64
	for _, g := range groups {
		m := mean(g)
		ssb += float64(len(g)) * (m - grand) * (m - grand)
		for _, v := range g {
			ssw += (v - m) * (v - m)
		}
	}
	if ssw == 0 {
		return ANOVA{}, ErrNoVariance
	}
	dfB := float64(len(groups) - 1)
	dfW := float64(total - len(groups))
	f := (ssb / dfB) / (ssw / dfW)

	return ANOVA{
		K: len(groups), F: f,
		DFBetween: dfB, DFWithin: dfW,
		SSBetween: ssb, SSWithin: ssw,
		P: fDistPUpper(f, dfB, dfW),
	}, nil
}

// CronbachAlpha computes Cronbach's alpha for reliability. items[i] holds the
// scores for item i across all respondents; every item must have the same
// number of respondents.
func CronbachAlpha(items [][]float64) (float64, error) {
	k := len(items)
	if k < 2 {
		return 0, ErrInsufficientData
	}
	n := len(items[0])
	if n < 2 {
		return 0, ErrInsufficientData
	}

	var sumItemVar float64
	totals := make([]float64, n)
	for _, item := range items {
		if len(item) != n {
			return 0, ErrLengthMismatch
		}
		sumItemVar += sampleVar(item, mean(item))
		for r, v := range item {
			totals[r] += v
		}
	}
	totalVar := sampleVar(totals, mean(totals))
	if totalVar == 0 {
		return 0, ErrNoVariance
	}
	kf := float64(k)
	return (kf / (kf - 1)) * (1 - sumItemVar/totalVar), nil
}
