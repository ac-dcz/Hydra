package pool

import (
	"WuKong/logger"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	logger.SetOutput(logger.InfoLevel, logger.NewFileWriter("./info.log"))
	pool := NewPool(DefaultParameters, 4, 0)
	time.Sleep(time.Second * 10)
	pool.GetBatch()
}
