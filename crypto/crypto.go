package crypto

import "crypto/sha256"

const HashSize int = 32

type Digest [HashSize]byte

// Hasher
type Hasher struct {
	data []byte
}

func NewHasher() *Hasher {
	return &Hasher{
		data: nil,
	}
}

func (h *Hasher) Add(data []byte) *Hasher {
	h.data = append(h.data, data...)
	return h
}

func (h *Hasher) Sum256(data []byte) Digest {
	defer func() {
		h.data = nil
	}()
	return sha256.Sum256(append(h.data, data...))
}

type PublickKey struct {
	Pubkey string
}

type PrivateKey struct {
	PriKey string
}
