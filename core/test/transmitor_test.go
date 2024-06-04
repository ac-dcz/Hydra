package test

import (
	"context"
	"lightDAG/config"
	"lightDAG/core"
	"lightDAG/crypto"
	"lightDAG/network"
	"sync"
	"testing"
	"time"
)

func TestTransimtor(t *testing.T) {
	parameters := core.DefaultParameters
	committee, priKeys, shareKeys := config.GenDefaultCommittee(4)
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	var node1, node2 core.NodeID = 0, 1

	//node 1
	wg.Add(1)
	go func() {
		defer wg.Done()
		sigService := crypto.NewSigService(priKeys[node1], shareKeys[node1])
		addr := committee.Address(node1)
		sender, receiver := network.NewSender(), network.NewReceiver(addr)
		go sender.Run()
		go receiver.Run()
		transmitor := core.NewTransmitor(sender, receiver, core.DefaultMsgTypes, parameters, committee)
		time.Sleep(time.Second * 2)
		for i := 0; i < 20; i++ {
			go transmitor.Send(node1, node2, GetMessage(i%core.TotalNums, sigService))
		}
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-transmitor.RecvChannel():
				DisplayMessage(msg, t)
			}
		}
	}()

	//node 2
	wg.Add(1)
	go func() {
		defer wg.Done()
		sigService := crypto.NewSigService(priKeys[node2], shareKeys[node2])
		addr := committee.Address(node2)
		sender, receiver := network.NewSender(), network.NewReceiver(addr)
		go sender.Run()
		go receiver.Run()
		transmitor := core.NewTransmitor(sender, receiver, core.DefaultMsgTypes, parameters, committee)
		time.Sleep(time.Second * 2)

		for i := 0; i < 20; i++ {
			go transmitor.Send(node2, node1, GetMessage(i%core.TotalNums, sigService))
		}
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-transmitor.RecvChannel():
				DisplayMessage(msg, t)
			}
		}
	}()
	time.Sleep(time.Second * 5)

	cancel()
	wg.Wait()
}
