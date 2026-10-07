module intelligence-node/osint-go

go 1.26.4

require (
	github.com/lib/pq v1.10.9
	github.com/mmcdole/gofeed v1.4.0
	github.com/shopspring/decimal v1.4.0
	google.golang.org/grpc v1.82.1
	trading-system/contracts/go v0.0.0-00010101000000-000000000000
)

require (
	github.com/mmcdole/goxpp/v2 v2.0.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace trading-system/contracts/go => ../../contracts/go
