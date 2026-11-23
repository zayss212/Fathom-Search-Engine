package index

import "fathom/internal/trigram"

type Builder struct {
	docs     []DocInfo
	postings map[uint32][]uint32 // trigram -> ascending docIDs
}

func NewBuilder() *Builder {
	return &Builder{
		postings: make(map[uint32][]uint32),
	}
}

func (b *Builder) AddDocument(path string, size uint64, modTime int64, content []byte) uint32 {
	return b.AddDocumentTrigrams(path, size, modTime, trigram.Extract(content))
}

func (b *Builder) AddDocumentTrigrams(path string, size uint64, modTime int64, trigrams []uint32) uint32 {
	docID := uint32(len(b.docs))
	b.docs = append(b.docs, DocInfo{Path: path, Size: size, ModTime: modTime})

	for _, t := range trigrams {
		list := b.postings[t]
		if len(list) == 0 || list[len(list)-1] != docID {
			b.postings[t] = append(list, docID)
		}
	}
	return docID
}

func (b *Builder) NumDocs() int {
	return len(b.docs)
}
