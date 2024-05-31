package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"

	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/tbls"
)

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
	Pubkey ed25519.PublicKey
}

type PrivateKey struct {
	Prikey ed25519.PrivateKey
}

type SecretShareKey struct {
	PubPoly  *share.PubPoly
	PriShare *share.PriShare
	N        int
	T        int
}

type Signature struct {
	Sig []byte
}

func (s *Signature) Verify(pub PublickKey, digest Digest) bool {
	return ed25519.Verify(pub.Pubkey, digest[:], s.Sig)
}

type SignatureShare struct {
	PartialSig []byte
}

func (s *SignatureShare) Verify(Digest) bool {
	return true
}

func VerifyTs(skey SecretShareKey, digest Digest, intactSig []byte) error {
	suite := bn256.NewSuite()
	err := bls.Verify(suite, skey.PubPoly.Commit(), digest[:], intactSig)
	return err
}

// CombineIntactTSPartial assembles the intact threshold signature.
func CombineIntactTSPartial(sigShares []SignatureShare, skey SecretShareKey, digest Digest) ([]byte, error) {
	suite := bn256.NewSuite()
	var partialSigs [][]byte
	for _, sig := range sigShares {
		partialSigs = append(partialSigs, sig.PartialSig)
	}
	return tbls.Recover(suite, skey.PubPoly, digest[:], partialSigs, skey.T, skey.N)
}

type sigReq struct {
	typ    int
	digest Digest
	ret    any
	err    error
	Done   chan *sigReq
}

func (s *sigReq) done() {
	s.Done <- s
}

const (
	SIG = iota
	SHARE
)

type SigService struct {
	PriKey   PrivateKey
	ShareKey SecretShareKey
	reqCh    chan *sigReq
}

func NewSigService(pri PrivateKey, share SecretShareKey) *SigService {
	srvc := &SigService{
		PriKey:   pri,
		ShareKey: share,
		reqCh:    make(chan *sigReq, 100),
	}
	go func() {
		for req := range srvc.reqCh {
			switch req.typ {
			case SIG:
				sig := ed25519.Sign(srvc.PriKey.Prikey, req.digest[:])
				req.ret = Signature{
					Sig: sig,
				}
				req.done()
			case SHARE:
				go func(r *sigReq) {
					suite := bn256.NewSuite()
					partialSig, err := tbls.Sign(suite, srvc.ShareKey.PriShare, r.digest[:])
					r.ret = SignatureShare{
						partialSig,
					}
					r.err = err
					r.done()
				}(req)
			}
		}
	}()
	return srvc
}

func (s *SigService) RequestSignature(digest Digest) (Signature, error) {
	req := &sigReq{
		typ:    SIG,
		digest: digest,
		Done:   make(chan *sigReq, 1),
	}
	s.reqCh <- req
	<-req.Done
	sig, _ := req.ret.(Signature)
	return sig, nil
}

func (s *SigService) RequestTsSugnature(digest Digest) (SignatureShare, error) {
	req := &sigReq{
		typ:    SHARE,
		digest: digest,
		Done:   make(chan *sigReq, 1),
	}
	s.reqCh <- req
	<-req.Done
	sig, _ := req.ret.(SignatureShare)
	return sig, req.err
}

func GenED25519Keys() (ed25519.PrivateKey, ed25519.PublicKey) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	return privKey, pubKey
}

// GenTSKeys generate private and public keys for threshold signature.
// @t: the number of threshold (e.g., 2f+1 in BFT).
// @return: []*share.PriShare includes t private keys, each of which is assigned to a peer.
// @return: *share.PubPoly is the public key, the same public key is assigned to each peer.
func GenTSKeys(t, n int) ([]*share.PriShare, *share.PubPoly) {
	suite := bn256.NewSuite()
	secret := suite.G1().Scalar().Pick(suite.RandomStream())
	priPoly := share.NewPriPoly(suite.G2(), t, secret, suite.RandomStream())
	pubPoly := priPoly.Commit(suite.G2().Point().Base())
	shares := priPoly.Shares(n)
	return shares, pubPoly
}

// encode encodes the data into bytes.
// Data can be of any type.
// Examples can be seen form the tests.
func encode(data interface{}) ([]byte, error) {
	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decode decodes bytes into the data.
// Data should be passed in the format of a pointer to a type.
// Examples can be seen form the tests.
func decode(s []byte, data interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(s))
	if err := dec.Decode(data); err != nil {
		return err
	}
	return nil
}

// TSPartialMarshalled defines an intermediate type to marshall the partial key,
// since private share of a threshold signature cannot be encoded as bytes directly.
type TSPartialMarshalled struct {
	I      int
	Binary []byte
}

// MarshallTSPartialKey marshalls the V field in the private share as bytes,
// and create the intermediate type: TSPartialMarshalled.
func MarshallTSPartialKey(priShare *share.PriShare) (*TSPartialMarshalled, error) {
	shareAsBytes, err := priShare.V.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &TSPartialMarshalled{
		I:      priShare.I,
		Binary: shareAsBytes,
	}, nil
}

// UnMarshallTSPartialKey unmarshalls the V field in the private share from TSPartialMarshalled.Binary.
func UnMarshallTSPartialKey(par *TSPartialMarshalled) (*share.PriShare, error) {
	suite := bn256.NewSuite()
	shareDecoded := &share.PriShare{
		I: par.I,
		V: suite.G2().Scalar(),
	}
	err := shareDecoded.V.UnmarshalBinary(par.Binary)
	if err != nil {
		return nil, err
	}
	return shareDecoded, nil
}

// EncodeTSPartialKey encodes the private share of a threshold signature.
func EncodeTSPartialKey(priShare *share.PriShare) ([]byte, error) {
	mtspk, err := MarshallTSPartialKey(priShare)
	if err != nil {
		return nil, err
	}
	return encode(mtspk)
}

// DecodeTSPartialKey decodes the private share of a threshold signature.
func DecodeTSPartialKey(data []byte) (*share.PriShare, error) {
	var par TSPartialMarshalled
	if err := decode(data, &par); err != nil {
		return nil, err
	}
	return UnMarshallTSPartialKey(&par)
}

// EqualTSPartialKey compares if two private shares equal.
func EqualTSPartialKey(p1, p2 *share.PriShare) bool {
	suite := bn256.NewSuite()
	return p1.I == p2.I && bytes.Equal(p1.Hash(suite), p2.Hash(suite))
}

// TSPublicMarshalled defines an intermediate type to marshall the public key,
// since public key of a threshold signature cannot be encoded as bytes directly.
type TSPublicMarshalled struct {
	BaseBytes   []byte
	CommitBytes [][]byte
}

// MarshallTSPublicKey marshalls the V field in the private share as bytes,
// and create the intermediate type: TSPartialMarshalled.
func MarshallTSPublicKey(pubKey *share.PubPoly) (*TSPublicMarshalled, error) {
	base, commits := pubKey.Info()

	baseAsBytes, err := base.MarshalBinary()
	if err != nil {
		return nil, err
	}

	commitCount := len(commits)
	commitBytes := make([][]byte, commitCount)
	for i, commit := range commits {
		commitBytes[i], err = commit.MarshalBinary()
		if err != nil {
			return nil, err
		}
	}

	return &TSPublicMarshalled{
		BaseBytes:   baseAsBytes,
		CommitBytes: commitBytes,
	}, nil
}

// UnMarshallTSPublicKey unmarshalls the public key from TSPublicMarshalled.
func UnMarshallTSPublicKey(tspm *TSPublicMarshalled) (*share.PubPoly, error) {
	baseDecoded := bn256.NewSuite().G2().Point()
	err := baseDecoded.UnmarshalBinary(tspm.BaseBytes)
	if err != nil {
		return nil, err
	}
	commitsDecoded := make([]kyber.Point, len(tspm.CommitBytes))
	for i, cb := range tspm.CommitBytes {
		tmp := bn256.NewSuite().G2().Point()
		err = tmp.UnmarshalBinary(cb)
		if err != nil {
			return nil, err
		}
		commitsDecoded[i] = tmp
	}

	return share.NewPubPoly(bn256.NewSuite().G2(), baseDecoded, commitsDecoded), nil
}

// EncodeTSPublicKey encodes the public key of a threshold signature.
func EncodeTSPublicKey(pubkey *share.PubPoly) ([]byte, error) {
	tspm, err := MarshallTSPublicKey(pubkey)
	if err != nil {
		return nil, err
	}

	return encode(tspm)
}

// DecodeTSPublicKey decodes the public key of a threshold signature.
func DecodeTSPublicKey(data []byte) (*share.PubPoly, error) {
	var tspm TSPublicMarshalled
	if err := decode(data, &tspm); err != nil {
		return nil, err
	}
	return UnMarshallTSPublicKey(&tspm)
}

func EncodePublicKey(pub PublickKey) []byte {
	byt := make([]byte, 2*len(pub.Pubkey))
	hex.Encode(byt, pub.Pubkey)
	return byt
}

func DecodePublicKey(data []byte) (PublickKey, error) {
	pub := make([]byte, len(data)/2)
	_, err := hex.Decode(pub, data)
	if err != nil {
		return PublickKey{}, err
	}
	return PublickKey{
		Pubkey: pub,
	}, nil
}

func EncodePrivateKey(pri PrivateKey) []byte {
	byt := make([]byte, 2*len(pri.Prikey))
	hex.Encode(byt, pri.Prikey)
	return byt
}

func DecodePrivateKey(data []byte) (PrivateKey, error) {
	pri := make([]byte, len(data)/2)
	_, err := hex.Decode(pri, data)
	if err != nil {
		return PrivateKey{}, err
	}
	return PrivateKey{
		Prikey: pri,
	}, nil
}
