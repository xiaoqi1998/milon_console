package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
)

// CheckSimulateSuccess returns an error when the simulate receipt is not
// successful, reporting the on-chain error code and message when available.
func CheckSimulateSuccess(result *milon.SimulateTxResult) error {
	if result == nil {
		return fmt.Errorf("simulate result is nil")
	}
	if result.BodySimulateReceipt.State != api.TxStateSuccess {
		code := uint16(0)
		message := ""
		if result.BodySimulateReceipt.Error != nil {
			code = result.BodySimulateReceipt.Error.Code
			message = result.BodySimulateReceipt.Error.Message
		}
		return fmt.Errorf("simulate failed on chain: error code = %d, message = %s", code, message)
	}
	return nil
}
