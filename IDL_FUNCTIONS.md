# Milon IDL 函数清单

> 导出自 `gosdk-develop/provider/IDL/`，共 **12** 个 app、**337** 个函数（view 126 + entry 211）。

> 表格列：`appname`(应用名) · `meth`(方法名/指令名) · `handler`(链上入口名) · `id`(discriminator 指令编号) · `说明`(中文释义，来自 handler/idl_method_docs.go 的文档映射，与 /api/idl/metadata 输出一致)

## View（只读查询，kind=view）

共 **126** 个。

| appname | meth | handler | id | 说明 |
|---------|------|---------|-----|------|
| system | UpgradeStatus | upgrade_status | 17384 | 【功能】查询升级治理全量状态：已激活特性、进行中提案（票数/合格人数/阈值）、当前链参数与提案历史。<br>【返回】UpgradeStatus：{active:[{feature_id, activated_height}], proposals:[{proposal, votes, eligible, fast_threshold, pass_threshold}], params, history_len, history}。 |
| system | ProposalHistory | proposal_history | 57418 | 【功能】分页查询已结算提案的历史记录。<br>【返回】vec<ProposalHistoryItem>：{seq, feature_id, proposed_height, outcome:{result: Passed\|Expired\|Cancelled, height}}。 |
| system | AvailableMethods | available_methods | 30595 | 【功能】查询全部 app 当前可调用的指令清单（名称/discriminator/是否 view），用于链端能力发现。<br>【返回】vec<AppMethods>：{app_id, app, methods:[{name, discriminator, view}]}。 |
| system | ProposalOutcome | proposal_outcome | 12558 | 【功能】查询某提案（feature_id + 发起高度）的结算结局。<br>【返回】option<ProposalOutcome>：{result: Passed\|Expired\|Cancelled, height}，无记录返回 null。 |
| account | GetAccount | get_account | 11094 | 【功能】查询账户核心状态（多签位图、权重、阈值等）。<br>【返回】Account：{bitmap（signer 位图）、weight（总权重）、threshold（阈值）、last_modified_block}。注意：账户首笔交易前资源不存在，报 AccountNotFound（256）。 |
| account | ListSigners | list_signers | 24766 | 【功能】查询账户状态及全部 signer 列表。<br>【返回】[Account, [[PublicKey, 权重, 槽位号], ...]]（JSON 数组或 {"0":..,"1":..}）。 |
| account | ResolveSigners | resolve_signers | 27426 | 【功能】按 signer 槽位位图解析对应公钥列表，并校验权重/人数是否达标——交易提交前预检多签签名组合。<br>【返回】签名者公钥数组（hex/base58）。<br>【限制】位图为空报 SigBitEmpty（266）；位超 0..63 报 267；权重不足报 InsufficientSignerWeight（268）；人数不足报 SigBitTooFew（269）。 |
| account | GetVote | get_vote | 52139 | 【功能】查询某个投票意图的详情。<br>【返回】[VoteMeta{intent_hash, expires_at_ms, source_tx_hash}, 已投票 signer 位图, 是否已达阈值]。 |
| account | ListActiveVotes | list_active_votes | 46107 | 【功能】列出账户当前全部活跃投票意图。<br>【返回】[[VoteMeta, 已投票位图, 是否达标], ...] 数组。 |
| token | BalanceOf | balance_of | 59285 | 【功能】查询某地址在某代币上的余额。<br>【返回】u64，最小单位。 |
| token | FrozenOf | frozen_of | 64893 | 【功能】查询某地址被冻结的代币数量。<br>【返回】u64，最小单位。 |
| token | ApprovalOf | approval_of | 43638 | 【功能】查询 owner 授权给 spender 的剩余额度。<br>【返回】u64，最小单位。 |
| token | TotalSupply | total_supply | 35429 | 【功能】查询代币总供应量。<br>【返回】u64，最小单位。 |
| token | Metadata | metadata | 7880 | 【功能】查询代币元数据。<br>【返回】Metadata：{name, symbol, decimals, icon, uri}。 |
| token | Compliance | compliance | 43186 | 【功能】查询代币合规策略配置。<br>【返回】Compliance：{mode: "All"\|"Any", requirements: [凭证 ID 数组]}。 |
| token | FaucetCooldownRemaining | faucet_cooldown_remaining | 29892 | 【功能】查询某地址水龙头冷却剩余时间。<br>【返回】u64 毫秒数；0 表示可立即领取。 |
| token | HongbaoMessage | hongbao_message | 30775 | 【功能】查询红包附言（创建时 message 参数）。<br>【返回】String。 |
| token | HongbaoInfo | hongbao_info | 4156 | 【功能】查询红包实时状态。<br>【返回】HongbaoState：{creator, hongbao_id, token, total_amount, remaining_amount, claim_count, claimed_count, expires_at_ms, allocation, eligibility, claimed_recipients}。 |
| token | HongbaoSettlement | hongbao_settlement | 56611 | 【功能】查询红包清算进度。<br>【返回】HongbaoSettlement：{creator, status, claim_records, cleanup_cursor}。 |
| token | CanClaimHongbao | can_claim_hongbao | 5289 | 【功能】判断某地址当前能否领取某红包。<br>【返回】bool。 |
| token | VestingInfo | vesting_info | 14246 | 【功能】查询 Vesting 全量状态。<br>【返回】VestingState：{vesting_id, creator, token, start_at_ms, recipients（含各自 total/initial/periods/amount_per_period）, claimed_amounts, total_amount, remaining_amount, gas_budget}；不存在报 538。 |
| token | VestingClaimable | vesting_claimable | 6878 | 【功能】查询指定领款人在某 Vesting 中当前可领取的数量。<br>【返回】u64（最小单位）；Vesting 不存在报 538。 |
| staking | Pool | pool | 60310 | 【功能】查询池的全量记账快照。<br>【返回】Pool：{pool_id, owner, manager_app, app_permissions, stake_asset, reward_asset, cycle_config, commission 配置, 状态与记账字段}；不存在报 768。 |
| staking | TotalStake | total_stake | 11487 | 【功能】查询池的总本金（principal_assets）。<br>【返回】u64（最小单位）；池不存在报 769。 |
| staking | Position | position | 65179 | 【功能】查询某地址在某池的聚合持仓视图：激活本金、奖励进度、可领奖励/本金与各周期挂账（staking 查询主入口，取代旧按 stake_id 的逐单查询）。<br>【返回】Position：{owner, pool_id, active_principal, reward_checkpoint, reward_fraction, claimable_rewards, claimable_principal, deposits: [{cycle, amount}], exits: [{cycle, amount}], tracked}。 |
| staking | PoolTreasuryBalance | pool_treasury_balance | 309 | 【功能】查询池奖励金库（reward_asset）当前托管余额。<br>【返回】u64（最小单位）；池不存在报 768。 |
| staking | CommissionBalance | commission_balance | 51494 | 【功能】查询某池佣金收款人当前可领佣金余额。<br>【返回】u64（最小单位）。 |
| identity | VcAttestationCore | vc_attestation_core | 18681 | 【功能】查询某 (subject, schema, issuer) 组合下 VC 凭证的不可变核心内容。<br>【返回】VcAttestationCore：{credential_schema, credential_hash, valid_until_ms}；不存在报 1033。 |
| identity | VcAttestationLifecycle | vc_attestation_lifecycle | 41850 | 【功能】查询 VC 的生命周期状态（是否被撤销/冻结、最近变更时间）。<br>【返回】VcAttestationLifecycle：{status: "Active"\|"Revoked"\|"Frozen", updated_at_ms}。 |
| identity | AcceptedVcIssuerIndexMeta | accepted_vc_issuer_index_meta | 57307 | 【功能】查询 subject 某 schema 下「已接受 VC 签发方索引」的元信息。<br>【返回】String；索引不存在报 VcAcceptanceIndexNotFound（1067）。 |
| identity | AcceptedVcIssuers | accepted_vc_issuers | 6419 | 【功能】列出 subject 某 schema 下已接受凭证的签发方及凭证哈希。<br>【返回】[{issuer, credential_hash}]；数量有上限（TooManyAcceptedVcIssuers 1064）。 |
| identity | DisclosedVcSchemas | disclosed_vc_schemas | 48906 | 【功能】列出 subject 已披露 VC 的全部 schema。<br>【返回】字符串 JSON 数组；种类数有上限（TooManyDisclosedVcSchemas 1065）。 |
| identity | DisclosedVcs | disclosed_vcs | 27262 | 【功能】分页列出 subject 已披露 VC 的摘要。<br>【返回】[{credential_schema, issuer, credential_hash, valid_until_ms, status, is_valid}]。 |
| identity | HasValidVcFromIssuer | has_valid_vc_from_issuer | 18655 | 【功能】判断 subject 是否持有来自指定 issuer、指定 schema 且当前有效的 VC（合规校验常用，见 ORG_KYC_GUIDE 步骤 4）。<br>【返回】bool。 |
| identity | Core | core | 39696 | 【功能】查询 DID 核心状态（主体类型与 controller 地址）。<br>【返回】DidCoreState：{subject: {subject_type, address}, controller}。 |
| identity | Document | document | 21854 | 【功能】查询完整 DID 文档（DID 模型的主查询入口）。<br>【返回】DidDocumentState：{subject, controller, keys, services, alias, avatar_uri, updated_at_ms, deactivated}；未创建报 DidNotFound（1025）。 |
| identity | KeyIndex | key_index | 17805 | 【功能】查询 DID 密钥索引状态。<br>【返回】DidKeyIndexState：{initialized, key_ids（hex 字节串）}。 |
| identity | Keys | keys | 7171 | 【功能】列出该 DID 的全部密钥。<br>【返回】[{id, public_key, label}]。 |
| identity | Key | key | 49570 | 【功能】查询指定 id 的单把密钥。<br>【返回】DidKey；不存在报 1041。 |
| identity | ServiceIndex | service_index | 25961 | 【功能】查询 DID 服务索引状态。<br>【返回】DidServiceIndexState：{initialized, service_ids}。 |
| identity | Services | services | 567 | 【功能】列出该 DID 的全部服务条目。<br>【返回】[{id, label, service_endpoint}]。 |
| identity | Service | service | 62270 | 【功能】查询指定 id 的服务条目。<br>【返回】DidService；不存在报 1051。 |
| identity | Alias | alias | 8101 | 【功能】查询 DID 别名状态。<br>【返回】DidAliasState：{initialized, alias: {alias, suffix} 或 null}。 |
| identity | Avatar | avatar | 2498 | 【功能】查询 DID 头像状态。<br>【返回】DidAvatarState：{initialized, avatar_uri}。 |
| identity | UpdatedAt | updated_at | 46666 | 【功能】查询 DID 文档最近一次更新的毫秒时间戳。<br>【返回】u64。 |
| identity | Deactivated | deactivated | 12471 | 【功能】查询 DID 是否已停用。<br>【返回】bool。 |
| identity | NameBinding | name_binding | 60506 | 【功能】按别名反查其绑定的 DID 主体。<br>【返回】DidNameBinding：{name, subject}；未绑定报 NameNotFound（1029）。 |
| identity | CredentialDefinition | credential_definition | 42682 | 【功能】按凭证 ID 查询凭证定义（签发方与 schema）。<br>【返回】CredentialDefinition：{issuer, credential_schema}；不存在报 1063。 |
| identity | OrganizationCapabilities | organization_capabilities | 6251 | 【功能】查询组织已声明的角色与凭证 schema 列表。<br>【返回】OrganizationCapabilities：{roles, credential_schemas}；未注册报 1031。 |
| identity | OrganizationStatus | organization_status | 39483 | 【功能】查询组织状态。<br>【返回】"Active" 或 "Deactivated"。 |
| identity | OrganizationUpdatedAt | organization_updated_at | 23820 | 【功能】查询组织注册信息最近一次更新的毫秒时间戳。<br>【返回】u64。 |
| identity | CredentialId | credential_id | 63370 | 【功能】由 issuer 地址与 credential_schema 派生全局凭证 ID（用于 CredentialDefinition 等查询）。<br>【返回】String。 |
| sftoken | SftOwnerOf | sft_owner_of | 46443 | 【功能】查询 SFT 集合的 owner 地址。<br>【返回】Address。 |
| sftoken | SftMetadata | sft_metadata | 12747 | 【功能】查询 SFT 集合的元数据。<br>【返回】Metadata：{name, symbol, cover_url, metadata, attribute}；未创建报 SftNotFound（1285）。 |
| sftoken | SlotInfo | slot_info | 48907 | 【功能】查询指定槽位的信息。<br>【返回】SlotData：{metadata}（槽位元数据覆盖项）；不存在报 SlotNotFound（1284）。 |
| sftoken | SlotMetadata | slot_metadata | 26530 | 【功能】查询槽位生效的完整元数据（集合值 + 槽位覆盖项合成）。<br>【返回】Metadata；不存在报 1284。 |
| sftoken | IsSlotTransferable | is_slot_transferable | 16363 | 【功能】查询槽位下份额是否允许转移。<br>【返回】bool。 |
| sftoken | Token | token | 35083 | 【功能】查询 token 生效的完整元数据（集合/槽位/token 覆盖项逐级合成）。<br>【返回】Metadata；不存在报 TokenNotFound（1283）。 |
| sftoken | SlotOf | slot_of | 38850 | 【功能】查询指定 token 属于哪个槽位。<br>【返回】u64 slot_id。 |
| sftoken | BalanceOf | balance_of | 63018 | 【功能】查询 owner 在指定 token 上的持有数量（份额按 (token, owner) 记账，第二参数是 token_id）。<br>【返回】u64。 |
| sftoken | FrozenOf | frozen_of | 59068 | 【功能】查询 owner 在指定 token 上的份额是否被冻结。<br>【返回】bool。 |
| sftoken | ApprovalOf | approval_of | 63999 | 【功能】查询 spender 在 owner 的指定 token 份额上的剩余授权额度。<br>【返回】u64（未授权为 0）。 |
| sftoken | RoyaltyInfo | royalty_info | 30311 | 【功能】查询 SFT 的版税配置。<br>【返回】Royalty：{recipient, bps}（bps 为万分比）。 |
| sftoken | MintAuthority | mint_authority | 57868 | 【功能】查询 SFT 的铸造权限人地址。<br>【返回】Address。 |
| sftoken | UpdateAuthor | update_author | 6459 | 【功能】查询 SFT 的内容更新权限人地址。<br>【返回】Address。 |
| sftoken | FreezeAuthority | freeze_authority | 10975 | 【功能】查询 SFT 的冻结权限人地址。<br>【返回】Address。 |
| sftoken | TotalBalanceOf | total_balance_of | 32597 | 【功能】查询 owner 持有该 SFT 全部 token 的余额合计。<br>【返回】u64（最小单位）；聚合尚未回填报 TotalBalanceNotReady（1297）。 |
| sftoken | TotalBalanceStatus | total_balance_status | 56878 | 【功能】查询 owner 侧 SFT 余额聚合的就绪状态与进度。<br>【返回】SftBalanceState：{amount, next_token_id, ready}；ready=false 表示聚合尚未回填完成。 |
| sftoken | TotalSupply | total_supply | 35462 | 【功能】查询该 SFT 全链发行总量。<br>【返回】u64（最小单位）；聚合尚未回填报 TotalSupplyNotReady（1298）。 |
| sftoken | TotalSupplyStatus | total_supply_status | 30765 | 【功能】查询该 SFT 发行总量聚合的就绪状态与进度。<br>【返回】SftBalanceState：{amount, next_token_id, ready}。 |
| dex | AmmPoolInfo | amm_pool_info | 60394 | 【功能】查询 AMM 池当前状态。<br>【返回】AmmPoolInfo：{market_id, vault, vault_seed, status, reserve_base, reserve_quote, total_lp_supply}。 |
| dex | AmmLpBalance | amm_lp_balance | 18436 | 【功能】查询某地址在某 AMM 池的 LP 份额。<br>【返回】u64。 |
| dex | AmmSwapEnabled | amm_swap_enabled | 52994 | 【功能】查询市场 AMM 池是否处于可交易状态。<br>【返回】bool。 |
| dex | MarketInfo | market_info | 2326 | 【功能】查询市场完整配置。<br>【返回】Market：{base_token, quote_token, base_lot_atoms, quote_atoms_per_lot_tick, min_tick, max_tick, max_order_lots, max_fills_per_action, maker_fee_ppm, taker_fee_ppm, authority, status}。 |
| dex | OrderInfoView | order_info_view | 23456 | 【功能】按 order_id 查询订单实时状态（含锁定量 reserved_amount）。<br>【返回】OrderInfo。 |
| dex | BestBidAsk | best_bid_ask | 12137 | 【功能】查询订单簿最优买价/卖价档。<br>【返回】{bid: {tick, order_count, total_base_lots} 或 null, ask: 同}；空簿一侧为 null。 |
| dex | OrderbookDepth | orderbook_depth | 46884 | 【功能】查询买卖各价位档的深度。<br>【返回】{bids: [{tick, order_count, total_base_lots}], asks: [...]}。<br>【限制】depth 上限 50（InvalidViewLimit 1552）。 |
| dex | OrdersAtLevel | orders_at_level | 24461 | 【功能】分页查询某一价格档上的订单列表。<br>【返回】LevelOrderPage：{orders: [OrderInfo], next_order_id（翻页游标，0 表示没有更多）}。<br>【限制】limit 1..=100（1552）；档位不存在报 PriceLevelNotFound（1553）；游标非法报 InvalidOrderCursor（1554）。 |
| dex | VaultLiability | vault_liability | 6759 | 【功能】查询市场金库对某代币的负债总额（所有用户托管资金之和）。<br>【返回】u64。 |
| validator | ValidatorEpochActivity | validator_epoch_activity | 46392 | 【功能】查询某 epoch 的验证人出块活动统计。<br>【返回】EpochActivity：{blocks（出块数）, signers（参与签名验证人数）, last_block（末块高度）}。 |
| validator | ValidatorMemberships | validator_memberships | 43732 | 【功能】查询全体验证人集合成员关系。<br>【返回】vec<Membership>：每项 {validator, join_epoch, exit_epoch（option，null 表示在任）, minimum_self_stake}。 |
| validator | CurrentValidatorPools | current_validator_pools | 32584 | 【功能】查询当前 epoch 全体验证人池总览。<br>【返回】ValidatorPoolList：{epoch, validator_set_hash, validators: [{validator, consensus_account, pool_id, stake_asset, reward_asset, total_stake, commission_rate_bps, position_count}]}；集合未就绪报 1801。 |
| validator | ValidatorRewardBudget | validator_reward_budget | 24480 | 【功能】查询验证人奖励预算余额（FundValidatorRewards 注资的累计可发放额度）。<br>【返回】u64（最小单位）。 |
| validator | ValidatorProfile | validator_profile | 37544 | 【功能】查询验证人档案。<br>【返回】ValidatorProfile：{validator, owner, pool_id}；未注册报 1793。 |
| validator | ValidatorForOwner | validator_for_owner | 10754 | 【功能】按 owner 地址反查其绑定的 ValidatorId。<br>【返回】ValidatorId（B256）；未绑定报 1793。 |
| validator | ValidatorSelfStake | validator_self_stake | 13504 | 【功能】查询某验证人在目标 epoch 的自押金额（快照值）。<br>【返回】u64（最小单位）。 |
| validator | RegisteredValidators | registered_validators | 19434 | 【功能】查询全体验证人 ID 列表。<br>【返回】vec<B256>（base58）。 |
| keyless | GetSessions | get_sessions | 58087 | 【功能】查询某账户的全部活跃会话。<br>【返回】[{session_id, provider_id, audience_hash, session_version, session_pubkey, expires_at_ms, created_at_ms}]。 |
| keyless | GetBinding | get_binding | 4855 | 【功能】查询某 keyless 地址的绑定信息。<br>【返回】null 或 {owner, slot_index, bound_at_ms, active_at_ms}。 |
| keyless | ListProviders | list_providers | 12570 | 【功能】列出全部 provider（内置 + 自定义，无参数）。<br>【返回】[{provider_id, kind: "BuiltinOidc"\|"StaticJwk", display_name, issuer, allowed_algs, max_id_token_age_ms, max_session_ttl_ms, owner, keys, key_revision}]。 |
| keyless | GetProvider | get_provider | 20071 | 【功能】按 ID 查询单个 provider。<br>【返回】KeylessProvider 或 null。 |
| keyless | GetProviderStatus | get_provider_status | 62602 | 【功能】查询 provider 的启用状态与会话版本。<br>【返回】{enabled, session_version} 或 null。 |
| keyless | GetJwkReport | get_jwk_report | 44193 | 【功能】查询某验证人对某 provider 的 JWK 报告。<br>【返回】option<ValidatorJwkReport>：{validator, keys:[JWK JSON]}，无报告返回 null。 |
| keyless | GetSelectedJwk | get_selected_jwk | 65396 | 【功能】查询共识当前选中的某 provider/issuer/kid/algorithm 的 JWK。<br>【返回】option<String>（JWK JSON），未选中返回 null。 |
| keyless | GetAudience | get_audience | 13840 | 【功能】查询某 provider 下某 audience 的注册信息。<br>【返回】{provider_id, audience, registered_at_ms} 或 null。 |
| keyless | GetParams | get_params | 12312 | 【功能】查询 keyless 全局参数（无参数）。<br>【返回】KeylessParams：{max_session_ttl_ms（默认 1 天）, bind_activation_delay_ms（默认 1 天）}。 |
| keyless | IsKeylessAccount | is_keyless_account | 33494 | 【功能】判断某地址是否已绑定 keyless 身份。<br>【返回】bool。 |
| lucky_box | BoxView | box_view | 58937 | 【功能】查询盒子状态。<br>【返回】BoxState：{creator, box_id, status, claim_count, claimed_count, expires_at_ms, randomness_version}。 |
| lucky_box | AssetPool | asset_pool | 45729 | 【功能】查询盒子资产池。<br>【返回】AssetPoolState：{asset: {variant:"Token", value:{token}}, total_amount, remaining_amount, allocation: "Equal"\|"Lucky"}。 |
| social | IsFollowing | is_following | 49754 | 【功能】查询 follower 是否正关注 target。<br>【返回】bool。 |
| social | FollowSponsorAddress | follow_sponsor_address | 17041 | 【功能】按目标地址反查其关注赞助池地址。<br>【返回】Address。 |
| social | FollowSponsor | follow_sponsor | 13614 | 【功能】查询关注赞助池状态。<br>【返回】FollowSponsor：{owner, target, active}。 |
| social | SocialProfile | social_profile | 5863 | 【功能】查询某地址的 profile URI。<br>【返回】String。 |
| social | SourceApp | source_app | 2147 | 【功能】查询源应用信息。<br>【返回】SourceApp：{owner, publishing_enabled}；不存在报 2564。 |
| social | IsSourceAppPublisher | is_source_app_publisher | 14357 | 【功能】判断某地址是否为源应用的发布者。<br>【返回】bool。 |
| social | GetReference | get_reference | 7408 | 【功能】查询对象引用记录。<br>【返回】Reference：{subject, content_hash, uri, status("Active"\|"Retired")}；不存在报 ReferenceNotFound（2574）。 |
| social | Community | community | 59279 | 【功能】查询社区控制信息。<br>【返回】CommunityControl：{owner, join_mode, mutations_enabled, member_count, uri}。 |
| social | IsCommunityDissolved | is_community_dissolved | 23214 | 【功能】查询社区是否已解散。<br>【返回】bool。 |
| social | IsCommunityAdmin | is_community_admin | 56720 | 【功能】判断某地址是否为社区管理员（含 owner）。<br>【返回】bool。 |
| social | JoinRequest | join_request | 38404 | 【功能】查询某成员是否有 pending 的加入申请。<br>【返回】bool。 |
| social | CommunityMembership | community_membership | 36616 | 【功能】查询某地址是否为社区成员。<br>【返回】bool。 |
| social | CanClaimCommunityHongbao | can_claim_community_hongbao | 16618 | 【功能】判断某地址当前能否领取某社区红包。<br>【返回】bool。 |
| social | HongbaoCommunity | hongbao_community | 54956 | 【功能】查询社区红包所属的社区 ID。<br>【返回】u32。 |
| demo | TierCapDetail | tier_cap_detail | 30615 | 【功能】查询池中某积分等级的上限与设置人。<br>【返回】TierCap：{cap, set_by}。 |
| demo | LabelChangesOf | label_changes_of | 54498 | 【功能】查询池累计的标签变更次数。<br>【返回】u64。 |
| demo | OrderVault | order_vault | 9625 | 【功能】查询演示订单托管户头地址。<br>【返回】Address。 |
| demo | OrderLastInbound | order_last_inbound | 56285 | 【功能】查询演示订单最新一笔入账明细。<br>【返回】LastInbound：{token, from, amount}。 |
| demo | OrderBalance | order_balance | 23229 | 【功能】查询演示订单在某代币上的托管余额。<br>【返回】u64。 |
| demo | LabelOf | label_of | 36952 | 【功能】查询池标签。<br>【返回】Label：{text}。 |
| demo | ScoreOf | score_of | 24200 | 【功能】查询池内某账户的积分。<br>【返回】u64。 |
| demo | TierCapOf | tier_cap_of | 63079 | 【功能】查询某层级的积分上限。<br>【返回】u64。 |
| demo | EchoMode | echo_mode | 48471 | 【功能】枚举编码回显演示：传入 DemoMode 原样返回（用于验证枚举编码）。<br>【返回】DemoMode。 |
| demo | LabelTotal | label_total | 9869 | 【功能】map 类型演示：对传入的标签-数值映射求和。<br>【返回】u32。 |
| demo | SpecialTypes | special_types | 37219 | 【功能】复杂类型编码演示（enum/option/vec/map/tuple 全覆盖）。<br>【返回】u32（各输入的汇总校验值）。 |
| demo | SponsorPoolOf | sponsor_pool_of | 38217 | 【功能】按池编号查询赞助池地址。<br>【返回】Address。 |
| demo | ScheduledWakeCount | scheduled_wake_count | 45310 | 【功能】查询当前已登记待执行的定时唤醒数量。<br>【返回】u64。 |

## Entry（写状态交易，kind=entry）

共 **211** 个。

| appname | meth | handler | id | 说明 |
|---------|------|---------|-----|------|
| system | Noop | noop | 13507 | 【功能】空操作指令，无参数、无任何链上效果，常用于测试交易链路或触发账户资源的懒创建。<br>【限制】无。 |
| system | BootstrapEpochClock | bootstrap_epoch_clock | 57604 | 【功能】初始化协议 epoch 时钟（计时起点 + 最短纪元时长），是 staking 纪元结算与验证人 epoch 调度的前置条件。<br>【限制】一次性初始化，重复调用会失败（EpochClockFailure）；仅在链引导阶段调用。 |
| system | BootstrapValidatorSetConfig | bootstrap_validator_set_config | 37696 | 【功能】初始化验证人集合配置：最大验证人数，以及分组/出块/批次委员会三类阈值比例，全局一次性配置。<br>【限制】一次性初始化；比例以「分子/分母」分数表示。 |
| system | BootstrapRegisteredValidatorSet | bootstrap_registered_validator_set | 46958 | 【功能】引导期一次性写入验证人集合：validators 为已完成 validator.RegisterValidator 注册的验证人 ID 列表（替代旧 RegisterValidatorIdentity/PrepareValidatorSet/CommitValidatorSet 三步流程）。<br>【限制】仅链引导阶段调用；失败以 ValidatorSetFailure（99）包装。 |
| system | CommitBlockSeed | commit_block_seed | 49456 | 【功能】提交/更新区块随机种子（链上随机数信标的数据源）。<br>【限制】种子未初始化时不可提交（BlockSeedNotInitialized）；重复初始化报 BlockSeedAlreadyInitialized；底层失败以 RandomnessFailure 包装。 |
| system | BootstrapChainParams | bootstrap_chain_params | 34076 | 【功能】引导阶段直接写入链参数键值对（键为 u16 编号、值为 opaque bytes），不经过提案投票流程。<br>【限制】仅链引导（genesis）阶段调用。 |
| system | Propose | propose | 50539 | 【功能】发起特性升级提案（投票通过后于后续高度激活，Urgent 走快速阈值）。<br>【限制】交易须验证人签名授权（无显式 signer 参数，2026-11-20 起升级提案经验证人集合通道提交）；feature_id 未激活（UpgradeFeatureActive 105）；同 feature_id 提案不存在（UpgradeProposalExists 106）；该 kind 的提案槽空闲（UpgradeKindBusy 107）；须系统交易通道（UpgradeSystemTxOnly 114）。 |
| system | ProposeParams | propose_params | 54058 | 【功能】发起链参数修改提案（params 键值对经投票通过后生效）。<br>【限制】同 system.Propose；参数非法报 UpgradeInvalidParams（109）。 |
| system | Vote | vote | 56775 | 【功能】验证人对指定提案投赞成票（交易须验证人签名授权）。<br>【限制】提案不存在报 UpgradeProposalNotFound（110）；投票期已结束报 UpgradeVotingClosed（111）；重复投票报 UpgradeAlreadyVoted（112）。 |
| system | Cancel | cancel | 46921 | 【功能】提案发起人撤销提案。<br>【限制】仅提案发起人可撤销（UpgradeNotProposer 113）；提案不存在报 UpgradeProposalNotFound（110）。 |
| system | CloseVote | close_vote | 45954 | 【功能】关闭投票期并结算提案：达标进入激活队列，未达标标记过期。<br>【限制】提案不存在报 UpgradeProposalNotFound（110）。 |
| account | Create | create | 2182 | 【功能】为给定公钥显式创建链上账户资源（区别于首笔交易时的懒创建），账户默认单签模式。<br>【限制】账户已存在报 AccountAlreadyExists（257）；公钥非法报 InvalidPublicKey（273）。 |
| account | EnsureAccount | ensure_account | 38184 | 【功能】确保账户存在：不存在则创建、已存在则什么都不做（幂等建户）。<br>【限制】幂等，已存在不报错（与 Create 的区别）。 |
| account | CreateMultisig | create_multisig | 20289 | 【功能】将签名者账户配置为多签账户：登记 signer 公钥列表、每个 signer 的权重与生效阈值（权重和 ≥ 阈值才可通过）。<br>【限制】signer 总权重 1..=255（WeightSumExceeded 263）；signers 与 weights 长度须一致（264）；signer 槽位 0..63（261）；权重不得为 0（262）；阈值非法报 ThresholdIncorrect（265）。链端 2026-11-12 起多签授权全 PQ 化：signers 仅接受 FN-DSA-512 公钥（经典 ed25519/secp256k1 公钥报 273 InvalidPublicKey），owner 经典签名会被授权校验拒（69 PqSignatureRequired），owner+signers 须全 FN-DSA-512；注册消耗链上费用，owner 账户须先有余额。 |
| account | AddSigner | add_signer | 41092 | 【功能】向多签账户添加一个 signer（按公钥），自动分配最低空闲槽位。<br>【限制】账户必须已是多签账户（AccountNotMultisig 274）；公钥不可重复（258）；总权重不得超过 255（263）；槽位最多 64 个（261）。 |
| account | AddSignerByAddress | add_signer_by_address | 30603 | 【功能】向多签账户按地址添加一个 signer（区别于按公钥的 AddSigner），自动分配最低空闲槽位。<br>【限制】账户必须已是多签账户（AccountNotMultisig 274）；地址不可重复（258）；权重 1..=255（262）；总权重 ≤255（263）；槽位最多 64 个（261）；地址与已登记公钥不匹配报 SignerAddressKeyMismatch（288）。 |
| account | AddSigners | add_signers | 25813 | 【功能】批量添加多个 signer 并同步设置新阈值（CreateMultisig 的增量版）。<br>【限制】同 CreateMultisig：长度一致、权重 1..=255、总权重 ≤255、槽位 0..63、阈值合法。 |
| account | AddSignersByAddress | add_signers_by_address | 19656 | 【功能】按地址批量添加多个 signer 并同步设置新阈值（CreateMultisig 的按地址增量版）。<br>【限制】同 AddSigners：signers 与 weights 长度一致（264）、权重 1..=255、总权重 ≤255、槽位 0..63、阈值合法（265）；地址不可重复（258）。 |
| account | RemoveSigner | remove_signer | 61953 | 【功能】从多签账户移除指定槽位的 signer 并设置移除后的新阈值。<br>【限制】该槽位必须已有 signer（SignerNotFound 259）；须为多签账户（275）；新阈值须合法（265）。 |
| account | SetThreshold | set_threshold | 2386 | 【功能】修改多签账户的生效阈值。<br>【限制】阈值须合法且不超过当前总权重（ThresholdIncorrect 265）。 |
| account | SetSignerWeight | set_signer_weight | 43270 | 【功能】修改指定槽位 signer 的权重。<br>【限制】槽位须已有 signer（259）；权重 1..=255；总权重不得超过 255（263）。 |
| account | VoteInit | vote_init | 52917 | 【功能】登记一个投票意图（MIP-25 意图重放）：提交 intent_hash 与提案内容（待重放的编码指令 + 授权位图），设置过期时间，等待账户 signer 们投票。<br>【限制】提案编码后 ≤1024 字节（VoteProposalTooLarge 282）；提案须与 intent_hash 匹配（281）；过期时间必须晚于当前时间（276）且 TTL ≤24 小时（VoteTtlExceeded 280）；活跃意图数量有上限（VoteIndexFull 279）。 |
| account | SweepExpiredVotes | sweep_expired_votes | 37954 | 【功能】一次性清理自己名下已过期的投票意向，腾出投票索引位。<br>【限制】仅可清理已过期意向（VoteExpired 276）；清扫手续费不足报 VoteSweepFeeInsufficient 285；清扫唤醒被拒报 VoteWakeRejected 286。 |
| account | Vote | vote | 24406 | 【功能】对已登记的投票意图投出一票（按投票者权重累加），达到阈值后意图可被重放执行。<br>【限制】意图须存在且未过期（VoteExpired 276）；未达阈值时重放会被拒（VoteNotReady 277）。 |
| token | Create | create | 2581 | 【功能】创建新代币资源：登记名称/符号/精度/图标等元数据并设定 owner（管理权限）。发币方调用；token 地址由专用密钥对派生，且该密钥必须参与交易签名。<br>【限制】token 地址已被占用报 TokenAlreadyExists（516）；token 私钥必须签名，否则报 AddressNotSignatured（519）。 |
| token | AbandonOwner | abandon_owner | 64710 | 【功能】永久放弃代币 owner 权限，代币变为无主，此后所有管理操作（增发/冻结/合规配置等）失效。<br>【限制】仅当前 owner 生效（链上校验）；不可逆。 |
| token | TransferOwner | transfer_owner | 18518 | 【功能】将代币 owner 权限转移给新地址。<br>【限制】仅当前 owner 生效。 |
| token | AbandonFreezer | abandon_freezer | 1778 | 【功能】永久放弃代币 freezer（冻结管理）权限。<br>【限制】仅当前 freezer 生效；不可逆。 |
| token | TransferFreezer | transfer_freezer | 27042 | 【功能】将代币 freezer 权限转移给新地址。<br>【限制】仅当前 freezer 生效。 |
| token | Mint | mint | 20481 | 【功能】向指定地址增发代币（owner 操作）。<br>【限制】仅 owner 生效；代币须已创建（TokenNotFound 515）；增发后总量不得溢出 u64（Overflow 517）。 |
| token | MintBatch | mint_batch | 7494 | 【功能】一次性向多个地址增发代币。<br>【限制】仅 owner；to 与 amount 数组长度必须一致；任一项溢出即整笔失败（517）。 |
| token | Burn | burn | 38784 | 【功能】销毁签名者自己持有的代币（减少总量）。<br>【限制】holder 必须签名；可用余额不足报 InsufficientBalance（513）；冻结部分不可销毁。 |
| token | Transfer | transfer | 19694 | 【功能】从签名者向目标地址转账代币（最常用的用户操作）。<br>【限制】from 必须签名；余额不足报 InsufficientBalance（513）；合规代币的接收方不满足凭证要求会报 VcRequired（520）/VcPolicyRequired（521）。 |
| token | TransferBatch | transfer_batch | 28053 | 【功能】从签名者一次性向多个地址转账。<br>【限制】from 必须签名；to 与 amount 等长；余额须覆盖转账总额（513）。 |
| token | Freeze | freeze | 31050 | 【功能】冻结某持有人指定数量的代币，冻结部分不可转账/销毁（合规风控操作，freezer 调用）。<br>【限制】仅 freezer 生效；冻结量不得超过持有量。 |
| token | Unfreeze | unfreeze | 17977 | 【功能】解冻之前冻结的代币。<br>【限制】仅 freezer 生效；解冻量不得超过冻结量（Underflow 518）。 |
| token | Approve | approve | 9714 | 【功能】授权 spender 可动用自己名下代币的额度（ERC20 approve 语义）。<br>【限制】owner 必须签名。 |
| token | Revoke | revoke | 8619 | 【功能】撤销 spender 的授权（额度清零）。<br>【限制】owner 必须签名。 |
| token | TransferFrom | transfer_from | 18655 | 【功能】spender 动用授权额度，把 from 名下代币转给指定地址（委托转账）。<br>【限制】spender 必须签名；授权额度不足报 InsufficientApproval（514）；持有人余额不足报 513。 |
| token | SetIcon | set_icon | 40941 | 【功能】设置/更新代币图标 URL。<br>【限制】仅 owner 生效；代币须已创建（515）。 |
| token | SetUri | set_uri | 52758 | 【功能】设置/更新代币元数据 URI。<br>【限制】仅 owner 生效。 |
| token | CreateWithCompliance | create_with_compliance | 62196 | 【功能】创建带合规要求的代币：创建同时登记必需凭证（VC）ID，接收方须持有满足合规策略的凭证才能收到币（受监管资产发行用）。<br>【限制】同 token.Create（地址占用/签名）；后续转账会被合规校验拦截（521/522）。 |
| token | SetComplianceMode | set_compliance_mode | 33931 | 【功能】设置合规策略模式：All=接收方须持有全部已列凭证；Any=持有其中任意一个即可。<br>【限制】仅 owner 生效。 |
| token | AddComplianceRequirement | add_compliance_requirement | 59302 | 【功能】向合规要求列表批量添加必需凭证 ID（credential_ids 可一次加多个）。<br>【限制】仅 owner 生效。 |
| token | RemoveComplianceRequirement | remove_compliance_requirement | 14909 | 【功能】从合规要求列表批量移除凭证 ID（credential_ids 可一次移除多个）。<br>【限制】仅 owner 生效。 |
| token | ClearComplianceRequirements | clear_compliance_requirements | 50437 | 【功能】清空全部合规凭证要求，代币恢复为无合规限制。<br>【限制】仅 owner 生效。 |
| token | DiscloseTokenVcAttestation | disclose_token_vc_attestation | 39942 | 【功能】公开披露代币的 VC 认证材料：发行方对凭证（模式/哈希/有效期）的签名上链，供第三方核验代币合规背书。<br>【限制】issuer_signature 须为发行方私钥对凭证内容的签名；issuer_key_id 为发行方密钥序号；valid_until_ms 可空表示长期有效。 |
| token | ClaimFaucet | claim_faucet | 63796 | 【功能】从水龙头领取原生 MIL 代币（新账户获取 gas 费用）。本指令 gas 由链上赞助池代付（sponsor）。<br>【限制】每次领取 10^10 最小单位（6 位精度下 10,000 token）；每地址 24 小时冷却，冷却期内任何尝试（含失败）都会触发/续期冷却（FaucetCooldownActive 523），脚本领水务必单次尝试；低链 ID 下水龙头禁用（FaucetDisabled 522）。 |
| token | CreateEqualHongbao | create_equal_hongbao | 56849 | 【功能】创建均分红包：创建者托管 total_amount，供 claim_count 人均分领取；领取资格由 rules 约束（可选 VC 凭证要求 / 指定收款人名单 / 过期时间），可附言 message。hongbao_id 由链上派生，经交易事件或 HongbaoInfo 回读。<br>【限制】红包定义不合法报 InvalidHongbaoDefinition 529（claim_count 须非零等）；hongbao_id 不可重复（HongbaoAlreadyExists 525）；托管转入失败报 token 层错误。 |
| token | CreateLuckyHongbao | create_lucky_hongbao | 32577 | 【功能】创建拼手气红包：创建者托管 total_amount，claim_count 人随机份额领取（份额由链上安全随机源决定），可附言 message。<br>【限制】同 CreateEqualHongbao；随机源不可用报 HongbaoRandomnessUnavailable 533。 |
| token | CreateCredentialHongbao | create_credential_hongbao | 38296 | 【功能】创建凭证红包：创建者托管 total_amount，领取人须持有 credential_ids 列表中任一有效 VC 凭证方可领取，claim_count 人按 allocation（Equal/Lucky）分配，可设过期时间与附言。<br>【限制】同 CreateEqualHongbao；资格校验走 identity VC 体系（未持凭证报 HongbaoNotEligible 530）。 |
| token | CreateSftHongbao | create_sft_hongbao | 18018 | 【功能】创建 SFT 红包：创建者托管 total_amount，领取人须持有不少于 min_amount 个指定 SFT（sft 地址）方可领取，claim_count 人按 allocation（Equal/Lucky）分配，可设过期时间与附言。<br>【限制】同 CreateEqualHongbao；SFT 校验失败透传 sftoken 层错误（SftError 544）。 |
| token | ClaimHongbao | claim_hongbao | 46748 | 【功能】领取红包（claimant 签名）。<br>【限制】红包须存在（HongbaoNotFound 524）；须在有效期内（HongbaoExpired 526）；资格不符报 HongbaoNotEligible 530（rules 名单 / VC / SFT 要求）；不可重复领取（HongbaoAlreadyClaimed 531）。 |
| token | RefundHongbao | refund_hongbao | 51292 | 【功能】红包过期后把剩余托管金额退回创建者。<br>【限制】红包须存在（524）；须已过期（HongbaoNotExpired 527）；仍在进行中报 HongbaoStillActive 528。 |
| token | RunHongbaoCleanup | run_hongbao_cleanup | 43410 | 【功能】推进过期红包的清算（结算领取记录、回收托管），配合到期唤醒机制逐批执行。<br>【限制】红包须已过期（HongbaoStillActive 528）；清扫唤醒被拒报 HongbaoCleanupWakeRejected 534。 |
| token | CreateVesting | create_vesting | 47760 | 【功能】创建分期释放（Vesting）：creator 锁定代币进托管，按名单分批释放给各领款人（每人独立配置总额/首释量/周期时长/期数/每期释放量，start_at_ms 为起算时间；name 仅作展示标签，每行可重复、不参与唯一性判定）。<br>【限制】定义须自洽（InvalidVestingDefinition 537：periods>0、period_seconds>0、initial∈[0,total]、每期释放量 (total−initial)/periods 向下取整后须>0、start_at_ms 不能在过去、name 不能为空串；不整除时末期补足余数）。同 seed（creator+token+start_at_ms+recipients 全字段一致，name 亦计入）重复创建报 VestingAlreadyExists（539）；余额不足报 InsufficientBalance。 |
| token | ClaimVesting | claim_vesting | 14230 | 【功能】领款人领取自己在某 Vesting 中当前累计可释放的代币（支持 sponsor 代付 gas）。<br>【限制】Vesting 不存在报 VestingNotFound（538）；签名者不在领款名单报 VestingNotEligible（540）；当前无可领报 VestingNothingClaimable（541）；托管账实不一致报 543。 |
| staking | CreatePool | create_pool | 52671 | 【功能】创建质押池：指定质押/奖励代币、周期配置（质押激活/解冻/奖励发放/佣金变更四类延迟，周期基准支持 Epoch/块高/时间戳）与佣金配置（费率+收款地址），返回池 ID。池主对池拥有暂停/关闭/佣金收取等全权。<br>【返回】PoolId（新建池 ID，base58 字符串）。<br>【限制】佣金率 ≤10000 基点；nonce 用于池 ID 确定性派生，非法报 InvalidParameters（768）；周期配置须合法（768）。 |
| staking | PausePool | pause_pool | 55077 | 【功能】池主暂停池：暂停期间禁止新质押与解冻申请，已激活份额的奖励照常累计。<br>【限制】须为池主（769）；仅 Open 状态可暂停。 |
| staking | ResumePool | resume_pool | 55138 | 【功能】池主恢复已暂停的池至 Open 状态。<br>【限制】须为池主（769）；仅 Paused 状态可恢复。 |
| staking | ClosePool | close_pool | 40303 | 【功能】池主发起关池：池进入 Closing，按调度周期逐步结算存量质押，结算完毕后池进入 Drained。<br>【限制】须为池主（769）；Closing 后不再接受新质押；关池前通常先通过 RequestPoolRewardConfig 清零佣金。 |
| staking | SweepPoolResidual | sweep_pool_residual | 51163 | 【功能】清理关池后的池内残留资金（残差/尾款），转入池主账户，池进入 Drained 终态。<br>【返回】u64 实际清扫金额（最小单位）。<br>【限制】须为池主（769）；池须已完成排水结算（769）；无可清扫返回 0。 |
| staking | StakeIntoPool | stake_into_pool | 65380 | 【功能】向指定池质押代币，先记为待激活（deposits 挂账），过质押激活延迟后生效。<br>【限制】池状态须为 Open（Paused/Closing 拒绝）；余额不足报 TokenError（771）；金额须大于 0（768）。 |
| staking | CancelPendingStake | cancel_pending_stake | 26203 | 【功能】撤销指定生效周期（effective_cycle）尚未激活的质押挂账，全额取回本金。<br>【限制】须为持仓所有者；对应 pending 记录不存在报 768。 |
| staking | RequestUnstake | request_unstake | 34861 | 【功能】对某池持仓按代币数量（amount，非份额）申请解冻：记录解冻意向（exits 挂账），过解冻激活延迟后可领取本金。<br>【限制】金额不得超过该池可解冻持仓（768）；池须处于可解冻状态。 |
| staking | CancelUnstake | cancel_unstake | 53570 | 【功能】撤销指定生效周期尚未生效的解冻意向，本金继续留在池内计息。<br>【限制】须为持仓所有者；对应解冻意向不存在报 768。 |
| staking | ClaimPrincipal | claim_principal | 35421 | 【功能】领取某池中已过解冻延迟的可取本金。<br>【返回】u64 实际领取数量（最小单位）；无可领返回 0。<br>【限制】须为持仓所有者。 |
| staking | ClaimRewards | claim_rewards | 49863 | 【功能】领取某池累计的可领奖励（按池奖励进度线性累计）。<br>【返回】u64 实际领取数量（最小单位）；无可领返回 0。<br>【限制】须为持仓所有者。 |
| staking | AccruePoolReward | accrue_pool_reward | 61672 | 【功能】向指定池注入奖励资金（reward_asset），使全体质押人按份额分享。任何持币人可为池充奖。<br>【限制】余额不足报 TokenError（771）；金额须大于 0（768）。 |
| staking | ClaimCommission | claim_commission | 39476 | 【功能】佣金收款人领取某池累计佣金。<br>【返回】u64 实际领取数量（最小单位）；无可领返回 0。<br>【限制】仅佣金收款地址（recipient）可领。 |
| staking | ClaimPoolCommission | claim_pool_commission | 17449 | 【功能】佣金收款人领取累计佣金（按佣金费率从注入奖励中切分）。<br>【返回】u64 实际领取佣金数量（最小单位）；无可领返回 0。<br>【限制】仅佣金收款地址（recipient）可领（769）。 |
| staking | RequestPoolRewardConfig | request_pool_reward_config | 37925 | 【功能】池主申请变更佣金配置（费率/收款地址）：新配置记为待生效，过佣金变更延迟后自动启用。<br>【限制】须为池主（769）；配置须合法（768）；存在待生效配置时再次申请将覆盖。 |
| identity | DiscloseVcAttestation | disclose_vc_attestation | 23078 | 【功能】凭证持有人将 issuer 签发的可验证凭证（VC）披露上链：登记 issuer、schema、凭证哈希与 issuer 签名（issuer 只提供签名、不作为交易签名者）。组织 KYC 链路第三步。<br>【限制】subject 须有 DID；issuer 的 DID 须存在且未停用（IssuerDidNotActive 1033）；issuer_signature 由链上验签（InvalidVcIssuerSignature 1069）；同一 (subject, issuer, schema) 不可重复披露不同内容（1072）；已撤销凭证不可再披露（1073）。 |
| identity | RemoveVcDisclosure | remove_vc_disclosure | 51077 | 【功能】subject 撤下一条自己已披露的 VC（不影响 issuer 侧凭证状态）。<br>【限制】该披露须存在（VcAttestationNotFound 1032）。 |
| identity | RevokeVcAttestation | revoke_vc_attestation | 38692 | 【功能】issuer 永久撤销其签发给某 subject 的 VC（状态变为 Revoked，不可恢复）。<br>【限制】仅原 issuer 可撤销（issuer 为签名者）；凭证须存在（1032）；撤销后再披露报 VcAttestationRevoked（1072）。 |
| identity | FreezeVcAttestation | freeze_vc_attestation | 52523 | 【功能】issuer 临时冻结其签发给某 subject 的 VC（状态变为 Frozen，可解冻恢复——与 Revoke 的永久不可恢复区别）。<br>【限制】仅原 issuer（issuer 为签名者）；凭证须存在（1032）；已冻结再冻结报 VcAttestationAlreadyFrozen（1073）；已撤销的凭证不可冻结。 |
| identity | UnfreezeVcAttestation | unfreeze_vc_attestation | 10634 | 【功能】issuer 解除已冻结 VC 的冻结（状态恢复 Active）。<br>【限制】仅原 issuer（issuer 为签名者）；凭证须存在（1032）；未处于冻结状态报 VcAttestationNotFrozen（1074）。 |
| identity | Create | create | 28587 | 【功能】为地址创建 DID（去中心化身份）文档：主体类型（个人/组织）、密钥列表、服务端点、头像。identity 全部后续操作的前置。<br>【限制】同一 subject 只能创建一次（DidAlreadyExists 1024）；后续要注册组织必须以 subject_type="Organization" 创建（OrganizationDidRequired 1052）；密钥/服务数量与长度有上限校验（1040/1044/1046-1050）；avatar_uri 长度 1-512 字节（1045）。 |
| identity | CreateWithAlias | create_with_alias | 48723 | 【功能】创建 DID 并同时绑定全局唯一别名（等价 Create + SetAlias 一步完成）。<br>【限制】同 Create；别名须为「alias-数字」格式（NameInvalidFormat 1035）、各段长度校验（1035/1037/1038）、不可与已绑定名冲突（NameAlreadyBound 1028）。 |
| identity | AddKey | add_key | 24314 | 【功能】向 DID 文档添加一把新公钥。<br>【返回】u8 新密钥 id。<br>【限制】DID 须已创建且未停用（1025/1026）；密钥数量有上限（1040）；label 非空且不超长（1044）。 |
| identity | UpdateKey | update_key | 38726 | 【功能】替换 DID 文档中指定 id 的密钥（换公钥/改标签）。<br>【限制】密钥须存在（DidKeyNotFound 1040）。 |
| identity | RemoveKey | remove_key | 14295 | 【功能】从 DID 文档移除指定 id 的密钥。<br>【限制】不能移除最后一把密钥（CannotRemoveLastKey 1041）；密钥须存在（1041）。 |
| identity | AddService | add_service | 45574 | 【功能】为 DID 添加服务端点（官网/API 入口等）。<br>【返回】u8 新服务 id。<br>【限制】服务数量有上限（TooManyServices 1045）；endpoint 必须为合法绝对 URI 且 scheme 合法（1048-1050）。 |
| identity | UpdateService | update_service | 18018 | 【功能】更新指定 id 的服务条目。<br>【限制】服务须存在（ServiceNotFound 1050）；endpoint 校验同 AddService。 |
| identity | RemoveService | remove_service | 38683 | 【功能】移除指定 id 的服务条目。<br>【限制】服务须存在（1051）。 |
| identity | SetAvatarUri | set_avatar_uri | 7646 | 【功能】设置/更新 DID 头像 URI。<br>【限制】长度必须 1-512 字节（InvalidAvatarUri 1044）。 |
| identity | Deactivate | deactivate | 6825 | 【功能】停用该 DID（停用后 identity 的写操作均被拒绝）。<br>【限制】DID 须已创建（1025）；已停用再操作报 DidDeactivated（1026）。 |
| identity | SetAlias | set_alias | 44782 | 【功能】为已有 DID 设置/更换全局唯一别名。<br>【限制】别名不可被占用（NameAlreadyBound 1028）；格式与长度校验同 CreateWithAlias（1035-1039）。 |
| identity | RegisterOrganization | register_organization | 36868 | 【功能】为组织型 DID 注册能力声明：角色（VC 签发方 VcIssuer / KYC 服务方 KycProvider）与接受的凭证 schema 列表。组织 KYC 链路第二步（前置：subject_type=Organization 的 DID，见 ORG_KYC_GUIDE）。<br>【限制】必须先创建组织型 DID（OrganizationDidRequired 1052）；不可重复注册（1032）；声明任一角色必须至少带一个 credential_schema（IssuerCredentialSchemaRequired 1059）；角色/schema 数量与重复校验（1055-1059）。 |
| identity | UpdateOrganizationCapabilities | update_organization_capabilities | 13927 | 【功能】更新已注册组织的角色与凭证 schema 列表。<br>【限制】组织须已注册（OrganizationNotFound 1030）；已停用不可更新（1054）；roles/schemas 校验同 RegisterOrganization。 |
| identity | DeactivateOrganization | deactivate_organization | 48597 | 【功能】永久停用组织注册（不可恢复）。<br>【限制】组织须已注册（1031）；已停用再操作报 OrganizationDeactivated（1053）。 |
| sftoken | CreateSft | create_sft | 5334 | 【功能】创建 SFT（半同质化代币）集合：登记元数据（name/symbol 必填，cover_url/metadata 为 URI，attribute 可选）、初始 owner 与版税比例。SFT 为「集合 + 槽位(slot) + token 份额」三层模型的最上层。<br>【限制】由 sft 资源账户私钥签名（gas 由 owner 代付）；同一地址只能创建一次（SftAlreadyExists 1291）；name/symbol 等参数校验（InvalidParameters 1292）。 |
| sftoken | CreateSlot | create_slot | 56115 | 【功能】在 SFT 下创建槽位（slot）：登记槽位元数据覆盖项（SlotData.metadata，缺省继承集合）与份额可转移标志。slot_id 由链上递增分配。<br>【返回】u64 新 slot_id（也从回执 SlotCreatedEvent.slot_id 获取）。<br>【限制】仅 SFT owner 可创建（creator 签名）；SFT 须已存在（SftNotFound 1285）；id 耗尽报 SlotIdExhausted（1294）。 |
| sftoken | TransferOwner | transfer_owner | 35617 | 【功能】转让 SFT 集合 owner（集合管理权，也是 mint/update/freeze 三个权限的默认持有人；不含 token 份额）。<br>【限制】仅现任 owner（Unauthorized 1280）；SFT 须存在（1285）。 |
| sftoken | SetCoverUrl | set_cover_url | 480 | 【功能】设置/更新 SFT 集合的封面 URI。<br>【限制】仅 SFT owner（1280）；SFT 须存在（1285）。 |
| sftoken | SetUri | set_uri | 4819 | 【功能】设置/更新 SFT 集合的元数据 URI。<br>【限制】仅 SFT owner（1280）；SFT 须存在（1285）。 |
| sftoken | TransferMintAuthority | transfer_mint_authority | 33676 | 【功能】转让 SFT 的铸造权限人（mint authority，初始为 SFT owner）。<br>【限制】仅现任 mint authority（1280）；SFT 须存在（1285）。 |
| sftoken | TransferUpdateAuthor | transfer_update_author | 57787 | 【功能】转让 SFT 的内容更新权限人（update author，可改 slot/token 的元数据）。<br>【限制】仅现任 update author（1280）；SFT 须存在（1285）。 |
| sftoken | TransferFreezeAuthority | transfer_freeze_authority | 26719 | 【功能】转让 SFT 的冻结权限人（freeze authority，可冻结/解冻份额）。<br>【限制】仅现任 freeze authority（1280）；SFT 须存在（1285）。 |
| sftoken | TransferRoyaltyRecipient | transfer_royalty_recipient | 56414 | 【功能】修改 SFT 的版税接收人地址。<br>【限制】仅现任版税接收人（1280）。 |
| sftoken | SetSlotTransferable | set_slot_transferable | 57575 | 【功能】设置槽位 transferable 标志（开/关该槽位下份额的转移能力）。<br>【限制】仅 SFT owner（1280）；slot 须存在（SlotNotFound 1284）。 |
| sftoken | SetSlotAttribute | set_slot_attribute | 3272 | 【功能】更新槽位的 attribute 属性字符串（覆盖集合继承值）。<br>【限制】仅 update author（1280）；slot 须存在（1284）。 |
| sftoken | SetSlotCoverUrl | set_slot_cover_url | 15677 | 【功能】更新槽位的封面 URI。<br>【限制】仅 update author（1280）；slot 须存在（1284）。 |
| sftoken | SetSlotUri | set_slot_uri | 28914 | 【功能】更新槽位的元数据 URI。<br>【限制】仅 update author（1280）；slot 须存在（1284）。 |
| sftoken | Mint | mint | 43610 | 【功能】向目标地址铸造 amount 数量的份额，记入指定槽位下的一个新 token（metadata 可传覆盖项，缺省继承槽位/集合元数据）。token_id 由链上递增分配。<br>【返回】u64 新 token_id（也从回执 TokenMintedEvent.token_id 获取）。<br>【限制】仅 mint authority 签名；SFT/slot 须存在（1285/1284）；amount 必须 >0（ZeroValueOperation 1286）；id 耗尽报 TokenIdExhausted（1293）；溢出报 Overflow（1295）。 |
| sftoken | Burn | burn | 7015 | 【功能】销毁持有人在指定 token 上的 amount 数量份额（价值随之消失）。<br>【限制】由份额持有人签名；token 须存在（TokenNotFound 1283）；amount >0（1286）且不得超过持有量（InsufficientBalance 1282）。 |
| sftoken | Freeze | freeze | 36565 | 【功能】冻结某地址在指定 token 上的份额（冻结后该份额的转移/合并类操作被拒）。<br>【限制】仅 freeze authority；token 须存在（1283）；已冻结再冻结报 TokenFrozen（1290）。 |
| sftoken | Unfreeze | unfreeze | 49498 | 【功能】解除地址在指定 token 上份额的冻结。<br>【限制】仅 freeze authority；token 须存在（1283）；未处于冻结状态时操作失败。 |
| sftoken | SetTokenAttribute | set_token_attribute | 50607 | 【功能】更新 token 的 attribute 属性字符串（覆盖槽位/集合继承值）。<br>【限制】仅 update author（1280）；token 须存在（1283）。 |
| sftoken | SetTokenCoverUrl | set_token_cover_url | 58854 | 【功能】更新 token 的封面 URI。<br>【限制】仅 update author（1280）；token 须存在（1283）。 |
| sftoken | SetTokenUri | set_token_uri | 50621 | 【功能】更新 token 的元数据 URI。<br>【限制】仅 update author（1280）；token 须存在（1283）。 |
| sftoken | Approve | approve | 34407 | 【功能】份额持有人给 spender 授予其在指定 token 份额上的可花费额度（额度按 (token, owner, spender) 记账）。<br>【限制】仅份额持有人；token 须存在（1283）；amount >0（1286）。 |
| sftoken | RevokeApproval | revoke_approval | 57842 | 【功能】撤销 spender 对持有人在指定 token 份额上的授权额度。<br>【限制】仅份额持有人。 |
| sftoken | Split | split | 50774 | 【功能】把源 token 的部分份额拆给接收者并派生一个新 token（同槽位、继承元数据）。仅当业务需要独立 token_id 时使用——同一 token_id 多人持有本就是链上原生状态，一般转移用 Transfer 即可。<br>【返回】u64 新 token_id。<br>【限制】由源份额持有人签名；amount >0（1286）、必须小于源余额（FullBalanceSplit 1288）且不超过持有量（InsufficientBalance 1282）；接收者不得为本人（SelfTransfer 1287）。 |
| sftoken | Merge | merge | 110 | 【功能】把源 token 的全部份额并入目标 token 并删除源 token（同槽位份额合并）。<br>【限制】由源份额持有人签名；两个 token 必须属于同一槽位（SlotMismatch 1281）；不可自我合并（SelfMerge 1289）；token/slot 须存在（1283/1284）；被冻结份额报 TokenFrozen（1290）。 |
| sftoken | Transfer | transfer | 48437 | 【功能】转移份额：把持有人的 amount 数量记到接收方名下（token_id 不变，同一 token 可多人持有；全量转出后持有人余额记录清零）。<br>【限制】由份额持有人签名；amount >0（1286）且不超过持有量（InsufficientBalance 1282）；要求槽位 transferable=true；接收者不得为本人（SelfTransfer 1287）；被冻结份额报 TokenFrozen（1290）。 |
| sftoken | TransferFrom | transfer_from | 19098 | 【功能】被授权方（spender）动用 Approve 额度，把持有人 from 的份额转给接收者 to（额度按转移量扣减）。<br>【限制】spender 须有足额授权（InsufficientApproval 1296）；amount >0（1286）；其余同 Transfer：槽位可转移、不可自转（1287）、冻结报 1290。 |
| sftoken | BackfillTotalBalance | backfill_total_balance | 34242 | 【功能】为指定 owner 回填其名下该 SFT 全部 token 的余额聚合索引（供 TotalBalanceOf 查询），链升级/迁移后的一次性补偿操作。<br>【限制】sft 须已创建（SftNotFound 1285）。 |
| sftoken | BackfillTotalSupply | backfill_total_supply | 63205 | 【功能】回填该 SFT 全链发行总量聚合（供 TotalSupply 查询），链升级/迁移后的一次性补偿操作。<br>【限制】sft 须已创建（1285）。 |
| dex | CreateAmmPool | create_amm_pool | 60506 | 【功能】为已有市场创建 AMM 做市池并注入首笔流动性（恒定乘积池，与订单簿并行，供 SwapExactIn 混合路由）；authority 为市场管理员签名，provider 为流动性提供者签名。<br>【返回】AmmPoolInfo：{market_id, vault, vault_seed, status, reserve_base, reserve_quote, total_lp_supply}。<br>【限制】市场须存在（MarketNotFound 1542）；同市场不可重复建池（AmmPoolAlreadyExists 1559）；数量参数须非零（ZeroLiquidityParameter 1564）；资金转入失败报 TokenFailure 1555。 |
| dex | InitializeAmmPool | initialize_amm_pool | 37332 | 【功能】初始化市场的 AMM 池并注入流动性（参数与 CreateAmmPool 一致的显式初始化路径）。<br>【返回】AmmPoolInfo。<br>【限制】池须存在（AmmPoolNotFound 1560）；数量参数须非零（ZeroLiquidityParameter 1564）；资金转入失败报 TokenFailure 1555。 |
| dex | SetAmmStatus | set_amm_status | 20736 | 【功能】切换 AMM 池状态：Active（启用做市）/ Stopped（停止做市，停池后仍可撤流动性）。<br>【限制】须市场 authority 签名（MarketAuthorityMismatch 1546）；池须存在（AmmPoolNotFound 1560）；状态机校验（AmmPoolNotActive 1561 / AmmPoolNotStopped 1562）。 |
| dex | AddLiquidity | add_liquidity | 50155 | 【功能】向 AMM 池追加双边流动性：按当前池比例实际转入（不超过 base_max/quote_max），铸造 LP 份额。<br>【返回】AddLiquidityResult：{base_amount, quote_amount, lp_shares}（实际转入量与铸得份额）。<br>【限制】池须为 Active（AmmPoolNotActive 1561）；参数须非零（ZeroLiquidityParameter 1564）；铸得份额为零报 ZeroLpShares 1566、低于 min_lp_shares 报 MinLpSharesNotMet 1567；溢出报 AmmLiquidityOverflow 1565。 |
| dex | RemoveLiquidity | remove_liquidity | 15914 | 【功能】销毁 LP 份额，按池比例赎回 base/quote 双边资产。<br>【返回】RemoveLiquidityResult：{base_amount, quote_amount, lp_shares}。<br>【限制】LP 余额不足报 InsufficientLpShares 1568；实际赎回低于 min_base_out/min_quote_out 报 MinLiquidityOutputNotMet 1569。 |
| dex | SwapExactIn | swap_exact_in | 34800 | 【功能】混合换币（精确输入）：先吃订单簿（至多 max_book_makers 档成交），余量走 AMM 分段成交（至多 max_amm_segments 段），输入 input_token 的 amount_in，带 min_amount_out / worst_tick 保护。<br>【返回】HybridSwapResult：{amount_in_used, amount_out, refund, clob_fills, amm_segments}。<br>【限制】input_token 须属于该市场（InvalidSwapInputToken 1571）；保护/限额参数不合法报 InvalidSwapParameter 1570；输出低于 min_amount_out 报 MinSwapOutputNotMet 1573；算术溢出报 SwapOverflow 1572。 |
| dex | CreateMarket | create_market | 122 | 【功能】创建订单簿交易市场（base/quote 代币对），调用者成为市场 authority（管理员）。maker/taker 费率由链上常量固定为 0.3%/0.5%。<br>【返回】Address 新市场地址（由链上种子派生）。<br>【限制】base≠quote（SameTokenPair 1536）；所有数值参数非 0（ZeroMarketParameter 1537）；同代币对不可重复建市场（MarketAlreadyExists 1541）。 |
| dex | InitializeMarketDid | initialize_market_did | 27598 | 【功能】为市场初始化 DID 文档（市场身份/资质展示）。<br>【限制】须市场 authority 签名（MarketAuthorityMismatch 1546）；市场须存在（MarketNotFound 1542）；subject_type 必须为 Organization（MarketDidMustBeOrganization 1558）。 |
| dex | DiscloseMarketVcAttestation | disclose_market_vc_attestation | 47697 | 【功能】为市场披露一条 VC 凭证：登记签发方、schema、凭证哈希与签发方签名，供合规展示。<br>【限制】须市场 authority 签名；签发方身份须可解析（IdentityFailure 1557）。 |
| dex | PlaceLimitOrder | place_limit_order | 24195 | 【功能】下限价单：先尝试吃单成交，未成交部分按限价挂入订单簿并锁定资金。<br>【返回】PlaceOrderResult：{order_id, status, filled_lots, remaining_lots, fill_count, rested（是否纯挂单）}。<br>【限制】市场须为 Active 状态（MarketNotActive 1543）；price_tick 须在 [min_tick, max_tick] 内；资金不足报 TokenFailure（1555）。 |
| dex | CancelOrder | cancel_order | 54560 | 【功能】按链上 order_id 撤单并释放锁定资金。<br>【返回】OrderInfo：{order_id, owner, client_order_id, side, limit_tick, original_lots, remaining_lots, reserved_amount, status}。<br>【限制】订单须存在（OrderNotFound 1547）；仅订单本人可撤。 |
| dex | CancelByClientOrderId | cancel_by_client_order_id | 16334 | 【功能】按下单者自己的 client_order_id 撤单（无需记链上单号）。<br>【返回】OrderInfo。<br>【限制】该 owner 下须存在此 client 单号（ClientOrderNotFound 1548）。 |
| dex | BatchCancel | batch_cancel | 55242 | 【功能】批量撤单并释放锁定资金。<br>【返回】OrderInfo JSON 数组。<br>【限制】列表非空（EmptyBatchCancel 1549）且最多 32 个（BatchCancelLimitExceeded 1550）；不可重复（DuplicateBatchOrder 1551）。 |
| dex | SetMarketStatus | set_market_status | 48095 | 【功能】切换市场状态：Active（正常交易）/ CancelOnly（只可撤单）/ Paused（暂停）。<br>【限制】仅市场 authority 生效（1546）。 |
| validator | RecordValidatorActivity | record_validator_activity | 23268 | 【功能】记录当前验证人的出块活动（EpochActivity 的 blocks/signers/last_block 计数来源，通常由链内共识通道调用）。<br>【限制】须验证人集合就绪（1801）。 |
| validator | BootstrapValidatorRules | bootstrap_validator_rules | 54067 | 【功能】引导期一次性设置验证人规则：最低自押门槛（minimum_self_stake）。<br>【限制】仅链引导阶段调用，重复引导失败。 |
| validator | FreezeValidatorEpoch | freeze_validator_epoch | 32944 | 【功能】冻结当前 epoch：锁定验证人集合、最低自押与每验证人奖励额度等参数为 FrozenEpoch 快照（previous_set_hash/freeze_block/freeze_slot/start_slot），是 SettleValidatorEpoch 的前置。<br>【限制】须验证人集合就绪（ValidatorSetUnavailable 1801）；重复冻结报参数错误。 |
| validator | JoinValidator | join_validator | 9901 | 【功能】验证人 owner 申请加入验证人集合（记录 Membership，join_epoch 为生效纪元，随 epoch 冻结进入集合）。<br>【限制】ValidatorId 须已注册（NotRegistered 1793）；签名者须为 owner（OwnerMismatch 1794）。 |
| validator | ExitValidator | exit_validator | 47485 | 【功能】验证人 owner 申请退出验证人集合（Membership.exit_epoch 置位，随目标 epoch 结算退出）。<br>【限制】ValidatorId 须已注册（1793）；签名者须为 owner（1794）；未在集合中报参数错误（1796）。 |
| validator | TopUpSelfStake | top_up_self_stake | 39689 | 【功能】验证人 owner 为自己追加自押金额（满足/维持最低自押门槛 minimum_self_stake）。<br>【限制】签名者须为 owner（OwnerMismatch 1794）；金额须大于 0（1796）。 |
| validator | RequestPoolRewardConfig | request_pool_reward_config | 17593 | 【功能】验证人 owner 申请变更佣金配置（费率/收款地址，PoolRewardConfig）：新配置记为待生效，过佣金变更延迟后自动启用。<br>【限制】签名者须为 owner（OwnerMismatch 1794）；配置须合法（InvalidParameters 1796）；存在待生效配置时再次申请将覆盖。 |
| validator | ClaimPoolCommission | claim_pool_commission | 50141 | 【功能】验证人 owner 领取验证人池累计佣金（按佣金费率从注入奖励中切分）。<br>【返回】u64 实际领取数量（最小单位）；无可领返回 0。<br>【限制】签名者须为 owner（1794）；金库账实不一致报 RewardTreasuryBackingMismatch（1797）。 |
| validator | SweepPoolResidual | sweep_pool_residual | 61791 | 【功能】验证人 owner 清理退出/关停后池内残留资金，转入 owner 账户。<br>【返回】u64 实际清扫金额（最小单位）；无可清扫返回 0。<br>【限制】签名者须为 owner（1794）。 |
| validator | FundValidatorRewards | fund_validator_rewards | 27609 | 【功能】向验证人奖励预算注资（全局 ValidatorRewardBudget），作为各 epoch 验证人奖励的发放来源（任何持币人可资助，替代旧 FundValidatorRewardPolicy）。<br>【限制】余额不足透传 Token（1798）；金额须大于 0（1796）。 |
| validator | RegisterValidator | register_validator | 51820 | 【功能】注册验证人：owner 与 consensus_account 双签名，登记共识/BLS/ed25519 三把公钥并缴纳初始自押（self_stake），返回验证人 ID（直连模型入口，注册后即可被引导进验证人集合）。<br>【返回】ValidatorId（B256，base58）。<br>【限制】不可重复注册（AlreadyRegistered 1792）；签名者须为该验证人 owner（OwnerMismatch 1794）；共识身份未就绪报 ConsensusIdentityMissing（1795）；参数非法报 InvalidParameters（1796）；公钥长度按曲线校验（BLS 48 字节 / ed25519 32 字节）。 |
| validator | SettleValidatorEpoch | settle_validator_epoch | 15316 | 【功能】结算当前 epoch：按冻结快照结算出块活动并发放验证人奖励（自押利息/奖励预算切分）。<br>【限制】须先成功 FreezeValidatorEpoch；同一 epoch 不可重复结算；须验证人集合就绪（1801）。 |
| keyless | BootstrapKeyless | bootstrap_keyless | 7470 | 【功能】初始化 keyless 注册表：注册内置 Google/Apple OIDC provider 与默认参数（无参数，args 传 {}）。<br>【限制】仅 genesis 块 0 可调（GenesisOnly 2113）；只能初始化一次（AlreadyBootstrapped 2114）。 |
| keyless | InstallZkCircuit | install_zk_circuit | 18924 | 【功能】注册 Groth16 ZK 验证电路：验证密钥、约束哈希、公共输入数上限、支持的 JWT 算法集，并可同时开放 ZK 无密钥认证。<br>【限制】电路 ID 非法报 ZkCircuitIdInvalid（2122）；重复注册报 ZkCircuitAlreadyRegistered（2123）；电路参数非法报 ZkCircuitInvalid（2124）。 |
| keyless | UpdateJwkReport | update_jwk_report | 50524 | 【功能】验证人上报某 provider 的 JWKS 快照（动态 JWK 共识：多验证人报告一致后被选中；交易须验证人签名授权，2026-11-20 起不再携带 validator_account 参数）。<br>【限制】验证人集合不可用报 ValidatorSetUnavailable（2131）；JWKS 格式非法报 DynamicJwkSetInvalid（2132）；thumbprint 非规范报 JwkThumbprintNonCanonical（2134）。 |
| keyless | PruneInactiveJwkReports | prune_inactive_jwk_reports | 17484 | 【功能】清理不再活跃的验证人留下的 JWK 报告（无参数维护指令）。<br>【限制】仅系统/维护流程调用。 |
| keyless | Authenticate | authenticate | 21310 | 【功能】用 Groth16 ZK 证明完成无密钥认证，建立链上会话（后续以会话公钥签名操作，gas 可由赞助池代付）：auth={circuit_id, claim, proof}，claim 承载 JWT 语义承诺（provider_id/issuer/kid/jwk_thumbprint/audience_hash/address_seed/ephemeral_pk/iat/exp/session_expiry_ms 等），链上仅验证证明与 claim 一致性。<br>【限制】ZK 认证未开放报 ZkAuthenticationDisabled（2125）；证明无效报 ZkProofInvalid（2126）；circuit_id 须已注册（ZkCircuitIdInvalid 2122）；会话公钥非法报 InvalidSessionPubkey（2089）；session_expiry 越界报 2087；公共赞助每会话仅一次（SessionSponsorshipAlreadyUsed 2091）。 |
| keyless | Bind | bind | 59392 | 【功能】把 keyless 身份（经 Authenticate 认证后派生的地址）绑定到链上账户的一个 signer 槽位，使账户可无私钥托管；auth 为 Groth16 认证材料（结构同 Authenticate）。<br>【限制】该身份不可已被绑定（AlreadyBound 2099）；绑定有激活延迟（默认 1 天、下限 1 小时，active_at_ms = bound_at_ms + 延迟）；ZK 校验同 Authenticate；owner 须签名。 |
| keyless | Unbind | unbind | 57503 | 【功能】解除 keyless 身份与账户的绑定。<br>【限制】绑定须存在（BindingNotFound 2100）；仅绑定 owner 可解绑（BindingOwnerMismatch 2101）。 |
| keyless | RevokeSession | revoke_session | 15722 | 【功能】撤销一个活跃会话（会话公钥立即失效）。<br>【限制】会话须存在且属该 owner（SessionNotFound 2098）；已撤销报 SessionRevoked（2090）。 |
| keyless | RegisterAudience | register_audience | 44901 | 【功能】为 provider 注册允许的 audience（OIDC client ID），之后该 audience 的 JWT 才能通过校验。<br>【限制】provider 须已注册（ProviderNotRegistered 2092）；audience 长度 1..=256 字节（AudienceInvalid 2118）；内置 provider 元数据不可修改（BuiltinProviderImmutable 2127）。 |
| keyless | CreateStaticProvider | create_static_provider | 19418 | 【功能】注册自定义「静态 JWK」验证 provider（自带公钥集，不依赖内置 OIDC）。provider_id 由 (issuer, salt) 派生。<br>【限制】issuer 必须 https:// 开头（IssuerUrlInvalid 2095）；名称非空且 ≤64 字节（2096）；密钥 1..=8 把（StaticKeyCountInvalid 2129）；kid 唯一且 1..=128 字节（2130）；不可用保留命名空间（ReservedProviderId 2128）；重复注册报 ProviderAlreadyRegistered（2093）。 |
| keyless | UpdateStaticProviderKeys | update_static_provider_keys | 19292 | 【功能】轮换自定义 provider 的验证密钥集；invalidate_sessions=true 可使全部旧会话立即失效。<br>【限制】仅静态 provider 可改（BuiltinProviderImmutable 2127）；仅 provider owner（ProviderOwnerMismatch 2121）。 |
| lucky_box | CreateEqual | create_equal | 51385 | 【功能】创建「等概率」盲盒：total_amount 平分为 claim_count 份，每份金额相同；资金从创建者账户托管进盒子资产池。<br>【返回】u64（每份金额）。<br>【限制】box_id 不可重复（BoxAlreadyExists 2305）；金额/份数须非 0（InvalidDefinition 2310）；份数 ≤10000（TooManyItems 2311）；创建者须有足够代币托管；资格 bloom bits 32..=32768 字节；未设过期时间默认 24 小时（DEFAULT_EXPIRATION_MS）。 |
| lucky_box | CreateLucky | create_lucky | 12885 | 【功能】创建「拼手气」盲盒：领取时按链上安全随机数分配不等金额；参数与资金托管同 CreateEqual。<br>【返回】u64。<br>【限制】同 CreateEqual；随机数不可用报 SecureRandomnessUnavailable（2314）。 |
| lucky_box | Claim | claim | 62102 | 【功能】领取盲盒一份：Equal 得等份金额、Lucky 得随机金额，代币从托管转入领取者。gas 由赞助池代付（sponsor）。<br>【限制】盒子须存在且活跃（BoxNotFound 2304 / BoxNotActive 2306）、未过期（BoxExpired 2307）、尚有余量（BoxExhausted 2309）；须通过资格 bloom 校验（NotEligible 2312）；每人限领一次（QuotaExceeded 2313）。 |
| lucky_box | Refund | refund | 9120 | 【功能】盒子过期后回收剩余资产（未领完的资金退回创建者）。无需签名者，任何人可触发执行。<br>【限制】盒子必须已过期（BoxNotExpired 2308）且存在（2304）。 |
| social | Follow | follow | 1031 | 【功能】关注目标地址（建立 _Follow 社交关系记录）。<br>【限制】不可自关注（CannotFollowSelf 2599）；已关注报 AlreadyFollowing（2600）。 |
| social | Unfollow | unfollow | 62356 | 【功能】取消对目标地址的关注。<br>【限制】未关注报 NotFollowing（2601）。 |
| social | CreateFollowSponsor | create_follow_sponsor | 12987 | 【功能】创建关注赞助池：owner 注资 amount，为「关注 owner」的行为提供奖励；返回赞助池地址，他人经 FollowSponsored 关注 owner 时领取赞助。<br>【返回】Address（赞助池地址）。<br>【限制】amount 须大于 0（InvalidFollowSponsorAmount 2602）。 |
| social | CloseFollowSponsor | close_follow_sponsor | 63283 | 【功能】关闭关注赞助池并取回剩余资金。<br>【返回】u64 取回金额（最小单位）。<br>【限制】仅赞助池 owner（FollowSponsorOwnerRequired 2606）；赞助池须存在（FollowSponsorNotFound 2604）。 |
| social | FollowSponsored | follow_sponsored | 14127 | 【功能】经赞助池完成关注：follower 关注 target 并领取 sponsor 池的赞助奖励（同一 (follower,target) 对仅可领取一次）。<br>【限制】赞助池须存在（2604）且处于开启状态（FollowSponsorClosed 2607）；不可自关注（2599）；已关注报 2600；该 pair 已领过赞助报 FollowSponsorshipAlreadyUsed（2605）。 |
| social | UpsertSocialProfile | upsert_social_profile | 13067 | 【功能】写入/更新自己的社交 profile URI（链上只存 URI，内容存链下）。<br>【限制】须有有效未停用 DID（DidNotActive 2560）；URI 须合法（InvalidUri 2563）。 |
| social | RegisterSourceApp | register_source_app | 23225 | 【功能】注册「源应用」（公共数据发布主体），链上分配自增 ID。<br>【返回】u32 source_app_id。<br>【限制】owner 须有有效 DID；ID 序列耗尽报 SourceAppIdExhausted（2572）。 |
| social | AddSourceAppPublisher | add_source_app_publisher | 55820 | 【功能】为源应用添加发布者（允许其发布公共数据）。<br>【限制】仅应用 owner（SourceAppOwnerRequired 2567）；应用须存在（SourceAppNotFound 2564）；已是发布者报 2569。 |
| social | RemoveSourceAppPublisher | remove_source_app_publisher | 62309 | 【功能】移除源应用的某个发布者。<br>【限制】仅 owner；owner 自身的发布者身份不可移除（CannotRemoveOwnerPublisher 2571）；目标须是发布者（2570）。 |
| social | TransferSourceAppOwner | transfer_source_app_owner | 21283 | 【功能】转让源应用所有权。<br>【限制】仅当前 owner（2567）。 |
| social | DeactivateSourceApp | deactivate_source_app | 52796 | 【功能】停用源应用（暂停其数据发布；停用后发布报 SourceAppPublishingDisabled 2566）。<br>【限制】仅 owner。 |
| social | ReactivateSourceApp | reactivate_source_app | 55074 | 【功能】重新启用已停用的源应用。<br>【限制】仅 owner。 |
| social | UpsertReference | upsert_reference | 32627 | 【功能】发布者为跨应用对象引用写入/更新记录：(source_app_id, object_type, object_id) 定位对象，绑定 subject 地址与内容哈希（B256）、数据 URI。取代旧 PublicDataUri 系列的数据锚点能力。<br>【限制】调用者须是该应用发布者（SourceAppPublisherRequired 2568）；应用须存在且未停用发布（2564/2566）；object_id 须为合法稳定对象 ID（InvalidObjectId 2573）。 |
| social | RetireReference | retire_reference | 41015 | 【功能】将对象引用记录下线（retired 标记，记录保留）。<br>【限制】仅发布者；引用须存在（ReferenceNotFound 2574）；已下线再操作报 ReferenceRetired（2575）；修订号耗尽报 2577。 |
| social | CreateCommunity | create_community | 18868 | 【功能】创建社区，链上分配自增 community_id。<br>【返回】u32 community_id。<br>【限制】每 creator 最多 8 个社区（CommunityCreationLimitReached 2592）；URI 须合法（2563）。 |
| social | UpdateCommunityUri | update_community_uri | 63564 | 【功能】社区 owner 更新社区元数据 URI。<br>【限制】仅 owner（CommunityOwnerRequired 2581）；社区须存在（2578）且未停用（2580）。 |
| social | SetCommunityJoinMode | set_community_join_mode | 20649 | 【功能】owner 切换社区加入模式（Open/Approval）。<br>【限制】仅 owner；社区须存在且未停用。 |
| social | TransferCommunityOwner | transfer_community_owner | 5543 | 【功能】转让社区所有权。<br>【限制】仅当前 owner。 |
| social | DeactivateCommunity | deactivate_community | 32318 | 【功能】停用社区（成员与管理变更冻结）。<br>【限制】仅 owner。 |
| social | DissolveCommunity | dissolve_community | 12783 | 【功能】解散社区：解散后社区红包领取与成员变更被阻断（CommunityDissolved 2598）。<br>【限制】签名者须为社区 owner（CommunityOwnerRequired 2581）；社区须存在（2578）。 |
| social | AddCommunityMember | add_community_member | 15042 | 【功能】管理员向社区添加成员。<br>【限制】签名者须为社区管理员（CommunityAdminRequired 2582）；社区须存在（2578）且未解散（2598）；已是活跃成员报 MembershipAlreadyActive（2587）；成员数上限报 CommunityMemberLimitReached（2593）。 |
| social | GrantCommunityAdmin | grant_community_admin | 10690 | 【功能】owner 授予某地址社区管理员权限。<br>【限制】仅 owner；已是管理员报 CommunityAdminAlreadyExists（2584）。 |
| social | RevokeCommunityAdmin | revoke_community_admin | 62146 | 【功能】owner 撤销某地址的社区管理员权限。<br>【限制】仅 owner；目标须是管理员（CommunityAdminNotFound 2585）。 |
| social | JoinCommunity | join_community | 39196 | 【功能】直接加入社区（仅 Open 模式；Approval 模式须走申请审批）。<br>【限制】模式须为 Open（InvalidJoinMode 2586）；成员数不超 10000（CommunityMemberLimitReached 2593）；不可重复加入（MembershipAlreadyActive 2587）。 |
| social | RequestJoinCommunity | request_join_community | 57790 | 【功能】在 Approval 模式社区提交加入申请。<br>【限制】模式须为 Approval（2586）；不可重复申请（JoinRequestAlreadyPending 2589）；不可已是成员（2587）。 |
| social | CancelJoinRequest | cancel_join_request | 33199 | 【功能】撤回自己的加入申请。<br>【限制】须有 pending 申请（JoinRequestNotPending 2590）。 |
| social | ApproveJoinRequest | approve_join_request | 2182 | 【功能】管理员批准某用户的加入申请（成员数 +1）。<br>【限制】须管理员或 owner（CommunityAdminRequired 2582）；须有 pending 申请（2590）；成员数不超 10000（2594）。 |
| social | RejectJoinRequest | reject_join_request | 21924 | 【功能】管理员拒绝某用户的加入申请。<br>【限制】同 ApproveJoinRequest：管理员权限 + pending 申请存在。 |
| social | LeaveCommunity | leave_community | 7711 | 【功能】成员主动退出社区。<br>【限制】须是活跃成员（MembershipNotActive 2588）；owner 不能退出（CannotLeaveCommunityOwner 2597，须先转让）。 |
| social | RemoveCommunityMember | remove_community_member | 7071 | 【功能】管理员移除某成员。<br>【限制】须管理员（2582）；目标须是成员（2588）；不能移除 owner（CannotRemoveCommunityOwner 2583）。 |
| social | CreateHongbao | create_hongbao | 7621 | 【功能】创建社区红包：创建者向社区（community_id）发放 token 红包，total_amount 供 claim_count 人按 allocation（Equal 均分 / Lucky 拼手气）领取，可设过期时间与附言 message。<br>【限制】创建者须有有效 DID（DidNotActive 2560）；社区须存在（CommunityNotFound 2578）；社区已解散报 CommunityDissolved（2598）；托管转入失败透传 token 层错误（TokenError 2595）。 |
| social | ClaimHongbao | claim_hongbao | 40045 | 【功能】领取社区红包（claimant 签名）。<br>【限制】红包须存在（CommunityHongbaoNotFound 2596）；社区已解散报 2598；领取规则校验同 token 红包语义。 |
| demo | SetScore | set_score | 9531 | 【功能】由池管理侧直接设置某账户在池中的积分值（管理类指令，区别于 Draw 等玩法产出）。<br>【限制】池须存在（PoolNotFound 65024）；参数非法报 InvalidParameters（65027）。 |
| demo | OpenOrder | open_order | 47185 | 【功能】演示订单支付流第一步：operator 开一个演示订单（收款托管户头）。<br>【限制】order_id 不可重复（OrderAlreadyExists 65029）。 |
| demo | RejectOrderInbound | reject_order_inbound | 33284 | 【功能】演示订单拒收入账：operator 主动拒绝订单托管户头收到的最新一笔入账转账。<br>【限制】订单须存在（OrderNotFound 65028）；须开单 operator 本人（OrderOperatorMismatch 65030）；无可拒入账或状态不符报 InboundRejected 65032。 |
| demo | ForwardOrderInbound | forward_order_inbound | 12070 | 【功能】演示订单转发入账：operator 把订单最新一笔入账转发出去（转发目标由 forward_seed 参与派生）。<br>【限制】订单须存在（65028）；须开单 operator 本人（65030）；转发被拒报 InboundRejected 65032。 |
| demo | SplitOrderInbound | split_order_inbound | 2755 | 【功能】演示订单拆分入账：operator 把订单最新一笔入账按 first_bps 基点拆给 first，剩余部分给 second。<br>【限制】订单须存在（65028）；须开单 operator 本人（65030）；拆分被拒报 InboundRejected 65032。 |
| demo | PayOrder | pay_order | 49571 | 【功能】演示订单支付流第二步：付款人向订单打入指定代币与数量（进订单托管）。<br>【限制】订单须存在（OrderNotFound 65028）；付款人余额须充足。 |
| demo | SettleOrder | settle_order | 62622 | 【功能】演示订单支付流第三步：operator 把订单托管余额结算转出给指定收款地址。<br>【限制】须与开单相同的 operator（OrderOperatorMismatch 65030）；订单须存在（65028）；结算额不得超过托管余额。 |
| demo | InitPool | init_pool | 51459 | 【功能】初始化一个演示池（标签/积分载体），由 pool 资源账户签名。<br>【限制】不可重复初始化（PoolAlreadyExists 65025）。 |
| demo | InitDex | init_dex | 50094 | 【功能】同 InitPool，以 dex 为主题的演示初始化。<br>【限制】不可重复初始化（65281）。 |
| demo | SetLabel | set_label | 47391 | 【功能】修改池的标签。<br>【限制】池须存在（PoolNotFound 65024）。 |
| demo | BatchCredit | batch_credit | 60858 | 【功能】给一批接收者在池内加积分（触发 EventCreditApplied 事件）。<br>【限制】池须存在（65280）；接收者须已在池内登记（RecipientNotFound 65026）。 |
| demo | SetTierCap | set_tier_cap | 55600 | 【功能】设置某层级的积分上限（配合领用限速演示）。<br>【限制】池须存在（65280）。 |
| demo | DrawU64 | draw_u_6_4 | 1414 | 【功能】演示安全随机数：从链上安全随机源抽取一个 u64。<br>【返回】u64。<br>【限制】随机源不可用报 RandomnessUnavailable 65034。 |
| demo | DrawBytes32 | draw_bytes_3_2 | 52945 | 【功能】演示安全随机数：从链上安全随机源抽取 32 字节。<br>【返回】B256。<br>【限制】随机源不可用报 RandomnessUnavailable 65034。 |
| demo | DrawRange | draw_range | 31302 | 【功能】演示安全随机数：从链上安全随机源抽取 [lo, hi] 区间内的一个 u64。<br>【返回】u64。<br>【限制】随机源不可用报 RandomnessUnavailable 65034。 |
| demo | OpenGasSponsorPool | open_gas_sponsor_pool | 49440 | 【功能】注册 gas 赞助池，使 sponsor 标记的指令（keyless.Authenticate、lucky_box.Claim、demo.ClaimSponsoredScore 等）可由该池代付 gas。<br>【返回】u16 sponsor_seed（池编号，供赞助类指令引用）。<br>【限制】pool 地址的管理员（signer_lookup 从 pool 参数解析 admin）必须与交易付款人一致。 |
| demo | ClaimSponsoredScore | claim_sponsored_score | 1595 | 【功能】演示从赞助池领取 gas 赞助并给领取者记积分。gas 由赞助池代付（sponsor）。<br>【限制】池须已注册（SponsorPoolNotFound 65031）。 |
| demo | ScheduleWake | schedule_wake | 8376 | 【功能】演示定时唤醒：登记一个 due_ms 毫秒后到期的定时器，到期后由 RunScheduledWake 执行演示回调。<br>【限制】登记被拒报 WakeRejected 65033。 |
| demo | RunScheduledWake | run_scheduled_wake | 9624 | 【功能】执行已到期的定时唤醒（演示链上定时回调的执行半步，任何人可触发）。<br>【限制】无到期项或状态不符报 WakeRejected 65033。 |
