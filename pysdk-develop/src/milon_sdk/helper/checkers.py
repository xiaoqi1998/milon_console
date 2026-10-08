"""CheckSimulateSuccess/CheckTxSuccess. Faithful port of gosdk-develop/helper."""

from __future__ import annotations

from ..api import TX_STATE_SUCCESS


def check_simulate_success(result) -> None:
    """Panics (RuntimeError) when the simulate receipt is not successful."""
    if result is None or result.BodySimulateReceipt.State != TX_STATE_SUCCESS:
        code = 0
        message = ""
        if result is not None and result.BodySimulateReceipt.Error is not None:
            code = result.BodySimulateReceipt.Error.Code
            message = result.BodySimulateReceipt.Error.Message
        raise RuntimeError(
            f"simulate failed on chain: error code = {code}, message = {message}"
        )


def check_tx_success(tx_history) -> None:
    """Panics (RuntimeError) when the on-chain receipt is not successful."""
    if tx_history is None or tx_history.Receipt.State != TX_STATE_SUCCESS:
        code = 0
        if tx_history is not None and tx_history.Receipt.Error is not None:
            code = tx_history.Receipt.Error
        raise RuntimeError(f"transaction failed on chain: error code = {code}")
