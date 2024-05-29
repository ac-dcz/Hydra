package pool

type Parameters struct {
	Rate        int `json:"rate"`          //tx send rate
	TxSize      int `json:"tx_size"`       // byte
	BatchSize   int `json:"batch_size"`    // the max number of tx that a batch can hold
	MaxPoolSize int `json:"max_pool_size"` // max pool size
}

var DefaultParameters = &Parameters{
	Rate:        1_000,
	TxSize:      256,
	BatchSize:   200,
	MaxPoolSize: 1_000,
}
