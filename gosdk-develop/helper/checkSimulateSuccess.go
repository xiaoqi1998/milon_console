package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
)

// CheckSimulateSuccess panics when the simulate receipt is not successful,
// printing the on-chain error code and message when available.
func CheckSimulateSuccess(result *milon.SimulateTxResult) {
	if result == nil || result.BodySimulateReceipt.State != api.TxStateSuccess {
		code := uint16(0)
		message := ""
		if result != nil && result.BodySimulateReceipt.Error != nil {
			code = result.BodySimulateReceipt.Error.Code
			message = result.BodySimulateReceipt.Error.Message
		}
		panic(fmt.Sprintf("Simulate failed on chain: error code = %d, message = %s", code, message))
	}
}
