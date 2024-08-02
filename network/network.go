package network

import (
	"WuKong/logger"
	"io"
	"net"
)

type NetMessage struct {
	Msg     Messgae
	Address []string
}

type Sender struct {
	msgCh chan *NetMessage
	conns map[string]chan<- Messgae
	cc    *Codec
}

func NewSender(cc *Codec) *Sender {
	sender := &Sender{
		msgCh: make(chan *NetMessage, 1000),
		conns: make(map[string]chan<- Messgae),
		cc:    cc,
	}
	return sender
}

func (s *Sender) Run() {
	for msg := range s.msgCh {
		for _, addr := range msg.Address {
			if conn, ok := s.conns[addr]; ok {
				conn <- msg.Msg
			} else {
				conn, err := s.connect(addr)
				if err != nil {
					continue
				} else {
					s.conns[addr] = conn
					conn <- msg.Msg
				}
			}
		}
	}
}

func (s *Sender) Send(msg *NetMessage) {
	s.msgCh <- msg
}

func (s *Sender) SendChannel() chan<- *NetMessage {
	return s.msgCh
}

func (s *Sender) connect(addr string) (chan<- Messgae, error) {
	msgCh := make(chan Messgae, 1000)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		logger.Warn.Printf("Failed to connect to %s: %v \n", addr, err)
		return nil, err
	}
	logger.Info.Printf("Outgoing connection established with %s \n", addr)
	go func() {
		cc := s.cc.Bind(conn)
		for msg := range msgCh {
			if err := cc.Write(msg); err != nil {
				if err != io.ErrClosedPipe {
					logger.Warn.Printf("Failed to send message to %s: %v \n", addr, err)
				}
				return
			} else {
				logger.Debug.Printf("Successfully sent message to %s \n", addr)
			}
		}
	}()
	return msgCh, nil
}

type Receiver struct {
	addr string
	msg  chan Messgae
	cc   *Codec
}

func NewReceiver(addr string, cc *Codec) *Receiver {

	receiver := &Receiver{
		addr: addr,
		msg:  make(chan Messgae, 1000),
		cc:   cc,
	}

	return receiver
}

func (recv *Receiver) Run() {
	listen, err := net.Listen("tcp", recv.addr)
	if err != nil {
		logger.Error.Printf("Failed to bind to TCP addr : %s \n", err)
		panic(err)
	}
	logger.Debug.Printf("Listening on %s \n", recv.addr)

	for {
		conn, err := listen.Accept()
		if err != nil {
			logger.Warn.Printf("Failed to accept : %v \n", err)
			continue
		}
		logger.Info.Printf("Incoming connection established with %v \n", conn.RemoteAddr())
		go recv.serveConn(conn)
	}
}

func (recv *Receiver) serveConn(conn net.Conn) {
	cc := recv.cc.Bind(conn)
	for {
		if msg, err := cc.Read(); err != nil {
			// logger.Debug.Printf("Received %v", msg)
			if err != io.EOF {
				logger.Warn.Printf("failed to receive : %v \n", err)
			}
			return
		} else {
			recv.msg <- msg
		}
	}
}

func (recv *Receiver) Recv() Messgae {
	return <-recv.msg
}

func (recv *Receiver) RecvChannel() <-chan Messgae {
	return recv.msg
}
