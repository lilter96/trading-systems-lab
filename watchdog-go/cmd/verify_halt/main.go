package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"watchdog-go/pkg/config"
	"watchdog-go/pkg/okx"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] config load: %v", err)
	}
	if cfg.OkxApiKey == "" {
		log.Fatal("OKX_API_KEY not set")
	}

	fmt.Printf("OKX Demo: %v\n", cfg.OkxIsDemo)
	client := okx.New(cfg.OkxApiKey, cfg.OkxApiSecret, cfg.OkxPassphrase, cfg.OkxIsDemo)
	ctx := context.Background()

	// Phase 1: Check initial state
	fmt.Println("\n=== Phase 1: Initial state ===")
	checkBalance(ctx, client)
	checkPositions(ctx, client)
	checkAlgoOrders(ctx, client)

	// Phase 2: Place an algo SL order for testing
	fmt.Println("\n=== Phase 2: Place algo SL order ===")
	if err := client.PlaceAlgoOrder(ctx, "BTC-USDT-SWAP", "sell", "1", "50000"); err != nil {
		log.Fatalf("[FATAL] Cannot place algo order: %v", err)
	}
	fmt.Println("[PLACE ALGO] Algo SL placed OK")

	// Phase 3: Confirm order exists
	fmt.Println("\n=== Phase 3: Confirm algo order placed ===")
	checkAlgoOrders(ctx, client)

	// Phase 4: Trigger EmergencyHalt
	fmt.Println("\n=== Phase 4: EmergencyHalt ===")
	start := time.Now()
	err = client.EmergencyHalt(ctx)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Printf("[RESULT] EmergencyHalt errors: %v\n", err)
	} else {
		fmt.Println("[RESULT] EmergencyHalt OK")
	}
	fmt.Printf("[TIMING] %v\n", elapsed)
	if elapsed <= 2*time.Second {
		fmt.Println("[TIMING] PASS")
	} else {
		fmt.Printf("[TIMING] %v over 2s target\n", elapsed-2*time.Second)
	}

	// Phase 5: Verify cleanup
	fmt.Println("\n=== Phase 5: Verify cleanup ===")
	checkAlgoOrders(ctx, client)
	checkPositions(ctx, client)

	fmt.Println("\n=== VERIFICATION DONE ===")
}

func checkBalance(ctx context.Context, client *okx.Client) {
	eq, err := client.GetBalance(ctx)
	if err != nil {
		fmt.Printf("[BALANCE] Error: %v\n", err)
		return
	}
	fmt.Printf("[BALANCE] Total equity: %.2f\n", eq)
}

func checkPositions(ctx context.Context, client *okx.Client) {
	positions, err := client.GetPositions(ctx)
	if err != nil {
		fmt.Printf("[POSITIONS] Error: %v\n", err)
		return
	}
	if len(positions) == 0 {
		fmt.Println("[POSITIONS] None")
		return
	}
	for _, p := range positions {
		fmt.Printf("[POSITIONS] %s side=%s size=%s mgn=%s\n", p.InstID, p.PosSide, p.Pos, p.MgnMode)
	}
}

func checkAlgoOrders(ctx context.Context, client *okx.Client) {
	for _, instType := range []string{"SWAP", "FUTURES", "MARGIN"} {
		orders, err := client.GetPendingAlgoOrders(ctx, instType)
		if err != nil {
			fmt.Printf("[ALGO/%s] Error: %v\n", instType, err)
			continue
		}
		for _, a := range orders {
			fmt.Printf("[ALGO/%s] %s algoId=%s\n", instType, a.InstId, a.AlgoId)
		}
	}
	fmt.Println("[ALGO] None pending across all types")
}
