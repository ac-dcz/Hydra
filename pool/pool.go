package pool

import (
	"lightDAG/logger"
	"sync"
	"time"
)

type TxQueue struct {
	mu           sync.Mutex
	queue        []Batch
	wind         int // write index
	rind         int // read index
	maxQueueSize int
}

func NewTxQueue(maxQueueSize int) *TxQueue {
	return &TxQueue{
		mu:           sync.Mutex{},
		queue:        make([]Batch, maxQueueSize),
		wind:         0,
		rind:         -1,
		maxQueueSize: maxQueueSize,
	}
}

func (q *TxQueue) Add(batch Batch) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.wind == q.rind {
		logger.Warn.Println("Transaction pool is full")
		return
	}
	q.queue[q.wind] = batch
	q.wind = (q.wind + 1) % q.maxQueueSize
}

func (q *TxQueue) Get() Batch {
	q.mu.Lock()
	defer q.mu.Unlock()
	if (q.rind+1)%q.maxQueueSize == q.wind { // pool is empty
		return Batch{}
	} else {
		q.rind = (q.rind + 1) % q.maxQueueSize
		batch := q.queue[q.rind]
		return batch
	}
}

const PRECISION = 20 // Sample precision.
const BURST_DURATION = 1000 / PRECISION

type BatchMaker struct {
	txSize    int
	batchSize int
	rate      int
	N         int // the size of committee
	id        int
}

func NewBatchMaker(txSize, batchSize, rate int, N, Id int) *BatchMaker {
	return &BatchMaker{
		txSize:    txSize,
		batchSize: batchSize,
		rate:      rate,
	}
}

func (maker *BatchMaker) Run(batchChannel chan<- Batch) {
	ticker := time.NewTicker(time.Microsecond * BURST_DURATION)
	cnt := 0
	nums := maker.rate / PRECISION
	for range ticker.C {
		for i := 0; i < nums; i++ {
			batch := Batch{
				ID:  maker.id + maker.batchSize*cnt,
				Txs: make([]Transaction, maker.batchSize),
			}

			//BenchMark print batch create time
			logger.Info.Printf("Received Batch %d\n", batch.ID)

			batchChannel <- batch
			cnt++
		}
	}
}

type Pool struct {
	parameters Parameters
	queue      *TxQueue
	maker      *BatchMaker
}

func NewPool(parameters Parameters, N, Id int) *Pool {
	p := &Pool{
		parameters: parameters,
		queue:      NewTxQueue(parameters.MaxPoolSize),
		maker:      NewBatchMaker(parameters.TxSize, parameters.BatchSize, parameters.Rate, N, Id),
	}
	batchChannel := make(chan Batch, 1000)

	go p.maker.Run(batchChannel)

	go func() {
		for batch := range batchChannel {
			p.queue.Add(batch)
		}
	}()

	return p
}

func (p *Pool) GetBatch() Batch {
	return p.queue.Get()
}
