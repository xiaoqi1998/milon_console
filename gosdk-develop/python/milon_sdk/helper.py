"""高层辅助函数（对应 Go helper/*.go）。"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .api.responses import (
    TX_STATE_FAILED,
    TX_STATE_PENDING,
    TX_STATE_SUCCESS,
)
from .rpc import GetTxByHashResult, SimulateTxResult

if TYPE_CHECKING:  # pragma: no cover
    from .client import Client


def check_tx_success(result: GetTxByHashResult) -> GetTxByHashResult:
    """校验交易最终状态为 Success，否则抛异常。"""
    state = result.body_tx_history.receipt.state
    if state != TX_STATE_SUCCESS:
        err_code = result.body_tx_history.receipt.error
        raise RuntimeError(
            f"transaction not successful: state={state} "
            f"(0=pending, 1=success, 2=failed), error_code={err_code}"
        )
    return result


def check_simulate_success(result: SimulateTxResult) -> SimulateTxResult:
    """校验模拟交易状态为 Success，失败时附带 TxFailurePayload 信息。"""
    state = result.body_simulate_receipt.state
    if state != TX_STATE_SUCCESS:
        failure = result.body_simulate_receipt.error
        detail = ""
        if failure is not None:
            detail = f" code={failure.code} message={failure.message!r}"
        raise RuntimeError(
            f"simulation not successful: state={state}{detail}"
        )
    return result


def get_account(client: "Client", account_relaxed: Any):
    return client.get_account(account_relaxed).body_account_view


def list_signers(client: "Client", account_relaxed: Any) -> list:
    """列出账户签名者（返回 [account_map, signers_list]）。"""
    return client.list_account_signers(account_relaxed)


def events_by_tx_hash(client: "Client", tx_hash_relaxed: Any, type_tag_filter=None) -> list:
    return client.events_by_tx_hash(tx_hash_relaxed, type_tag_filter).body_events_by_tx_hash.events
