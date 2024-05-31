package core

type Commitor struct {
	elector       *Elector
	commitChannel chan<- *Block
}

func (c *Commitor) Push(waveNum int, leader NodeID) {

}
