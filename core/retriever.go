package core

import (
	"lightDAG/crypto"
	"lightDAG/logger"
	"lightDAG/store"
	"time"
)

const (
	ReqType = iota
	ReplyType
)

type reqRetrieve struct {
	typ    int
	reqID  int
	digest []crypto.Digest
	nodeid []NodeID
}

type Retriever struct {
	nodeID          NodeID
	transmitor      *Transmitor
	cnt             int
	pendding        map[crypto.Digest]struct{} //dealing request
	requests        map[int]*RequestBlockMsg   //Request
	loopBackBlocks  map[int]crypto.Digest      // loopback deal block
	loopBackCnts    map[int]int
	reqChannel      chan *reqRetrieve
	sigService      *crypto.SigService
	store           *store.Store
	parameters      Parameters
	loopBackChannel chan<- *Block
}

func NewRetriever(
	nodeID NodeID,
	store *store.Store,
	transmitor *Transmitor,
	sigService *crypto.SigService,
	parameters Parameters,
	loopBackChannel chan<- *Block,
) *Retriever {

	r := &Retriever{
		nodeID:          nodeID,
		cnt:             0,
		pendding:        make(map[crypto.Digest]struct{}),
		requests:        make(map[int]*RequestBlockMsg),
		loopBackBlocks:  make(map[int]crypto.Digest),
		loopBackCnts:    make(map[int]int),
		reqChannel:      make(chan *reqRetrieve, 1_00),
		store:           store,
		sigService:      sigService,
		transmitor:      transmitor,
		parameters:      parameters,
		loopBackChannel: loopBackChannel,
	}
	go r.run()

	return r
}

func (r *Retriever) run() {
	ticker := time.NewTicker(time.Duration(r.parameters.RetryDelay))
	for {
		select {
		case req := <-r.reqChannel:
			switch req.typ {
			case ReqType: //request Block
				{
					for i := 0; i < len(req.digest); i++ {
						if _, ok := r.pendding[req.digest[i]]; ok {
							continue
						}
						request, _ := NewRequestBlock(r.nodeID, req.digest[i], r.cnt, time.Now().UnixMilli(), r.sigService)

						logger.Debug.Printf("sending request for miss block reqID %d \n", req.reqID)
						_ = r.transmitor.Send(request.Author, req.nodeid[i], request)
						r.requests[request.ReqID] = request
						r.cnt++
					}
				}
			case ReplyType: //request finish
				{
					logger.Debug.Printf("receive reply for miss block reqID %d \n", req.reqID)
					if _, ok := r.requests[req.reqID]; ok {
						_req := r.requests[req.reqID]
						delete(r.pendding, _req.BlockHash) // delete
						delete(r.requests, _req.ReqID)     //delete request that finished

						if cnt, ok := r.loopBackCnts[req.reqID]; ok {
							r.loopBackCnts[req.reqID] = cnt - 1
							if cnt == 0 { //all miss reference hava been received
								if blockHash, ok := r.loopBackBlocks[req.reqID]; ok {
									//LoopBack
									go r.loopBack(blockHash)
								}
							}
						}
					}
				}
			}
		case <-ticker.C: // recycle request
			{
				now := time.Now().UnixMilli()
				for _, req := range r.requests {
					if now-req.Ts >= int64(r.parameters.RetryDelay) {
						req.Ts = now

						//BroadCast to all node
						r.transmitor.Send(r.nodeID, NONE, req)
					}
				}
			}
		}
	}
}

func (r *Retriever) requestBlocks(digest []crypto.Digest, nodeid []NodeID) {
	req := &reqRetrieve{
		typ:    ReqType,
		digest: digest,
		nodeid: nodeid,
	}
	r.reqChannel <- req
}

func (r *Retriever) processRequest(request *RequestBlockMsg) {
	if val, err := r.store.Read(request.BlockHash[:]); err != nil {
		logger.Warn.Println(err)
	} else {
		block := &Block{}
		if err := block.Decode(val); err != nil {
			logger.Warn.Println(err)
		} else {
			//reply
			reply, _ := NewReplyBlockMsg(r.nodeID, block, request.ReqID, r.sigService)
			r.transmitor.Send(r.nodeID, request.Author, reply)
		}
	}
}

func (r *Retriever) processReply(reply *ReplyBlockMsg) {
	req := &reqRetrieve{
		typ:   ReplyType,
		reqID: reply.ReqID,
	}
	r.reqChannel <- req
}

func (r *Retriever) loopBack(blockHash crypto.Digest) {
	// logger.Debug.Printf("processing loopback")
	if val, err := r.store.Read(blockHash[:]); err != nil {
		//must be  received
		panic(err)
	} else {
		block := &Block{}
		if err := block.Decode(val); err != nil {
			logger.Warn.Println(err)
		} else {

			if block.Round%WaveRound == 0 { //GRBC round
				propose, _ := NewGRBCProposeMsg(
					block.Author,
					block.Round,
					block,
					r.sigService,
				)

				//loopback
				r.transmitor.RecvChannel() <- propose
			} else { //PBC round
				propose, _ := NewPBCProposeMsg(
					block.Author,
					block.Round,
					block,
					r.sigService,
				)

				//loopback
				r.transmitor.RecvChannel() <- propose
			}
		}
	}
}
