package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trading-node/gateway-go/okxclient"
)

func main() {
	creds := loadCredentials()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	client := okxclient.NewPrivateClient(creds, true)
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Close()
	client.StartSupervisor(ctx)

	client.Subscribe("orders", "SWAP", "BTC-USDT-SWAP")
	client.Subscribe("positions", "SWAP", "BTC-USDT-SWAP")
	log.Println("subscribed to orders and positions")
	time.Sleep(2 * time.Second)

	acctLv := fetchAccountConfig(ctx, client)
	_ = acctLv

	inst := fetchInstrumentConfig(ctx, client)
	availUSDT := fetchBalance(ctx, client)
	setLeverage(ctx, client)
	currentPrice := fetchCurrentPrice(ctx, client)
	sz := calculateOrderSize(availUSDT, 5, inst, currentPrice)

	ordID := placeMarketOrder(ctx, client, sz)

	fillPrice, fillSz := waitForFill(ctx, client, ordID, sigCh)

	placeTPSL(ctx, client, fillPrice, fillSz)

	monitor(ctx, client, sigCh, 15*time.Second)

	log.Println("demo complete")
}

func loadCredentials() okxclient.Credentials {
	creds, err := okxclient.LoadCredentialsFromFile(".env")
	if err != nil {
		creds, err = okxclient.LoadCredentialsFromEnv()
		if err != nil {
			log.Fatalf("credentials: create .env file or set OKX_API_KEY/OKX_SECRET_KEY/OKX_PASSPHRASE env vars")
		}
	}
	return creds
}

func fetchAccountConfig(ctx context.Context, client *okxclient.PrivateClient) string {
	resp, err := client.Rest.GetAccountConfig(ctx)
	if err != nil {
		log.Fatalf("get account config: %v", err)
	}
	var data []struct {
		AcctLv string `json:"acctLv"`
	}
	if err := parseData(resp.Data, &data); err != nil {
		log.Fatalf("parse account config: %v", err)
	}
	lv := ""
	if len(data) > 0 {
		lv = data[0].AcctLv
	}
	log.Printf("account level: %s (1=simple, 2=single-ccy margin, 3=multi-ccy margin, 4=portfolio margin)", lv)
	return lv
}

func fetchInstrumentConfig(ctx context.Context, client *okxclient.PrivateClient) swapInstrument {
	resp, err := client.Rest.GetInstruments(ctx, "SWAP", "BTC-USDT-SWAP")
	if err != nil {
		log.Fatalf("get instruments: %v", err)
	}
	if resp.Code != "0" {
		log.Fatalf("instruments api error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	var insts []swapInstrument
	if err := parseData(resp.Data, &insts); err != nil {
		log.Fatalf("parse instruments: %v", err)
	}
	if len(insts) == 0 {
		log.Fatal("no instrument data")
	}
	inst := insts[0]
	log.Printf("instrument: ctVal=%s minSz=%s lotSz=%s", inst.CtVal, inst.MinSz, inst.LotSz)
	return inst
}

func fetchBalance(ctx context.Context, client *okxclient.PrivateClient) float64 {
	resp, err := client.Rest.GetBalance(ctx, "USDT")
	if err != nil {
		log.Fatalf("get balance: %v", err)
	}
	var bals []balanceData
	if err := parseData(resp.Data, &bals); err != nil {
		log.Fatalf("parse balance: %v", err)
	}
	if len(bals) == 0 || len(bals[0].Details) == 0 {
		log.Fatal("no balance data")
	}
	avail := parseFloat(bals[0].Details[0].AvailBal)
	log.Printf("available USDT: %.2f", avail)
	return avail
}

func setLeverage(ctx context.Context, client *okxclient.PrivateClient) {
	const lever = 5
	resp, err := client.Rest.SetLeverage(ctx, "BTC-USDT-SWAP", fmt.Sprintf("%d", lever), "cross")
	if err != nil {
		log.Fatalf("set leverage: %v", err)
	}
	if resp.Code != "0" {
		log.Fatalf("set leverage error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	log.Printf("leverage set to %dx", lever)
}

func fetchCurrentPrice(ctx context.Context, client *okxclient.PrivateClient) float64 {
	resp, err := client.Rest.GetTicker(ctx, "BTC-USDT-SWAP")
	if err != nil {
		log.Fatalf("get ticker: %v", err)
	}
	var data []struct {
		Last string `json:"last"`
	}
	if err := parseData(resp.Data, &data); err != nil {
		log.Fatalf("parse ticker: %v", err)
	}
	price := parseFloat(data[0].Last)
	log.Printf("current BTC-USDT-SWAP price: %.1f", price)
	return price
}

func calculateOrderSize(availUSDT float64, lever int, inst swapInstrument, price float64) string {
	ctVal := parseFloat(inst.CtVal)
	lotSz := parseFloat(inst.LotSz)
	minSz := parseFloat(inst.MinSz)

	notional90 := availUSDT * float64(lever) * 0.9
	sz := math.Floor(notional90/(ctVal*price)/lotSz) * lotSz

	if sz < minSz {
		log.Fatalf("sz=%.4f < minSz=%.4f — need more USDT", sz, minSz)
	}
	log.Printf("order size: sz=%.2f contracts (%.2f USDT, margin=%.2f)", sz, sz*ctVal*price, sz*ctVal*price/float64(lever))
	return fmt.Sprintf("%.2f", sz)
}

func placeMarketOrder(ctx context.Context, client *okxclient.PrivateClient, sz string) string {
	entry := &okxclient.OrderRequest{
		InstID:  "BTC-USDT-SWAP",
		TDMode:  "cross",
		Side:    "buy",
		OrdType: "market",
		Sz:      sz,
	}
	resp, err := client.Rest.PlaceOrder(ctx, entry)
	if err != nil {
		log.Fatalf("place entry: %v", err)
	}
	var data []struct {
		OrdID string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}
	if err := parseData(resp.Data, &data); err != nil {
		log.Fatalf("parse order response: %v", err)
	}
	if resp.Code != "0" || (len(data) > 0 && data[0].SCode != "0") {
		if len(data) > 0 {
			log.Fatalf("entry order rejected: code=%s msg=%s sCode=%s sMsg=%s", resp.Code, resp.Msg, data[0].SCode, data[0].SMsg)
		}
		log.Fatalf("entry order error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	log.Printf("entry order accepted: ordId=%s", data[0].OrdID)
	return data[0].OrdID
}

func waitForFill(ctx context.Context, client *okxclient.PrivateClient, ordID string, sigCh chan os.Signal) (fillPrice, fillSz string) {
	log.Println("waiting for fill notification...")
	timeout := time.After(30 * time.Second)

	for {
		select {
		case update := <-client.Orders:
			if update.State != "filled" {
				log.Printf("order update: state=%s ordId=%s side=%s accFillSz=%.0f",
					update.State, update.OrdID, update.Side, update.AccFillSz)
				continue
			}
			if update.OrdID != ordID && update.AccFillSz == 0 {
				continue
			}
			fillPrice = fmt.Sprintf("%.1f", update.AvgPx)
			fillSz = fmt.Sprintf("%.4f", update.AccFillSz)
			log.Printf("FILLED: avgPx=%s accFillSz=%s tradeId=%s", fillPrice, fillSz, update.TradeID)

			if update.AccFillSz == 0 {
				fillPrice, fillSz = fetchFillFromREST(ctx, client, ordID)
			}
			return

		case <-timeout:
			log.Fatal("timeout waiting for fill")
		case <-sigCh:
			log.Println("interrupted, exiting")
			os.Exit(0)
		}
	}
}

func fetchFillFromREST(ctx context.Context, client *okxclient.PrivateClient, ordID string) (string, string) {
	resp, err := client.Rest.GetOrder(ctx, "BTC-USDT-SWAP", ordID)
	if err != nil {
		log.Fatalf("get order: %v", err)
	}
	var data []struct {
		AccFillSz string `json:"accFillSz"`
		AvgPx     string `json:"avgPx"`
		State     string `json:"state"`
	}
	if err := parseData(resp.Data, &data); err != nil {
		log.Fatalf("parse order: %v", err)
	}
	if len(data) > 0 && data[0].State == "filled" {
		log.Printf("REST fill: avgPx=%s accFillSz=%s", data[0].AvgPx, data[0].AccFillSz)
		return data[0].AvgPx, data[0].AccFillSz
	}
	return "0", "0"
}

func placeTPSL(ctx context.Context, client *okxclient.PrivateClient, fillPrice, fillSz string) {
	const tpPct = 0.02
	const slPct = 0.02

	feeResp, err := client.Rest.GetTradeFee(ctx, "SWAP", "BTC-USDT-SWAP")
	if err != nil {
		log.Fatalf("get trade fee: %v", err)
	}
	var feeData []struct {
		Taker string `json:"taker"`
	}
	if err := parseData(feeResp.Data, &feeData); err != nil {
		log.Fatalf("parse trade fee: %v", err)
	}
	takerFee := 0.0005
	if len(feeData) > 0 {
		takerFee = parseFloat(feeData[0].Taker)
	}
	log.Printf("taker fee rate: %.4f%%", takerFee*100)

	entryPx := parseFloat(fillPrice)
	rSL := (1 + takerFee - slPct) / (1 - takerFee)
	rTP := (1 + takerFee + tpPct) / (1 - takerFee)

	slTrigger := fmt.Sprintf("%.1f", entryPx*rSL)
	tpTrigger := fmt.Sprintf("%.1f", entryPx*rTP)
	log.Printf("TP/SL: entry=%.1f netTP=%.1f%% netSL=%.1f%% SL@%s TP@%s",
		entryPx, tpPct*100, slPct*100, slTrigger, tpTrigger)

	algo := &okxclient.AlgoOrderRequest{
		InstID:      "BTC-USDT-SWAP",
		TDMode:      "cross",
		Side:        "sell",
		OrdType:     "conditional",
		Sz:          fillSz,
		SLTriggerPx: slTrigger,
		SLOrdPx:     "-1",
		TPTriggerPx: tpTrigger,
		TPOrdPx:     "-1",
	}
	resp, err := client.Rest.PlaceAlgoOrder(ctx, algo)
	if err != nil {
		log.Fatalf("place algo: %v", err)
	}
	if resp.Code != "0" {
		log.Fatalf("algo order error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	var data []struct {
		AlgoID string `json:"algoId"`
		SCode  string `json:"sCode"`
		SMsg   string `json:"sMsg"`
	}
	if err := parseData(resp.Data, &data); err != nil {
		log.Fatalf("parse algo response: %v", err)
	}
	if len(data) > 0 {
		log.Printf("algo order accepted: algoId=%s sCode=%s", data[0].AlgoID, data[0].SCode)
	}
}

func monitor(ctx context.Context, client *okxclient.PrivateClient, sigCh chan os.Signal, dur time.Duration) {
	log.Printf("monitoring positions and orders for %s...", dur)
	timer := time.After(dur)
	for {
		select {
		case pos := <-client.Positions:
			log.Printf("POSITION: inst=%s side=%s pos=%.4f avgPx=%.1f upl=%.2f uplRatio=%.4f",
				pos.InstID, pos.PosSide, pos.Pos, pos.AvgPx, pos.Upl, pos.UplRatio)
		case order := <-client.Orders:
			log.Printf("ORDER: state=%s ordId=%s side=%s posSide=%s accFillSz=%.0f",
				order.State, order.OrdID, order.Side, order.PosSide, order.AccFillSz)
		case <-timer:
			return
		case <-sigCh:
			log.Println("interrupted")
			return
		}
	}
}
