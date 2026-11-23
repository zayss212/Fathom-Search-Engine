package bitset

import "math/bits"

const (
	arrayMaxCard = 4096
	BitmapWords  = 1024
)

type container interface {
	card() int
}

type arrayContainer []uint16

func (a arrayContainer) card() int { return len(a) }

type bitmapContainer [BitmapWords]uint64

func (b *bitmapContainer) card() int {
	n := 0
	for _, w := range b {
		n += bits.OnesCount64(w)
	}
	return n
}

type Bitmap struct {
	keys       []uint16
	containers []container
}

func FromSorted(ids []uint32) *Bitmap {
	bm := &Bitmap{}
	i := 0
	for i < len(ids) {
		high := uint16(ids[i] >> 16)
		j := i
		for j < len(ids) && uint16(ids[j]>>16) == high {
			j++
		}
		chunk := ids[i:j]
		bm.keys = append(bm.keys, high)
		bm.containers = append(bm.containers, buildContainer(chunk))
		i = j
	}
	return bm
}

func buildContainer(chunk []uint32) container {
	if len(chunk) <= arrayMaxCard {
		arr := make(arrayContainer, len(chunk))
		for i, v := range chunk {
			arr[i] = uint16(v)
		}
		return arr
	}
	bm := &bitmapContainer{}
	for _, v := range chunk {
		low := uint16(v)
		bm[low/64] |= 1 << (low % 64)
	}
	return bm
}

func And(a, b *Bitmap) *Bitmap {
	out := &Bitmap{}
	i, j := 0, 0
	for i < len(a.keys) && j < len(b.keys) {
		switch {
		case a.keys[i] < b.keys[j]:
			i++
		case a.keys[i] > b.keys[j]:
			j++
		default:
			c := andContainers(a.containers[i], b.containers[j])
			if c.card() > 0 {
				out.keys = append(out.keys, a.keys[i])
				out.containers = append(out.containers, c)
			}
			i++
			j++
		}
	}
	return out
}

func andContainers(a, b container) container {
	aArr, aIsArr := a.(arrayContainer)
	bArr, bIsArr := b.(arrayContainer)

	if aIsArr && bIsArr {
		return andArrayArray(aArr, bArr)
	}
	if aIsArr {
		return andArrayBitmap(aArr, b.(*bitmapContainer))
	}
	if bIsArr {
		return andArrayBitmap(bArr, a.(*bitmapContainer))
	}
	return andBitmapBitmap(a.(*bitmapContainer), b.(*bitmapContainer))
}

func andArrayArray(a, b arrayContainer) container {
	out := make(arrayContainer, 0, min16(len(a), len(b)))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return out
}

func andArrayBitmap(arr arrayContainer, bm *bitmapContainer) container {
	out := make(arrayContainer, 0, len(arr))
	for _, v := range arr {
		if bm[v/64]&(1<<(v%64)) != 0 {
			out = append(out, v)
		}
	}
	return out
}

func andBitmapBitmap(a, b *bitmapContainer) container {
	out := &bitmapContainer{}
	for i := range a {
		out[i] = a[i] & b[i]
	}
	if out.card() <= arrayMaxCard {
		return bitmapToArray(out)
	}
	return out
}

func bitmapToArray(bm *bitmapContainer) arrayContainer {
	var out arrayContainer
	for w, word := range bm {
		for word != 0 {
			tz := bits.TrailingZeros64(word)
			out = append(out, uint16(w*64+tz))
			word &= word - 1
		}
	}
	return out
}

func (bm *Bitmap) ToSlice() []uint32 {
	var out []uint32
	for idx, c := range bm.containers {
		high := uint32(bm.keys[idx]) << 16
		switch cc := c.(type) {
		case arrayContainer:
			for _, v := range cc {
				out = append(out, high|uint32(v))
			}
		case *bitmapContainer:
			for w, word := range cc {
				for word != 0 {
					tz := bits.TrailingZeros64(word)
					out = append(out, high|uint32(w*64+tz))
					word &= word - 1
				}
			}
		}
	}
	return out
}

func (bm *Bitmap) Chunks() []Chunk {
	out := make([]Chunk, len(bm.keys))
	for i, k := range bm.keys {
		out[i] = Chunk{Key: k, container: bm.containers[i]}
	}
	return out
}

type Chunk struct {
	Key       uint16
	container container
}

func NewArrayChunk(key uint16, values []uint16) Chunk {
	return Chunk{Key: key, container: arrayContainer(values)}
}

func NewBitmapChunk(key uint16, words [BitmapWords]uint64) Chunk {
	bm := bitmapContainer(words)
	return Chunk{Key: key, container: &bm}
}

func (c Chunk) Array() ([]uint16, bool) {
	a, ok := c.container.(arrayContainer)
	return []uint16(a), ok
}

func (c Chunk) Words() ([]uint64, bool) {
	b, ok := c.container.(*bitmapContainer)
	if !ok {
		return nil, false
	}
	return b[:], true
}

func NewFromChunks(chunks []Chunk) *Bitmap {
	bm := &Bitmap{keys: make([]uint16, len(chunks)), containers: make([]container, len(chunks))}
	for i, c := range chunks {
		bm.keys[i] = c.Key
		bm.containers[i] = c.container
	}
	return bm
}

func min16(a, b int) int {
	if a < b {
		return a
	}
	return b
}
