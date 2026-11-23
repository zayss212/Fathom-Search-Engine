package trigram

import "math/bits"

const (
	space = 1 << 24
	words = space / 64
)

type Set struct {
	bits [words]uint64
}

func NewSet() *Set {
	return &Set{}
}

func (s *Set) Add(t uint32) {
	s.bits[t>>6] |= 1 << (t & 63)
}

func (s *Set) Or(other *Set) {
	for i := range s.bits {
		s.bits[i] |= other.bits[i]
	}
}

func (s *Set) ToSlice() []uint32 {
	out := make([]uint32, 0, s.count())
	for i, w := range s.bits {
		for w != 0 {
			tz := bits.TrailingZeros64(w)
			out = append(out, uint32(i*64+tz))
			w &= w - 1
		}
	}
	return out
}

func (s *Set) count() int {
	n := 0
	for _, w := range s.bits {
		n += bits.OnesCount64(w)
	}
	return n
}
