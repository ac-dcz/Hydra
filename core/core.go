package core

import (
	"lightDAG/crypto"
	"lightDAG/logger"
	"lightDAG/pool"
	"lightDAG/store"
)

const (
	WaveRound = 2
	GradeOne  = 1
	GradeTwo  = 2
)

type Core struct {
	nodeID              NodeID
	round               int
	committee           Committee
	parameters          Parameters
	txpool              *pool.Pool
	transmitor          *Transmitor
	sigService          *crypto.SigService
	store               store.Store
	retriever           *Retriever
	loopBackChannel     chan *Block
	grbcCallBackChannel chan *callBackReq
	grbcInstances       map[int]map[NodeID]*GRBC
	blockDigests        map[crypto.Digest]NodeID         // store hash of block that has received
	localDAG            map[int]map[NodeID]crypto.Digest // local DAG
	proposedFlag        map[int]struct{}
}

func NewCore(
	nodeID NodeID,
	committee Committee,
	parameters Parameters,
	txpool *pool.Pool,
	transmitor *Transmitor,
	store store.Store,
	sigService *crypto.SigService,
) *Core {

	loopBackChannel := make(chan *Block, 1_000)
	grbcCallBackChannel := make(chan *callBackReq, 1_000)
	corer := &Core{
		nodeID:              nodeID,
		committee:           committee,
		round:               0,
		parameters:          parameters,
		txpool:              txpool,
		transmitor:          transmitor,
		sigService:          sigService,
		store:               store,
		loopBackChannel:     loopBackChannel,
		grbcCallBackChannel: grbcCallBackChannel,
		grbcInstances:       make(map[int]map[NodeID]*GRBC),
		blockDigests:        make(map[crypto.Digest]NodeID),
		localDAG:            make(map[int]map[NodeID]crypto.Digest),
		proposedFlag:        make(map[int]struct{}),
	}

	corer.retriever = NewRetriever(nodeID, store, transmitor, sigService, parameters, loopBackChannel)

	return corer
}

func (corer *Core) storeBlock(block *Block) error {
	key := block.Hash()
	if val, err := block.Encode(); err != nil {
		return err
	} else {
		corer.store.Write(key[:], val)
		return nil
	}
}

func (corer *Core) getGRBCInstance(node NodeID, round int) *GRBC {
	instances := corer.grbcInstances[round]
	if instances == nil {
		instances = make(map[NodeID]*GRBC)
	}
	if _, ok := instances[node]; !ok {
		instances[node] = NewGRBC(corer, node, round, corer.grbcCallBackChannel)
	}
	corer.grbcInstances[round] = instances
	return instances[node]
}

func (corer *Core) checkReference(block *Block) bool {
	for d := range block.Reference {
		if _, ok := corer.blockDigests[d]; !ok {
			return false
		}
	}
	return true
}

func (corer *Core) retrieveBlock(block *Block) {
	missDigests, missNodes := make([]crypto.Digest, 0), make([]NodeID, 0)
	for d, id := range block.Reference {
		if _, ok := corer.blockDigests[d]; !ok {
			missDigests = append(missDigests, d)
			missNodes = append(missNodes, id)
		}
	}
	corer.retriever.requestBlocks(missDigests, missNodes)
}

/*********************************Protocol***********************************************/
func (corer *Core) generatorBlock(round int) *Block {
	var block *Block

	if _, ok := corer.proposedFlag[round]; !ok {
		// GRBC round
		if round%WaveRound == 0 {
			if round == 0 {
				block = &Block{
					Author:    corer.nodeID,
					Round:     round,
					Batch:     corer.txpool.GetBatch(),
					Reference: nil,
				}
			} else {
				if pbcSlot, ok := corer.localDAG[round-1]; ok {
					// >=2f+1?
					if len(pbcSlot) >= corer.committee.HightThreshold() {
						reference := make(map[crypto.Digest]NodeID)
						for id, d := range pbcSlot {
							reference[d] = id
						}
						block = &Block{
							Author:    corer.nodeID,
							Round:     round,
							Batch:     corer.txpool.GetBatch(),
							Reference: reference,
						}
					}
				}
			}
		} else { // PBC round
			if grbcInstances, ok := corer.grbcInstances[round-1]; ok {
				cnt := 0
				for _, instance := range grbcInstances {
					if instance.Grade() == GradeTwo {
						cnt++
					}
				}
				// grade 2 >= 2f+1?
				if cnt >= corer.committee.HightThreshold() {
					grbcSlot := corer.localDAG[round-1]
					reference := make(map[crypto.Digest]NodeID)
					for id, d := range grbcSlot {
						reference[d] = id
					}
					block = &Block{
						Author:    corer.nodeID,
						Round:     round,
						Batch:     corer.txpool.GetBatch(),
						Reference: reference,
					}
				}
			}
		}
	}

	if block != nil {
		corer.proposedFlag[round] = struct{}{}
		if block.Batch.Txs != nil {
			//BenchMark Log
			logger.Info.Printf("create Block round %d node %d \n", block.Round, block.Author)
		}
	}

	return block
}

func (corer *Core) handleGRBCPropose(propose *GRBCProposeMsg) error {
	logger.Debug.Printf("procesing grbc propose round %d node %d \n", propose.Round, propose.Author)

	//Step 1: verify signature
	if !propose.Verify(corer.committee) {
		return ErrSignature(propose.MsgType(), propose.Round, int(propose.Author))
	}

	//Step 2: store Block
	if err := corer.storeBlock(propose.B); err != nil {
		return err
	}

	//Step 3: check reference
	if !corer.checkReference(propose.B) {
		//retrieve miss block
		corer.retrieveBlock(propose.B)

		return ErrReference(propose.MsgType(), propose.Round, int(propose.Author))
	}

	//Step 4: process
	instance := corer.getGRBCInstance(propose.Author, propose.Round)
	go instance.processPropose(propose.B)

	return nil
}

func (corer *Core) handleEcho(echo *EchoMsg) error {
	logger.Debug.Printf("procesing grbc echo round %d node %d \n", echo.Round, echo.Proposer)

	//Step 1: verify signature
	if !echo.Verify(corer.committee) {
		return ErrSignature(echo.MsgType(), echo.Round, int(echo.Author))
	}

	instance := corer.getGRBCInstance(echo.Author, echo.Round)
	go instance.processEcho(echo)

	return nil
}

func (corer *Core) handleReady(ready *ReadyMsg) error {
	logger.Debug.Printf("procesing grbc ready round %d node %d \n", ready.Round, ready.Proposer)

	//Step 1: verify signature
	if !ready.Verify(corer.committee) {
		return ErrSignature(ready.MsgType(), ready.Round, int(ready.Author))
	}

	instance := corer.getGRBCInstance(ready.Author, ready.Round)
	go instance.processReady(ready)

	return nil
}

func (corer *Core) handlePBCPropose(propose *PBCProposeMsg) error {
	logger.Debug.Printf("procesing pbc propose round %d node %d \n", propose.Round, propose.Author)

	//Step 1: verify signature
	if !propose.Verify(corer.committee) {
		return ErrSignature(propose.MsgType(), propose.Round, int(propose.Author))
	}

	//Step 2: store Block
	if err := corer.storeBlock(propose.B); err != nil {
		return err
	}

	//Step 3: check reference
	if !corer.checkReference(propose.B) {
		//retrieve miss block
		corer.retrieveBlock(propose.B)

		return ErrReference(propose.MsgType(), propose.Round, int(propose.Author))
	}

	//Step 4
	corer.handleOutPut(propose.B.Round, propose.B.Author, propose.B.Hash())

	return nil
}

func (corer *Core) handleOutPut(round int, node NodeID, digest crypto.Digest) error {
	logger.Debug.Printf("procesing output round %d node %d \n", round, node)

	corer.blockDigests[digest] = node
	slot, ok := corer.localDAG[round]
	if !ok {
		slot = make(map[NodeID]crypto.Digest)
		corer.localDAG[round] = slot
	}
	slot[node] = digest

	if len(slot) >= corer.committee.HightThreshold() {
		return corer.advanceRound(round + 1)
	}

	return nil
}

func (corer *Core) advanceRound(round int) error {

	if block := corer.generatorBlock(round); block != nil {
		if round%WaveRound == 0 {
			if propose, err := NewGRBCProposeMsg(corer.nodeID, round, block, corer.sigService); err != nil {
				return err
			} else {
				corer.transmitor.Send(corer.nodeID, NONE, propose)
				corer.transmitor.RecvChannel() <- propose
			}
		} else {
			if propose, err := NewPBCProposeMsg(corer.nodeID, round, block, corer.sigService); err != nil {
				return err
			} else {
				corer.transmitor.Send(corer.nodeID, NONE, propose)
				corer.transmitor.RecvChannel() <- propose
			}
		}
	}

	return nil
}

func (corer *Core) handleElect(elect *ElectMsg) error {
	logger.Debug.Printf("procesing elect wave %d node %d \n", elect.Round%WaveRound, elect.Author)

	return nil
}

func (corer *Core) handleRequestBlock(request *RequestBlockMsg) error {
	logger.Debug.Println("procesing block request")

	//Step 1: verify signature
	if !request.Verify(corer.committee) {
		return ErrSignature(request.MsgType(), -1, int(request.Author))
	}

	go corer.retriever.processRequest(request)

	return nil
}

func (corer *Core) handleReplyBlock(reply *ReplyBlockMsg) error {
	logger.Debug.Println("procesing block reply")

	//Step 1: verify signature
	if !reply.Verify(corer.committee) {
		return ErrSignature(reply.MsgType(), -1, int(reply.Author))
	}

	if reply.B.Round%WaveRound == 0 {
		corer.getGRBCInstance(reply.B.Author, reply.B.Round).SetGrade(GradeOne)
	}

	corer.storeBlock(reply.B)
	corer.handleOutPut(reply.B.Round, reply.B.Author, reply.B.Hash())

	go corer.retriever.processReply(reply)

	return nil
}

func (corer *Core) handleLoopBack(block *Block) error {
	logger.Debug.Println("procesing block loop back")

	//GRBC round
	if block.Round%WaveRound == 0 {
		instance := corer.getGRBCInstance(block.Author, block.Round)
		go instance.processPropose(block)
	} else {
		return corer.handleOutPut(block.Round, block.Author, block.Hash())
	}

	return nil
}

func (corer *Core) handleCallBack(req *callBackReq) error {

	return corer.handleOutPut(req.round, req.nodeID, req.digest)
}

func (corer *Core) Run() {

	//first propose
	block := corer.generatorBlock(0)
	if propose, err := NewGRBCProposeMsg(corer.nodeID, 0, block, corer.sigService); err != nil {
		panic(err)
	} else {
		corer.transmitor.Send(corer.nodeID, NONE, propose)
		corer.transmitor.RecvChannel() <- propose
	}

	for {
		var err error
		select {
		case msg := <-corer.transmitor.RecvChannel():
			{
				switch msg.MsgType() {

				case GRBCProposeType:
					err = corer.handleGRBCPropose(msg.(*GRBCProposeMsg))
				case EchoType:
					err = corer.handleEcho(msg.(*EchoMsg))
				case ReadyType:
					err = corer.handleReady(msg.(*ReadyMsg))
				case PBCProposeType:
					err = corer.handlePBCPropose(msg.(*PBCProposeMsg))
				case ElectType:
					err = corer.handleElect(msg.(*ElectMsg))
				case RequestBlockType:
					err = corer.handleRequestBlock(msg.(*RequestBlockMsg))
				case ReplyBlockType:
					err = corer.handleReplyBlock(msg.(*ReplyBlockMsg))
				}
			}

		case block := <-corer.loopBackChannel:
			{
				err = corer.handleLoopBack(block)
			}
		case cbReq := <-corer.grbcCallBackChannel:
			{
				err = corer.handleCallBack(cbReq)
			}
		}

		logger.Warn.Println(err)
	}
}
