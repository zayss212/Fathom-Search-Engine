package trigram

import "io"

func Extract(data []byte) []uint32 {
	s := NewSet()
	addAll(s, data)
	return s.ToSlice()
}

func addAll(s *Set, data []byte) {
	if len(data) < 3 {
		return
	}
	window := uint32(data[0])<<8 | uint32(data[1])
	for i := 2; i < len(data); i++ {
		window = (window<<8 | uint32(data[i])) & 0xFFFFFF
		s.Add(window)
	}
}

func ExtractReader(r io.Reader) ([]uint32, error) {
	s := NewSet()
	if err := extractInto(s, r); err != nil {
		return nil, err
	}
	return s.ToSlice(), nil
}

func extractInto(s *Set, r io.Reader) error {
	const chunkSize = 4 << 20
	buf := make([]byte, chunkSize)
	var carry [2]byte
	carryLen := 0

	for {
		n, err := r.Read(buf[carryLen:])
		if n > 0 {
			window := buf[:carryLen+n]
			addAll(s, window)
			if len(window) >= 2 {
				carry[0], carry[1] = window[len(window)-2], window[len(window)-1]
				carryLen = 2
			} else {
				carryLen = len(window)
				copy(carry[:], window)
			}
			copy(buf[:carryLen], carry[:carryLen])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func Pack(a, b, c byte) uint32 {
	return uint32(a)<<16 | uint32(b)<<8 | uint32(c)
}

func OfString(s string) []uint32 {
	b := []byte(s)
	if len(b) < 3 {
		return nil
	}
	out := make([]uint32, 0, len(b)-2)
	for i := 0; i+2 < len(b); i++ {
		out = append(out, Pack(b[i], b[i+1], b[i+2]))
	}
	return out
}
