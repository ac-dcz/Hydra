package core

import "sync/atomic"

type GRBC struct {
	corer    *Core
	proposer NodeID
	round    int
	grade    atomic.Int32
}

func NewGRBC(corer *Core, proposer NodeID, round int) *GRBC {
	grbc := &GRBC{
		corer:    corer,
		proposer: proposer,
		round:    round,
	}

	return grbc
}

func (g *GRBC) processPropose(propose *GRBCProposeMsg) {

}

func (g *GRBC) processEcho(echo *EchoMsg) {

}

func (g *GRBC) processReady(echo *ReadyMsg) {

}

func (g *GRBC) Grade() int {
	return int(g.grade.Load())
}
