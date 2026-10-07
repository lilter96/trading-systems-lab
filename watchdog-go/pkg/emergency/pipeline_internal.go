package emergency

import (
	pb "trading-system/contracts/go/commonpb"
	"watchdog-go/pkg/okx"
)

// NewWithClients creates a pipeline from already-built dependencies, bypassing TLS setup.
// Used by tests and for advanced injection scenarios.
func NewWithClients(
	grpcClients []pb.EmergencyServiceClient,
	okxClient *okx.Client,
	telegramToken, telegramChatId string,
) *ExecutionPipeline {
	return &ExecutionPipeline{
		grpcClients:    grpcClients,
		okxClient:      okxClient,
		telegramToken:  telegramToken,
		telegramChatId: telegramChatId,
	}
}
