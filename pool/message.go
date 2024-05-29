package pool

type Transaction []byte

type Batch struct {
	ID  int
	Txs []Transaction
}
