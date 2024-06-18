package network

import (
	"encoding/gob"
	"io"
	"reflect"
)

type Messgae interface {
	MsgType() int
}

type Codec struct {
	types   map[int]reflect.Type
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func NewCodec(types map[int]reflect.Type) *Codec {
	return &Codec{
		types: types,
	}
}

// BindConn: only bind once
func (cc *Codec) Bind(conn io.ReadWriter) *Codec {
	return &Codec{
		types:   cc.types,
		encoder: gob.NewEncoder(conn),
		decoder: gob.NewDecoder(conn),
	}
}

func (cc *Codec) Write(msg Messgae) error {
	typeId := msg.MsgType()
	if err := cc.encoder.Encode(typeId); err != nil {
		return err
	}
	if err := cc.encoder.Encode(msg); err != nil {
		return err
	}
	return nil
}

func (cc *Codec) Read() (Messgae, error) {
	var typeId int
	if err := cc.decoder.Decode(&typeId); err != nil {
		return nil, err
	}
	msg := reflect.New(cc.types[typeId]).Interface()
	if err := cc.decoder.Decode(msg); err != nil {
		return nil, err
	}
	return msg.(Messgae), nil
}
