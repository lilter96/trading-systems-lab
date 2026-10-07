package okxclient

import "sync"

type TickRingBuffer struct {
	mu    sync.RWMutex
	slots []Slot
	mask  uint64
	write uint64
}

func NewTickRingBuffer(capacity int) *TickRingBuffer {
	power := 1
	for power < capacity {
		power <<= 1
	}
	return &TickRingBuffer{
		slots: make([]Slot, power),
		mask:  uint64(power - 1),
	}
}

func (rb *TickRingBuffer) Push(tick *Tick) {
	rb.mu.Lock()
	idx := rb.write & rb.mask
	rb.slots[idx].Tick = *tick
	rb.write++
	rb.mu.Unlock()
}

func (rb *TickRingBuffer) GetRecent(dest []Tick) int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	n := len(dest)
	if n == 0 || rb.write == 0 {
		return 0
	}

	size := uint64(len(rb.slots))
	count := uint64(n)
	if count > size {
		count = size
	}
	if count > rb.write {
		count = rb.write
	}

	start := rb.write - count
	for i := uint64(0); i < count; i++ {
		dest[i] = rb.slots[(start+i)&rb.mask].Tick
	}
	return int(count)
}
