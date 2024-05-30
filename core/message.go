package core

import (
	"lightDAG/crypto"
	"lightDAG/pool"
	"reflect"
	"strconv"
)

const (
	GRBCProposeType int = iota
	EchoType
	ReadyType
	ElectType
	PBCProposeType
)

type Message interface {
	MsgType() int
	Hash() crypto.Digest
}

type Block struct {
	Author    crypto.PublickKey
	Round     int
	Batch     pool.Batch
	Reference []crypto.Digest
}

func (b *Block) Hash() crypto.Digest {

	hasher := crypto.NewHasher()
	hasher.Add(b.Author.Pubkey)
	// strconv.AppendInt()
	hasher.Add(strconv.AppendInt(nil, int64(b.Round), 2))
	for _, tx := range b.Batch.Txs {
		hasher.Add(tx)
	}
	for _, d := range b.Reference {
		hasher.Add(d[:])
	}

	return hasher.Sum256(nil)
}

// ProposeMsg
type GRBCProposeMsg struct {
	Author    crypto.PublickKey
	Round     int
	B         *Block
	Signature crypto.Signature
}

func NewGRBCProposeMsg(
	Author crypto.PublickKey,
	Round int,
	B *Block,
	sigService *crypto.SigService,
) (*GRBCProposeMsg, error) {

	msg := &GRBCProposeMsg{
		Author: Author,
		Round:  Round,
		B:      B,
	}

	if sig, err := sigService.RequestSignature(msg.Hash()); err != nil {
		return nil, err
	} else {
		msg.Signature = sig
		return msg, nil
	}
}

func (msg *GRBCProposeMsg) Verify() bool {
	return msg.Signature.Verify(msg.Author, msg.Hash())
}

func (msg *GRBCProposeMsg) Hash() crypto.Digest {

	hasher := crypto.NewHasher()
	hasher.Add(msg.Author.Pubkey)
	hasher.Add(strconv.AppendInt(nil, int64(msg.Round), 2))
	digest := msg.B.Hash()
	hasher.Add(digest[:])
	return hasher.Sum256(nil)
}

func (msg *GRBCProposeMsg) MsgType() int {
	return GRBCProposeType
}

// EchoMsg
type EchoMsg struct {
	Author    crypto.PublickKey
	Proposer  crypto.PublickKey
	BlockHash crypto.Digest
	Round     int
	Signature crypto.Signature
}

func NewEchoMsg(
	Author crypto.PublickKey,
	Proposer crypto.PublickKey,
	BlockHash crypto.Digest,
	Round int,
	sigService *crypto.SigService,
) (*EchoMsg, error) {
	msg := &EchoMsg{
		Author:    Author,
		Proposer:  Proposer,
		BlockHash: BlockHash,
		Round:     Round,
	}
	sig, err := sigService.RequestSignature(msg.Hash())
	if err != nil {
		return nil, err
	}
	msg.Signature = sig
	return msg, nil
}

func (msg *EchoMsg) Verify() bool {
	return msg.Signature.Verify(msg.Author, msg.Hash())
}

func (msg *EchoMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(msg.Author.Pubkey)
	hasher.Add(msg.Proposer.Pubkey)
	hasher.Add(msg.BlockHash[:])
	hasher.Add(strconv.AppendInt(nil, int64(msg.Round), 2))
	return hasher.Sum256(nil)
}

func (msg *EchoMsg) MsgType() int {
	return EchoType
}

// ReadyMsg
type ReadyMsg struct {
	Author    crypto.PublickKey
	Proposer  crypto.PublickKey
	BlockHash crypto.Digest
	Round     int
	Signature crypto.Signature
}

func NewReadyMsg(
	Author crypto.PublickKey,
	Proposer crypto.PublickKey,
	BlockHash crypto.Digest,
	Round int,
	sigService *crypto.SigService,
) (*ReadyMsg, error) {
	msg := &ReadyMsg{
		Author:    Author,
		Proposer:  Proposer,
		BlockHash: BlockHash,
		Round:     Round,
	}
	sig, err := sigService.RequestSignature(msg.Hash())
	if err != nil {
		return nil, err
	}
	msg.Signature = sig
	return msg, nil
}

func (msg *ReadyMsg) Verify() bool {
	return msg.Signature.Verify(msg.Author, msg.Hash())
}

func (msg *ReadyMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(msg.Author.Pubkey)
	hasher.Add(msg.Proposer.Pubkey)
	hasher.Add(msg.BlockHash[:])
	hasher.Add(strconv.AppendInt(nil, int64(msg.Round), 2))
	return hasher.Sum256(nil)
}

func (msg *ReadyMsg) MsgType() int {
	return ReadyType
}

// PBCProposeMsg
type PBCProposeMsg struct {
	Author    crypto.PublickKey
	Round     int
	B         *Block
	Signature crypto.Signature
}

func NewPBCProposeMsg(
	Author crypto.PublickKey,
	Round int,
	B *Block,
	sigService *crypto.SigService,
) (*PBCProposeMsg, error) {

	msg := &PBCProposeMsg{
		Author: Author,
		Round:  Round,
		B:      B,
	}

	if sig, err := sigService.RequestSignature(msg.Hash()); err != nil {
		return nil, err
	} else {
		msg.Signature = sig
		return msg, nil
	}
}

func (msg *PBCProposeMsg) Verify() bool {
	return msg.Signature.Verify(msg.Author, msg.Hash())
}

func (msg *PBCProposeMsg) Hash() crypto.Digest {

	hasher := crypto.NewHasher()
	hasher.Add(msg.Author.Pubkey)
	hasher.Add(strconv.AppendInt(nil, int64(msg.Round), 2))
	digest := msg.B.Hash()
	hasher.Add(digest[:])
	return hasher.Sum256(nil)
}

func (msg *PBCProposeMsg) MsgType() int {
	return PBCProposeType
}

// ElectMsg
type ElectMsg struct {
	Author   crypto.PublickKey
	Round    int
	SigShare crypto.SignatureShare
}

func NewElectMsg(Author crypto.PublickKey, Round int, sigService *crypto.SigService) (*ElectMsg, error) {
	msg := &ElectMsg{
		Author: Author,
		Round:  Round,
	}
	share, err := sigService.RequestTsSugnature(msg.Hash())
	if err != nil {
		return nil, err
	}
	msg.SigShare = share

	return msg, nil
}

func (msg *ElectMsg) Verify() bool {
	return msg.SigShare.Verify(msg.Hash())
}

func (msg *ElectMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(msg.Author.Pubkey)
	hasher.Add(strconv.AppendInt(nil, int64(msg.Round), 2))
	return hasher.Sum256(nil)
}

func (msg *ElectMsg) MsgType() int {
	return ElectType
}

var DefaultMsgValues = map[int]reflect.Type{
	GRBCProposeType: reflect.TypeOf(&GRBCProposeMsg{}),
	EchoType:        reflect.TypeOf(&EchoMsg{}),
	ReadyType:       reflect.TypeOf(&ReadyMsg{}),
	ElectType:       reflect.TypeOf(&ElectMsg{}),
	PBCProposeType:  reflect.TypeOf(&PBCProposeMsg{}),
}
