package pool

type Parameters struct {
	Rate         int `json:"rate"`           //tx send rate
	TxSize       int `json:"tx_size"`        // byte
	BatchSize    int `json:"batch_size"`     // the max number of tx that a batch can hold
	MaxQueueSize int `json:"max_queue_size"` // max queue size
}

var DefaultParameters = Parameters{
	Rate:         1_000,
	TxSize:       16,
	BatchSize:    200,
	MaxQueueSize: 10_000,
}
