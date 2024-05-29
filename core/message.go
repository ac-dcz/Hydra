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
	Author    *crypto.PublickKey
	Round     int
	B         *Block
	Signature *crypto.Signature
}

func (msg *GRBCProposeMsg) MsgType() int {
	return GRBCProposeType
}

// VoteMsg
type EchoMsg struct {
	Author    *crypto.PublickKey
	Proposer  *crypto.PublickKey
	Round     int
	Signature *crypto.Signature
}

func (msg *EchoMsg) MsgType() int {
	return EchoType
}

// ReadyMsg
type ReadyMsg struct {
	Author    *crypto.PublickKey
	Proposer  *crypto.PublickKey
	Round     int
	Signature *crypto.Signature
}

func (msg *ReadyMsg) MsgType() int {
	return ReadyType
}

// PBCProposeMsg
type PBCProposeMsg struct {
	Author    *crypto.PublickKey
	Round     int
	B         *Block
	Signature *crypto.Signature
}

func (msg *PBCProposeMsg) MsgType() int {
	return PBCProposeType
}

// ElectMsg
type ElectMsg struct {
	Author   *crypto.PublickKey
	Round    int
	SigShare *crypto.SignatureShare
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
