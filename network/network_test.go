package network

import (
	"sync"
	"testing"
	"time"
)

func TestNetwork(t *testing.T) {
	// logger.SetOutput(logger.InfoLevel|logger.DebugLevel|logger.ErrorLevel|logger.WarnLevel, logger.NewFileWriter("./default.log"))
	addr := ":8080"
	receiver := NewReceiver(addr)
	go receiver.Run()
	time.Sleep(time.Second)
	sender := NewSender()
	go sender.Run()

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(ind int) {
			defer wg.Done()
			msg := &NetMessage{
				Msg: &Messgae{
					Typ:  ind,
					Data: []byte("dcz"),
				},
				Address: []string{addr},
			}
			sender.Send(msg)
		}(i)
	}

	for i := 0; i < 10; i++ {
		msg := receiver.Recv()
		t.Logf("Messsage: %d-%s\n", msg.Typ, msg.Data)
	}
	wg.Wait()
}
