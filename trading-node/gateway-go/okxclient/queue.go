package okxclient

import "sync/atomic"

type SPSCTickQueue struct {
	_padding0 [8]uint64
	slots     []Tick
	mask      uint64
	_padding1 [8]uint64
	writePtr  uint64
	_padding2 [8]uint64
	readPtr   uint64
	_padding3 [8]uint64
}

func NewSPSCTickQueue(capacity int) *SPSCTickQueue {
	power := 1
	for power < capacity {
		power <<= 1
	}
	return &SPSCTickQueue{
		slots: make([]Tick, power),
		mask:  uint64(power - 1),
	}
}

func (q *SPSCTickQueue) Push(tick *Tick) bool {
	write := atomic.LoadUint64(&q.writePtr)
	read := atomic.LoadUint64(&q.readPtr)

	if write-read >= uint64(len(q.slots)) {
		return false
	}

	idx := write & q.mask
	q.slots[idx] = *tick

	atomic.StoreUint64(&q.writePtr, write+1)
	return true
}

func (q *SPSCTickQueue) Pop(dest *Tick) bool {
	read := atomic.LoadUint64(&q.readPtr)
	write := atomic.LoadUint64(&q.writePtr)

	if read == write {
		return false
	}

	idx := read & q.mask
	*dest = q.slots[idx]

	atomic.StoreUint64(&q.readPtr, read+1)
	return true
}
