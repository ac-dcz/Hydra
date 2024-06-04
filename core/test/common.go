package test

import (
	"lightDAG/core"
	"lightDAG/crypto"
	"lightDAG/logger"
	"lightDAG/pool"
	"testing"
)

func InitConfig() {
	logger.SetLevel(logger.TestLevel)
}

func GetBatch(batchSize int) pool.Batch {
	batch := pool.Batch{
		ID: 0,
	}
	for i := 0; i < batchSize; i++ {
		batch.Txs = append(batch.Txs, make(pool.Transaction, 16))
	}
	return batch
}

func GetBlock(batchSize int) *core.Block {
	block := &core.Block{
		Author:    -1,
		Round:     -1,
		Batch:     GetBatch(batchSize),
		Reference: make(map[crypto.Digest]core.NodeID),
	}
	block.Reference[block.Hash()] = -1
	return block
}

func GetDigest() crypto.Digest {
	return crypto.NewHasher().Sum256([]byte("123"))
}

func GetMessage(Typ int, sigService *crypto.SigService) core.ConsensusMessage {
	var msg core.ConsensusMessage
	switch Typ {
	case core.GRBCProposeType:
		msg, _ = core.NewGRBCProposeMsg(-1, -1, GetBlock(10), sigService)
	case core.EchoType:
		msg, _ = core.NewEchoMsg(-1, -1, GetDigest(), -1, sigService)
	case core.ReadyType:
		msg, _ = core.NewReadyMsg(-1, -1, GetDigest(), -1, sigService)
	case core.PBCProposeType:
		msg, _ = core.NewPBCProposeMsg(-1, -1, GetBlock(10), sigService)
	case core.ReplyBlockType:
		msg, _ = core.NewReplyBlockMsg(-1, []*core.Block{GetBlock(10)}, -1, sigService)
	case core.RequestBlockType:
		msg, _ = core.NewRequestBlock(-1, []crypto.Digest{GetDigest()}, -1, 0, sigService)
	case core.ElectType:
		msg, _ = core.NewElectMsg(-1, -1, sigService)
	}
	return msg
}

func DisplayMessage(msg core.ConsensusMessage, t *testing.T) {
	switch msg.MsgType() {

	case core.GRBCProposeType:
		temp := msg.(*core.GRBCProposeMsg)
		t.Logf("%v \n", temp)
	case core.EchoType:
		temp := msg.(*core.EchoMsg)
		t.Logf("%v \n", temp)
	case core.ReadyType:
		temp := msg.(*core.ReadyMsg)
		t.Logf("%v \n", temp)
	case core.PBCProposeType:
		temp := msg.(*core.PBCProposeMsg)
		t.Logf("%v \n", temp)
	case core.ElectType:
		temp := msg.(*core.ElectMsg)
		t.Logf("%v \n", temp)
	case core.RequestBlockType:
		temp := msg.(*core.RequestBlockMsg)
		t.Logf("%v \n", temp)
	case core.ReplyBlockType:
		temp := msg.(*core.ReplyBlockMsg)
		t.Logf("%v \n", temp)
	}
}
