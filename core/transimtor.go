package core

import (
	"bytes"
	"encoding/gob"
	"io"
	"lightDAG/logger"
	"lightDAG/network"
	"reflect"
	"sync"
	"time"
)

type codec struct {
	types    map[int]reflect.Type
	bufEns   map[NodeID]*bytes.Buffer
	encoders map[NodeID]*gob.Encoder
	decoders map[NodeID]*gob.Decoder
	bufDes   map[NodeID]*bytes.Buffer
}

type Transmitor struct {
	sender     *network.Sender
	receiver   *network.Receiver
	cc         *codec
	mu         sync.Mutex
	recvCh     chan ConsensusMessage
	msgCh      chan *network.NetMessage
	parameters Parameters
	committee  Committee
}

func NewTransmitor(
	sender *network.Sender,
	receiver *network.Receiver,
	types map[int]reflect.Type,
	parameters Parameters,
	committee Committee,
) *Transmitor {

	tr := &Transmitor{
		sender:     sender,
		receiver:   receiver,
		mu:         sync.Mutex{},
		recvCh:     make(chan ConsensusMessage, 1_000),
		msgCh:      make(chan *network.NetMessage, 1_000),
		parameters: parameters,
		committee:  committee,
	}

	tr.cc = &codec{
		types:    types,
		bufEns:   make(map[NodeID]*bytes.Buffer),
		encoders: make(map[NodeID]*gob.Encoder),
		bufDes:   make(map[NodeID]*bytes.Buffer),
		decoders: make(map[NodeID]*gob.Decoder),
	}

	for i := 0; i < committee.Size(); i++ {
		buf1, buf2 := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
		tr.cc.bufEns[NodeID(i)] = buf1
		tr.cc.encoders[NodeID(i)] = gob.NewEncoder(buf1)
		tr.cc.bufDes[NodeID(i)] = buf2
		tr.cc.decoders[NodeID(i)] = gob.NewDecoder(buf2)
	}

	go func() {
		for msg := range tr.msgCh {
			tr.sender.Send(msg)
		}
	}()

	go func() {
		for msg := range tr.receiver.RecvChannel() {
			tr.cc.bufDes[NodeID(msg.From)].Write(msg.Data)
			val := reflect.New(tr.cc.types[msg.Typ]).Interface()
			if err := tr.cc.decoders[NodeID(msg.From)].Decode(val); err != nil {
				logger.Error.Println(msg.Typ, err)
			} else {
				tr.recvCh <- val.(ConsensusMessage)
			}
		}
	}()

	return tr
}

func (tr *Transmitor) _send(from, to NodeID, msg ConsensusMessage) error {
	//Address
	var addr = tr.committee.Address(to)

	netMsg := &network.NetMessage{
		Msg: &network.Messgae{
			Typ:  msg.MsgType(),
			From: int(from),
		},
		Address: addr,
	}

	//Encoder
	tr.mu.Lock()
	if err := tr.cc.encoders[to].Encode(msg); err != nil {
		tr.cc.bufEns[to].Reset()
		tr.mu.Unlock()
		logger.Error.Println(err)
		return err
	}
	data, _ := io.ReadAll(tr.cc.bufEns[to])
	netMsg.Msg.Data = data
	tr.mu.Unlock()

	// Filter Delay
	if msg.MsgType() == GRBCProposeType && tr.parameters.DDos {
		time.AfterFunc(time.Millisecond*time.Duration(tr.parameters.NetwrokDelay), func() {
			tr.msgCh <- netMsg
		})
	} else {
		tr.msgCh <- netMsg
	}

	return nil
}

func (tr *Transmitor) Send(from, to NodeID, msg ConsensusMessage) error {
	if to == NONE {
		for i := 0; i < tr.committee.Size(); i++ {
			if i != int(from) {
				tr._send(from, NodeID(i), msg)
			}
		}
	} else {
		tr._send(from, to, msg)
	}

	return nil
}

func (tr *Transmitor) Recv() ConsensusMessage {
	return <-tr.recvCh
}

func (tr *Transmitor) RecvChannel() chan ConsensusMessage {
	return tr.recvCh
}
