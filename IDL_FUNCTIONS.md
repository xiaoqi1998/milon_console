# Milon IDL 函数清单

> 导出自 `gosdk-develop/provider/IDL/`，共 **9** 个 app、**164** 个函数。

> 表格列：`appname`(应用名) · `meth`(方法名/指令名) · `handler`(链上入口名) · `id`(discriminator 指令编号) · `说明`(中文释义，逻辑移植自 handler/idl_handler.go 的 describeInstruction)


## View（只读查询，kind=view）

共 **75** 个。

| appname | meth | handler | id | 说明 |
|---------|------|---------|-----|------|
| account | GetAccount | get_account | 11094 | （只读查询）查询账户相关信息（GetAccount）。 |
| account | ListSigners | list_signers | 24766 | （只读查询）查询账户相关信息（ListSigners）。 |
| account | ResolveSigners | resolve_signers | 27426 | （只读查询）查询账户的 ResolveSigners 数据。 |
| account | GetVote | get_vote | 52139 | （只读查询）查询账户相关信息（GetVote）。 |
| account | ListActiveVotes | list_active_votes | 46107 | （只读查询）查询账户相关信息（ListActiveVotes）。 |
| token | BalanceOf | balance_of | 59285 | （只读查询）查询代币相关信息（BalanceOf）。 |
| token | FrozenOf | frozen_of | 64893 | （只读查询）查询代币的 FrozenOf 数据。 |
| token | ApprovalOf | approval_of | 43638 | （只读查询）查询代币的 ApprovalOf 数据。 |
| token | TotalSupply | total_supply | 35429 | （只读查询）查询代币相关信息（TotalSupply）。 |
| token | Metadata | metadata | 7880 | （只读查询）查询代币的 Metadata 数据。 |
| token | Compliance | compliance | 43186 | （只读查询）查询代币的 Compliance 数据。 |
| token | FaucetCooldownRemaining | faucet_cooldown_remaining | 29892 | （只读查询）查询代币的 FaucetCooldownRemaining 数据。 |
| staking | ValidatorProfile | validator_profile | 43284 | （只读查询）查询质押的 ValidatorProfile 数据。 |
| staking | ValidatorPool | validator_pool | 51337 | （只读查询）查询质押的 ValidatorPool 数据。 |
| staking | StakePosition | stake_position | 24660 | （只读查询）查询质押的 StakePosition 数据。 |
| staking | PositionSummary | position_summary | 24212 | （只读查询）查询质押的 PositionSummary 数据。 |
| staking | CandidatePool | candidate_pool | 57732 | （只读查询）查询质押的 CandidatePool 数据。 |
| staking | ActiveSetSnapshot | active_set_snapshot | 54522 | （只读查询）查询质押的 ActiveSetSnapshot 数据。 |
| staking | ActiveSetHash | active_set_hash | 41226 | （只读查询）查询质押的 ActiveSetHash 数据。 |
| staking | CurrentActiveSetSnapshot | current_active_set_snapshot | 44040 | （只读查询）查询质押的 CurrentActiveSetSnapshot 数据。 |
| staking | CurrentActiveSetHash | current_active_set_hash | 58924 | （只读查询）查询质押的 CurrentActiveSetHash 数据。 |
| staking | EpochTransition | epoch_transition | 24093 | （只读查询）查询质押的 EpochTransition 数据。 |
| staking | EpochConfig | epoch_config | 41632 | （只读查询）查询质押的 EpochConfig 数据。 |
| staking | EpochState | epoch_state | 29979 | （只读查询）查询质押的 EpochState 数据。 |
| staking | RewardTreasury | reward_treasury | 46445 | （只读查询）查询质押的 RewardTreasury 数据。 |
| staking | HeldPrincipal | held_principal | 45674 | （只读查询）查询质押的 HeldPrincipal 数据。 |
| staking | EpochTransitionAttempt | epoch_transition_attempt | 13587 | （只读查询）查询质押的 EpochTransitionAttempt 数据。 |
| staking | ConsensusActiveSet | consensus_active_set | 54057 | （只读查询）查询质押的 ConsensusActiveSet 数据。 |
| staking | CurrentConsensusActiveSet | current_consensus_active_set | 45423 | （只读查询）查询质押的 CurrentConsensusActiveSet 数据。 |
| staking | ConsensusActiveValidator | consensus_active_validator | 41871 | （只读查询）查询质押的 ConsensusActiveValidator 数据。 |
| staking | ConsensusActiveValidatorIndex | consensus_active_validator_index | 1580 | （只读查询）查询质押的 ConsensusActiveValidatorIndex 数据。 |
| staking | ConsensusActiveValidatorIndexByPubkey | consensus_active_validator_index_by_pubkey | 45763 | （只读查询）查询质押的 ConsensusActiveValidatorIndexByPubkey 数据。 |
| staking | ConsensusActiveValidatorsByBitmap | consensus_active_validators_by_bitmap | 60094 | （只读查询）查询质押的 ConsensusActiveValidatorsByBitmap 数据。 |
| identity | VcAttestationCore | vc_attestation_core | 18681 | （只读查询）查询身份（DID）的 VcAttestationCore 数据。 |
| identity | VcAttestationLifecycle | vc_attestation_lifecycle | 41850 | （只读查询）查询身份（DID）的 VcAttestationLifecycle 数据。 |
| identity | AcceptedVcIssuerIndexMeta | accepted_vc_issuer_index_meta | 57307 | （只读查询）查询身份（DID）的 AcceptedVcIssuerIndexMeta 数据。 |
| identity | AcceptedVcIssuers | accepted_vc_issuers | 6419 | （只读查询）查询身份（DID）的 AcceptedVcIssuers 数据。 |
| identity | HasValidVcFromIssuer | has_valid_vc_from_issuer | 18655 | （只读查询）查询身份（DID）的 HasValidVcFromIssuer 数据。 |
| identity | Core | core | 39696 | （只读查询）查询身份（DID）的 Core 数据。 |
| identity | Document | document | 21854 | （只读查询）查询身份（DID）的 Document 数据。 |
| identity | KeyIndex | key_index | 17805 | （只读查询）查询身份（DID）的 KeyIndex 数据。 |
| identity | Keys | keys | 7171 | （只读查询）查询身份（DID）的 Keys 数据。 |
| identity | Key | key | 49570 | （只读查询）查询身份（DID）的 Key 数据。 |
| identity | ServiceIndex | service_index | 25961 | （只读查询）查询身份（DID）的 ServiceIndex 数据。 |
| identity | Services | services | 567 | （只读查询）查询身份（DID）的 Services 数据。 |
| identity | Service | service | 62270 | （只读查询）查询身份（DID）的 Service 数据。 |
| identity | Alias | alias | 8101 | （只读查询）查询身份（DID）的 Alias 数据。 |
| identity | Avatar | avatar | 2498 | （只读查询）查询身份（DID）的 Avatar 数据。 |
| identity | UpdatedAt | updated_at | 46666 | （只读查询）更新身份（DID）相关信息（UpdatedAt）。 |
| identity | Deactivated | deactivated | 12471 | （只读查询）查询身份（DID）的 Deactivated 数据。 |
| identity | NameBinding | name_binding | 60506 | （只读查询）查询身份（DID）的 NameBinding 数据。 |
| identity | CredentialDefinition | credential_definition | 42682 | （只读查询）查询身份（DID）的 CredentialDefinition 数据。 |
| identity | OrganizationCapabilities | organization_capabilities | 6251 | （只读查询）查询身份（DID）的 OrganizationCapabilities 数据。 |
| identity | OrganizationStatus | organization_status | 39483 | （只读查询）查询身份（DID）的 OrganizationStatus 数据。 |
| identity | OrganizationUpdatedAt | organization_updated_at | 23820 | （只读查询）查询身份（DID）的 OrganizationUpdatedAt 数据。 |
| nft | CollectionMetadata | collection_metadata | 10326 | （只读查询）查询NFT的 CollectionMetadata 数据。 |
| nft | MetadataUri | metadata_uri | 26166 | （只读查询）查询NFT的 MetadataUri 数据。 |
| nft | Attributes | attributes | 3323 | （只读查询）查询NFT的 Attributes 数据。 |
| nft | Properties | properties | 19051 | （只读查询）查询NFT的 Properties 数据。 |
| nft | MintConfigView | mint_config_view | 6407 | （只读查询）铸造/增发NFT相关信息（MintConfigView）。 |
| nft | TotalSupply | total_supply | 35462 | （只读查询）查询NFT相关信息（TotalSupply）。 |
| nft | BalanceOf | balance_of | 63018 | （只读查询）查询NFT相关信息（BalanceOf）。 |
| nft | RoyaltyInfo | royalty_info | 30311 | （只读查询）查询NFT的 RoyaltyInfo 数据。 |
| randomness | LatestBeacon | latest_beacon | 49324 | （只读查询）查询随机数（VRF）信标的 LatestBeacon 数据。 |
| randomness | Beacon | beacon | 14904 | （只读查询）查询随机数（VRF）信标的 Beacon 数据。 |
| randomness_demo | Reveal | reveal | 17191 | （只读查询）查询随机数示例的 Reveal 数据。 |
| demo | OrderBalance | order_balance | 24910 | （只读查询）查询示例/demo的 OrderBalance 数据。 |
| demo | SponsorPoolOf | sponsor_pool_of | 35462 | （只读查询）查询示例/demo的 SponsorPoolOf 数据。 |
| demo | LabelOf | label_of | 10409 | （只读查询）查询示例/demo的 LabelOf 数据。 |
| demo | ScoreOf | score_of | 54597 | （只读查询）查询示例/demo的 ScoreOf 数据。 |
| demo | TierCapOf | tier_cap_of | 48456 | （只读查询）查询示例/demo的 TierCapOf 数据。 |
| demo | EchoMode | echo_mode | 47924 | （只读查询）回显示例/demo相关信息（EchoMode）。 |
| demo | LabelTotal | label_total | 4770 | （只读查询）查询示例/demo的 LabelTotal 数据。 |
| demo | SpecialTypes | special_types | 43240 | （只读查询）查询示例/demo的 SpecialTypes 数据。 |
| demo | Reveal | reveal | 21874 | （只读查询）查询示例/demo的 Reveal 数据。 |

## Entry（写操作，kind=entry）

共 **89** 个。

| appname | meth | handler | id | 说明 |
|---------|------|---------|-----|------|
| system | Noop | noop | 13507 | 调用 系统 的 Noop 方法（handler: noop）。 |
| system | PublishLocalBlockBeacon | publish_local_block_beacon | 9713 | 调用 系统 的 PublishLocalBlockBeacon 方法（handler: publish_local_block_beacon）。 |
| system | PrepareStakingEpoch | prepare_staking_epoch | 42334 | 调用 系统 的 PrepareStakingEpoch 方法（handler: prepare_staking_epoch）。 |
| system | AdvanceStakingEpoch | advance_staking_epoch | 10079 | 调用 系统 的 AdvanceStakingEpoch 方法（handler: advance_staking_epoch）。 |
| account | Create | create | 2182 | 创建账户（handler: create）。 |
| account | EnsureAccount | ensure_account | 38184 | 调用 账户 的 EnsureAccount 方法（handler: ensure_account）。 |
| account | CreateMultisig | create_multisig | 20289 | 创建账户（handler: create_multisig）。 |
| account | AddSigner | add_signer | 41092 | 添加账户（handler: add_signer）。 |
| account | AddSigners | add_signers | 25813 | 添加账户（handler: add_signers）。 |
| account | RemoveSigner | remove_signer | 61953 | 移除账户（handler: remove_signer）。 |
| account | SetThreshold | set_threshold | 2386 | 设置账户（handler: set_threshold）。 |
| account | SetSignerWeight | set_signer_weight | 43270 | 设置账户（handler: set_signer_weight）。 |
| account | VoteInit | vote_init | 52917 | 投票账户（handler: vote_init）。 |
| account | Vote | vote | 24406 | 投票账户（handler: vote）。 |
| token | Create | create | 2581 | 创建代币（handler: create）。 |
| token | AbandonOwner | abandon_owner | 64710 | 调用 代币 的 AbandonOwner 方法（handler: abandon_owner）。 |
| token | TransferOwner | transfer_owner | 18518 | 转账代币（handler: transfer_owner）。 |
| token | AbandonFreezer | abandon_freezer | 1778 | 调用 代币 的 AbandonFreezer 方法（handler: abandon_freezer）。 |
| token | TransferFreezer | transfer_freezer | 27042 | 转账代币（handler: transfer_freezer）。 |
| token | Mint | mint | 20481 | 铸造/增发代币（handler: mint）。 |
| token | MintBatch | mint_batch | 7494 | 铸造/增发代币（handler: mint_batch）。 |
| token | Burn | burn | 38784 | 销毁代币（handler: burn）。 |
| token | Transfer | transfer | 19694 | 转账代币（handler: transfer）。 |
| token | TransferWithTag | transfer_with_tag | 25012 | 转账代币（handler: transfer_with_tag）。 |
| token | TransferBatch | transfer_batch | 28053 | 转账代币（handler: transfer_batch）。 |
| token | Freeze | freeze | 31050 | 冻结代币（handler: freeze）。 |
| token | Unfreeze | unfreeze | 17977 | 解冻代币（handler: unfreeze）。 |
| token | Approve | approve | 9714 | 授权代币（handler: approve）。 |
| token | Revoke | revoke | 8619 | 撤销授权代币（handler: revoke）。 |
| token | TransferFrom | transfer_from | 18655 | 转账代币（handler: transfer_from）。 |
| token | SetIcon | set_icon | 40941 | 设置代币（handler: set_icon）。 |
| token | CreateWithCompliance | create_with_compliance | 62196 | 创建代币（handler: create_with_compliance）。 |
| token | SetComplianceMode | set_compliance_mode | 33931 | 设置代币（handler: set_compliance_mode）。 |
| token | AddComplianceRequirement | add_compliance_requirement | 59302 | 添加代币（handler: add_compliance_requirement）。 |
| token | RemoveComplianceRequirement | remove_compliance_requirement | 14909 | 移除代币（handler: remove_compliance_requirement）。 |
| token | ClearComplianceRequirements | clear_compliance_requirements | 50437 | 调用 代币 的 ClearComplianceRequirements 方法（handler: clear_compliance_requirements）。 |
| token | ClaimFaucet | claim_faucet | 63796 | 领取代币（handler: claim_faucet）。 |
| staking | CreateValidator | create_validator | 16533 | 创建质押（handler: create_validator）。 |
| staking | JoinCandidatePool | join_candidate_pool | 36277 | 调用 质押 的 JoinCandidatePool 方法（handler: join_candidate_pool）。 |
| staking | LeaveCandidatePool | leave_candidate_pool | 62212 | 调用 质押 的 LeaveCandidatePool 方法（handler: leave_candidate_pool）。 |
| staking | FundRewardTreasury | fund_reward_treasury | 26413 | 调用 质押 的 FundRewardTreasury 方法（handler: fund_reward_treasury）。 |
| staking | Stake | stake | 7586 | 调用 质押 的 Stake 方法（handler: stake）。 |
| staking | CancelPendingStake | cancel_pending_stake | 26203 | 调用 质押 的 CancelPendingStake 方法（handler: cancel_pending_stake）。 |
| staking | ClaimRewards | claim_rewards | 49863 | 领取质押（handler: claim_rewards）。 |
| staking | ClaimOperatorRewards | claim_operator_rewards | 13178 | 领取质押（handler: claim_operator_rewards）。 |
| staking | RequestUnstake | request_unstake | 34861 | 调用 质押 的 RequestUnstake 方法（handler: request_unstake）。 |
| identity | DiscloseVcAttestation | disclose_vc_attestation | 23078 | 披露身份（DID）（handler: disclose_vc_attestation）。 |
| identity | RemoveVcDisclosure | remove_vc_disclosure | 51077 | 移除身份（DID）（handler: remove_vc_disclosure）。 |
| identity | RevokeVcAttestation | revoke_vc_attestation | 38692 | 撤销授权身份（DID）（handler: revoke_vc_attestation）。 |
| identity | Create | create | 28587 | 创建身份（DID）（handler: create）。 |
| identity | CreateWithAlias | create_with_alias | 48723 | 创建身份（DID）（handler: create_with_alias）。 |
| identity | AddKey | add_key | 24314 | 添加身份（DID）（handler: add_key）。 |
| identity | UpdateKey | update_key | 38726 | 更新身份（DID）（handler: update_key）。 |
| identity | RemoveKey | remove_key | 14295 | 移除身份（DID）（handler: remove_key）。 |
| identity | AddService | add_service | 45574 | 添加身份（DID）（handler: add_service）。 |
| identity | UpdateService | update_service | 18018 | 更新身份（DID）（handler: update_service）。 |
| identity | RemoveService | remove_service | 38683 | 移除身份（DID）（handler: remove_service）。 |
| identity | SetAvatarUri | set_avatar_uri | 7646 | 设置身份（DID）（handler: set_avatar_uri）。 |
| identity | Deactivate | deactivate | 6825 | 调用 身份（DID） 的 Deactivate 方法（handler: deactivate）。 |
| identity | SetAlias | set_alias | 44782 | 设置身份（DID）（handler: set_alias）。 |
| identity | RegisterOrganization | register_organization | 36868 | 注册身份（DID）（handler: register_organization）。 |
| identity | UpdateOrganizationCapabilities | update_organization_capabilities | 13927 | 更新身份（DID）（handler: update_organization_capabilities）。 |
| identity | DeactivateOrganization | deactivate_organization | 48597 | 调用 身份（DID） 的 DeactivateOrganization 方法（handler: deactivate_organization）。 |
| nft | CreateCollection | create_collection | 20543 | 创建NFT（handler: create_collection）。 |
| nft | SetCollectionMetadata | set_collection_metadata | 58161 | 设置NFT（handler: set_collection_metadata）。 |
| nft | SetMetadata | set_metadata | 61094 | 设置NFT（handler: set_metadata）。 |
| nft | SetAttributes | set_attributes | 47350 | 设置NFT（handler: set_attributes）。 |
| nft | SetProperties | set_properties | 48498 | 设置NFT（handler: set_properties）。 |
| nft | CreateUnique | create_unique | 33212 | 创建NFT（handler: create_unique）。 |
| nft | CreateBatch | create_batch | 43953 | 创建NFT（handler: create_batch）。 |
| nft | MintBatch | mint_batch | 51281 | 铸造/增发NFT（handler: mint_batch）。 |
| nft | Transfer | transfer | 48437 | 转账NFT（handler: transfer）。 |
| nft | Burn | burn | 7015 | 销毁NFT（handler: burn）。 |
| nft | SetRoyalty | set_royalty | 53745 | 设置NFT（handler: set_royalty）。 |
| nft | TransferRoyaltyRecipient | transfer_royalty_recipient | 56414 | 转账NFT（handler: transfer_royalty_recipient）。 |
| randomness_demo | RequestReveal | request_reveal | 25685 | 调用 随机数示例 的 RequestReveal 方法（handler: request_reveal）。 |
| randomness_demo | SettleReveal | settle_reveal | 37019 | 设置随机数示例（handler: settle_reveal）。 |
| demo | OpenOrder | open_order | 57564 | 开启/创建示例/demo（handler: open_order）。 |
| demo | PayOrder | pay_order | 58464 | 调用 示例/demo 的 PayOrder 方法（handler: pay_order）。 |
| demo | SettleOrder | settle_order | 1751 | 设置示例/demo（handler: settle_order）。 |
| demo | OpenGasSponsorPool | open_gas_sponsor_pool | 6639 | 开启/创建示例/demo（handler: open_gas_sponsor_pool）。 |
| demo | ClaimSponsoredScore | claim_sponsored_score | 11876 | 领取示例/demo（handler: claim_sponsored_score）。 |
| demo | InitPool | init_pool | 27056 | 初始化示例/demo（handler: init_pool）。 |
| demo | InitDex | init_dex | 44919 | 初始化示例/demo（handler: init_dex）。 |
| demo | SetLabel | set_label | 58132 | 设置示例/demo（handler: set_label）。 |
| demo | BatchCredit | batch_credit | 8311 | 批量处理示例/demo（handler: batch_credit）。 |
| demo | SetTierCap | set_tier_cap | 58369 | 设置示例/demo（handler: set_tier_cap）。 |
| demo | RequestReveal | request_reveal | 59064 | 调用 示例/demo 的 RequestReveal 方法（handler: request_reveal）。 |
| demo | SettleReveal | settle_reveal | 25360 | 设置示例/demo（handler: settle_reveal）。 |

## 各 App 统计

| appname | app_id | view | entry | 合计 |
|---------|--------|------|-------|------|
| system | 0 | 0 | 4 | 4 |
| account | 1 | 5 | 10 | 15 |
| token | 2 | 7 | 23 | 30 |
| staking | 3 | 21 | 9 | 30 |
| identity | 4 | 22 | 17 | 39 |
| nft | 5 | 8 | 12 | 20 |
| randomness | 7 | 2 | 0 | 2 |
| randomness_demo | 254 | 1 | 2 | 3 |
| demo | 255 | 9 | 12 | 21 |
