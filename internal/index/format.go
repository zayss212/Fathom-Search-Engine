package index

var magic = [4]byte{'F', 'T', 'H', '1'}

const version = uint32(3)

type DocInfo struct {
	Path    string
	Size    uint64
	ModTime int64
}
