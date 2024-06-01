package core

import (
	"lightDAG/crypto"
	"sync"
)

type LocalDAG struct {
	muBlock      *sync.RWMutex
	blockDigests map[crypto.Digest]NodeID // store hash of block that has received
	muDAG        *sync.RWMutex
	localDAG     map[int]map[NodeID]crypto.Digest // local DAG
	muGrade      *sync.RWMutex
	gradeDAG     map[int]map[NodeID]int
}

func NewLocalDAG() *LocalDAG {
	return &LocalDAG{
		muBlock:      &sync.RWMutex{},
		muDAG:        &sync.RWMutex{},
		muGrade:      &sync.RWMutex{},
		blockDigests: make(map[crypto.Digest]NodeID),
		localDAG:     make(map[int]map[NodeID]crypto.Digest),
		gradeDAG:     make(map[int]map[NodeID]int),
	}
}

// IsReceived: digests is received ?
func (local *LocalDAG) IsReceived(digests ...crypto.Digest) (bool, []crypto.Digest) {
	local.muBlock.RLock()
	defer local.muBlock.RUnlock()

	var miss []crypto.Digest
	var flag bool = true
	for _, d := range digests {
		if _, ok := local.blockDigests[d]; !ok {
			miss = append(miss, d)
			flag = false
		}
	}

	return flag, miss
}

func (local *LocalDAG) ReceiveBlock(round int, node NodeID, digest crypto.Digest) {
	local.muBlock.Lock()
	local.blockDigests[digest] = node
	local.muBlock.Unlock()

	local.muDAG.Lock()
	slot, ok := local.localDAG[round]
	if !ok {
		slot = make(map[NodeID]crypto.Digest)
		local.localDAG[round] = slot
	}
	slot[node] = digest
	local.muDAG.Unlock()
}

func (local *LocalDAG) GetReceivedBlockNums(round int) (nums, grade2nums int) {
	local.muDAG.RLock()
	defer local.muDAG.RUnlock()
	local.muGrade.RLock()
	defer local.muGrade.RUnlock()

	nums = len(local.localDAG[round])
	if round%WaveRound == 0 {
		for _, g := range local.gradeDAG[round] {
			if g == GradeTwo {
				grade2nums++
			}
		}
	}

	return
}

func (local *LocalDAG) GetReceivedBlock(round int) (digests map[crypto.Digest]NodeID) {
	local.muDAG.RLock()
	defer local.muDAG.RUnlock()
	digests = make(map[crypto.Digest]NodeID)
	for id, d := range local.localDAG[round] {
		digests[d] = id
	}

	return digests
}

func (local *LocalDAG) UpdateGrade(round, node, grade int) {
	if round%WaveRound == 0 {
		local.muGrade.Lock()
		local.gradeDAG[round][NodeID(node)] = grade
		local.muGrade.Unlock()
	}
}

type Commitor struct {
	elector       *Elector
	commitChannel chan<- *Block
	localDAG      *LocalDAG
}

func NewCommitor(electot *Elector, localDAG *LocalDAG, commitChannel chan<- *Block) *Commitor {
	return &Commitor{
		elector:       electot,
		localDAG:      localDAG,
		commitChannel: commitChannel,
	}
}

func (c *Commitor) Push(waveNum int, leader NodeID) {

}
