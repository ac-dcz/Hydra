package core

import (
	"Hydra/crypto"
	"Hydra/logger"
	"Hydra/pool"
	"Hydra/store"
	"sync"
	"time"
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
	store               *store.Store
	retriever           *Retriever
	eletor              *Elector
	commitor            *Commitor
	localDAG            *LocalDAG
	loopBackChannel     chan *Block
	grbcCallBackChannel chan *callBackReq
	commitChannel       chan<- *Block
	proposedNotify      map[int]*sync.Mutex
	proposedFlag        map[int]struct{}
	grbcInstances       map[int]map[NodeID]*GRBC
}

func NewCore(
	nodeID NodeID,
	committee Committee,
	parameters Parameters,
	txpool *pool.Pool,
	transmitor *Transmitor,
	store *store.Store,
	sigService *crypto.SigService,
	commitChannel chan<- *Block,
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
		commitChannel:       commitChannel,
		proposedNotify:      make(map[int]*sync.Mutex),
		grbcInstances:       make(map[int]map[NodeID]*GRBC),
		localDAG:            NewLocalDAG(),
		proposedFlag:        make(map[int]struct{}),
	}

	corer.retriever = NewRetriever(nodeID, store, transmitor, sigService, parameters, loopBackChannel)
	corer.eletor = NewElector(sigService, committee)
	corer.commitor = NewCommitor(corer.eletor, corer.localDAG, store, commitChannel, committee.Size())

	return corer
}

func storeBlock(store *store.Store, block *Block) error {
	key := block.Hash()
	if val, err := block.Encode(); err != nil {
		return err
	} else {
		store.Write(key[:], val)
		return nil
	}
}

func getBlock(store *store.Store, digest crypto.Digest) (*Block, error) {
	block := &Block{}
	data, err := store.Read(digest[:])
	if err != nil {
		return nil, err
	}
	if err := block.Decode(data); err != nil {
		return nil, err
	}
	return block, nil
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

func (corer *Core) checkReference(block *Block) (bool, []crypto.Digest) {
	var temp []crypto.Digest
	for d := range block.Reference {
		temp = append(temp, d)
	}
	ok, missDeigest := corer.localDAG.IsReceived(temp...)
	return ok, missDeigest
}

/*********************************Protocol***********************************************/
func (corer *Core) generatorBlock(round int) *Block {
	logger.Debug.Printf("procesing generatorBlock round %d \n", round)

	var block *Block
	if _, ok := corer.proposedFlag[round]; !ok {
		// GRBC round
		if round%WaveRound == 0 {
			if round == 0 {
				block = &Block{
					Author:    corer.nodeID,
					Round:     round,
					Batch:     corer.txpool.GetBatch(),
					Reference: make(map[crypto.Digest]NodeID),
				}
			} else {
				reference := corer.localDAG.GetRoundReceivedBlock(round - 1)
				if len(reference) >= corer.committee.HightThreshold() {
					block = &Block{
						Author:    corer.nodeID,
						Round:     round,
						Batch:     corer.txpool.GetBatch(),
						Reference: reference,
					}
				}
			}
		} else { // PBC round
			_, grade2nums := corer.localDAG.GetRoundReceivedBlockNums(round - 1)
			if grade2nums >= corer.committee.HightThreshold() {
				reference := corer.localDAG.GetRoundReceivedBlock(round - 1)
				block = &Block{
					Author:    corer.nodeID,
					Round:     round,
					Batch:     corer.txpool.GetBatch(),
					Reference: reference,
				}
			}
		}
	}

	if block != nil {
		corer.proposedFlag[round] = struct{}{}
		if block.Batch.Txs != nil {
			//BenchMark Log
			logger.Info.Printf("create Block round %d node %d batch_id %d \n", block.Round, block.Author, block.Batch.ID)
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
	if err := storeBlock(corer.store, propose.B); err != nil {
		return err
	}

	//Step 3: check reference
	if ok, miss := corer.checkReference(propose.B); !ok {
		//retrieve miss block
		corer.retriever.requestBlocks(miss, propose.Author, propose.B.Hash())

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

	instance := corer.getGRBCInstance(echo.Proposer, echo.Round)
	go instance.processEcho(echo)

	return nil
}

func (corer *Core) handleReady(ready *ReadyMsg) error {
	logger.Debug.Printf("procesing grbc ready round %d node %d \n", ready.Round, ready.Proposer)

	//Step 1: verify signature
	if !ready.Verify(corer.committee) {
		return ErrSignature(ready.MsgType(), ready.Round, int(ready.Author))
	}

	instance := corer.getGRBCInstance(ready.Proposer, ready.Round)
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
	if err := storeBlock(corer.store, propose.B); err != nil {
		return err
	}

	// Step 3: check reference
	if ok, miss := corer.checkReference(propose.B); !ok {
		//retrieve miss block
		corer.retriever.requestBlocks(miss, propose.Author, propose.B.Hash())

		if (propose.Round-1)%WaveRound != 0 { //如果前一轮是一个PB Round，必须等收到区块后开始投票
			return ErrReference(propose.MsgType(), propose.Round, int(propose.Author))
		}
	}

	//Step 4
	corer.handleOutPut(propose.B.Round, propose.B.Author, propose.B.Hash(), propose.B.Reference)

	return nil
}

func (corer *Core) handleOutPut(round int, node NodeID, digest crypto.Digest, references map[crypto.Digest]NodeID) error {
	logger.Debug.Printf("procesing output round %d node %d \n", round, node)

	corer.localDAG.ReceiveBlock(round, node, digest, references)

	if n, grade2nums := corer.localDAG.GetRoundReceivedBlockNums(round); n >= corer.committee.HightThreshold() {
		if round%WaveRound == 0 {
			if grade2nums >= corer.committee.HightThreshold() {
				if _, ok := corer.proposedNotify[round+1]; !ok {
					corer.proposedNotify[round+1] = &sync.Mutex{} // first
					//timeout
					time.AfterFunc(time.Millisecond*time.Duration(corer.parameters.NetwrokDelay), func() {
						mu := corer.proposedNotify[round+1]
						if mu.TryLock() {
							corer.advanceRound(round + 1)
						}
					})
				}
				if grade2nums == corer.committee.Size() {
					mu := corer.proposedNotify[round+1] // second
					if mu.TryLock() {
						corer.advanceRound(round + 1)
					}
				}
			}

		} else {
			return corer.advanceRound(round + 1)
		}
	}

	return nil
}

func (corer *Core) advanceRound(round int) error {

	logger.Debug.Printf("procesing advance round %d \n", round)

	if block := corer.generatorBlock(round); block != nil {
		if round%WaveRound == 0 {
			if propose, err := NewGRBCProposeMsg(corer.nodeID, round, block, corer.sigService); err != nil {
				return err
			} else {
				corer.transmitor.Send(corer.nodeID, NONE, propose)
				time.Sleep(time.Millisecond * time.Duration(corer.parameters.MinBlockDelay))
				corer.transmitor.RecvChannel() <- propose
			}
		} else {
			if propose, err := NewPBCProposeMsg(corer.nodeID, round, block, corer.sigService); err != nil {
				return err
			} else {
				corer.transmitor.Send(corer.nodeID, NONE, propose)
				time.Sleep(time.Millisecond * time.Duration(corer.parameters.MinBlockDelay))
				// invoke elect phase
				corer.transmitor.RecvChannel() <- propose
				corer.invokeElect(round)
			}
		}
	}

	return nil
}

func (corer *Core) invokeElect(round int) error {
	if round%WaveRound == 1 {
		elect, err := NewElectMsg(
			corer.nodeID,
			round,
			corer.sigService,
		)
		if err != nil {
			return err
		}
		corer.transmitor.Send(corer.nodeID, NONE, elect)
		corer.transmitor.RecvChannel() <- elect
	}
	return nil
}

func (corer *Core) handleElect(elect *ElectMsg) error {
	logger.Debug.Printf("procesing elect wave %d node %d \n", elect.Round/WaveRound, elect.Author)

	if leader, err := corer.eletor.Add(elect); err != nil {
		return err
	} else if leader != NONE {
		grade := corer.localDAG.GetGrade(elect.Round-1, int(leader))
		logger.Debug.Printf("Elector: wave %d leader %d grade %d \n", elect.Round/WaveRound, leader, grade)
		//is grade two?
		if grade == GradeTwo {
			corer.commitor.NotifyToCommit(elect.Round / WaveRound)
		}

	}

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

	for _, block := range reply.Blocks {
		if block.Round%WaveRound == 0 {
			corer.localDAG.UpdateGrade(block.Round, int(block.Author), GradeOne)
		}

		//maybe execute more one
		storeBlock(corer.store, block)

		corer.handleOutPut(block.Round, block.Author, block.Hash(), block.Reference)
	}

	go corer.retriever.processReply(reply)

	return nil
}

func (corer *Core) handleLoopBack(block *Block) error {
	logger.Debug.Printf("procesing block loop back round %d node %d \n", block.Round, block.Author)

	//GRBC round
	if block.Round%WaveRound == 0 {
		instance := corer.getGRBCInstance(block.Author, block.Round)
		go instance.processPropose(block)
	} else {
		return corer.handleOutPut(block.Round, block.Author, block.Hash(), block.Reference)
	}

	return nil
}

func (corer *Core) handleCallBack(req *callBackReq) error {
	logger.Debug.Printf("procesing block call back round %d node %d \n", req.round, req.nodeID)

	//Update grade
	corer.localDAG.UpdateGrade(req.round, int(req.nodeID), req.grade)

	//try to advance round
	if req.tag == UpdateGrade {
		return corer.advanceRound(req.round + 1)
	} else if req.tag == NotifyOutPut {
		return corer.handleOutPut(req.round, req.nodeID, req.digest, req.reference)
	}

	return nil
}

func (corer *Core) Run() {
	if corer.nodeID >= NodeID(corer.parameters.Faults) {
		//first propose
		block := corer.generatorBlock(0)
		if propose, err := NewGRBCProposeMsg(corer.nodeID, 0, block, corer.sigService); err != nil {
			logger.Error.Println(err)
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

			if err != nil {
				logger.Warn.Println(err)
			}

		}
	}
}
