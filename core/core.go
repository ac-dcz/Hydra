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
	nodeID        NodeID
	name          crypto.PublickKey
	round         int
	committee     Committee
	parameters    Parameters
	txpool        *pool.Pool
	transmitor    *Transmitor
	sigService    *crypto.SigService
	store         store.Store
	retriever     *Retriever
	grbcInstances map[int]map[NodeID]*GRBC
	blockDigests  map[crypto.Digest]NodeID         // store hash of block that has received
	localDAG      map[int]map[NodeID]crypto.Digest // local DAG
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
	corer := &Core{
		nodeID:        nodeID,
		committee:     committee,
		round:         1,
		parameters:    parameters,
		txpool:        txpool,
		transmitor:    transmitor,
		sigService:    sigService,
		grbcInstances: make(map[int]map[NodeID]*GRBC),
		blockDigests:  make(map[crypto.Digest]NodeID),
		localDAG:      make(map[int]map[NodeID]crypto.Digest),
	}
	corer.retriever = NewRetriever(corer)
	corer.name = committee.Name(nodeID)
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
		instances[node] = NewGRBC(corer, node, round)
	}
	if _, ok := instances[node]; !ok {
		instances[node] = NewGRBC(corer, node, round)
	}
	corer.grbcInstances[round] = instances
	return instances[node]
}

func (corer *Core) checkReference(block *Block) bool {
	for d, _ := range block.Reference {
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
	// GRBC round
	var block *Block
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

	if block != nil && block.Batch.Txs != nil {
		//BenchMark Log
		logger.Info.Printf("create Block round %d node %d \n", block.Round, block.Author)
	}

	return block
}

func (corer *Core) handleGRBCPropose(propose *GRBCProposeMsg) error {

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
	go instance.processPropose(propose)
	return nil
}

func (corer *Core) handleEcho(echo *EchoMsg) {
	instance := corer.getGRBCInstance(echo.Author, echo.Round)
	go instance.processEcho(echo)
}

func (corer *Core) handleReady(ready *ReadyMsg) {
	instance := corer.getGRBCInstance(ready.Author, ready.Round)
	go instance.processReady(ready)
}

func (corer *Core) handlePBCPropose(propose *PBCProposeMsg) {

}

func (corer *Core) handleElect(elect *ElectMsg) {

}

func (corer *Core) Run() {
	for {
		select {
		case msg := <-corer.transmitor.RecvChannel():
			{
				switch msg.MsgType() {

				case GRBCProposeType:
					corer.handleGRBCPropose(msg.(*GRBCProposeMsg))
				case EchoType:
					corer.handleEcho(msg.(*EchoMsg))
				case ReadyType:
					corer.handleReady(msg.(*ReadyMsg))
				case PBCProposeType:
					corer.handlePBCPropose(msg.(*PBCProposeMsg))
				case ElectType:
					corer.handleElect(msg.(*ElectMsg))
				}
			}
		default:
			//TODO: delete
			logger.Debug.Println("nothing")
		}
	}
}
