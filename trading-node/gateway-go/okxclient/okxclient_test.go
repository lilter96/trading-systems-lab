package okxclient

import (
    "runtime"
    "sync"
    "sync/atomic"
    "testing"
    "time"
)

// ============================================================================
// 1. Race Detector Validation (Run with: go test -race -run=TestRace)
// ============================================================================

func TestSPSCTickQueue_Race(t *testing.T) {
    q := NewSPSCTickQueue(1024)
    const total = 50000

    var wg sync.WaitGroup
    wg.Add(2)

    // Single Producer
    go func() {
        defer wg.Done()
        for i := 0; i < total; i++ {
            tick := Tick{Price: float64(i)}
            // Spin-yield if full
            for !q.Push(&tick) {
                runtime.Gosched()
            }
        }
    }()

    // Single Consumer
    go func() {
        defer wg.Done()
        count := 0
        var tick Tick
        for count < total {
            if q.Pop(&tick) {
                if tick.Price != float64(count) {
                    t.Errorf("Data corruption: expected sequence %d, got %f", count, tick.Price)
                }
                count++
            } else {
                runtime.Gosched()
            }
        }
    }()

    wg.Wait()
}

func TestTickRingBuffer_Race(t *testing.T) {
    rb := NewTickRingBuffer(1024)
    const duration = 500 * time.Millisecond

    var wg sync.WaitGroup
    wg.Add(2)

    var stop int32

    // Concurrent Writer (e.g. WS Reader)
    go func() {
        defer wg.Done()
        var tick Tick
        i := 0
        for atomic.LoadInt32(&stop) == 0 {
            tick.Price = float64(i)
            rb.Push(&tick)
            i++
        }
    }()

    // Concurrent Reader (e.g. Indicator/Strategy Loop)
    go func() {
        defer wg.Done()
        dest := make([]Tick, 10)
        for atomic.LoadInt32(&stop) == 0 {
            n := rb.GetRecent(dest)
            // Read the returned elements to verify no read-write collision panics
            for j := 0; j < n; j++ {
                _ = dest[j].Price
            }
            runtime.Gosched()
        }
    }()

    time.Sleep(duration)
    atomic.StoreInt32(&stop, 1)
    wg.Wait()
}

// ============================================================================
// 2. Stress Test for Queue (50k ticks/sec with Flat Memory)
// ============================================================================

func TestSPSCTickQueue_50kTPS_FlatMemory(t *testing.T) {
    client := NewOKXClient("wss://dummy")
    queue := NewSPSCTickQueue(262144) // 256k slot queue to hold stress volume

    err := client.Subscribe("tickers", "BTC-USDT", queue)
    if err != nil {
        t.Fatalf("Failed to initialize subscription: %v", err)
    }

    routingMap := make(map[RouteKey]*SPSCTickQueue)
    var rKey RouteKey
    copy(rKey.Channel[:], "tickers")
    copy(rKey.InstID[:], "BTC-USDT")
    routingMap[rKey] = queue
    client.routingTable.Store(routingMap)

    // 1. Prepare raw public tickers channel payload
    rawMsg := []byte(`{"arg":{"channel":"tickers","instId":"BTC-USDT"},"data":[{"last":"59123.15","ts":"1694212341234"}]}`)

    // 2. Stabilize heap before measurement
    runtime.GC()
    var memStart runtime.MemStats
    runtime.ReadMemStats(&memStart)

    // 200,000 ticks equates to 4 seconds of continuous execution at 50k ticks/sec
    totalTicks := 200000
    start := time.Now()

    var tick Tick
    for i := 0; i < totalTicks; i++ {
        // Exercise key scanning, CoW routing, data splitting, float/int parsing, and pushing
        client.routeMessage(rawMsg)

        // Dequeue concurrently inside the loop to simulate real-time processing
        if !queue.Pop(&tick) {
            t.Fatalf("Queue encountered underflow at step %d", i)
        }
    }

    elapsed := time.Since(start)
    throughput := float64(totalTicks) / elapsed.Seconds()

    // GC to clear transient runtime stacks if any
    runtime.GC()
    var memEnd runtime.MemStats
    runtime.ReadMemStats(&memEnd)

    // Calculate change in overall heap allocations
    allocDiff := memEnd.TotalAlloc - memStart.TotalAlloc

    t.Logf("Processed %d ticks in %v", totalTicks, elapsed)
    t.Logf("Measured Throughput: %.2f ticks/sec", throughput)
    t.Logf("Allocated Memory on Hot Path: %d bytes", allocDiff)

    // Threshold checks
    if throughput < 50000 {
        t.Errorf("Throughput of %.2f ticks/sec fell below the 50,000 target", throughput)
    }

    // Any value under 8KB accounts for minor Go runtime system book-keeping allocations.
    // The hot path itself operates at 0 bytes.
    if allocDiff > 8192 {
        t.Errorf("High memory growth detected on hot path: %d bytes allocated", allocDiff)
    }
}

// ============================================================================
// 3. Stress Test for Ring Buffer (50k ticks/sec with Flat Memory)
// ============================================================================

func TestTickRingBuffer_50kTPS_FlatMemory(t *testing.T) {
    rb := NewTickRingBuffer(16384)
    runtime.GC()
    var memStart runtime.MemStats
    runtime.ReadMemStats(&memStart)

    totalTicks := 200000
    start := time.Now()

    var tick Tick
    for i := 0; i < totalTicks; i++ {
        tick.Price = float64(i)
        rb.Push(&tick)

        if i%1000 == 0 {
            dest := make([]Tick, 10)
            rb.GetRecent(dest)
        }
    }

    elapsed := time.Since(start)
    throughput := float64(totalTicks) / elapsed.Seconds()

    runtime.GC()
    var memEnd runtime.MemStats
    runtime.ReadMemStats(&memEnd)

    allocDiff := memEnd.TotalAlloc - memStart.TotalAlloc

    t.Logf("Processed %d ticks in %v", totalTicks, elapsed)
    t.Logf("Measured Throughput: %.2f ticks/sec", throughput)
    t.Logf("Allocated Memory on Hot Path: %d bytes", allocDiff)

    if throughput < 50000 {
        t.Errorf("Throughput of %.2f ticks/sec fell below the 50,000 target", throughput)
    }

    if allocDiff > 8192 {
        t.Errorf("High memory growth detected on hot path: %d bytes allocated", allocDiff)
    }
}

// ============================================================================
// 4. Hot-Path Microbenchmark (Run with: go test -bench=.)
// ============================================================================

func BenchmarkWSClient_RouteMessage_HotPath(b *testing.B) {
    client := NewOKXClient("wss://dummy")
    queue := NewSPSCTickQueue(131072)

    err := client.Subscribe("tickers", "BTC-USDT", queue)
    if err != nil {
        b.Fatalf("Failed to subscribe: %v", err)
    }

    routingMap := make(map[RouteKey]*SPSCTickQueue)
    var rKey RouteKey
    copy(rKey.Channel[:], "tickers")
    copy(rKey.InstID[:], "BTC-USDT")
    routingMap[rKey] = queue
    client.routingTable.Store(routingMap)

    rawMsg := []byte(`{"arg":{"channel":"tickers","instId":"BTC-USDT"},"data":[{"last":"59123.15","ts":"1694212341234"}]}`)

    b.ReportAllocs()
    b.ResetTimer()

    var tick Tick
    for i := 0; i < b.N; i++ {
        client.routeMessage(rawMsg)
        queue.Pop(&tick)
    }
}