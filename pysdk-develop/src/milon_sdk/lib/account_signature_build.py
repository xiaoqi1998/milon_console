"""AccountSignatureBuilder. Faithful port of gosdk-develop/lib/accountSignatureBuild.go."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Optional

from ..api import TxHash
from ..crypto import Address, SecretKeyer
from ..types.bitbap import Bitmap64
from .account_signature import (
    AccountSignature,
    AccountSignatureMode,
    IxHashItem,
    MultisigKeySignatureMode,
    PubKeySignatureMode,
    auth_ix_and_payer,
    auth_ixes,
    auth_payer,
    sign,
    simulate_sign,
    unsigned,
)


@dataclass
class AccountSignatureBuilder:
    """Fluent API for building AccountSignature.

    Usage:
        # Path 1: auth → sign (real signing)
        sig = (new_account_signature_builder()
            .authorize_ix_and_payer(0)
            .sign(owner, sk, tx_hash, None, mode)
            .build())

        # Path 2: auth → simulate sign
        sig = (new_account_signature_builder()
            .authorize_ix_and_payer(0)
            .simulate_sign(owner, mode)
            .build())
    """

    acSig: AccountSignature = field(default_factory=lambda: unsigned(Bitmap64(0)))
    errs: List[ValueError] = field(default_factory=list)

    def _or_auth_bit(self, bits: Bitmap64) -> None:
        self.acSig.AuthBit = Bitmap64(self.acSig.AuthBit.raw() | bits.raw())

    def authorize_ixes(self, indices) -> "AccountSignatureBuilder":
        if self.errs:
            return self
        try:
            bits = auth_ixes(indices)
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self._or_auth_bit(bits)
        return self

    def authorize_payer(self) -> "AccountSignatureBuilder":
        if self.errs:
            return self
        self._or_auth_bit(auth_payer())
        return self

    def authorize_ix_and_payer(self, ix: int) -> "AccountSignatureBuilder":
        if self.errs:
            return self
        try:
            bit = auth_ix_and_payer(ix)
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self._or_auth_bit(bit)
        return self

    def sign(
        self,
        account: Address,
        sk: SecretKeyer,
        tx_hash: TxHash,
        ix_hashes: Optional[List[IxHashItem]],
        mode: AccountSignatureMode,
    ) -> "AccountSignatureBuilder":
        if self.errs:
            return self
        try:
            sig = sign(account, sk, self.acSig.AuthBit, tx_hash, ix_hashes or [], mode)
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self.acSig = sig
        return self

    def simulate_sign(
        self, account: Address, mode: AccountSignatureMode
    ) -> "AccountSignatureBuilder":
        if self.errs:
            return self
        try:
            sig = simulate_sign(account, self.acSig.AuthBit, mode)
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self.acSig = sig
        return self

    def sign_multisig_key(
        self,
        account: Address,
        sk: SecretKeyer,
        tx_hash: TxHash,
        ix_hashes: List[IxHashItem],
        mode: AccountSignatureMode,
    ) -> "AccountSignatureBuilder":
        if self.errs:
            return self
        try:
            index, public_key = _resolve_multisig_mode(mode, "SignMultisigKey")
        except ValueError as exc:
            self.errs.append(exc)
            return self

        try:
            msg = self.acSig.auth_message(account, tx_hash, ix_hashes)
        except ValueError as exc:
            self.errs.append(exc)
            return self
        try:
            signature = sk.sign_for(public_key, msg)
        except ValueError as exc:
            self.errs.append(ValueError(f"failed to sign message: {exc}"))
            return self
        try:
            self.acSig.add_multisig_key(index, signature)
        except ValueError as exc:
            self.errs.append(exc)
        return self

    def simulate_sign_multisig_key(
        self, mode: AccountSignatureMode
    ) -> "AccountSignatureBuilder":
        from .account_signature import placeholder_signature

        if self.errs:
            return self
        try:
            index, public_key = _resolve_multisig_mode(
                mode, "SimulateSignMultisigKey"
            )
        except ValueError as exc:
            self.errs.append(exc)
            return self

        try:
            self.acSig.add_multisig_key(index, placeholder_signature(public_key))
        except ValueError as exc:
            self.errs.append(exc)
        return self

    def build(self) -> AccountSignature:
        """Finalizes and returns a copy of the AccountSignature."""
        if self.errs:
            raise self.errs[0]
        return AccountSignature(
            AuthBit=self.acSig.AuthBit,
            SigBit=self.acSig.SigBit,
            Signatures=list(self.acSig.Signatures),
            PubKey=self.acSig.PubKey,
        )


def _resolve_multisig_mode(mode: AccountSignatureMode, caller: str):
    if not isinstance(mode, MultisigKeySignatureMode):
        raise ValueError(f"{caller} requires MultisigKeySignatureMode")
    if mode.Index >= 64:
        raise ValueError(
            f"multisig key index {mode.Index} out of range (max 63)"
        )
    return mode.Index, mode.PublicKey


def new_account_signature_builder() -> AccountSignatureBuilder:
    """Creates a new AccountSignatureBuilder."""
    return AccountSignatureBuilder(acSig=unsigned(Bitmap64(0)))
