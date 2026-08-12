package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
)

// CheckTxSuccess panics when the on-chain receipt is not successful,
// printing the on-chain error code when available.
func CheckTxSuccess(txHistory *api.TxHistory) {
	if txHistory == nil || txHistory.Receipt.State != api.TxStateSuccess {
		code := uint16(0)
		if txHistory != nil && txHistory.Receipt.Error != nil {
			code = *txHistory.Receipt.Error
		}
		panic(fmt.Sprintf("Transaction failed on chain: error code = %d", code))
	}
}
