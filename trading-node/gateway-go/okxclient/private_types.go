package okxclient

type OrderUpdate struct {
	InstID    string
	OrdID     string
	ClOrdID   string
	State     string
	Side      string
	PosSide   string
	FillPx    float64
	FillSz    float64
	AvgPx     float64
	AccFillSz float64
	Fee       float64
	FeeCcy    string
	TradeID   string
	Px        float64
	Sz        float64
	OrdType   string
	Category  string
	UTime     int64
	CTime     int64
}

type PositionUpdate struct {
	InstID    string
	PosSide   string
	Pos       float64
	AvgPx     float64
	Upl       float64
	UplRatio  float64
	Lever     string
	MgnMode   string
	Ccy       string
	PosID     string
	TradeID   string
	InstType  string
	UTime     int64
}

type PrivateSub struct {
	Channel  string
	InstType string
	InstID   string
}
