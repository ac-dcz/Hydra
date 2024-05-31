package core

import "fmt"

var (
	ErrSignature = func(msgTyp, round, node int) error {
		return fmt.Errorf("[type-%d-round-%d-node-%d] message signature verify error", msgTyp, round, node)
	}

	ErrReference = func(msgTyp, round, node int) error {
		return fmt.Errorf("[type-%d-round-%d-node-%d] not receive all block reference ", msgTyp, round, node)
	}

	ErrUsedElect = func(msgTyp, round, node int) error {
		return fmt.Errorf("[type-%d-round-%d-node-%d] receive one more elect msg from %d ", msgTyp, round, node, node)
	}
)
