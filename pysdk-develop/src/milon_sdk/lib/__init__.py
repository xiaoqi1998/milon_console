"""milon_sdk.lib — Faithful port of gosdk-develop/lib."""

from .rpc_request import (
    CONTENT_TYPE_MILON_JSON,
    CONTENT_TYPE_MILON_POSTCARD,
    METHOD_TYPE_BATCH_GET_RESOURCE_PATH_BY_HASH,
    METHOD_TYPE_CHAIN_HEAD,
    METHOD_TYPE_EVENTS_BY_TX_HASH,
    METHOD_TYPE_GET_ACCESS_VALUE,
    METHOD_TYPE_GET_ACCOUNT,
    METHOD_TYPE_GET_BLOCK_BY_HEIGHT,
    METHOD_TYPE_GET_RESOURCE,
    METHOD_TYPE_GET_RESOURCE_PATH_BY_HASH,
    METHOD_TYPE_GET_TX_BY_HASH,
    METHOD_TYPE_GET_TX_HISTORY_PROOF,
    METHOD_TYPE_SIMULATE_TX,
    METHOD_TYPE_SUBMIT_TX,
    METHOD_TYPE_VIEW,
    MethodType,
    RpcRequest,
    new_rpc_request,
)
from .rpc_response import (
    RPC_RESPONSE_STATUS_DISABLED,
    RPC_RESPONSE_STATUS_FAILED,
    RPC_RESPONSE_STATUS_INTERNAL,
    RPC_RESPONSE_STATUS_INVALID,
    RPC_RESPONSE_STATUS_NOT_FOUND,
    RPC_RESPONSE_STATUS_OK,
    RPC_RESPONSE_STATUS_UNAVAILABLE,
    RpcResponse,
    RpcResponseError,
)
from .transaction import Transaction, TransactionSignatures
from .chain_id import GetChainId, get_chain_id, set_chain_id

# Go 类型别名（值类型；Python 侧以 int 表达）
from typing import Any, Callable, Dict, List, Optional, Union

RequestID = int
TransactionStamp = int
RpcResponseStatus = int
RpcResponseStatusOk = 0
RpcResponseStatusInvalid = 1
RpcResponseStatusNotFound = 2
RpcResponseStatusDisabled = 3
RpcResponseStatusUnavailable = 4
RpcResponseStatusInternal = 5
RpcResponseStatusFailed = 6
from .account_signature import (
    AUTH_PAYER_BIT,
    AUTH_RESERVED_BIT,
    AUTH_VOTE_BIT,
    AccountSignature,
    AccountSignatureMode,
    IxHashItem,
    MultisigKeySignatureMode,
    PubKeySignatureMode,
    auth_ix,
    auth_ix_and_payer,
    auth_ixes,
    auth_payer,
    auth_vote_ixes,
    collect_ix_hashes,
    sign,
    simulate_sign,
    unsigned,
    vote_batch_hash,
)
from .account_signature_build import (
    AccountSignatureBuilder,
    new_account_signature_builder,
)
from .transaction_builder import (
    Signer,
    SigningSlot,
    TransactionBuilder,
    new_transaction_builder,
)

# 模块级别名（Go 常量名）
AuthPayerBit = AUTH_PAYER_BIT
AuthReservedBit = AUTH_RESERVED_BIT
AuthVoteBit = AUTH_VOTE_BIT

__all__ = [n for n in dir() if not n.startswith("_")]
