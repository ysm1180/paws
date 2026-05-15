package ui

// Region is a screen-cell rectangle. W/H are exclusive: X=0,W=3 covers
// columns 0,1,2. Zero is a valid Index, so callers needing "unset"
// semantics must use slice presence rather than Index == 0.
type Region struct {
	X, Y, W, H int
	Index      int
}

func (r Region) Contains(x, y int) bool {
	if r.W <= 0 || r.H <= 0 {
		return false
	}
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}
