package core

const (
	ProposeType = iota
)

type Message interface {
	MsgType() int
}
