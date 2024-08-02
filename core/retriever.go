package core

import (
	"WuKong/crypto"
	"WuKong/logger"
	"WuKong/store"
	"time"
)

const (
	ReqType = iota
	ReplyType
)

type reqRetrieve struct {
	typ       int
	reqID     int
	digest    []crypto.Digest
	nodeID    NodeID
	backBlock crypto.Digest
}

type Retriever struct {
	nodeID          NodeID
	transmitor      *Transmitor
	cnt             int
	pendding        map[crypto.Digest]struct{} //dealing request
	requests        map[int]*RequestBlockMsg   //Request
	loopBackBlocks  map[int]crypto.Digest      // loopback deal block
	loopBackCnts    map[int]int
	miss2Blocks     map[crypto.Digest][]int //
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
		miss2Blocks:     make(map[crypto.Digest][]int),
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
					r.loopBackBlocks[r.cnt] = req.backBlock
					r.loopBackCnts[r.cnt] = len(req.digest)
					var missBlocks []crypto.Digest
					for i := 0; i < len(req.digest); i++ { //filter block that dealing
						r.miss2Blocks[req.digest[i]] = append(r.miss2Blocks[req.digest[i]], r.cnt)
						if _, ok := r.pendding[req.digest[i]]; ok {
							continue
						}
						missBlocks = append(missBlocks, req.digest[i])
						r.pendding[req.digest[i]] = struct{}{}
					}

					if len(missBlocks) > 0 {
						request, _ := NewRequestBlock(r.nodeID, missBlocks, r.cnt, time.Now().UnixMilli(), r.sigService)
						logger.Debug.Printf("sending request for miss block reqID %d \n", r.cnt)
						_ = r.transmitor.Send(request.Author, req.nodeID, request)
						r.requests[request.ReqID] = request
					}

					r.cnt++
				}
			case ReplyType: //request finish
				{
					logger.Debug.Printf("receive reply for miss block reqID %d \n", req.reqID)
					if _, ok := r.requests[req.reqID]; ok {
						_req := r.requests[req.reqID]
						for _, d := range _req.MissBlock {
							for _, id := range r.miss2Blocks[d] {
								r.loopBackCnts[id]--
								if r.loopBackCnts[id] == 0 {
									go r.loopBack(r.loopBackBlocks[id])
								}
							}
							delete(r.pendding, d) // delete
						}
						delete(r.requests, _req.ReqID) //delete request that finished
					}
				}
			}
		case <-ticker.C: // recycle request
			{
				now := time.Now().UnixMilli()
				for _, req := range r.requests {
					if now-req.Ts >= int64(r.parameters.RetryDelay) {
						request, _ := NewRequestBlock(req.Author, req.MissBlock, req.ReqID, now, r.sigService)
						r.requests[req.ReqID] = request
						//BroadCast to all node
						r.transmitor.Send(r.nodeID, NONE, request)
					}
				}
			}
		}
	}
}

func (r *Retriever) requestBlocks(digest []crypto.Digest, nodeid NodeID, backBlock crypto.Digest) {
	req := &reqRetrieve{
		typ:       ReqType,
		digest:    digest,
		nodeID:    nodeid,
		backBlock: backBlock,
	}
	r.reqChannel <- req
}

func (r *Retriever) processRequest(request *RequestBlockMsg) {
	var blocks []*Block
	for _, missBlock := range request.MissBlock {
		if val, err := r.store.Read(missBlock[:]); err != nil {
			logger.Warn.Println(err)
		} else {
			block := &Block{}
			if err := block.Decode(val); err != nil {
				logger.Warn.Println(err)
				return
			} else {
				blocks = append(blocks, block)
			}
		}
	}
	//reply
	reply, _ := NewReplyBlockMsg(r.nodeID, blocks, request.ReqID, r.sigService)
	r.transmitor.Send(r.nodeID, request.Author, reply)
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
		logger.Error.Println(err)
		panic(err)
	} else {
		block := &Block{}
		if err := block.Decode(val); err != nil {
			logger.Warn.Println(err)
			panic(err)
		} else {
			r.loopBackChannel <- block
		}
	}
}
