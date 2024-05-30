package core

import (
	"lightDAG/crypto"
	"time"
)

const (
	ReqType = iota
	ReplyType
)

type reqBlock struct {
	typ    int
	reqID  int
	digest []crypto.Digest
	nodeid []NodeID
}

type Retriever struct {
	corer      *Core
	cnt        int
	requests   map[int]*RequestBlockMsg
	reqChannel chan *reqBlock
}

func NewRetriever(corer *Core) *Retriever {
	return &Retriever{
		corer:      corer,
		cnt:        0,
		requests:   make(map[int]*RequestBlockMsg),
		reqChannel: make(chan *reqBlock, 1_00),
	}
}

func (r *Retriever) run() {
	ticker := time.NewTicker(time.Duration(r.corer.parameters.RetryDelay))
	for {
		select {
		case req := <-r.reqChannel:
			switch req.typ {
			case ReqType:
				for i := 0; i < len(req.digest); i++ {
					request, _ := NewRequestBlock(r.corer.nodeID, req.digest[i], r.cnt, time.Now().UnixMilli(), r.corer.sigService)
					_ = r.corer.transmitor.Send(request.Author, req.nodeid[i], request)
					r.requests[request.ReqID] = request
					r.cnt++
				}
			case ReplyType:
				delete(r.requests, req.reqID)
			}
		case <-ticker.C:
		}
	}
}

func (r *Retriever) requestBlocks(digest []crypto.Digest, nodeid []NodeID) {

}
