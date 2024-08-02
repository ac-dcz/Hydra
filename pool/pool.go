package pool

import (
	"WuKong/logger"
	"time"
)

type txQueue struct {
	queue        []Transaction
	batchChannel chan Batch
	wind         int // write index
	rind         int // read index
	nums         int
	maxQueueSize int
	batchSize    int
	N            int
	Id           int
	bcnt         int
}

func newTxQueue(
	maxQueueSize, batchSize int,
	batchChannel chan Batch,
	N, Id int,
) *txQueue {
	r := &txQueue{
		queue:        make([]Transaction, maxQueueSize),
		batchChannel: batchChannel,
		wind:         0,
		rind:         -1,
		nums:         0,
		maxQueueSize: maxQueueSize,
		batchSize:    batchSize,
		N:            N,
		Id:           Id,
		bcnt:         0,
	}
	return r
}

func (q *txQueue) run(txChannel <-chan Transaction) {

	for tx := range txChannel {
		if q.wind == q.rind {
			logger.Warn.Println("Transaction pool is full")
			continue
		}
		q.queue[q.wind] = tx
		q.wind = (q.wind + 1) % q.maxQueueSize
		q.nums++
		if q.nums >= q.batchSize {
			q.make()
		}
	}
}

func (q *txQueue) make() {
	batch := Batch{
		ID: q.Id + q.N*q.bcnt,
	}
	defer func() {
		q.bcnt++
		//BenchMark print batch create time
		logger.Info.Printf("Received Batch %d\n", batch.ID)
	}()

	for i := 0; i < q.batchSize; i++ {
		q.rind = (q.rind + 1) % q.maxQueueSize
		batch.Txs = append(batch.Txs, q.queue[q.rind])
		q.nums--
	}
	q.batchChannel <- batch
}

func (q *txQueue) get() Batch {
	if len(q.batchChannel) > 0 {
		return <-q.batchChannel
	} else {
		return Batch{
			ID:  -1,
			Txs: nil,
		}
	}
}

// func (q *TxQueue)

const PRECISION = 20 // Sample precision.
const BURST_DURATION = 1000 / PRECISION

type txMaker struct {
	txSize int
	rate   int
}

func newTxMaker(txSize, rate int) *txMaker {
	return &txMaker{
		txSize: txSize,
		rate:   rate,
	}
}

func (maker *txMaker) run(txChannel chan<- Transaction) {
	ticker := time.NewTicker(time.Millisecond * BURST_DURATION)
	nums := maker.rate / PRECISION
	for range ticker.C {
		for i := 0; i < nums; i++ {
			tx := make(Transaction, maker.txSize)
			txChannel <- tx
		}
	}
}

type Pool struct {
	parameters   Parameters
	queue        *txQueue
	maker        *txMaker
	txChannel    chan Transaction
	batchChannel chan Batch
}

func NewPool(parameters Parameters, N, Id int) *Pool {

	logger.Info.Printf(
		"Transaction pool queue capacity set to %d \n",
		parameters.MaxQueueSize,
	)
	logger.Info.Printf(
		"Transaction pool tx size set to %d \n",
		parameters.TxSize,
	)
	logger.Info.Printf(
		"Transaction pool batch size set to %d \n",
		parameters.BatchSize,
	)
	logger.Info.Printf(
		"Transaction pool tx rate set to %d \n",
		parameters.Rate,
	)

	batchChannel, txChannel := make(chan Batch, 1_000), make(chan Transaction, 10_000)
	p := &Pool{
		parameters:   parameters,
		txChannel:    txChannel,
		batchChannel: batchChannel,
	}

	p.queue = newTxQueue(
		parameters.MaxQueueSize,
		parameters.BatchSize,
		batchChannel,
		N,
		Id,
	)

	p.maker = newTxMaker(
		parameters.TxSize,
		parameters.Rate,
	)

	return p
}

func (p *Pool) Run() {
	go p.queue.run(p.txChannel)
	go p.maker.run(p.txChannel)
}

func (p *Pool) GetBatch() Batch {
	return p.queue.get()
}
