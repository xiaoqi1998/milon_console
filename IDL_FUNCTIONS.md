# Milon IDL 函数清单

> 导出自 `gosdk-develop/provider/IDL/`，共 **11** 个 app、**209** 个函数（view 85 + entry 124）。

> 表格列：`appname`(应用名) · `meth`(方法名/指令名) · `handler`(链上入口名) · `id`(discriminator 指令编号) · `说明`(中文释义，逻辑移植自 handler/idl_handler.go 的 describeInstruction)


## View（只读查询，kind=view）

共 **85** 个。

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
| staking | EpochTransition | epoch_transition | 24093 | （只读查询）查询质押的 EpochTransition 数据。 |
| staking | EpochConfig | epoch_config | 41632 | （只读查询）查询质押的 EpochConfig 数据。 |
| staking | EpochState | epoch_state | 29979 | （只读查询）查询质押的 EpochState 数据。 |
| staking | RewardTreasury | reward_treasury | 46445 | （只读查询）查询质押的 RewardTreasury 数据。 |
| staking | HeldPrincipal | held_principal | 45674 | （只读查询）查询质押的 HeldPrincipal 数据。 |
| staking | ListDeclaredValidatorsForEpoch | list_declared_validators_for_epoch | 13153 | （只读查询）查询质押相关信息（ListDeclaredValidatorsForEpoch）。 |
| identity | VcAttestationCore | vc_attestation_core | 18681 | （只读查询）查询身份（DID）的 VcAttestationCore 数据。 |
| identity | VcAttestationLifecycle | vc_attestation_lifecycle | 41850 | （只读查询）查询身份（DID）的 VcAttestationLifecycle 数据。 |
| identity | AcceptedVcIssuerIndexMeta | accepted_vc_issuer_index_meta | 57307 | （只读查询）查询身份（DID）的 AcceptedVcIssuerIndexMeta 数据。 |
| identity | AcceptedVcIssuers | accepted_vc_issuers | 6419 | （只读查询）查询身份（DID）的 AcceptedVcIssuers 数据。 |
| identity | DisclosedVcSchemas | disclosed_vc_schemas | 48906 | （只读查询）披露身份（DID）相关信息（DisclosedVcSchemas）。 |
| identity | DisclosedVcs | disclosed_vcs | 27262 | （只读查询）披露身份（DID）相关信息（DisclosedVcs）。 |
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
| nft | CollectionUpdateAuthor | collection_update_author | 23904 | （只读查询）查询NFT的 CollectionUpdateAuthor 数据。 |
| nft | CollectionFreezeAuthor | collection_freeze_author | 53202 | （只读查询）查询NFT的 CollectionFreezeAuthor 数据。 |
| nft | CollectionTransferable | collection_transferable | 49000 | （只读查询）查询NFT的 CollectionTransferable 数据。 |
| nft | MintCoverUri | mint_cover_uri | 13043 | （只读查询）铸造/增发NFT相关信息（MintCoverUri）。 |
| nft | Attributes | attributes | 3323 | （只读查询）查询NFT的 Attributes 数据。 |
| nft | Properties | properties | 19051 | （只读查询）查询NFT的 Properties 数据。 |
| nft | IsFrozen | is_frozen | 46041 | （只读查询）查询NFT的 IsFrozen 数据。 |
| nft | MintDefinition | mint_definition | 62578 | （只读查询）铸造/增发NFT相关信息（MintDefinition）。 |
| nft | TotalSupply | total_supply | 35462 | （只读查询）查询NFT相关信息（TotalSupply）。 |
| nft | BalanceOf | balance_of | 63018 | （只读查询）查询NFT相关信息（BalanceOf）。 |
| nft | CurrentMintSeed | current_mint_seed | 50676 | （只读查询）查询NFT的 CurrentMintSeed 数据。 |
| nft | RoyaltyInfo | royalty_info | 30311 | （只读查询）查询NFT的 RoyaltyInfo 数据。 |
| dex | MarketInfo | market_info | 2326 | （只读查询）查询DEX的 MarketInfo 数据。 |
| dex | OrderInfoView | order_info_view | 23456 | （只读查询）查询DEX的 OrderInfoView 数据。 |
| dex | BestBidAsk | best_bid_ask | 12137 | （只读查询）查询DEX的 BestBidAsk 数据。 |
| dex | OrderbookDepth | orderbook_depth | 46884 | （只读查询）查询DEX的 OrderbookDepth 数据。 |
| dex | OrdersAtLevel | orders_at_level | 24461 | （只读查询）查询DEX的 OrdersAtLevel 数据。 |
| dex | VaultLiability | vault_liability | 6759 | （只读查询）查询DEX的 VaultLiability 数据。 |
| randomness | LatestBeacon | latest_beacon | 49324 | （只读查询）查询随机数（VRF）信标的 LatestBeacon 数据。 |
| randomness | Beacon | beacon | 14904 | （只读查询）查询随机数（VRF）信标的 Beacon 数据。 |
| keyless | GetSessions | get_sessions | 58087 | （只读查询）查询无密钥相关信息（GetSessions）。 |
| keyless | GetBinding | get_binding | 4855 | （只读查询）查询无密钥相关信息（GetBinding）。 |
| keyless | ListProviders | list_providers | 12570 | （只读查询）查询无密钥相关信息（ListProviders）。 |
| keyless | GetProvider | get_provider | 20071 | （只读查询）查询无密钥相关信息（GetProvider）。 |
| keyless | GetParams | get_params | 12312 | （只读查询）查询无密钥相关信息（GetParams）。 |
| keyless | IsKeylessAccount | is_keyless_account | 33494 | （只读查询）查询无密钥的 IsKeylessAccount 数据。 |
| lucky_box | BoxView | box_view | 58937 | （只读查询）查询幸运盒的 BoxView 数据。 |
| lucky_box | AssetPool | asset_pool | 45729 | （只读查询）查询幸运盒的 AssetPool 数据。 |
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

共 **124** 个。

| appname | meth | handler | id | 说明 |
|---------|------|---------|-----|------|
| system | Noop | noop | 13507 | 调用 系统 的 Noop 方法（handler: noop）。 |
| system | RegisterValidatorIdentity | register_validator_identity | 16918 | 注册系统（handler: register_validator_identity）。 |
| system | PrepareValidatorSet | prepare_validator_set | 55212 | 调用 系统 的 PrepareValidatorSet 方法（handler: prepare_validator_set）。 |
| system | SettleStakingEpoch | settle_staking_epoch | 61700 | 设置系统（handler: settle_staking_epoch）。 |
| system | CommitValidatorSet | commit_validator_set | 59552 | 调用 系统 的 CommitValidatorSet 方法（handler: commit_validator_set）。 |
| system | ConfigureStakingToken | configure_staking_token | 31505 | 调用 系统 的 ConfigureStakingToken 方法（handler: configure_staking_token）。 |
| system | InitializeStakingEpochConfig | initialize_staking_epoch_config | 59536 | 初始化系统（handler: initialize_staking_epoch_config）。 |
| system | FundStakingRewardTreasury | fund_staking_reward_treasury | 32864 | 调用 系统 的 FundStakingRewardTreasury 方法（handler: fund_staking_reward_treasury）。 |
| system | BootstrapStakingValidator | bootstrap_staking_validator | 43888 | 调用 系统 的 BootstrapStakingValidator 方法（handler: bootstrap_staking_validator）。 |
| system | BootstrapStakingEpochZero | bootstrap_staking_epoch_zero | 15870 | 调用 系统 的 BootstrapStakingEpochZero 方法（handler: bootstrap_staking_epoch_zero）。 |
| system | PublishLocalBlockBeacon | publish_local_block_beacon | 9713 | 调用 系统 的 PublishLocalBlockBeacon 方法（handler: publish_local_block_beacon）。 |
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
| staking | DeclareValidatorAvailability | declare_validator_availability | 62003 | 调用 质押 的 DeclareValidatorAvailability 方法（handler: declare_validator_availability）。 |
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
| nft | TransferUpdateAuthor | transfer_update_author | 57787 | 转账NFT（handler: transfer_update_author）。 |
| nft | TransferFreezeAuthor | transfer_freeze_author | 50377 | 转账NFT（handler: transfer_freeze_author）。 |
| nft | SetCollectionTransferable | set_collection_transferable | 19091 | 设置NFT（handler: set_collection_transferable）。 |
| nft | TransferCollectionOwner | transfer_collection_owner | 64238 | 转账NFT（handler: transfer_collection_owner）。 |
| nft | SetCollectionCoverUri | set_collection_cover_uri | 32634 | 设置NFT（handler: set_collection_cover_uri）。 |
| nft | SetCollectionExternalUrl | set_collection_external_url | 8749 | 设置NFT（handler: set_collection_external_url）。 |
| nft | SetMintCoverUri | set_mint_cover_uri | 12898 | 设置NFT（handler: set_mint_cover_uri）。 |
| nft | SetMintExternalUrl | set_mint_external_url | 43141 | 设置NFT（handler: set_mint_external_url）。 |
| nft | SetAttributes | set_attributes | 47350 | 设置NFT（handler: set_attributes）。 |
| nft | SetProperties | set_properties | 48498 | 设置NFT（handler: set_properties）。 |
| nft | CreateUnique | create_unique | 33212 | 创建NFT（handler: create_unique）。 |
| nft | CreateBatch | create_batch | 43953 | 创建NFT（handler: create_batch）。 |
| nft | MintBatch | mint_batch | 51281 | 铸造/增发NFT（handler: mint_batch）。 |
| nft | Freeze | freeze | 36565 | 冻结NFT（handler: freeze）。 |
| nft | Unfreeze | unfreeze | 49498 | 解冻NFT（handler: unfreeze）。 |
| nft | Transfer | transfer | 48437 | 转账NFT（handler: transfer）。 |
| nft | Burn | burn | 7015 | 销毁NFT（handler: burn）。 |
| nft | TransferRoyaltyRecipient | transfer_royalty_recipient | 56414 | 转账NFT（handler: transfer_royalty_recipient）。 |
| dex | CreateMarket | create_market | 122 | 创建DEX（handler: create_market）。 |
| dex | InitializeMarketDid | initialize_market_did | 27598 | 初始化DEX（handler: initialize_market_did）。 |
| dex | DiscloseMarketVcAttestation | disclose_market_vc_attestation | 47697 | 披露DEX（handler: disclose_market_vc_attestation）。 |
| dex | PlaceLimitOrder | place_limit_order | 24195 | 调用 DEX 的 PlaceLimitOrder 方法（handler: place_limit_order）。 |
| dex | CancelOrder | cancel_order | 54560 | 调用 DEX 的 CancelOrder 方法（handler: cancel_order）。 |
| dex | CancelByClientOrderId | cancel_by_client_order_id | 16334 | 调用 DEX 的 CancelByClientOrderId 方法（handler: cancel_by_client_order_id）。 |
| dex | BatchCancel | batch_cancel | 55242 | 批量处理DEX（handler: batch_cancel）。 |
| dex | SetMarketStatus | set_market_status | 48095 | 设置DEX（handler: set_market_status）。 |
| keyless | SetAdmin | set_admin | 44485 | 设置无密钥（handler: set_admin）。 |
| keyless | RegisterProvider | register_provider | 47102 | 注册无密钥（handler: register_provider）。 |
| keyless | UpdateProvider | update_provider | 12186 | 更新无密钥（handler: update_provider）。 |
| keyless | SetProviderGovernanceEnabled | set_provider_governance_enabled | 24046 | 设置无密钥（handler: set_provider_governance_enabled）。 |
| keyless | ControllerSetProviderEnabled | controller_set_provider_enabled | 27342 | 调用 无密钥 的 ControllerSetProviderEnabled 方法（handler: controller_set_provider_enabled）。 |
| keyless | SetParams | set_params | 11932 | 设置无密钥（handler: set_params）。 |
| keyless | Authenticate | authenticate | 21310 | 调用 无密钥 的 Authenticate 方法（handler: authenticate）。 |
| keyless | Bind | bind | 59392 | 调用 无密钥 的 Bind 方法（handler: bind）。 |
| keyless | Unbind | unbind | 57503 | 调用 无密钥 的 Unbind 方法（handler: unbind）。 |
| keyless | RevokeSession | revoke_session | 15722 | 撤销授权无密钥（handler: revoke_session）。 |
| lucky_box | CreateEqual | create_equal | 51385 | 创建幸运盒（handler: create_equal）。 |
| lucky_box | CreateLucky | create_lucky | 12885 | 创建幸运盒（handler: create_lucky）。 |
| lucky_box | CreateNft | create_nft | 13669 | 创建幸运盒（handler: create_nft）。 |
| lucky_box | Claim | claim | 62102 | 领取幸运盒（handler: claim）。 |
| lucky_box | Refund | refund | 9120 | 调用 幸运盒 的 Refund 方法（handler: refund）。 |
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
| system | 0 | 0 | 11 | 11 |
| account | 1 | 5 | 10 | 15 |
| token | 2 | 7 | 22 | 29 |
| staking | 3 | 11 | 10 | 21 |
| identity | 4 | 24 | 17 | 41 |
| nft | 5 | 13 | 19 | 32 |
| dex | 6 | 6 | 8 | 14 |
| randomness | 7 | 2 | 0 | 2 |
| keyless | 8 | 6 | 10 | 16 |
| lucky_box | 9 | 2 | 5 | 7 |
| demo | 255 | 9 | 12 | 21 |
| **合计** | — | **85** | **124** | **209** |

## 备注

- 数据源：`gosdk-develop/provider/IDL/index.json`（SDK 实际注册的 app 列表），与 `gen/idl_gen.go` 的 `DefaultIDLs` 一致。
- `provider/IDL/randomness_demo.idl.json` 文件存在但未登记在 `index.json`，SDK 不会加载，故未收录。
- 本次（2026-09-11，SDK zip 8 / commit `8a58799`）相比上一版的变化：新增 app `dex`(6) / `keyless`(8) / `lucky_box`(9)；`nft` 由 20 → 32 个方法（issuer/metadata/royalty 模型改为 provider/mint 模型）；`staking` 移除 ActiveSet*/ConsensusActive* 等 10 个视图；`token` 移除 `transfer_with_tag`。
- `说明` 列由 `handler/idl_handler.go` 的 `describeInstruction` 逻辑生成。其中 `dex`、`keyless`、`lucky_box` 三个 app 的名称尚未列入 `appNameCN` / obj 映射，服务端 `/api/idl/metadata` 目前会回退为「`dex`对象」这类原始名；本表按语义显示为 DEX / 无密钥 / 幸运盒。
