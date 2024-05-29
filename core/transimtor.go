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
	types   map[int]reflect.Type
	encoder *gob.Encoder
	decoder *gob.Decoder
}

type Transmitor struct {
	sender     *network.Sender
	receiver   *network.Receiver
	cc         *codec
	mu         sync.Mutex
	bufEn      *bytes.Buffer
	bufDe      *bytes.Buffer
	recvCh     chan Message
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
		bufEn:      bytes.NewBuffer(nil),
		bufDe:      bytes.NewBuffer(nil),
		recvCh:     make(chan Message, 1_000),
		msgCh:      make(chan *network.NetMessage, 1_000),
		parameters: parameters,
		committee:  committee,
	}
	tr.cc = &codec{
		types:   types,
		encoder: gob.NewEncoder(tr.bufEn),
		decoder: gob.NewDecoder(tr.bufDe),
	}

	go func() {
		for msg := range tr.msgCh {
			tr.sender.Send(msg)
		}
	}()

	go func() {
		for msg := range tr.receiver.RecvChannel() {
			_, _ = tr.bufDe.Write(msg.Data)
			val := reflect.New(tr.cc.types[msg.Typ]).Interface()
			if err := tr.cc.decoder.Decode(val); err != nil {
				logger.Error.Println(err)
			} else {
				tr.recvCh <- val.(Message)
			}
		}
	}()

	return tr
}

func (tr *Transmitor) Send(from, to NodeID, msg Message) error {
	//Address
	var addr []string
	if to != None {
		addr = tr.committee.BroadCast(from)
	} else {
		addr = append(addr, tr.committee.Address(to))
	}

	netMsg := &network.NetMessage{
		Msg: &network.Messgae{
			Typ: msg.MsgType(),
		},
		Address: addr,
	}

	//Encoder
	tr.mu.Lock()
	if err := tr.cc.encoder.Encode(msg); err != nil {
		logger.Warn.Println(err)
		return err
	}
	data, err := io.ReadAll(tr.bufEn)
	if err != nil {
		tr.mu.Unlock()
		logger.Error.Println(err)
		return err
	}
	netMsg.Msg.Data = data
	tr.mu.Unlock()

	// Filter Delay
	if msg.MsgType() == ProposeType && tr.parameters.DDos {
		time.AfterFunc(time.Millisecond*time.Duration(tr.parameters.NetwrokDelay), func() {
			tr.msgCh <- netMsg
		})
	} else {
		tr.msgCh <- netMsg
	}

	return nil
}

func (tr *Transmitor) Recv() Message {
	return <-tr.recvCh
}

func (tr *Transmitor) RecvChannel() chan Message {
	return tr.recvCh
}
