package main

type swapInstrument struct {
	InstID   string `json:"instId"`
	CtVal    string `json:"ctVal"`
	MinSz    string `json:"minSz"`
	LotSz    string `json:"lotSz"`
	TickSz   string `json:"tickSz"`
	CtMult   string `json:"ctMult"`
	CtValCcy string `json:"ctValCcy"`
}

type balanceDetail struct {
	Eq       string `json:"eq"`
	AvailBal string `json:"availBal"`
}

type balanceData struct {
	Details []balanceDetail `json:"details"`
}
