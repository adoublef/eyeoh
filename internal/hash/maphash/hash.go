package maphash

import (
	"hash/maphash"
)

type Hasher struct {
	h *maphash.Hash
}

func (h *Hasher) Hash(p []byte) uint64 {
	defer h.h.Reset()
	// gob encoding or protobuf would allow generics
	h.h.Write(p)
	return h.h.Sum64()
}

func (b *Hasher) Reset() {
	if b.h == nil {
		b.h = &maphash.Hash{}
	}
	b.h.SetSeed(maphash.MakeSeed())
}
