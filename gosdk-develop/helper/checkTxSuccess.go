package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
)

// CheckTxSuccess returns an error when the on-chain receipt is not successful,
// reporting the on-chain error code when available.
func CheckTxSuccess(txHistory *api.TxHistory) error {
	if txHistory == nil {
		return fmt.Errorf("transaction history is nil")
	}
	if txHistory.Receipt.State != api.TxStateSuccess {
		code := uint16(0)
		if txHistory.Receipt.Error != nil {
			code = *txHistory.Receipt.Error
		}
		return fmt.Errorf("transaction failed on chain: error code = %d", code)
	}
	return nil
}
