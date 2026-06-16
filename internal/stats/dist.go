// Package stats provides a small, dependency-free statistics engine for the
// "statistical assistant" feature (P3): descriptive statistics and the common
// inferential tests (t-test, Pearson correlation, one-way ANOVA, Cronbach's
// alpha). p-values come from the regularized incomplete beta function, so they
// are accurate rather than approximated.
package stats

import "math"

// betacf evaluates the continued fraction for the incomplete beta function
// (Numerical Recipes). It converges for x < (a+1)/(a+b+2).
func betacf(a, b, x float64) float64 {
	const (
		maxIter = 300
		eps     = 3e-14
		fpmin   = 1e-300
	)
	qab := a + b
	qap := a + 1
	qam := a - 1
	c := 1.0
	d := 1 - qab*x/qap
	if math.Abs(d) < fpmin {
		d = fpmin
	}
	d = 1 / d
	h := d
	for m := 1; m <= maxIter; m++ {
		mf := float64(m)
		m2 := 2 * mf
		aa := mf * (b - mf) * x / ((qam + m2) * (a + m2))
		d = 1 + aa*d
		if math.Abs(d) < fpmin {
			d = fpmin
		}
		c = 1 + aa/c
		if math.Abs(c) < fpmin {
			c = fpmin
		}
		d = 1 / d
		h *= d * c
		aa = -(a + mf) * (qab + mf) * x / ((a + m2) * (qap + m2))
		d = 1 + aa*d
		if math.Abs(d) < fpmin {
			d = fpmin
		}
		c = 1 + aa/c
		if math.Abs(c) < fpmin {
			c = fpmin
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < eps {
			break
		}
	}
	return h
}

// regIncBeta is the regularized incomplete beta function I_x(a, b).
func regIncBeta(a, b, x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	la, _ := math.Lgamma(a + b)
	lb, _ := math.Lgamma(a)
	lc, _ := math.Lgamma(b)
	bt := math.Exp(la - lb - lc + a*math.Log(x) + b*math.Log(1-x))
	if x < (a+1)/(a+b+2) {
		return bt * betacf(a, b, x) / a
	}
	return 1 - bt*betacf(b, a, 1-x)/b
}

// tDistPTwoTailed returns the two-tailed p-value P(|T| > |t|) for the Student's
// t distribution with df degrees of freedom.
func tDistPTwoTailed(t, df float64) float64 {
	if df <= 0 {
		return math.NaN()
	}
	return regIncBeta(df/2, 0.5, df/(df+t*t))
}

// fDistPUpper returns the upper-tail p-value P(F > f) for the F distribution
// with (d1, d2) degrees of freedom.
func fDistPUpper(f, d1, d2 float64) float64 {
	if f <= 0 || d1 <= 0 || d2 <= 0 {
		return math.NaN()
	}
	return regIncBeta(d2/2, d1/2, d2/(d2+d1*f))
}
