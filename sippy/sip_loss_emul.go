package sippy

import (
	"fmt"
	"sync/atomic"
)

// LossEmulMode selects which SIP packet directions to drop for testing.
type LossEmulMode uint8

const (
	LossEmulRecv LossEmulMode = 1 << iota
	LossEmulSend
	LossEmulBoth = LossEmulRecv | LossEmulSend
)

type sipLossBudget struct {
	recv, send atomic.Int64
}

// SetLossEmul replaces the manager-wide loss budget. It drops the next count
// packets in each selected direction (count per direction in LossEmulBoth).
// A zero count disables it. Incoming packets rejected by the minimum SIP size
// check and outgoing packets already dropped by legacy response loss emulation
// do not consume this budget. The settings are safe to replace during traffic.
func (self *sipTransactionManager) SetLossEmul(mode LossEmulMode, count int) error {
	if mode == 0 || mode&^LossEmulBoth != 0 || count < 0 {
		return fmt.Errorf("invalid SIP loss emulation mode or count: %d, %d", mode, count)
	}
	if count == 0 {
		self.loss_emul.Store(nil)
		return nil
	}
	budget := &sipLossBudget{}
	if mode&LossEmulRecv != 0 {
		budget.recv.Store(int64(count))
	}
	if mode&LossEmulSend != 0 {
		budget.send.Store(int64(count))
	}
	self.loss_emul.Store(budget)
	return nil
}

func consumeSipLoss(remaining *atomic.Int64) bool {
	for n := remaining.Load(); n > 0; n = remaining.Load() {
		if remaining.CompareAndSwap(n, n-1) {
			return true
		}
	}
	return false
}
