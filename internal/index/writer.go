package index

import (
	"bufio"
	"encoding/binary"
	"os"
	"sort"

	"fathom/internal/bitset"
)

func (b *Builder) Write(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, 1<<20)

	if _, err := w.Write(magic[:]); err != nil {
		return err
	}
	if err := writeU32(w, version); err != nil {
		return err
	}
	if err := writeU32(w, uint32(len(b.docs))); err != nil {
		return err
	}
	for _, d := range b.docs {
		if err := writeU32(w, uint32(len(d.Path))); err != nil {
			return err
		}
		if _, err := w.WriteString(d.Path); err != nil {
			return err
		}
		if err := writeU64(w, d.Size); err != nil {
			return err
		}
		if err := writeI64(w, d.ModTime); err != nil {
			return err
		}
	}

	keys := make([]uint32, 0, len(b.postings))
	for k := range b.postings {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	if err := writeU32(w, uint32(len(keys))); err != nil {
		return err
	}
	for _, k := range keys {
		if err := writeU32(w, k); err != nil {
			return err
		}
		chunks := bitset.FromSorted(b.postings[k]).Chunks()
		if err := writeU32(w, uint32(len(chunks))); err != nil {
			return err
		}
		for _, c := range chunks {
			if err := writeChunk(w, c); err != nil {
				return err
			}
		}
	}

	return w.Flush()
}

func writeChunk(w *bufio.Writer, c bitset.Chunk) error {
	if err := writeU16(w, c.Key); err != nil {
		return err
	}
	if values, ok := c.Array(); ok {
		if _, err := w.Write([]byte{0}); err != nil {
			return err
		}
		if err := writeU32(w, uint32(len(values))); err != nil {
			return err
		}
		for _, v := range values {
			if err := writeU16(w, v); err != nil {
				return err
			}
		}
		return nil
	}

	words, _ := c.Words()
	if _, err := w.Write([]byte{1}); err != nil {
		return err
	}
	for _, word := range words {
		if err := writeU64(w, word); err != nil {
			return err
		}
	}
	return nil
}

func writeU16(w *bufio.Writer, v uint16) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func writeU32(w *bufio.Writer, v uint32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func writeU64(w *bufio.Writer, v uint64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func writeI64(w *bufio.Writer, v int64) error {
	return writeU64(w, uint64(v))
}
