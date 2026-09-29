# Milon IDL 函数清单

> 导出自 `gosdk-develop/provider/IDL/`，共 **11** 个 app、**245** 个函数（view 94 + entry 151）。

> 表格列：`appname`(应用名) · `meth`(方法名/指令名) · `handler`(链上入口名) · `id`(discriminator 指令编号) · `说明`(中文释义，来自 handler/idl_method_docs.go 的文档映射，与 /api/idl/metadata 输出一致)


## View（只读查询，kind=view）

共 **94** 个。

| appname | meth | handler | id | 说明 |
|---------|------|---------|-----|------|
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
| staking | ValidatorProfile | validator_profile | 43284 | 【功能】查询验证人基本档案。<br>【返回】ValidatorProfile：{validator, operator, status: Inactive\|Candidate\|Active\|Leaving, commission_rate_bps}；不存在报 768。 |
| staking | ValidatorPool | validator_pool | 51337 | 【功能】查询验证人池记账快照（自有/受托质押、奖励累计等）。<br>【返回】ValidatorPoolView：{operator_stake, delegated_stake, total_stake, total_shares, reward_index, claimable_operator_commission, commission_rate_bps}（金额均为最小单位）。 |
| staking | StakePosition | stake_position | 24660 | 【功能】查询某用户在某验证人处的质押仓位原始数据。<br>【返回】StakePosition：{shares, reward_debt, claimable_rewards}；无仓位报 783。 |
| staking | PositionSummary | position_summary | 24212 | 【功能】查询质押仓位汇总（面向前端的聚合视图）。<br>【返回】PositionSummary：{pending_stake, settled_shares, pending_unstake_shares, unstakeable_shares, claimable_rewards, next_settlement_epoch}。 |
| staking | CandidatePool | candidate_pool | 57732 | 【功能】查询当前候选池中的全部验证人地址（无参数）。<br>【返回】CandidatePool：{validators: [地址数组]}。 |
| staking | EpochTransition | epoch_transition | 24093 | 【功能】查询某次 epoch 结算的执行明细。<br>【返回】EpochTransition：{reward_per_validator, distributed_rewards, rewarded_validator_count, settled_stake_intent_count, settled_unstake_intent_count, applied_candidate_intent_count, candidate_rejections}；未发生过报 EpochTransitionNotFound（798）。 |
| staking | EpochConfig | epoch_config | 41632 | 【功能】查询 epoch 配置（无参数）。<br>【返回】EpochConfig：{reward_per_validator}；未初始化报 EpochConfigNotSet（780）。 |
| staking | EpochState | epoch_state | 29979 | 【功能】查询 epoch 结算进度（无参数）。<br>【返回】EpochState：{last_settled_epoch}。 |
| staking | RewardTreasury | reward_treasury | 46445 | 【功能】查询奖励金库状态（无参数）。<br>【返回】RewardTreasury：{address, balance, available_rewards, reserved_rewards}（最小单位）。 |
| staking | HeldPrincipal | held_principal | 45674 | 【功能】查询某地址被质押模块托管的本金总额（含 pending 与待释放部分）。<br>【返回】u64（最小单位）。 |
| staking | ListDeclaredValidatorsForEpoch | list_declared_validators_for_epoch | 13153 | 【功能】查询某 epoch 已声明可用性的验证人列表。<br>【返回】地址 JSON 数组。 |
| identity | VcAttestationCore | vc_attestation_core | 18681 | 【功能】查询某 (subject, schema, issuer) 组合下 VC 凭证的不可变核心内容。<br>【返回】VcAttestationCore：{credential_schema, credential_hash, valid_until_ms}；不存在报 1033。 |
| identity | VcAttestationLifecycle | vc_attestation_lifecycle | 41850 | 【功能】查询 VC 的生命周期状态（是否被撤销/冻结、最近变更时间）。<br>【返回】VcAttestationLifecycle：{status: "Active"\|"Revoked"\|"Frozen", updated_at_ms}。 |
| identity | AcceptedVcIssuerIndexMeta | accepted_vc_issuer_index_meta | 57307 | 【功能】查询 subject 某 schema 下「已接受 VC 签发方索引」的元信息。<br>【返回】String；索引不存在报 VcAcceptanceIndexNotFound（1068）。 |
| identity | AcceptedVcIssuers | accepted_vc_issuers | 6419 | 【功能】列出 subject 某 schema 下已接受凭证的签发方及凭证哈希。<br>【返回】[{issuer, credential_hash}]；数量有上限（TooManyAcceptedVcIssuers 1065）。 |
| identity | DisclosedVcSchemas | disclosed_vc_schemas | 48906 | 【功能】列出 subject 已披露 VC 的全部 schema。<br>【返回】字符串 JSON 数组；种类数有上限（TooManyDisclosedVcSchemas 1066）。 |
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
| dex | MarketInfo | market_info | 2326 | 【功能】查询市场完整配置。<br>【返回】Market：{base_token, quote_token, base_lot_atoms, quote_atoms_per_lot_tick, min_tick, max_tick, max_order_lots, max_fills_per_action, maker_fee_ppm, taker_fee_ppm, authority, status}。 |
| dex | OrderInfoView | order_info_view | 23456 | 【功能】按 order_id 查询订单实时状态（含锁定量 reserved_amount）。<br>【返回】OrderInfo。 |
| dex | BestBidAsk | best_bid_ask | 12137 | 【功能】查询订单簿最优买价/卖价档。<br>【返回】{bid: {tick, order_count, total_base_lots} 或 null, ask: 同}；空簿一侧为 null。 |
| dex | OrderbookDepth | orderbook_depth | 46884 | 【功能】查询买卖各价位档的深度。<br>【返回】{bids: [{tick, order_count, total_base_lots}], asks: [...]}。<br>【限制】depth 上限 50（InvalidViewLimit 1552）。 |
| dex | OrdersAtLevel | orders_at_level | 24461 | 【功能】分页查询某一价格档上的订单列表。<br>【返回】LevelOrderPage：{orders: [OrderInfo], next_order_id（翻页游标，0 表示没有更多）}。<br>【限制】limit 1..=100（1552）；档位不存在报 PriceLevelNotFound（1553）；游标非法报 InvalidOrderCursor（1554）。 |
| dex | VaultLiability | vault_liability | 6759 | 【功能】查询市场金库对某代币的负债总额（所有用户托管资金之和）。<br>【返回】u64。 |
| keyless | GetSessions | get_sessions | 58087 | 【功能】查询某账户的全部活跃会话。<br>【返回】[{session_id, provider_id, audience_hash, session_version, session_pubkey, expires_at_ms, created_at_ms}]。 |
| keyless | GetBinding | get_binding | 4855 | 【功能】查询某 keyless 地址的绑定信息。<br>【返回】null 或 {owner, slot_index, bound_at_ms, active_at_ms}。 |
| keyless | ListProviders | list_providers | 12570 | 【功能】列出全部 provider（内置 + 自定义，无参数）。<br>【返回】[{provider_id, kind: "BuiltinOidc"\|"StaticJwk", display_name, issuer, allowed_algs, max_id_token_age_ms, max_session_ttl_ms, owner, keys, key_revision}]。 |
| keyless | GetProvider | get_provider | 20071 | 【功能】按 ID 查询单个 provider。<br>【返回】KeylessProvider 或 null。 |
| keyless | GetProviderStatus | get_provider_status | 62602 | 【功能】查询 provider 的启用状态与会话版本。<br>【返回】{enabled, session_version} 或 null。 |
| keyless | GetAudience | get_audience | 13840 | 【功能】查询某 provider 下某 audience 的注册信息。<br>【返回】{provider_id, audience, registered_at_ms} 或 null。 |
| keyless | GetParams | get_params | 12312 | 【功能】查询 keyless 全局参数（无参数）。<br>【返回】KeylessParams：{max_session_ttl_ms（默认 1 天）, bind_activation_delay_ms（默认 1 天）}。 |
| keyless | IsKeylessAccount | is_keyless_account | 33494 | 【功能】判断某地址是否已绑定 keyless 身份。<br>【返回】bool。 |
| lucky_box | BoxView | box_view | 58937 | 【功能】查询盒子状态。<br>【返回】BoxState：{creator, box_id, status, claim_count, claimed_count, expires_at_ms, randomness_version}。 |
| lucky_box | AssetPool | asset_pool | 45729 | 【功能】查询盒子资产池。<br>【返回】AssetPoolState：{asset: {variant:"Token", value:{token}}, total_amount, remaining_amount, allocation: "Equal"\|"Lucky"}。 |
| social | SocialProfile | social_profile | 5863 | 【功能】查询某地址的 profile URI。<br>【返回】String。 |
| social | SourceApp | source_app | 2147 | 【功能】查询源应用信息。<br>【返回】SourceApp：{owner, publishing_enabled}；不存在报 2564。 |
| social | IsSourceAppPublisher | is_source_app_publisher | 14357 | 【功能】判断某地址是否为源应用的发布者。<br>【返回】bool。 |
| social | GetReference | get_reference | 7408 | 【功能】查询对象引用记录。<br>【返回】Reference：{subject, content_hash, uri, status("Active"\|"Retired")}；不存在报 ReferenceNotFound（2574）。 |
| social | Community | community | 59279 | 【功能】查询社区控制信息。<br>【返回】CommunityControl：{owner, join_mode, mutations_enabled, member_count, uri}。 |
| social | IsCommunityAdmin | is_community_admin | 56720 | 【功能】判断某地址是否为社区管理员（含 owner）。<br>【返回】bool。 |
| social | JoinRequest | join_request | 38404 | 【功能】查询某成员是否有 pending 的加入申请。<br>【返回】bool。 |
| social | CommunityMembership | community_membership | 36616 | 【功能】查询某地址是否为社区成员。<br>【返回】bool。 |
| demo | OrderBalance | order_balance | 24910 | 【功能】查询演示订单在某代币上的托管余额。<br>【返回】u64。 |
| demo | SponsorPoolOf | sponsor_pool_of | 35462 | 【功能】按池编号查询赞助池地址。<br>【返回】Address。 |
| demo | LabelOf | label_of | 10409 | 【功能】查询池标签。<br>【返回】Label：{text}。 |
| demo | ScoreOf | score_of | 54597 | 【功能】查询池内某账户的积分。<br>【返回】u64。 |
| demo | TierCapOf | tier_cap_of | 48456 | 【功能】查询某层级的积分上限。<br>【返回】u64。 |
| demo | EchoMode | echo_mode | 47924 | 【功能】枚举编码回显演示：传入 DemoMode 原样返回（用于验证枚举编码）。<br>【返回】DemoMode。 |
| demo | LabelTotal | label_total | 4770 | 【功能】map 类型演示：对传入的标签-数值映射求和。<br>【返回】u32。 |
| demo | SpecialTypes | special_types | 43240 | 【功能】复杂类型编码演示（enum/option/vec/map/tuple 全覆盖）。<br>【返回】u32（各输入的汇总校验值）。 |

## Entry（写操作，kind=entry）

共 **151** 个。

| appname | meth | handler | id | 说明 |
|---------|------|---------|-----|------|
| system | Noop | noop | 13507 | 【功能】空操作指令，无参数、无任何链上效果，常用于测试交易链路或触发账户资源的懒创建。<br>【限制】无。 |
| system | BootstrapEpochClock | bootstrap_epoch_clock | 57604 | 【功能】初始化协议 epoch 时钟（计时起点 + 最短纪元时长），是 staking 纪元结算（InitializeEpochConfig / SettleStakingEpoch）的前置条件。<br>【限制】一次性初始化，重复调用会失败（EpochClockFailure）；仅在链引导阶段调用。 |
| system | BootstrapValidatorSetConfig | bootstrap_validator_set_config | 37696 | 【功能】初始化验证人集合配置：最大验证人数，以及分组/出块/批次委员会三类阈值比例，全局一次性配置。<br>【限制】一次性初始化；比例以「分子/分母」分数表示。 |
| system | RegisterValidatorIdentity | register_validator_identity | 16918 | 【功能】为验证人登记系统层共识身份：共识账户地址与共识/BLS/ed25519 三把公钥，生成共识验证人身份记录（与 staking.CreateValidator 的绑定配套）。<br>【限制】共识账户必须对交易签名；公钥长度按曲线校验（secp256k1 压缩 33 字节 / BLS 48 字节 / ed25519 32 字节）；地址与公钥唯一性由链上校验。 |
| system | PrepareValidatorSet | prepare_validator_set | 55212 | 【功能】为目标 epoch 准备验证人集合：提交随机种子作为集合选举输入（两阶段提交第一步，之后 CommitValidatorSet 生效）。<br>【限制】依赖 epoch 时钟已初始化；同一 epoch 不可重复准备。 |
| system | SettleStakingEpoch | settle_staking_epoch | 61700 | 【功能】结算当前 epoch 的质押：处理 pending 质押/解冻意向、候选池进出申请，并按配置发放奖励（即触发 staking 的 epoch 结算）。<br>【限制】需先 InitializeEpochConfig 且 epoch 时钟就绪；失败以 StakingFailure 包装返回；同一 epoch 不可重复结算。 |
| system | CommitValidatorSet | commit_validator_set | 59552 | 【功能】提交已准备的验证人集合并使其生效（PrepareValidatorSet 的第二步）。<br>【限制】须先成功执行 PrepareValidatorSet，否则报 ValidatorSetFailure。 |
| system | CommitBlockSeed | commit_block_seed | 49456 | 【功能】提交/更新区块随机种子（链上随机数信标的数据源）。<br>【限制】种子未初始化时不可提交（BlockSeedNotInitialized）；重复初始化报 BlockSeedAlreadyInitialized；底层失败以 RandomnessFailure 包装。 |
| account | Create | create | 2182 | 【功能】为给定公钥显式创建链上账户资源（区别于首笔交易时的懒创建），账户默认单签模式。<br>【限制】账户已存在报 AccountAlreadyExists（257）；公钥非法报 InvalidPublicKey（273）。 |
| account | EnsureAccount | ensure_account | 38184 | 【功能】确保账户存在：不存在则创建、已存在则什么都不做（幂等建户）。<br>【限制】幂等，已存在不报错（与 Create 的区别）。 |
| account | CreateMultisig | create_multisig | 20289 | 【功能】将签名者账户配置为多签账户：登记 signer 公钥列表、每个 signer 的权重与生效阈值（权重和 ≥ 阈值才可通过）。<br>【限制】signer 总权重 1..=255（WeightSumExceeded 263）；signers 与 weights 长度须一致（264）；signer 槽位 0..63（261）；权重不得为 0（262）；阈值非法报 ThresholdIncorrect（265）。 |
| account | AddSigner | add_signer | 41092 | 【功能】向多签账户添加一个 signer，自动分配最低空闲槽位。<br>【限制】账户必须已是多签账户（AccountNotMultisig 275）；总权重不得超过 255（263）；槽位最多 64 个（261）。 |
| account | AddSigners | add_signers | 25813 | 【功能】批量添加多个 signer 并同步设置新阈值（CreateMultisig 的增量版）。<br>【限制】同 CreateMultisig：长度一致、权重 1..=255、总权重 ≤255、槽位 0..63、阈值合法。 |
| account | RemoveSigner | remove_signer | 61953 | 【功能】从多签账户移除指定槽位的 signer 并设置移除后的新阈值。<br>【限制】该槽位必须已有 signer（SignerNotFound 259）；须为多签账户（275）；新阈值须合法（265）。 |
| account | SetThreshold | set_threshold | 2386 | 【功能】修改多签账户的生效阈值。<br>【限制】阈值须合法且不超过当前总权重（ThresholdIncorrect 265）。 |
| account | SetSignerWeight | set_signer_weight | 43270 | 【功能】修改指定槽位 signer 的权重。<br>【限制】槽位须已有 signer（259）；权重 1..=255；总权重不得超过 255（263）。 |
| account | VoteInit | vote_init | 52917 | 【功能】登记一个投票意图（MIP-25 意图重放）：提交 intent_hash 与提案内容（待重放的编码指令 + 授权位图），设置过期时间，等待账户 signer 们投票。<br>【限制】提案编码后 ≤1024 字节（VoteProposalTooLarge 283）；提案须与 intent_hash 匹配（282）；过期时间必须晚于当前时间（276）且 TTL ≤24 小时（VoteTtlExceeded 281）；活跃意图数量有上限（VoteIndexFull 280）。 |
| account | Vote | vote | 24406 | 【功能】对已登记的投票意图投出一票（按投票者权重累加），达到阈值后意图可被重放执行。<br>【限制】意图须存在且未过期（VoteExpired 277）；未达阈值时重放会被拒（VoteNotReady 278）。 |
| token | Create | create | 2581 | 【功能】创建新代币资源：登记名称/符号/精度/图标等元数据并设定 owner（管理权限）。发币方调用；token 地址由专用密钥对派生，且该密钥必须参与交易签名。<br>【限制】token 地址已被占用报 TokenAlreadyExists（516）；token 私钥必须签名，否则报 AddressNotSignatured（519）。 |
| token | AbandonOwner | abandon_owner | 64710 | 【功能】永久放弃代币 owner 权限，代币变为无主，此后所有管理操作（增发/冻结/合规配置等）失效。<br>【限制】仅当前 owner 生效（链上校验）；不可逆。 |
| token | TransferOwner | transfer_owner | 18518 | 【功能】将代币 owner 权限转移给新地址。<br>【限制】仅当前 owner 生效。 |
| token | AbandonFreezer | abandon_freezer | 1778 | 【功能】永久放弃代币 freezer（冻结管理）权限。<br>【限制】仅当前 freezer 生效；不可逆。 |
| token | TransferFreezer | transfer_freezer | 27042 | 【功能】将代币 freezer 权限转移给新地址。<br>【限制】仅当前 freezer 生效。 |
| token | Mint | mint | 20481 | 【功能】向指定地址增发代币（owner 操作）。<br>【限制】仅 owner 生效；代币须已创建（TokenNotFound 515）；增发后总量不得溢出 u64（Overflow 517）。 |
| token | MintBatch | mint_batch | 7494 | 【功能】一次性向多个地址增发代币。<br>【限制】仅 owner；to 与 amount 数组长度必须一致；任一项溢出即整笔失败（517）。 |
| token | Burn | burn | 38784 | 【功能】销毁签名者自己持有的代币（减少总量）。<br>【限制】holder 必须签名；可用余额不足报 InsufficientBalance（513）；冻结部分不可销毁。 |
| token | Transfer | transfer | 19694 | 【功能】从签名者向目标地址转账代币（最常用的用户操作）。<br>【限制】from 必须签名；余额不足报 InsufficientBalance（513）；合规代币的接收方不满足凭证要求会报 VcRequired（521）/VcPolicyRequired（522）。 |
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
| token | ClaimFaucet | claim_faucet | 63796 | 【功能】从水龙头领取原生 MIL 代币（新账户获取 gas 费用）。本指令 gas 由链上赞助池代付（sponsor）。<br>【限制】每次领取 10^10 最小单位（6 位精度下 10,000 token）；每地址 24 小时冷却，冷却期内任何尝试（含失败）都会触发/续期冷却（FaucetCooldownActive 524），脚本领水务必单次尝试；低链 ID 下水龙头禁用（FaucetDisabled 523）。 |
| staking | CreateValidator | create_validator | 16533 | 【功能】注册验证人：绑定 validator 地址与 operator、共识账户、共识公钥、BLS 公钥并设定佣金率（参与候选池/出块的基础）。<br>【限制】validator/operator/共识账户三者须互不相同（771）；地址与公钥均不可被其他验证人占用（772-776）；不可重复注册（767）；佣金率 ≤10000 基点。 |
| staking | SetStakingToken | set_staking_token | 3396 | 【功能】设置质押/奖励所用的代币地址（全局一次性配置，通常设为原生 MIL）。<br>【限制】一次性配置，权限由链上校验（协议引导阶段调用）。 |
| staking | InitializeEpochConfig | initialize_epoch_config | 36863 | 【功能】初始化 epoch 配置：设定每个验证人每个 epoch 的奖励额度。<br>【限制】只能初始化一次（EpochConfigAlreadySet 779）；需 epoch 时钟就绪（EpochClockNotReady 801，先调 system.BootstrapEpochClock）。 |
| staking | JoinCandidatePool | join_candidate_pool | 36277 | 【功能】验证人申请加入候选池，参与后续 epoch 的验证人集合选举（生成 Join 申请，随目标 epoch 结算生效）。<br>【限制】签名者必须是该验证人的 operator（OperatorMismatch 784）；候选池有最低质押门槛（CandidatePoolStakeTooLow 785）；同一 epoch 不可重复申请（786）；需 epoch 配置已初始化（780）。 |
| staking | LeaveCandidatePool | leave_candidate_pool | 62212 | 【功能】验证人申请退出候选池（Leave 申请，随目标 epoch 结算生效）。<br>【限制】同 JoinCandidatePool：operator 权限、同一 epoch 不可重复申请。 |
| staking | DeclareValidatorAvailability | declare_validator_availability | 62003 | 【功能】operator 为下一 epoch 声明验证人出块可用性（结果可用 ListDeclaredValidatorsForEpoch 查询）。<br>【限制】签名者须为 operator（784）；epoch 输入冻结期不可提交（EpochTransitionInProgress 799）。 |
| staking | FundRewardTreasury | fund_reward_treasury | 26413 | 【功能】向奖励金库转入质押代币，作为各 epoch 奖励发放的资金来源（任何持币人可资助）。<br>【限制】余额不足报 InsufficientBalance；金库记账须与实际负债一致。 |
| staking | Stake | stake | 7586 | 【功能】向某验证人质押代币：先记为待结算（pending）意向，epoch 切换结算后转为有效份额（shares）。<br>【限制】金额不得低于最小质押额（PendingStakeRemainingBelowMinimum 789）；余额不足报 InsufficientBalance；需 epoch 配置就绪（780）。 |
| staking | CancelPendingStake | cancel_pending_stake | 26203 | 【功能】撤销尚未结算的 pending 质押，取回代币。<br>【限制】须存在对应 pending 意向（StakeIntentNotFound 787）；撤销量不得超过 pending 量（788）；撤销后剩余量不得低于最小值（789）。 |
| staking | ClaimRewards | claim_rewards | 49863 | 【功能】领取某验证人池中已结算的可领奖励。<br>【返回】u64 实际领取数量（最小单位）。<br>【限制】须已有质押仓位（PositionNotFound 783）；金库可用余额不足报 InsufficientRewardTreasuryAvailable（792）。 |
| staking | ClaimOperatorRewards | claim_operator_rewards | 13178 | 【功能】验证人 operator 领取累计佣金与运营奖励。<br>【返回】OperatorRewardReceipt：{share_rewards_paid（份额奖励）, commission_paid（佣金）, total_paid}。<br>【限制】签名者必须是注册的 operator（784）；金库不足报 792。 |
| staking | RequestUnstake | request_unstake | 34861 | 【功能】按份额（shares，非代币数量）申请解冻质押：生成解冻意向，目标 epoch 结算时按比例折算代币释放。<br>【限制】份额不足报 InsufficientShares（790）；解冻后剩余份额不得低于最小值（791）；须已有仓位（783）。 |
| identity | DiscloseVcAttestation | disclose_vc_attestation | 23078 | 【功能】凭证持有人将 issuer 签发的可验证凭证（VC）披露上链：登记 issuer、schema、凭证哈希与 issuer 签名（issuer 只提供签名、不作为交易签名者）。组织 KYC 链路第三步。<br>【限制】subject 须有 DID；issuer 的 DID 须存在且未停用（IssuerDidNotActive 1034）；issuer_signature 由链上验签（InvalidVcIssuerSignature 1070）；同一 (subject, issuer, schema) 不可重复披露不同内容（1072）；已撤销凭证不可再披露（1073）。 |
| identity | RemoveVcDisclosure | remove_vc_disclosure | 51077 | 【功能】subject 撤下一条自己已披露的 VC（不影响 issuer 侧凭证状态）。<br>【限制】该披露须存在（VcAttestationNotFound 1033）。 |
| identity | RevokeVcAttestation | revoke_vc_attestation | 38692 | 【功能】issuer 永久撤销其签发给某 subject 的 VC（状态变为 Revoked，不可恢复）。<br>【限制】仅原 issuer 可撤销（issuer 为签名者）；凭证须存在（1033）；撤销后再披露报 VcAttestationRevoked（1073）。 |
| identity | FreezeVcAttestation | freeze_vc_attestation | 52523 | 【功能】issuer 临时冻结其签发给某 subject 的 VC（状态变为 Frozen，可解冻恢复——与 Revoke 的永久不可恢复区别）。<br>【限制】仅原 issuer（issuer 为签名者）；凭证须存在（1033）；已冻结再冻结报 VcAttestationAlreadyFrozen（1074）；已撤销的凭证不可冻结。 |
| identity | UnfreezeVcAttestation | unfreeze_vc_attestation | 10634 | 【功能】issuer 解除已冻结 VC 的冻结（状态恢复 Active）。<br>【限制】仅原 issuer（issuer 为签名者）；凭证须存在（1033）；未处于冻结状态报 VcAttestationNotFrozen（1075）。 |
| identity | Create | create | 28587 | 【功能】为地址创建 DID（去中心化身份）文档：主体类型（个人/组织）、密钥列表、服务端点、头像。identity 全部后续操作的前置。<br>【限制】同一 subject 只能创建一次（DidAlreadyExists 1024）；后续要注册组织必须以 subject_type="Organization" 创建（OrganizationDidRequired 1053）；密钥/服务数量与长度有上限校验（1040/1044/1046-1050）；avatar_uri 长度 1-512 字节（1045）。 |
| identity | CreateWithAlias | create_with_alias | 48723 | 【功能】创建 DID 并同时绑定全局唯一别名（等价 Create + SetAlias 一步完成）。<br>【限制】同 Create；别名须为「alias-数字」格式（NameInvalidFormat 1036）、各段长度校验（1035/1037/1038）、不可与已绑定名冲突（NameAlreadyBound 1028）。 |
| identity | AddKey | add_key | 24314 | 【功能】向 DID 文档添加一把新公钥。<br>【返回】u8 新密钥 id。<br>【限制】DID 须已创建且未停用（1025/1026）；密钥数量有上限（1040）；label 非空且不超长（1044）。 |
| identity | UpdateKey | update_key | 38726 | 【功能】替换 DID 文档中指定 id 的密钥（换公钥/改标签）。<br>【限制】密钥须存在（DidKeyNotFound 1041）。 |
| identity | RemoveKey | remove_key | 14295 | 【功能】从 DID 文档移除指定 id 的密钥。<br>【限制】不能移除最后一把密钥（CannotRemoveLastKey 1042）；密钥须存在（1041）。 |
| identity | AddService | add_service | 45574 | 【功能】为 DID 添加服务端点（官网/API 入口等）。<br>【返回】u8 新服务 id。<br>【限制】服务数量有上限（TooManyServices 1046）；endpoint 必须为合法绝对 URI 且 scheme 合法（1048-1050）。 |
| identity | UpdateService | update_service | 18018 | 【功能】更新指定 id 的服务条目。<br>【限制】服务须存在（ServiceNotFound 1051）；endpoint 校验同 AddService。 |
| identity | RemoveService | remove_service | 38683 | 【功能】移除指定 id 的服务条目。<br>【限制】服务须存在（1051）。 |
| identity | SetAvatarUri | set_avatar_uri | 7646 | 【功能】设置/更新 DID 头像 URI。<br>【限制】长度必须 1-512 字节（InvalidAvatarUri 1045）。 |
| identity | Deactivate | deactivate | 6825 | 【功能】停用该 DID（停用后 identity 的写操作均被拒绝）。<br>【限制】DID 须已创建（1025）；已停用再操作报 DidDeactivated（1026）。 |
| identity | SetAlias | set_alias | 44782 | 【功能】为已有 DID 设置/更换全局唯一别名。<br>【限制】别名不可被占用（NameAlreadyBound 1028）；格式与长度校验同 CreateWithAlias（1035-1039）。 |
| identity | RegisterOrganization | register_organization | 36868 | 【功能】为组织型 DID 注册能力声明：角色（VC 签发方 VcIssuer / KYC 服务方 KycProvider）与接受的凭证 schema 列表。组织 KYC 链路第二步（前置：subject_type=Organization 的 DID，见 ORG_KYC_GUIDE）。<br>【限制】必须先创建组织型 DID（OrganizationDidRequired 1053）；不可重复注册（1032）；声明任一角色必须至少带一个 credential_schema（IssuerCredentialSchemaRequired 1060）；角色/schema 数量与重复校验（1055-1059）。 |
| identity | UpdateOrganizationCapabilities | update_organization_capabilities | 13927 | 【功能】更新已注册组织的角色与凭证 schema 列表。<br>【限制】组织须已注册（OrganizationNotFound 1031）；已停用不可更新（1054）；roles/schemas 校验同 RegisterOrganization。 |
| identity | DeactivateOrganization | deactivate_organization | 48597 | 【功能】永久停用组织注册（不可恢复）。<br>【限制】组织须已注册（1031）；已停用再操作报 OrganizationDeactivated（1054）。 |
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
| dex | CreateMarket | create_market | 122 | 【功能】创建订单簿交易市场（base/quote 代币对），调用者成为市场 authority（管理员）。maker/taker 费率由链上常量固定为 0.3%/0.5%。<br>【返回】Address 新市场地址（由链上种子派生）。<br>【限制】base≠quote（SameTokenPair 1536）；所有数值参数非 0（ZeroMarketParameter 1537）；同代币对不可重复建市场（MarketAlreadyExists 1541）。 |
| dex | InitializeMarketDid | initialize_market_did | 27598 | 【功能】为市场初始化 DID 文档（市场身份/资质展示）。<br>【限制】须市场 authority 签名（MarketAuthorityMismatch 1546）；市场须存在（MarketNotFound 1542）；subject_type 必须为 Organization（MarketDidMustBeOrganization 1562）。 |
| dex | DiscloseMarketVcAttestation | disclose_market_vc_attestation | 47697 | 【功能】为市场披露一条 VC 凭证：登记签发方、schema、凭证哈希与签发方签名，供合规展示。<br>【限制】须市场 authority 签名；签发方身份须可解析（IdentityFailure 1561）。 |
| dex | PlaceLimitOrder | place_limit_order | 24195 | 【功能】下限价单：先尝试吃单成交，未成交部分按限价挂入订单簿并锁定资金。<br>【返回】PlaceOrderResult：{order_id, status, filled_lots, remaining_lots, fill_count, rested（是否纯挂单）}。<br>【限制】市场须为 Active 状态（MarketNotActive 1543）；price_tick 须在 [min_tick, max_tick] 内；资金不足报 TokenFailure（1555）。 |
| dex | CancelOrder | cancel_order | 54560 | 【功能】按链上 order_id 撤单并释放锁定资金。<br>【返回】OrderInfo：{order_id, owner, client_order_id, side, limit_tick, original_lots, remaining_lots, reserved_amount, status}。<br>【限制】订单须存在（OrderNotFound 1547）；仅订单本人可撤。 |
| dex | CancelByClientOrderId | cancel_by_client_order_id | 16334 | 【功能】按下单者自己的 client_order_id 撤单（无需记链上单号）。<br>【返回】OrderInfo。<br>【限制】该 owner 下须存在此 client 单号（ClientOrderNotFound 1548）。 |
| dex | BatchCancel | batch_cancel | 55242 | 【功能】批量撤单并释放锁定资金。<br>【返回】OrderInfo JSON 数组。<br>【限制】列表非空（EmptyBatchCancel 1549）且最多 32 个（BatchCancelLimitExceeded 1550）；不可重复（DuplicateBatchOrder 1551）。 |
| dex | SetMarketStatus | set_market_status | 48095 | 【功能】切换市场状态：Active（正常交易）/ CancelOnly（只可撤单）/ Paused（暂停）。<br>【限制】仅市场 authority 生效（1546）。 |
| keyless | BootstrapKeyless | bootstrap_keyless | 7470 | 【功能】初始化 keyless 注册表：注册内置 Google/Apple OIDC provider 与默认参数（无参数，args 传 {}）。<br>【限制】仅 genesis 块 0 可调（GenesisOnly 2113）；只能初始化一次（AlreadyBootstrapped 2114）。 |
| keyless | Authenticate | authenticate | 21310 | 【功能】用 OIDC id_token（JWT）+ 临时公钥完成无密钥认证，建立链上会话（后续以会话公钥签名操作）。gas 由赞助池代付（sponsor）。<br>【限制】JWT 校验严格：须三段式 JWS（2049）、未过期（2059）且签发时间不过旧（≤10 分钟，JwtTooOld 2061）、issuer/audience 与 provider 匹配（2062-2064）、签名验证通过（2066）；nonce 须以 milon-keyless-v1. 开头且内嵌 ephemeral_pk 的 commitment（2067-2075）；公共赞助每会话仅一次（2090）；provider 须已注册并启用（2091/2093）。 |
| keyless | Bind | bind | 59392 | 【功能】把 keyless 身份（JWT 认证通过后派生的地址）绑定到链上账户的一个 signer 槽位，使账户可无私钥托管。<br>【限制】该身份不可已被绑定（AlreadyBound 2098）；绑定有激活延迟（默认 1 天、下限 1 小时，active_at_ms = bound_at_ms + 延迟）；JWT 校验同 Authenticate；owner 须签名。 |
| keyless | Unbind | unbind | 57503 | 【功能】解除 keyless 身份与账户的绑定。<br>【限制】绑定须存在（BindingNotFound 2099）；仅绑定 owner 可解绑（BindingOwnerMismatch 2100）。 |
| keyless | RevokeSession | revoke_session | 15722 | 【功能】撤销一个活跃会话（会话公钥立即失效）。<br>【限制】会话须存在且属该 owner（SessionNotFound 2097）；已撤销报 SessionRevoked（2089）。 |
| keyless | RegisterAudience | register_audience | 44901 | 【功能】为 provider 注册允许的 audience（OIDC client ID），之后该 audience 的 JWT 才能通过校验。<br>【限制】provider 须已注册（ProviderNotRegistered 2091）；audience 长度 1..=256 字节（AudienceInvalid 2118）；内置 provider 元数据不可修改（BuiltinProviderImmutable 2122）。 |
| keyless | CreateStaticProvider | create_static_provider | 19418 | 【功能】注册自定义「静态 JWK」验证 provider（自带公钥集，不依赖内置 OIDC）。provider_id 由 (issuer, salt) 派生。<br>【限制】issuer 必须 https:// 开头（IssuerUrlInvalid 2094）；名称非空且 ≤64 字节（2095）；密钥 1..=8 把（StaticKeyCountInvalid 2124）；kid 唯一且 1..=128 字节（2125）；不可用保留命名空间（ReservedProviderId 2123）；重复注册报 ProviderAlreadyRegistered（2092）。 |
| keyless | UpdateStaticProviderKeys | update_static_provider_keys | 19292 | 【功能】轮换自定义 provider 的验证密钥集；invalidate_sessions=true 可使全部旧会话立即失效。<br>【限制】仅静态 provider 可改（BuiltinProviderImmutable 2122）；仅 provider owner（ProviderOwnerMismatch 2121）。 |
| lucky_box | CreateEqual | create_equal | 51385 | 【功能】创建「等概率」盲盒：total_amount 平分为 claim_count 份，每份金额相同；资金从创建者账户托管进盒子资产池。<br>【返回】u64（每份金额）。<br>【限制】box_id 不可重复（BoxAlreadyExists 2305）；金额/份数须非 0（InvalidDefinition 2310）；份数 ≤10000（TooManyItems 2311）；创建者须有足够代币托管；资格 bloom bits 32..=32768 字节；未设过期时间默认 24 小时（DEFAULT_EXPIRATION_MS）。 |
| lucky_box | CreateLucky | create_lucky | 12885 | 【功能】创建「拼手气」盲盒：领取时按链上安全随机数分配不等金额；参数与资金托管同 CreateEqual。<br>【返回】u64。<br>【限制】同 CreateEqual；随机数不可用报 SecureRandomnessUnavailable（2314）。 |
| lucky_box | Claim | claim | 62102 | 【功能】领取盲盒一份：Equal 得等份金额、Lucky 得随机金额，代币从托管转入领取者。gas 由赞助池代付（sponsor）。<br>【限制】盒子须存在且活跃（BoxNotFound 2304 / BoxNotActive 2306）、未过期（BoxExpired 2307）、尚有余量（BoxExhausted 2309）；须通过资格 bloom 校验（NotEligible 2312）；每人限领一次（QuotaExceeded 2313）。 |
| lucky_box | Refund | refund | 9120 | 【功能】盒子过期后回收剩余资产（未领完的资金退回创建者）。无需签名者，任何人可触发执行。<br>【限制】盒子必须已过期（BoxNotExpired 2308）且存在（2304）。 |
| social | UpsertSocialProfile | upsert_social_profile | 13067 | 【功能】写入/更新自己的社交 profile URI（链上只存 URI，内容存链下）。<br>【限制】须有有效未停用 DID（DidNotActive 2560）；URI 须合法（InvalidUri 2563）。 |
| social | RegisterSourceApp | register_source_app | 23225 | 【功能】注册「源应用」（公共数据发布主体），链上分配自增 ID。<br>【返回】u32 source_app_id。<br>【限制】owner 须有有效 DID；ID 序列耗尽报 SourceAppIdExhausted（2572）。 |
| social | AddSourceAppPublisher | add_source_app_publisher | 55820 | 【功能】为源应用添加发布者（允许其发布公共数据）。<br>【限制】仅应用 owner（SourceAppOwnerRequired 2567）；应用须存在（SourceAppNotFound 2564）；已是发布者报 2569。 |
| social | RemoveSourceAppPublisher | remove_source_app_publisher | 62309 | 【功能】移除源应用的某个发布者。<br>【限制】仅 owner；owner 自身的发布者身份不可移除（CannotRemoveOwnerPublisher 2571）；目标须是发布者（2570）。 |
| social | TransferSourceAppOwner | transfer_source_app_owner | 21283 | 【功能】转让源应用所有权。<br>【限制】仅当前 owner（2567）。 |
| social | DeactivateSourceApp | deactivate_source_app | 52796 | 【功能】停用源应用（暂停其数据发布；停用后发布报 SourceAppPublishingDisabled 2566）。<br>【限制】仅 owner。 |
| social | ReactivateSourceApp | reactivate_source_app | 55074 | 【功能】重新启用已停用的源应用。<br>【限制】仅 owner。 |
| social | UpsertReference | upsert_reference | 32627 | 【功能】发布者为跨应用对象引用写入/更新记录：(source_app_id, object_type, object_id) 定位对象，绑定 subject 地址与内容哈希（B256）、数据 URI。取代旧 PublicDataUri 系列的数据锚点能力。<br>【限制】调用者须是该应用发布者（SourceAppPublisherRequired 2568）；应用须存在且未停用发布（2564/2566）；object_id 须为合法稳定对象 ID（InvalidObjectId 2573）。 |
| social | RetireReference | retire_reference | 41015 | 【功能】将对象引用记录下线（retired 标记，记录保留）。<br>【限制】仅发布者；引用须存在（ReferenceNotFound 2574）；已下线再操作报 ReferenceRetired（2575）；修订号耗尽报 2577。 |
| social | CreateCommunity | create_community | 18868 | 【功能】创建社区，链上分配自增 community_id。<br>【返回】u32 community_id。<br>【限制】每 creator 最多 8 个社区（CommunityCreationLimitReached 2593）；URI 须合法（2563）。 |
| social | UpdateCommunityUri | update_community_uri | 63564 | 【功能】社区 owner 更新社区元数据 URI。<br>【限制】仅 owner（CommunityOwnerRequired 2581）；社区须存在（2578）且未停用（2580）。 |
| social | SetCommunityJoinMode | set_community_join_mode | 20649 | 【功能】owner 切换社区加入模式（Open/Approval）。<br>【限制】仅 owner；社区须存在且未停用。 |
| social | TransferCommunityOwner | transfer_community_owner | 5543 | 【功能】转让社区所有权。<br>【限制】仅当前 owner。 |
| social | DeactivateCommunity | deactivate_community | 32318 | 【功能】停用社区（成员与管理变更冻结）。<br>【限制】仅 owner。 |
| social | GrantCommunityAdmin | grant_community_admin | 10690 | 【功能】owner 授予某地址社区管理员权限。<br>【限制】仅 owner；已是管理员报 CommunityAdminAlreadyExists（2584）。 |
| social | RevokeCommunityAdmin | revoke_community_admin | 62146 | 【功能】owner 撤销某地址的社区管理员权限。<br>【限制】仅 owner；目标须是管理员（CommunityAdminNotFound 2585）。 |
| social | JoinCommunity | join_community | 39196 | 【功能】直接加入社区（仅 Open 模式；Approval 模式须走申请审批）。<br>【限制】模式须为 Open（InvalidJoinMode 2586）；成员数不超 10000（CommunityMemberLimitReached 2594）；不可重复加入（MembershipAlreadyActive 2587）。 |
| social | RequestJoinCommunity | request_join_community | 57790 | 【功能】在 Approval 模式社区提交加入申请。<br>【限制】模式须为 Approval（2586）；不可重复申请（JoinRequestAlreadyPending 2589）；不可已是成员（2587）。 |
| social | CancelJoinRequest | cancel_join_request | 33199 | 【功能】撤回自己的加入申请。<br>【限制】须有 pending 申请（JoinRequestNotPending 2590）。 |
| social | ApproveJoinRequest | approve_join_request | 2182 | 【功能】管理员批准某用户的加入申请（成员数 +1）。<br>【限制】须管理员或 owner（CommunityAdminRequired 2582）；须有 pending 申请（2590）；成员数不超 10000（2594）。 |
| social | RejectJoinRequest | reject_join_request | 21924 | 【功能】管理员拒绝某用户的加入申请。<br>【限制】同 ApproveJoinRequest：管理员权限 + pending 申请存在。 |
| social | LeaveCommunity | leave_community | 7711 | 【功能】成员主动退出社区。<br>【限制】须是活跃成员（MembershipNotActive 2588）；owner 不能退出（CannotLeaveCommunityOwner 2596，须先转让）。 |
| social | RemoveCommunityMember | remove_community_member | 7071 | 【功能】管理员移除某成员。<br>【限制】须管理员（2582）；目标须是成员（2588）；不能移除 owner（CannotRemoveCommunityOwner 2583）。 |
| demo | OpenOrder | open_order | 57564 | 【功能】演示订单支付流第一步：operator 开一个演示订单（收款托管户头）。<br>【限制】order_id 不可重复（OrderAlreadyExists 65285）。 |
| demo | PayOrder | pay_order | 58464 | 【功能】演示订单支付流第二步：付款人向订单打入指定代币与数量（进订单托管）。<br>【限制】订单须存在（OrderNotFound 65284）；付款人余额须充足。 |
| demo | SettleOrder | settle_order | 1751 | 【功能】演示订单支付流第三步：operator 把订单托管余额结算转出给指定收款地址。<br>【限制】须与开单相同的 operator（OrderOperatorMismatch 65286）；订单须存在（65284）；结算额不得超过托管余额。 |
| demo | OpenGasSponsorPool | open_gas_sponsor_pool | 6639 | 【功能】注册 gas 赞助池，使 sponsor 标记的指令（keyless.Authenticate、lucky_box.Claim、demo.ClaimSponsoredScore 等）可由该池代付 gas。<br>【返回】u16 sponsor_seed（池编号，供赞助类指令引用）。<br>【限制】pool 地址的管理员（signer_lookup 从 pool 参数解析 admin）必须与交易付款人一致。 |
| demo | ClaimSponsoredScore | claim_sponsored_score | 11876 | 【功能】演示从赞助池领取 gas 赞助并给领取者记积分。gas 由赞助池代付（sponsor）。<br>【限制】池须已注册（SponsorPoolNotFound 65287）。 |
| demo | InitPool | init_pool | 27056 | 【功能】初始化一个演示池（标签/积分载体），由 pool 资源账户签名。<br>【限制】不可重复初始化（PoolAlreadyExists 65281）。 |
| demo | InitDex | init_dex | 44919 | 【功能】同 InitPool，以 dex 为主题的演示初始化。<br>【限制】不可重复初始化（65281）。 |
| demo | SetLabel | set_label | 58132 | 【功能】修改池的标签。<br>【限制】池须存在（PoolNotFound 65280）。 |
| demo | BatchCredit | batch_credit | 8311 | 【功能】给一批接收者在池内加积分（触发 EventCreditApplied 事件）。<br>【限制】池须存在（65280）；接收者须已在池内登记（RecipientNotFound 65282）。 |
| demo | SetTierCap | set_tier_cap | 58369 | 【功能】设置某层级的积分上限（配合领用限速演示）。<br>【限制】池须存在（65280）。 |

## 各 App 统计

| appname | app_id | view | entry | 合计 |
|---------|--------|------|-------|------|
| system | 0 | 0 | 8 | 8 |
| account | 1 | 5 | 10 | 15 |
| token | 2 | 7 | 23 | 30 |
| staking | 3 | 11 | 12 | 23 |
| identity | 4 | 25 | 19 | 44 |
| sftoken | 5 | 14 | 26 | 40 |
| dex | 6 | 6 | 8 | 14 |
| keyless | 8 | 8 | 8 | 16 |
| lucky_box | 9 | 2 | 4 | 6 |
| social | 10 | 8 | 23 | 31 |
| demo | 255 | 8 | 10 | 18 |
| **合计** | — | **94** | **151** | **245** |
