package index

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"fathom/internal/bitset"
)

type Index struct {
	Docs     []DocInfo
	postings map[uint32]*bitset.Bitmap
}

func Open(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 1<<20)

	var got [4]byte
	if _, err := io.ReadFull(r, got[:]); err != nil {
		return nil, err
	}
	if got != magic {
		return nil, fmt.Errorf("fathom: not an index file (bad magic)")
	}
	v, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if v != version {
		return nil, fmt.Errorf("fathom: unsupported index version %d", v)
	}

	numDocs, err := readU32(r)
	if err != nil {
		return nil, err
	}
	docs := make([]DocInfo, numDocs)
	for i := range docs {
		pathLen, err := readU32(r)
		if err != nil {
			return nil, err
		}
		pathBuf := make([]byte, pathLen)
		if _, err := io.ReadFull(r, pathBuf); err != nil {
			return nil, err
		}
		size, err := readU64(r)
		if err != nil {
			return nil, err
		}
		modTime, err := readU64(r)
		if err != nil {
			return nil, err
		}
		docs[i] = DocInfo{Path: string(pathBuf), Size: size, ModTime: int64(modTime)}
	}

	numTrigrams, err := readU32(r)
	if err != nil {
		return nil, err
	}
	postings := make(map[uint32]*bitset.Bitmap, numTrigrams)
	for i := uint32(0); i < numTrigrams; i++ {
		key, err := readU32(r)
		if err != nil {
			return nil, err
		}
		numChunks, err := readU32(r)
		if err != nil {
			return nil, err
		}
		chunks := make([]bitset.Chunk, numChunks)
		for j := range chunks {
			c, err := readChunk(r)
			if err != nil {
				return nil, err
			}
			chunks[j] = c
		}
		postings[key] = bitset.NewFromChunks(chunks)
	}

	return &Index{Docs: docs, postings: postings}, nil
}

func readChunk(r *bufio.Reader) (bitset.Chunk, error) {
	key, err := readU16(r)
	if err != nil {
		return bitset.Chunk{}, err
	}
	kind, err := r.ReadByte()
	if err != nil {
		return bitset.Chunk{}, err
	}
	if kind == 0 {
		count, err := readU32(r)
		if err != nil {
			return bitset.Chunk{}, err
		}
		values := make([]uint16, count)
		for i := range values {
			v, err := readU16(r)
			if err != nil {
				return bitset.Chunk{}, err
			}
			values[i] = v
		}
		return bitset.NewArrayChunk(key, values), nil
	}

	var words [bitset.BitmapWords]uint64
	for i := range words {
		w, err := readU64(r)
		if err != nil {
			return bitset.Chunk{}, err
		}
		words[i] = w
	}
	return bitset.NewBitmapChunk(key, words), nil
}

func (idx *Index) Postings(t uint32) []uint32 {
	bm := idx.postings[t]
	if bm == nil {
		return nil
	}
	return bm.ToSlice()
}

func (idx *Index) PostingsBitmap(t uint32) *bitset.Bitmap {
	return idx.postings[t]
}

func (idx *Index) TrigramsByDoc() map[uint32][]uint32 {
	out := make(map[uint32][]uint32, len(idx.Docs))
	for t, bm := range idx.postings {
		for _, docID := range bm.ToSlice() {
			out[docID] = append(out[docID], t)
		}
	}
	return out
}

func readU16(r *bufio.Reader) (uint16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(buf[:]), nil
}

func readU32(r *bufio.Reader) (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

func readU64(r *bufio.Reader) (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}
