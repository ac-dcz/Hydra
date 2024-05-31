package core

import (
	"lightDAG/crypto"
	"lightDAG/logger"
	"sync"
	"sync/atomic"
)

type callBackReq struct {
	digest crypto.Digest
	nodeID NodeID
	round  int
}

type GRBC struct {
	nodeID     NodeID
	proposer   NodeID
	round      int
	digest     atomic.Value
	grade      atomic.Int32
	sigService *crypto.SigService
	transmitor *Transmitor
	committee  Committee

	echoMu       *sync.Mutex
	unHandleEcho []*EchoMsg
	echoCnt      atomic.Int32

	readyMu       *sync.Mutex
	unHandleReady []*ReadyMsg
	readyCnt      atomic.Int32

	rflag atomic.Bool
	once  sync.Once

	callBackChannel chan<- *callBackReq
}

func NewGRBC(corer *Core, proposer NodeID, round int, callBackChannel chan<- *callBackReq) *GRBC {
	grbc := &GRBC{
		proposer:        proposer,
		round:           round,
		echoMu:          &sync.Mutex{},
		readyMu:         &sync.Mutex{},
		once:            sync.Once{},
		sigService:      corer.sigService,
		transmitor:      corer.transmitor,
		committee:       corer.committee,
		nodeID:          corer.nodeID,
		callBackChannel: callBackChannel,
	}

	return grbc
}

func (g *GRBC) processPropose(block *Block) {

	//Step 1: store digest
	digest := block.Hash()
	g.digest.Store(digest)

	//Step 2: send echo message
	echo, err := NewEchoMsg(g.nodeID, block.Author, digest, block.Round, g.sigService)
	if err != nil {
		logger.Warn.Println(err)
	}

	g.transmitor.Send(g.nodeID, NONE, echo)
	g.transmitor.RecvChannel() <- echo

	//Step 3: handle remain msg
	g.echoMu.Lock()
	for _, echo := range g.unHandleEcho {
		go g.processEcho(echo)
	}
	g.unHandleEcho = nil
	g.echoMu.Unlock()

	g.readyMu.Lock()
	for _, r := range g.unHandleReady {
		go g.processReady(r)
	}
	g.unHandleReady = nil
	g.readyMu.Unlock()

}

func (g *GRBC) processEcho(echo *EchoMsg) {
	//Step 1: compare hash
	if val := g.digest.Load(); val == nil {
		g.echoMu.Lock()
		g.unHandleEcho = append(g.unHandleEcho, echo)
		g.echoMu.Unlock()
		return
	} else {
		hash := val.(crypto.Digest)
		if hash != echo.BlockHash {
			return
		}
	}

	//Step 2: 2f+1?
	if g.echoCnt.Add(1) == int32(g.committee.HightThreshold()) && !g.rflag.Load() {
		// ready
		g.once.Do(func() {
			ready, _ := NewReadyMsg(g.nodeID, g.proposer, echo.BlockHash, echo.Round, g.sigService)
			g.transmitor.Send(g.nodeID, NONE, ready)
			g.transmitor.RecvChannel() <- ready
			g.grade.Store(GradeOne)
			g.rflag.Store(true)

			//TODO: callback
			g.callBackChannel <- &callBackReq{
				nodeID: g.proposer,
				digest: echo.BlockHash,
				round:  g.round,
			}
		})
	}

}

func (g *GRBC) processReady(ready *ReadyMsg) {
	//Step 1: compare hash
	if val := g.digest.Load(); val == nil {
		g.readyMu.Lock()
		g.unHandleReady = append(g.unHandleReady, ready)
		g.readyMu.Unlock()
		return
	} else {
		hash := val.(crypto.Digest)
		if hash != ready.BlockHash {
			return
		}
	}

	//Step 2:
	if cnt := g.readyCnt.Add(1); cnt == int32(g.committee.LowThreshold()) && !g.rflag.Load() {
		g.once.Do(func() {
			ready, _ := NewReadyMsg(g.nodeID, g.proposer, ready.BlockHash, ready.Round, g.sigService)
			g.transmitor.Send(g.nodeID, NONE, ready)
			g.transmitor.RecvChannel() <- ready
			g.grade.Store(GradeOne)
			g.rflag.Store(true)

			//TODO: callback
			g.callBackChannel <- &callBackReq{
				nodeID: g.proposer,
				digest: ready.BlockHash,
				round:  g.round,
			}
		})
	} else if cnt == int32(g.committee.HightThreshold()) {
		g.grade.Store(GradeTwo)
	}

}

func (g *GRBC) SetGrade(grade int) {
	g.grade.Store(int32(grade))
}

func (g *GRBC) Grade() int {
	return int(g.grade.Load())
}
