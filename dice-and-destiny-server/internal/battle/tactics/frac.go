package tactics

import "fmt"

// Frac is an exact time in [0, 1]: Num/Den in lowest terms. Times are never
// rounded; comparisons cross-multiply in int64.
type Frac struct {
	Num int64 `json:"num"`
	Den int64 `json:"den"`
}

// F returns num/den in lowest terms (den > 0).
func F(num, den int64) Frac {
	if den == 0 {
		panic("tactics: zero denominator")
	}
	if den < 0 {
		num, den = -num, -den
	}
	g := gcd(abs64(num), den)
	if g > 1 {
		num, den = num/g, den/g
	}
	return Frac{Num: num, Den: den}
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// Zero and One bound the round.
var (
	Zero = Frac{0, 1}
	One  = Frac{1, 1}
)

// Cmp returns -1, 0 or 1 as f is before, at or after g.
func (f Frac) Cmp(g Frac) int {
	left, right := f.Num*g.Den, g.Num*f.Den
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}

func (f Frac) Less(g Frac) bool  { return f.Cmp(g) < 0 }
func (f Frac) Equal(g Frac) bool { return f.Cmp(g) == 0 }

// Add returns f + g in lowest terms.
func (f Frac) Add(g Frac) Frac { return F(f.Num*g.Den+g.Num*f.Den, f.Den*g.Den) }

// Float is for display only; rules never use it.
func (f Frac) Float() float64 { return float64(f.Num) / float64(f.Den) }

func (f Frac) String() string {
	if f.Den == 1 {
		return fmt.Sprintf("%d", f.Num)
	}
	return fmt.Sprintf("%d/%d", f.Num, f.Den)
}
