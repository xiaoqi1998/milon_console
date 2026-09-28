package handler

// IDL 方法中文文档映射。键为 "app.Method"，值为多行说明（【功能】/【限制】/【返回】分段，\n 换行）。
// 内容依据各 app IDL JSON（args/types/errors/constants）、gosdk 示例与 ORG_KYC_GUIDE.md 交叉整理；
// 限制条目括注的错误码均来自对应 IDL 的 errors 段。
// 覆盖率由 idl_docs_test.go 保证：IDL 增删方法时测试会指出需要增删的条目。
var idlMethodDocs = map[string]string{
	// ==================== system 模块（协议/共识引导，8 个 entry）====================

	"system.Noop": "【功能】空操作指令，无参数、无任何链上效果，常用于测试交易链路或触发账户资源的懒创建。\n【限制】无。",

	"system.BootstrapEpochClock": "【功能】初始化协议 epoch 时钟（计时起点 + 最短纪元时长），是 staking 纪元结算（InitializeEpochConfig / SettleStakingEpoch）的前置条件。\n【限制】一次性初始化，重复调用会失败（EpochClockFailure）；仅在链引导阶段调用。",

	"system.BootstrapValidatorSetConfig": "【功能】初始化验证人集合配置：最大验证人数，以及分组/出块/批次委员会三类阈值比例，全局一次性配置。\n【限制】一次性初始化；比例以「分子/分母」分数表示。",

	"system.RegisterValidatorIdentity": "【功能】为验证人登记系统层共识身份：共识账户地址与共识/BLS/ed25519 三把公钥，生成共识验证人身份记录（与 staking.CreateValidator 的绑定配套）。\n【限制】共识账户必须对交易签名；公钥长度按曲线校验（secp256k1 压缩 33 字节 / BLS 48 字节 / ed25519 32 字节）；地址与公钥唯一性由链上校验。",

	"system.PrepareValidatorSet": "【功能】为目标 epoch 准备验证人集合：提交随机种子作为集合选举输入（两阶段提交第一步，之后 CommitValidatorSet 生效）。\n【限制】依赖 epoch 时钟已初始化；同一 epoch 不可重复准备。",

	"system.SettleStakingEpoch": "【功能】结算当前 epoch 的质押：处理 pending 质押/解冻意向、候选池进出申请，并按配置发放奖励（即触发 staking 的 epoch 结算）。\n【限制】需先 InitializeEpochConfig 且 epoch 时钟就绪；失败以 StakingFailure 包装返回；同一 epoch 不可重复结算。",

	"system.CommitValidatorSet": "【功能】提交已准备的验证人集合并使其生效（PrepareValidatorSet 的第二步）。\n【限制】须先成功执行 PrepareValidatorSet，否则报 ValidatorSetFailure。",

	"system.CommitBlockSeed": "【功能】提交/更新区块随机种子（链上随机数信标的数据源）。\n【限制】种子未初始化时不可提交（BlockSeedNotInitialized）；重复初始化报 BlockSeedAlreadyInitialized；底层失败以 RandomnessFailure 包装。",

	// ==================== account 模块（app_id=1，10 entry + 5 view）====================

	"account.Create": "【功能】为给定公钥显式创建链上账户资源（区别于首笔交易时的懒创建），账户默认单签模式。\n【限制】账户已存在报 AccountAlreadyExists（257）；公钥非法报 InvalidPublicKey（273）。",

	"account.EnsureAccount": "【功能】确保账户存在：不存在则创建、已存在则什么都不做（幂等建户）。\n【限制】幂等，已存在不报错（与 Create 的区别）。",

	"account.CreateMultisig": "【功能】将签名者账户配置为多签账户：登记 signer 公钥列表、每个 signer 的权重与生效阈值（权重和 ≥ 阈值才可通过）。\n【限制】signer 总权重 1..=255（WeightSumExceeded 263）；signers 与 weights 长度须一致（264）；signer 槽位 0..63（261）；权重不得为 0（262）；阈值非法报 ThresholdIncorrect（265）。",

	"account.AddSigner": "【功能】向多签账户添加一个 signer，自动分配最低空闲槽位。\n【限制】账户必须已是多签账户（AccountNotMultisig 275）；总权重不得超过 255（263）；槽位最多 64 个（261）。",

	"account.AddSigners": "【功能】批量添加多个 signer 并同步设置新阈值（CreateMultisig 的增量版）。\n【限制】同 CreateMultisig：长度一致、权重 1..=255、总权重 ≤255、槽位 0..63、阈值合法。",

	"account.RemoveSigner": "【功能】从多签账户移除指定槽位的 signer 并设置移除后的新阈值。\n【限制】该槽位必须已有 signer（SignerNotFound 259）；须为多签账户（275）；新阈值须合法（265）。",

	"account.SetThreshold": "【功能】修改多签账户的生效阈值。\n【限制】阈值须合法且不超过当前总权重（ThresholdIncorrect 265）。",

	"account.SetSignerWeight": "【功能】修改指定槽位 signer 的权重。\n【限制】槽位须已有 signer（259）；权重 1..=255；总权重不得超过 255（263）。",

	"account.VoteInit": "【功能】登记一个投票意图（MIP-25 意图重放）：提交 intent_hash 与提案内容（待重放的编码指令 + 授权位图），设置过期时间，等待账户 signer 们投票。\n【限制】提案编码后 ≤1024 字节（VoteProposalTooLarge 283）；提案须与 intent_hash 匹配（282）；过期时间必须晚于当前时间（276）且 TTL ≤24 小时（VoteTtlExceeded 281）；活跃意图数量有上限（VoteIndexFull 280）。",

	"account.Vote": "【功能】对已登记的投票意图投出一票（按投票者权重累加），达到阈值后意图可被重放执行。\n【限制】意图须存在且未过期（VoteExpired 277）；未达阈值时重放会被拒（VoteNotReady 278）。",

	"account.GetAccount": "【功能】查询账户核心状态（多签位图、权重、阈值等）。\n【返回】Account：{bitmap（signer 位图）、weight（总权重）、threshold（阈值）、last_modified_block}。注意：账户首笔交易前资源不存在，报 AccountNotFound（256）。",

	"account.ListSigners": "【功能】查询账户状态及全部 signer 列表。\n【返回】[Account, [[PublicKey, 权重, 槽位号], ...]]（JSON 数组或 {\"0\":..,\"1\":..}）。",

	"account.ResolveSigners": "【功能】按 signer 槽位位图解析对应公钥列表，并校验权重/人数是否达标——交易提交前预检多签签名组合。\n【返回】签名者公钥数组（hex/base58）。\n【限制】位图为空报 SigBitEmpty（266）；位超 0..63 报 267；权重不足报 InsufficientSignerWeight（268）；人数不足报 SigBitTooFew（269）。",

	"account.GetVote": "【功能】查询某个投票意图的详情。\n【返回】[VoteMeta{intent_hash, expires_at_ms, source_tx_hash}, 已投票 signer 位图, 是否已达阈值]。",

	"account.ListActiveVotes": "【功能】列出账户当前全部活跃投票意图。\n【返回】[[VoteMeta, 已投票位图, 是否达标], ...] 数组。",

	// ==================== token 模块（app_id=2，23 entry + 7 view）====================

	"token.Create": "【功能】创建新代币资源：登记名称/符号/精度/图标等元数据并设定 owner（管理权限）。发币方调用；token 地址由专用密钥对派生，且该密钥必须参与交易签名。\n【限制】token 地址已被占用报 TokenAlreadyExists（516）；token 私钥必须签名，否则报 AddressNotSignatured（519）。",

	"token.AbandonOwner": "【功能】永久放弃代币 owner 权限，代币变为无主，此后所有管理操作（增发/冻结/合规配置等）失效。\n【限制】仅当前 owner 生效（链上校验）；不可逆。",

	"token.TransferOwner": "【功能】将代币 owner 权限转移给新地址。\n【限制】仅当前 owner 生效。",

	"token.AbandonFreezer": "【功能】永久放弃代币 freezer（冻结管理）权限。\n【限制】仅当前 freezer 生效；不可逆。",

	"token.TransferFreezer": "【功能】将代币 freezer 权限转移给新地址。\n【限制】仅当前 freezer 生效。",

	"token.Mint": "【功能】向指定地址增发代币（owner 操作）。\n【限制】仅 owner 生效；代币须已创建（TokenNotFound 515）；增发后总量不得溢出 u64（Overflow 517）。",

	"token.MintBatch": "【功能】一次性向多个地址增发代币。\n【限制】仅 owner；to 与 amount 数组长度必须一致；任一项溢出即整笔失败（517）。",

	"token.Burn": "【功能】销毁签名者自己持有的代币（减少总量）。\n【限制】holder 必须签名；可用余额不足报 InsufficientBalance（513）；冻结部分不可销毁。",

	"token.Transfer": "【功能】从签名者向目标地址转账代币（最常用的用户操作）。\n【限制】from 必须签名；余额不足报 InsufficientBalance（513）；合规代币的接收方不满足凭证要求会报 VcRequired（521）/VcPolicyRequired（522）。",

	"token.TransferBatch": "【功能】从签名者一次性向多个地址转账。\n【限制】from 必须签名；to 与 amount 等长；余额须覆盖转账总额（513）。",

	"token.Freeze": "【功能】冻结某持有人指定数量的代币，冻结部分不可转账/销毁（合规风控操作，freezer 调用）。\n【限制】仅 freezer 生效；冻结量不得超过持有量。",

	"token.Unfreeze": "【功能】解冻之前冻结的代币。\n【限制】仅 freezer 生效；解冻量不得超过冻结量（Underflow 518）。",

	"token.Approve": "【功能】授权 spender 可动用自己名下代币的额度（ERC20 approve 语义）。\n【限制】owner 必须签名。",

	"token.Revoke": "【功能】撤销 spender 的授权（额度清零）。\n【限制】owner 必须签名。",

	"token.TransferFrom": "【功能】spender 动用授权额度，把 from 名下代币转给指定地址（委托转账）。\n【限制】spender 必须签名；授权额度不足报 InsufficientApproval（514）；持有人余额不足报 513。",

	"token.SetIcon": "【功能】设置/更新代币图标 URL。\n【限制】仅 owner 生效；代币须已创建（515）。",

	"token.SetUri": "【功能】设置/更新代币元数据 URI。\n【限制】仅 owner 生效。",

	"token.CreateWithCompliance": "【功能】创建带合规要求的代币：创建同时登记必需凭证（VC）ID，接收方须持有满足合规策略的凭证才能收到币（受监管资产发行用）。\n【限制】同 token.Create（地址占用/签名）；后续转账会被合规校验拦截（521/522）。",

	"token.SetComplianceMode": "【功能】设置合规策略模式：All=接收方须持有全部已列凭证；Any=持有其中任意一个即可。\n【限制】仅 owner 生效。",

	"token.AddComplianceRequirement": "【功能】向合规要求列表添加一个必需凭证 ID。\n【限制】仅 owner 生效。",

	"token.RemoveComplianceRequirement": "【功能】从合规要求列表移除一个凭证 ID。\n【限制】仅 owner 生效。",

	"token.ClearComplianceRequirements": "【功能】清空全部合规凭证要求，代币恢复为无合规限制。\n【限制】仅 owner 生效。",

	"token.ClaimFaucet": "【功能】从水龙头领取原生 MIL 代币（新账户获取 gas 费用）。本指令 gas 由链上赞助池代付（sponsor）。\n【限制】每次领取 10^10 最小单位（6 位精度下 10,000 token）；每地址 24 小时冷却，冷却期内任何尝试（含失败）都会触发/续期冷却（FaucetCooldownActive 524），脚本领水务必单次尝试；低链 ID 下水龙头禁用（FaucetDisabled 523）。",

	"token.BalanceOf": "【功能】查询某地址在某代币上的余额。\n【返回】u64，最小单位。",

	"token.FrozenOf": "【功能】查询某地址被冻结的代币数量。\n【返回】u64，最小单位。",

	"token.ApprovalOf": "【功能】查询 owner 授权给 spender 的剩余额度。\n【返回】u64，最小单位。",

	"token.TotalSupply": "【功能】查询代币总供应量。\n【返回】u64，最小单位。",

	"token.Metadata": "【功能】查询代币元数据。\n【返回】Metadata：{name, symbol, decimals, icon, uri}。",

	"token.Compliance": "【功能】查询代币合规策略配置。\n【返回】Compliance：{mode: \"All\"|\"Any\", requirements: [凭证 ID 数组]}。",

	"token.FaucetCooldownRemaining": "【功能】查询某地址水龙头冷却剩余时间。\n【返回】u64 毫秒数；0 表示可立即领取。",

	// ==================== staking 模块（app_id=3，12 entry + 11 view）====================

	"staking.CreateValidator": "【功能】注册验证人：绑定 validator 地址与 operator、共识账户、共识公钥、BLS 公钥并设定佣金率（参与候选池/出块的基础）。\n【限制】validator/operator/共识账户三者须互不相同（771）；地址与公钥均不可被其他验证人占用（772-776）；不可重复注册（767）；佣金率 ≤10000 基点。",

	"staking.SetStakingToken": "【功能】设置质押/奖励所用的代币地址（全局一次性配置，通常设为原生 MIL）。\n【限制】一次性配置，权限由链上校验（协议引导阶段调用）。",

	"staking.InitializeEpochConfig": "【功能】初始化 epoch 配置：设定每个验证人每个 epoch 的奖励额度。\n【限制】只能初始化一次（EpochConfigAlreadySet 779）；需 epoch 时钟就绪（EpochClockNotReady 801，先调 system.BootstrapEpochClock）。",

	"staking.JoinCandidatePool": "【功能】验证人申请加入候选池，参与后续 epoch 的验证人集合选举（生成 Join 申请，随目标 epoch 结算生效）。\n【限制】签名者必须是该验证人的 operator（OperatorMismatch 784）；候选池有最低质押门槛（CandidatePoolStakeTooLow 785）；同一 epoch 不可重复申请（786）；需 epoch 配置已初始化（780）。",

	"staking.LeaveCandidatePool": "【功能】验证人申请退出候选池（Leave 申请，随目标 epoch 结算生效）。\n【限制】同 JoinCandidatePool：operator 权限、同一 epoch 不可重复申请。",

	"staking.DeclareValidatorAvailability": "【功能】operator 为下一 epoch 声明验证人出块可用性（结果可用 ListDeclaredValidatorsForEpoch 查询）。\n【限制】签名者须为 operator（784）；epoch 输入冻结期不可提交（EpochTransitionInProgress 799）。",

	"staking.FundRewardTreasury": "【功能】向奖励金库转入质押代币，作为各 epoch 奖励发放的资金来源（任何持币人可资助）。\n【限制】余额不足报 InsufficientBalance；金库记账须与实际负债一致。",

	"staking.Stake": "【功能】向某验证人质押代币：先记为待结算（pending）意向，epoch 切换结算后转为有效份额（shares）。\n【限制】金额不得低于最小质押额（PendingStakeRemainingBelowMinimum 789）；余额不足报 InsufficientBalance；需 epoch 配置就绪（780）。",

	"staking.CancelPendingStake": "【功能】撤销尚未结算的 pending 质押，取回代币。\n【限制】须存在对应 pending 意向（StakeIntentNotFound 787）；撤销量不得超过 pending 量（788）；撤销后剩余量不得低于最小值（789）。",

	"staking.ClaimRewards": "【功能】领取某验证人池中已结算的可领奖励。\n【返回】u64 实际领取数量（最小单位）。\n【限制】须已有质押仓位（PositionNotFound 783）；金库可用余额不足报 InsufficientRewardTreasuryAvailable（792）。",

	"staking.ClaimOperatorRewards": "【功能】验证人 operator 领取累计佣金与运营奖励。\n【返回】OperatorRewardReceipt：{share_rewards_paid（份额奖励）, commission_paid（佣金）, total_paid}。\n【限制】签名者必须是注册的 operator（784）；金库不足报 792。",

	"staking.RequestUnstake": "【功能】按份额（shares，非代币数量）申请解冻质押：生成解冻意向，目标 epoch 结算时按比例折算代币释放。\n【限制】份额不足报 InsufficientShares（790）；解冻后剩余份额不得低于最小值（791）；须已有仓位（783）。",

	"staking.ValidatorProfile": "【功能】查询验证人基本档案。\n【返回】ValidatorProfile：{validator, operator, status: Inactive|Candidate|Active|Leaving, commission_rate_bps}；不存在报 768。",

	"staking.ValidatorPool": "【功能】查询验证人池记账快照（自有/受托质押、奖励累计等）。\n【返回】ValidatorPoolView：{operator_stake, delegated_stake, total_stake, total_shares, reward_index, claimable_operator_commission, commission_rate_bps}（金额均为最小单位）。",

	"staking.StakePosition": "【功能】查询某用户在某验证人处的质押仓位原始数据。\n【返回】StakePosition：{shares, reward_debt, claimable_rewards}；无仓位报 783。",

	"staking.PositionSummary": "【功能】查询质押仓位汇总（面向前端的聚合视图）。\n【返回】PositionSummary：{pending_stake, settled_shares, pending_unstake_shares, unstakeable_shares, claimable_rewards, next_settlement_epoch}。",

	"staking.CandidatePool": "【功能】查询当前候选池中的全部验证人地址（无参数）。\n【返回】CandidatePool：{validators: [地址数组]}。",

	"staking.EpochTransition": "【功能】查询某次 epoch 结算的执行明细。\n【返回】EpochTransition：{reward_per_validator, distributed_rewards, rewarded_validator_count, settled_stake_intent_count, settled_unstake_intent_count, applied_candidate_intent_count, candidate_rejections}；未发生过报 EpochTransitionNotFound（798）。",

	"staking.EpochConfig": "【功能】查询 epoch 配置（无参数）。\n【返回】EpochConfig：{reward_per_validator}；未初始化报 EpochConfigNotSet（780）。",

	"staking.EpochState": "【功能】查询 epoch 结算进度（无参数）。\n【返回】EpochState：{last_settled_epoch}。",

	"staking.RewardTreasury": "【功能】查询奖励金库状态（无参数）。\n【返回】RewardTreasury：{address, balance, available_rewards, reserved_rewards}（最小单位）。",

	"staking.HeldPrincipal": "【功能】查询某地址被质押模块托管的本金总额（含 pending 与待释放部分）。\n【返回】u64（最小单位）。",

	"staking.ListDeclaredValidatorsForEpoch": "【功能】查询某 epoch 已声明可用性的验证人列表。\n【返回】地址 JSON 数组。",

	// ==================== identity 模块（app_id=4，17 entry + 25 view）====================

	"identity.Create": "【功能】为地址创建 DID（去中心化身份）文档：主体类型（个人/组织）、密钥列表、服务端点、头像。identity 全部后续操作的前置。\n【限制】同一 subject 只能创建一次（DidAlreadyExists 1024）；后续要注册组织必须以 subject_type=\"Organization\" 创建（OrganizationDidRequired 1053）；密钥/服务数量与长度有上限校验（1040/1044/1046-1050）；avatar_uri 长度 1-512 字节（1045）。",

	"identity.CreateWithAlias": "【功能】创建 DID 并同时绑定全局唯一别名（等价 Create + SetAlias 一步完成）。\n【限制】同 Create；别名须为「alias-数字」格式（NameInvalidFormat 1036）、各段长度校验（1035/1037/1038）、不可与已绑定名冲突（NameAlreadyBound 1028）。",

	"identity.AddKey": "【功能】向 DID 文档添加一把新公钥。\n【返回】u8 新密钥 id。\n【限制】DID 须已创建且未停用（1025/1026）；密钥数量有上限（1040）；label 非空且不超长（1044）。",

	"identity.UpdateKey": "【功能】替换 DID 文档中指定 id 的密钥（换公钥/改标签）。\n【限制】密钥须存在（DidKeyNotFound 1041）。",

	"identity.RemoveKey": "【功能】从 DID 文档移除指定 id 的密钥。\n【限制】不能移除最后一把密钥（CannotRemoveLastKey 1042）；密钥须存在（1041）。",

	"identity.AddService": "【功能】为 DID 添加服务端点（官网/API 入口等）。\n【返回】u8 新服务 id。\n【限制】服务数量有上限（TooManyServices 1046）；endpoint 必须为合法绝对 URI 且 scheme 合法（1048-1050）。",

	"identity.UpdateService": "【功能】更新指定 id 的服务条目。\n【限制】服务须存在（ServiceNotFound 1051）；endpoint 校验同 AddService。",

	"identity.RemoveService": "【功能】移除指定 id 的服务条目。\n【限制】服务须存在（1051）。",

	"identity.SetAvatarUri": "【功能】设置/更新 DID 头像 URI。\n【限制】长度必须 1-512 字节（InvalidAvatarUri 1045）。",

	"identity.Deactivate": "【功能】停用该 DID（停用后 identity 的写操作均被拒绝）。\n【限制】DID 须已创建（1025）；已停用再操作报 DidDeactivated（1026）。",

	"identity.SetAlias": "【功能】为已有 DID 设置/更换全局唯一别名。\n【限制】别名不可被占用（NameAlreadyBound 1028）；格式与长度校验同 CreateWithAlias（1035-1039）。",

	"identity.RegisterOrganization": "【功能】为组织型 DID 注册能力声明：角色（VC 签发方 VcIssuer / KYC 服务方 KycProvider）与接受的凭证 schema 列表。组织 KYC 链路第二步（前置：subject_type=Organization 的 DID，见 ORG_KYC_GUIDE）。\n【限制】必须先创建组织型 DID（OrganizationDidRequired 1053）；不可重复注册（1032）；声明任一角色必须至少带一个 credential_schema（IssuerCredentialSchemaRequired 1060）；角色/schema 数量与重复校验（1055-1059）。",

	"identity.UpdateOrganizationCapabilities": "【功能】更新已注册组织的角色与凭证 schema 列表。\n【限制】组织须已注册（OrganizationNotFound 1031）；已停用不可更新（1054）；roles/schemas 校验同 RegisterOrganization。",

	"identity.DeactivateOrganization": "【功能】永久停用组织注册（不可恢复）。\n【限制】组织须已注册（1031）；已停用再操作报 OrganizationDeactivated（1054）。",

	"identity.DiscloseVcAttestation": "【功能】凭证持有人将 issuer 签发的可验证凭证（VC）披露上链：登记 issuer、schema、凭证哈希与 issuer 签名（issuer 只提供签名、不作为交易签名者）。组织 KYC 链路第三步。\n【限制】subject 须有 DID；issuer 的 DID 须存在且未停用（IssuerDidNotActive 1034）；issuer_signature 由链上验签（InvalidVcIssuerSignature 1070）；同一 (subject, issuer, schema) 不可重复披露不同内容（1072）；已撤销凭证不可再披露（1073）。",

	"identity.RemoveVcDisclosure": "【功能】subject 撤下一条自己已披露的 VC（不影响 issuer 侧凭证状态）。\n【限制】该披露须存在（VcAttestationNotFound 1033）。",

	"identity.RevokeVcAttestation": "【功能】issuer 永久撤销其签发给某 subject 的 VC（状态变为 Revoked，不可恢复）。\n【限制】仅原 issuer 可撤销（issuer 为签名者）；凭证须存在（1033）；撤销后再披露报 VcAttestationRevoked（1073）。",

	"identity.VcAttestationCore": "【功能】查询某 (subject, schema, issuer) 组合下 VC 凭证的不可变核心内容。\n【返回】VcAttestationCore：{credential_schema, credential_hash, valid_until_ms}；不存在报 1033。",

	"identity.VcAttestationLifecycle": "【功能】查询 VC 的生命周期状态（是否被撤销、最近变更时间）。\n【返回】VcAttestationLifecycle：{status: \"Active\"|\"Revoked\", updated_at_ms}。",

	"identity.AcceptedVcIssuerIndexMeta": "【功能】查询 subject 某 schema 下「已接受 VC 签发方索引」的元信息。\n【返回】String；索引不存在报 VcAcceptanceIndexNotFound（1068）。",

	"identity.AcceptedVcIssuers": "【功能】列出 subject 某 schema 下已接受凭证的签发方及凭证哈希。\n【返回】[{issuer, credential_hash}]；数量有上限（TooManyAcceptedVcIssuers 1065）。",

	"identity.DisclosedVcSchemas": "【功能】列出 subject 已披露 VC 的全部 schema。\n【返回】字符串 JSON 数组；种类数有上限（TooManyDisclosedVcSchemas 1066）。",

	"identity.DisclosedVcs": "【功能】分页列出 subject 已披露 VC 的摘要。\n【返回】[{credential_schema, issuer, credential_hash, valid_until_ms, status, is_valid}]。",

	"identity.HasValidVcFromIssuer": "【功能】判断 subject 是否持有来自指定 issuer、指定 schema 且当前有效的 VC（合规校验常用，见 ORG_KYC_GUIDE 步骤 4）。\n【返回】bool。",

	"identity.Core": "【功能】查询 DID 核心状态（主体类型与 controller 地址）。\n【返回】DidCoreState：{subject: {subject_type, address}, controller}。",

	"identity.Document": "【功能】查询完整 DID 文档（DID 模型的主查询入口）。\n【返回】DidDocumentState：{subject, controller, keys, services, alias, avatar_uri, updated_at_ms, deactivated}；未创建报 DidNotFound（1025）。",

	"identity.KeyIndex": "【功能】查询 DID 密钥索引状态。\n【返回】DidKeyIndexState：{initialized, key_ids（hex 字节串）}。",

	"identity.Keys": "【功能】列出该 DID 的全部密钥。\n【返回】[{id, public_key, label}]。",

	"identity.Key": "【功能】查询指定 id 的单把密钥。\n【返回】DidKey；不存在报 1041。",

	"identity.ServiceIndex": "【功能】查询 DID 服务索引状态。\n【返回】DidServiceIndexState：{initialized, service_ids}。",

	"identity.Services": "【功能】列出该 DID 的全部服务条目。\n【返回】[{id, label, service_endpoint}]。",

	"identity.Service": "【功能】查询指定 id 的服务条目。\n【返回】DidService；不存在报 1051。",

	"identity.Alias": "【功能】查询 DID 别名状态。\n【返回】DidAliasState：{initialized, alias: {alias, suffix} 或 null}。",

	"identity.Avatar": "【功能】查询 DID 头像状态。\n【返回】DidAvatarState：{initialized, avatar_uri}。",

	"identity.UpdatedAt": "【功能】查询 DID 文档最近一次更新的毫秒时间戳。\n【返回】u64。",

	"identity.Deactivated": "【功能】查询 DID 是否已停用。\n【返回】bool。",

	"identity.NameBinding": "【功能】按别名反查其绑定的 DID 主体。\n【返回】DidNameBinding：{name, subject}；未绑定报 NameNotFound（1029）。",

	"identity.CredentialDefinition": "【功能】按凭证 ID 查询凭证定义（签发方与 schema）。\n【返回】CredentialDefinition：{issuer, credential_schema}；不存在报 1063。",

	"identity.OrganizationCapabilities": "【功能】查询组织已声明的角色与凭证 schema 列表。\n【返回】OrganizationCapabilities：{roles, credential_schemas}；未注册报 1031。",

	"identity.OrganizationStatus": "【功能】查询组织状态。\n【返回】\"Active\" 或 \"Deactivated\"。",

	"identity.OrganizationUpdatedAt": "【功能】查询组织注册信息最近一次更新的毫秒时间戳。\n【返回】u64。",

	"identity.CredentialId": "【功能】由 issuer 地址与 credential_schema 派生全局凭证 ID（用于 CredentialDefinition 等查询）。\n【返回】String。",

	// ==================== sftoken 模块（app_id=5，19 entry + 15 view）====================

	"sftoken.CreateSft": "【功能】创建 SFT（半同质化代币）集合：登记元数据、初始 owner 与版税比例。SFT 为「集合 + 槽位(slot) + token」三层模型的最上层。\n【限制】由 sft 资源账户私钥签名；同一地址只能创建一次（SftAlreadyExists 1289）。",

	"sftoken.CreateSlot": "【功能】在 SFT 下创建槽位（slot）：设定元数据/属性、是否可转移，以及槽位 owner（即铸造权限）、内容更新人、冻结权限人三个可选角色。slot_id 由链上从 1 递增分配。\n【限制】仅 SFT owner 可创建（creator 签名）；SFT 须已存在（SftNotFound 1285）；slot_id 不可自选，从回执事件 SlotCreatedEvent.slot_id 获取；id 耗尽报 SlotIdExhausted（1292）。",

	"sftoken.TransferOwner": "【功能】转让 SFT 集合 owner（集合管理权，不含 token 资产）。\n【限制】仅现任 owner（Unauthorized 1280）；SFT 须存在（1285）。",

	"sftoken.TransferSlotOwner": "【功能】转让槽位的 slot_owner（槽位所有/铸造权限）。\n【限制】仅现任 slot_owner（1280）；slot 须存在（SlotNotFound 1284）。",

	"sftoken.TransferSlotUpdateAuthor": "【功能】转让槽位的内容更新权限（update_author）。\n【限制】仅现任 update_author（1280）；slot 须存在（1284）。",

	"sftoken.TransferSlotFreezeAuthority": "【功能】转让槽位的冻结权限（freeze_authority）。\n【限制】仅现任 freeze_authority（1280）；slot 须存在（1284）。",

	"sftoken.TransferRoyaltyRecipient": "【功能】修改 SFT 的版税接收人地址。\n【限制】仅现任版税接收人（1280）。",

	"sftoken.SetSlotTransferable": "【功能】设置槽位 is_transferable 标志（开/关该槽位 token 的部分转移与合并能力）。\n【限制】须槽位管理权限（1280）；slot 须存在（1284）。",

	"sftoken.FreezeSlot": "【功能】冻结槽位（冻结后该槽位 token 的转移/合并类操作被拒）。\n【限制】仅 freeze_authority 生效；冻结后相关操作报 SlotFrozen（1287）。",

	"sftoken.UnfreezeSlot": "【功能】解除槽位冻结。\n【限制】仅 freeze_authority 生效；slot 须存在（1284）。",

	"sftoken.Mint": "【功能】向目标地址铸造一个属于指定槽位的新 token（含初始价值/元数据/属性）。token_id 由链上从 1 递增分配。\n【返回】u64 新 token_id（也从回执 TokenMintedEvent.token_id 获取）。\n【限制】须槽位铸造权限人（slot_owner）签名；SFT/slot 须存在（1285/1284）；initial_value 必须 >0（ZeroValueOperation 1286）；id 耗尽报 1291；溢出报 Overflow（1293）。",

	"sftoken.Burn": "【功能】销毁指定 token（其价值随之消失）。\n【限制】仅 token 现任 owner；token 须存在（TokenNotFound 1283）。",

	"sftoken.Split": "【功能】把源 token 的一部分价值拆给接收者，产生一个新 token（继承槽位/元数据/属性），源 token 保留剩余价值。\n【返回】u64 新 token_id（回执 TokenSplitEvent.new_token_id）。\n【限制】签名者为源 token owner（不要求槽位可转移）；split_value 必须 >0（1286）且不得超过源 token 价值（InsufficientBalance 1282）。",

	"sftoken.Merge": "【功能】把源 token 的全部价值并入目标 token 并销毁源 token（同槽位价值合并）。\n【限制】两个 token 必须属于同一槽位（SlotMismatch 1281）；要求槽位 is_transferable=true；不可自我合并（SelfMerge 1294）；token/slot 须存在（1283/1284）。",

	"sftoken.SetAttribute": "【功能】更新指定 token 的 attribute 属性字符串。\n【限制】须相应权限（token owner / 槽位 update_author）；token 须存在（1283）。",

	"sftoken.Approve": "【功能】token owner 给 spender 授予该 token 上的可花费额度（额度记录在 (token_id, spender) 上）。\n【限制】仅 token owner；token 须存在（1283）。",

	"sftoken.RevokeApproval": "【功能】撤销 spender 在指定 token 上的授权额度。\n【限制】仅 token owner。",

	"sftoken.Transfer": "【功能】转移 token 价值：全量转移保留 token_id 仅改 owner（不要求可转移）；部分转移走拆分路径（为接收者新建 token，要求槽位 is_transferable=true）。\n【返回】u64（部分转移时为新 token_id）。\n【限制】value 必须 >0（1286）；不得超过持有量（1282）；槽位冻结报 1287。",

	"sftoken.TransferFrom": "【功能】被授权方（spender）动用 Approve 额度转移他人 token 价值（始终走拆分路径，要求槽位可转移；额度按转移量扣减）。\n【返回】u64 新 token_id。\n【限制】spender 须有足额授权；value >0（1286）；槽位冻结/不可转移则失败（1287）。",

	"sftoken.SftInfo": "【功能】查询 SFT 集合信息。\n【返回】Sft：{owner, metadata}；未创建报 SftNotFound（1285）。",

	"sftoken.SlotInfo": "【功能】查询指定槽位的信息。\n【返回】SlotData：{metadata, attribute, is_transferable}；不存在报 1284。",

	"sftoken.RoyaltyInfo": "【功能】查询 SFT 的版税配置。\n【返回】Royalty：{recipient, bps}（bps 为万分比）。",

	"sftoken.SlotOf": "【功能】查询指定 token 属于哪个槽位。\n【返回】u64 slot_id。",

	"sftoken.Token": "【功能】查询 token 的完整链上状态。\n【返回】Token：{token_id, slot_id, owner, metadata, value, attribute}。",

	"sftoken.OwnerOf": "【功能】查询指定 token 的当前持有者地址。\n【返回】Address。",

	"sftoken.ValueOf": "【功能】查询指定 token 的当前价值。\n【返回】u64。",

	"sftoken.BalanceOf": "【功能】查询指定 token 的价值余额（注意第二参数是 token_id 而非账户地址）。\n【返回】u64（与 ValueOf 同口径）。",

	"sftoken.AttributeOf": "【功能】查询指定 token 的 attribute 属性字符串。\n【返回】String。",

	"sftoken.IsSlotFrozen": "【功能】查询指定槽位是否处于冻结状态。\n【返回】bool。",

	"sftoken.SlotUpdateAuthor": "【功能】查询指定槽位的内容更新权限人。\n【返回】Address。",

	"sftoken.SlotFreezeAuthority": "【功能】查询指定槽位的冻结权限人。\n【返回】Address。",

	"sftoken.SftOwnerOf": "【功能】查询 SFT 集合的 owner 地址。\n【返回】Address。",

	"sftoken.SlotOwnerOf": "【功能】查询指定槽位的 slot_owner 地址。\n【返回】Address。",

	"sftoken.ApprovalOf": "【功能】查询 spender 在指定 token 上的剩余授权额度。\n【返回】u64（未授权为 0）。",

	// ==================== dex 模块（app_id=6，8 entry + 6 view）====================

	"dex.CreateMarket": "【功能】创建订单簿交易市场（base/quote 代币对），调用者成为市场 authority（管理员）。maker/taker 费率由链上常量固定为 0.3%/0.5%。\n【返回】Address 新市场地址（由链上种子派生）。\n【限制】base≠quote（SameTokenPair 1536）；所有数值参数非 0（ZeroMarketParameter 1537）；同代币对不可重复建市场（MarketAlreadyExists 1541）。",

	"dex.InitializeMarketDid": "【功能】为市场初始化 DID 文档（市场身份/资质展示）。\n【限制】须市场 authority 签名（MarketAuthorityMismatch 1546）；市场须存在（MarketNotFound 1542）；subject_type 必须为 Organization（MarketDidMustBeOrganization 1562）。",

	"dex.DiscloseMarketVcAttestation": "【功能】为市场披露一条 VC 凭证：登记签发方、schema、凭证哈希与签发方签名，供合规展示。\n【限制】须市场 authority 签名；签发方身份须可解析（IdentityFailure 1561）。",

	"dex.PlaceLimitOrder": "【功能】下限价单：先尝试吃单成交，未成交部分按限价挂入订单簿并锁定资金。\n【返回】PlaceOrderResult：{order_id, status, filled_lots, remaining_lots, fill_count, rested（是否纯挂单）}。\n【限制】市场须为 Active 状态（MarketNotActive 1543）；price_tick 须在 [min_tick, max_tick] 内；资金不足报 TokenFailure（1555）。",

	"dex.CancelOrder": "【功能】按链上 order_id 撤单并释放锁定资金。\n【返回】OrderInfo：{order_id, owner, client_order_id, side, limit_tick, original_lots, remaining_lots, reserved_amount, status}。\n【限制】订单须存在（OrderNotFound 1547）；仅订单本人可撤。",

	"dex.CancelByClientOrderId": "【功能】按下单者自己的 client_order_id 撤单（无需记链上单号）。\n【返回】OrderInfo。\n【限制】该 owner 下须存在此 client 单号（ClientOrderNotFound 1548）。",

	"dex.BatchCancel": "【功能】批量撤单并释放锁定资金。\n【返回】OrderInfo JSON 数组。\n【限制】列表非空（EmptyBatchCancel 1549）且最多 32 个（BatchCancelLimitExceeded 1550）；不可重复（DuplicateBatchOrder 1551）。",

	"dex.SetMarketStatus": "【功能】切换市场状态：Active（正常交易）/ CancelOnly（只可撤单）/ Paused（暂停）。\n【限制】仅市场 authority 生效（1546）。",

	"dex.MarketInfo": "【功能】查询市场完整配置。\n【返回】Market：{base_token, quote_token, base_lot_atoms, quote_atoms_per_lot_tick, min_tick, max_tick, max_order_lots, max_fills_per_action, maker_fee_ppm, taker_fee_ppm, authority, status}。",

	"dex.OrderInfoView": "【功能】按 order_id 查询订单实时状态（含锁定量 reserved_amount）。\n【返回】OrderInfo。",

	"dex.BestBidAsk": "【功能】查询订单簿最优买价/卖价档。\n【返回】{bid: {tick, order_count, total_base_lots} 或 null, ask: 同}；空簿一侧为 null。",

	"dex.OrderbookDepth": "【功能】查询买卖各价位档的深度。\n【返回】{bids: [{tick, order_count, total_base_lots}], asks: [...]}。\n【限制】depth 上限 50（InvalidViewLimit 1552）。",

	"dex.OrdersAtLevel": "【功能】分页查询某一价格档上的订单列表。\n【返回】LevelOrderPage：{orders: [OrderInfo], next_order_id（翻页游标，0 表示没有更多）}。\n【限制】limit 1..=100（1552）；档位不存在报 PriceLevelNotFound（1553）；游标非法报 InvalidOrderCursor（1554）。",

	"dex.VaultLiability": "【功能】查询市场金库对某代币的负债总额（所有用户托管资金之和）。\n【返回】u64。",

	// ==================== keyless 模块（app_id=8，8 entry + 8 view）====================

	"keyless.BootstrapKeyless": "【功能】初始化 keyless 注册表：注册内置 Google/Apple OIDC provider 与默认参数（无参数，args 传 {}）。\n【限制】仅 genesis 块 0 可调（GenesisOnly 2113）；只能初始化一次（AlreadyBootstrapped 2114）。",

	"keyless.Authenticate": "【功能】用 OIDC id_token（JWT）+ 临时公钥完成无密钥认证，建立链上会话（后续以会话公钥签名操作）。gas 由赞助池代付（sponsor）。\n【限制】JWT 校验严格：须三段式 JWS（2049）、未过期（2059）且签发时间不过旧（≤10 分钟，JwtTooOld 2061）、issuer/audience 与 provider 匹配（2062-2064）、签名验证通过（2066）；nonce 须以 milon-keyless-v1. 开头且内嵌 ephemeral_pk 的 commitment（2067-2075）；公共赞助每会话仅一次（2090）；provider 须已注册并启用（2091/2093）。",

	"keyless.Bind": "【功能】把 keyless 身份（JWT 认证通过后派生的地址）绑定到链上账户的一个 signer 槽位，使账户可无私钥托管。\n【限制】该身份不可已被绑定（AlreadyBound 2098）；绑定有激活延迟（默认 1 天、下限 1 小时，active_at_ms = bound_at_ms + 延迟）；JWT 校验同 Authenticate；owner 须签名。",

	"keyless.Unbind": "【功能】解除 keyless 身份与账户的绑定。\n【限制】绑定须存在（BindingNotFound 2099）；仅绑定 owner 可解绑（BindingOwnerMismatch 2100）。",

	"keyless.RevokeSession": "【功能】撤销一个活跃会话（会话公钥立即失效）。\n【限制】会话须存在且属该 owner（SessionNotFound 2097）；已撤销报 SessionRevoked（2089）。",

	"keyless.RegisterAudience": "【功能】为 provider 注册允许的 audience（OIDC client ID），之后该 audience 的 JWT 才能通过校验。\n【限制】provider 须已注册（ProviderNotRegistered 2091）；audience 长度 1..=256 字节（AudienceInvalid 2118）；内置 provider 元数据不可修改（BuiltinProviderImmutable 2122）。",

	"keyless.CreateStaticProvider": "【功能】注册自定义「静态 JWK」验证 provider（自带公钥集，不依赖内置 OIDC）。provider_id 由 (issuer, salt) 派生。\n【限制】issuer 必须 https:// 开头（IssuerUrlInvalid 2094）；名称非空且 ≤64 字节（2095）；密钥 1..=8 把（StaticKeyCountInvalid 2124）；kid 唯一且 1..=128 字节（2125）；不可用保留命名空间（ReservedProviderId 2123）；重复注册报 ProviderAlreadyRegistered（2092）。",

	"keyless.UpdateStaticProviderKeys": "【功能】轮换自定义 provider 的验证密钥集；invalidate_sessions=true 可使全部旧会话立即失效。\n【限制】仅静态 provider 可改（BuiltinProviderImmutable 2122）；仅 provider owner（ProviderOwnerMismatch 2121）。",

	"keyless.GetSessions": "【功能】查询某账户的全部活跃会话。\n【返回】[{session_id, provider_id, audience_hash, session_version, session_pubkey, expires_at_ms, created_at_ms}]。",

	"keyless.GetBinding": "【功能】查询某 keyless 地址的绑定信息。\n【返回】null 或 {owner, slot_index, bound_at_ms, active_at_ms}。",

	"keyless.ListProviders": "【功能】列出全部 provider（内置 + 自定义，无参数）。\n【返回】[{provider_id, kind: \"BuiltinOidc\"|\"StaticJwk\", display_name, issuer, allowed_algs, max_id_token_age_ms, max_session_ttl_ms, owner, keys, key_revision}]。",

	"keyless.GetProvider": "【功能】按 ID 查询单个 provider。\n【返回】KeylessProvider 或 null。",

	"keyless.GetProviderStatus": "【功能】查询 provider 的启用状态与会话版本。\n【返回】{enabled, session_version} 或 null。",

	"keyless.GetAudience": "【功能】查询某 provider 下某 audience 的注册信息。\n【返回】{provider_id, audience, registered_at_ms} 或 null。",

	"keyless.GetParams": "【功能】查询 keyless 全局参数（无参数）。\n【返回】KeylessParams：{max_session_ttl_ms（默认 1 天）, bind_activation_delay_ms（默认 1 天）}。",

	"keyless.IsKeylessAccount": "【功能】判断某地址是否已绑定 keyless 身份。\n【返回】bool。",

	// ==================== lucky_box 模块（app_id=9，4 entry + 2 view）====================

	"lucky_box.CreateEqual": "【功能】创建「等概率」盲盒：total_amount 平分为 claim_count 份，每份金额相同；资金从创建者账户托管进盒子资产池。\n【返回】u64（每份金额）。\n【限制】box_id 不可重复（BoxAlreadyExists 2305）；金额/份数须非 0（InvalidDefinition 2310）；份数 ≤10000（TooManyItems 2311）；创建者须有足够代币托管；资格 bloom bits 32..=32768 字节；未设过期时间默认 24 小时（DEFAULT_EXPIRATION_MS）。",

	"lucky_box.CreateLucky": "【功能】创建「拼手气」盲盒：领取时按链上安全随机数分配不等金额；参数与资金托管同 CreateEqual。\n【返回】u64。\n【限制】同 CreateEqual；随机数不可用报 SecureRandomnessUnavailable（2314）。",

	"lucky_box.Claim": "【功能】领取盲盒一份：Equal 得等份金额、Lucky 得随机金额，代币从托管转入领取者。gas 由赞助池代付（sponsor）。\n【限制】盒子须存在且活跃（BoxNotFound 2304 / BoxNotActive 2306）、未过期（BoxExpired 2307）、尚有余量（BoxExhausted 2309）；须通过资格 bloom 校验（NotEligible 2312）；每人限领一次（QuotaExceeded 2313）。",

	"lucky_box.Refund": "【功能】盒子过期后回收剩余资产（未领完的资金退回创建者）。无需签名者，任何人可触发执行。\n【限制】盒子必须已过期（BoxNotExpired 2308）且存在（2304）。",

	"lucky_box.BoxView": "【功能】查询盒子状态。\n【返回】BoxState：{creator, box_id, status, claim_count, claimed_count, expires_at_ms, randomness_version}。",

	"lucky_box.AssetPool": "【功能】查询盒子资产池。\n【返回】AssetPoolState：{asset: {variant:\"Token\", value:{token}}, total_amount, remaining_amount, allocation: \"Equal\"|\"Lucky\"}。",

	// ==================== social 模块（app_id=10，24 entry + 8 view）====================

	"social.UpsertSocialProfile": "【功能】写入/更新自己的社交 profile URI（链上只存 URI，内容存链下）。\n【限制】须有有效未停用 DID（DidNotActive 2560）；URI 须合法（InvalidUri 2563）。",

	"social.RegisterSourceApp": "【功能】注册「源应用」（公共数据发布主体），链上分配自增 ID。\n【返回】u32 source_app_id。\n【限制】owner 须有有效 DID；ID 序列耗尽报 SourceAppIdExhausted（2572）。",

	"social.AddSourceAppPublisher": "【功能】为源应用添加发布者（允许其发布公共数据）。\n【限制】仅应用 owner（SourceAppOwnerRequired 2567）；应用须存在（SourceAppNotFound 2564）；已是发布者报 2569。",

	"social.RemoveSourceAppPublisher": "【功能】移除源应用的某个发布者。\n【限制】仅 owner；owner 自身的发布者身份不可移除（CannotRemoveOwnerPublisher 2571）；目标须是发布者（2570）。",

	"social.TransferSourceAppOwner": "【功能】转让源应用所有权。\n【限制】仅当前 owner（2567）。",

	"social.DeactivateSourceApp": "【功能】停用源应用（暂停其数据发布；停用后发布报 SourceAppPublishingDisabled 2566）。\n【限制】仅 owner。",

	"social.ReactivateSourceApp": "【功能】重新启用已停用的源应用。\n【限制】仅 owner。",

	"social.UpsertPublicDataUri": "【功能】发布者为 (source_app_id, dataset_kind, scope_key) 三元组写入/更新公共数据 URI（链上数据锚点）。\n【限制】调用者须是该应用发布者（SourceAppPublisherRequired 2568）；应用须存在且未停用发布（2564/2566）；scope_key/URI 须合法（2573/2563）。",

	"social.RetirePublicDataUri": "【功能】将数据锚点下线（retired 标记，URI 保留）。\n【限制】仅发布者；锚点须存在（DataAnchorNotFound 2574）且未下线（DataAnchorRetired 2575）。",

	"social.RestorePublicDataUri": "【功能】恢复已下线的数据锚点并更新 URI。\n【限制】仅发布者；锚点须存在且处于 retired 状态（2574/2576）。",

	"social.CreateCommunity": "【功能】创建社区，链上分配自增 community_id。\n【返回】u32 community_id。\n【限制】每 creator 最多 8 个社区（CommunityCreationLimitReached 2592）；URI 须合法（2563）。",

	"social.UpdateCommunityUri": "【功能】社区 owner 更新社区元数据 URI。\n【限制】仅 owner（CommunityOwnerRequired 2580）；社区须存在（2577）且未停用（2579）。",

	"social.SetCommunityJoinMode": "【功能】owner 切换社区加入模式（Open/Approval）。\n【限制】仅 owner；社区须存在且未停用。",

	"social.TransferCommunityOwner": "【功能】转让社区所有权。\n【限制】仅当前 owner。",

	"social.DeactivateCommunity": "【功能】停用社区（成员与管理变更冻结）。\n【限制】仅 owner。",

	"social.GrantCommunityAdmin": "【功能】owner 授予某地址社区管理员权限。\n【限制】仅 owner；已是管理员报 CommunityAdminAlreadyExists（2583）。",

	"social.RevokeCommunityAdmin": "【功能】owner 撤销某地址的社区管理员权限。\n【限制】仅 owner；目标须是管理员（CommunityAdminNotFound 2584）。",

	"social.JoinCommunity": "【功能】直接加入社区（仅 Open 模式；Approval 模式须走申请审批）。\n【限制】模式须为 Open（InvalidJoinMode 2585）；成员数不超 10000（CommunityMemberLimitReached 2593）；不可重复加入（2586）。",

	"social.RequestJoinCommunity": "【功能】在 Approval 模式社区提交加入申请。\n【限制】模式须为 Approval（2585）；不可重复申请（JoinRequestAlreadyPending 2588）；不可已是成员（2586）。",

	"social.CancelJoinRequest": "【功能】撤回自己的加入申请。\n【限制】须有 pending 申请（JoinRequestNotPending 2589）。",

	"social.ApproveJoinRequest": "【功能】管理员批准某用户的加入申请（成员数 +1）。\n【限制】须管理员或 owner（CommunityAdminRequired 2581）；须有 pending 申请（2589）；成员数不超 10000（2593）。",

	"social.RejectJoinRequest": "【功能】管理员拒绝某用户的加入申请。\n【限制】同 ApproveJoinRequest：管理员权限 + pending 申请存在。",

	"social.LeaveCommunity": "【功能】成员主动退出社区。\n【限制】须是活跃成员（MembershipNotActive 2587）；owner 不能退出（CannotLeaveCommunityOwner 2595，须先转让）。",

	"social.RemoveCommunityMember": "【功能】管理员移除某成员。\n【限制】须管理员（2581）；目标须是成员（2587）；不能移除 owner（CannotRemoveCommunityOwner 2582）。",

	"social.SocialProfile": "【功能】查询某地址的 profile URI。\n【返回】String。",

	"social.SourceApp": "【功能】查询源应用信息。\n【返回】SourceApp：{owner, publishing_enabled}；不存在报 2564。",

	"social.IsSourceAppPublisher": "【功能】判断某地址是否为源应用的发布者。\n【返回】bool。",

	"social.PublicDataAnchor": "【功能】查询公共数据锚点。\n【返回】PublicDataAnchor：{uri, retired}；不存在报 2574。",

	"social.Community": "【功能】查询社区控制信息。\n【返回】CommunityControl：{owner, join_mode, mutations_enabled, member_count, uri}。",

	"social.IsCommunityAdmin": "【功能】判断某地址是否为社区管理员（含 owner）。\n【返回】bool。",

	"social.JoinRequest": "【功能】查询某成员是否有 pending 的加入申请。\n【返回】bool。",

	"social.CommunityMembership": "【功能】查询某地址是否为社区成员。\n【返回】bool。",

	// ==================== demo 模块（app_id=255，10 entry + 8 view）====================

	"demo.OpenOrder": "【功能】演示订单支付流第一步：operator 开一个演示订单（收款托管户头）。\n【限制】order_id 不可重复（OrderAlreadyExists 65285）。",

	"demo.PayOrder": "【功能】演示订单支付流第二步：付款人向订单打入指定代币与数量（进订单托管）。\n【限制】订单须存在（OrderNotFound 65284）；付款人余额须充足。",

	"demo.SettleOrder": "【功能】演示订单支付流第三步：operator 把订单托管余额结算转出给指定收款地址。\n【限制】须与开单相同的 operator（OrderOperatorMismatch 65286）；订单须存在（65284）；结算额不得超过托管余额。",

	"demo.OrderBalance": "【功能】查询演示订单在某代币上的托管余额。\n【返回】u64。",

	"demo.OpenGasSponsorPool": "【功能】注册 gas 赞助池，使 sponsor 标记的指令（keyless.Authenticate、lucky_box.Claim、demo.ClaimSponsoredScore 等）可由该池代付 gas。\n【返回】u16 sponsor_seed（池编号，供赞助类指令引用）。\n【限制】pool 地址的管理员（signer_lookup 从 pool 参数解析 admin）必须与交易付款人一致。",

	"demo.ClaimSponsoredScore": "【功能】演示从赞助池领取 gas 赞助并给领取者记积分。gas 由赞助池代付（sponsor）。\n【限制】池须已注册（SponsorPoolNotFound 65287）。",

	"demo.SponsorPoolOf": "【功能】按池编号查询赞助池地址。\n【返回】Address。",

	"demo.InitPool": "【功能】初始化一个演示池（标签/积分载体），由 pool 资源账户签名。\n【限制】不可重复初始化（PoolAlreadyExists 65281）。",

	"demo.InitDex": "【功能】同 InitPool，以 dex 为主题的演示初始化。\n【限制】不可重复初始化（65281）。",

	"demo.SetLabel": "【功能】修改池的标签。\n【限制】池须存在（PoolNotFound 65280）。",

	"demo.BatchCredit": "【功能】给一批接收者在池内加积分（触发 EventCreditApplied 事件）。\n【限制】池须存在（65280）；接收者须已在池内登记（RecipientNotFound 65282）。",

	"demo.LabelOf": "【功能】查询池标签。\n【返回】Label：{text}。",

	"demo.ScoreOf": "【功能】查询池内某账户的积分。\n【返回】u64。",

	"demo.SetTierCap": "【功能】设置某层级的积分上限（配合领用限速演示）。\n【限制】池须存在（65280）。",

	"demo.TierCapOf": "【功能】查询某层级的积分上限。\n【返回】u64。",

	"demo.EchoMode": "【功能】枚举编码回显演示：传入 DemoMode 原样返回（用于验证枚举编码）。\n【返回】DemoMode。",

	"demo.LabelTotal": "【功能】map 类型演示：对传入的标签-数值映射求和。\n【返回】u32。",

	"demo.SpecialTypes": "【功能】复杂类型编码演示（enum/option/vec/map/tuple 全覆盖）。\n【返回】u32（各输入的汇总校验值）。",
}

// IDL 参数中文文档映射。键为 "app.Method:arg"，值为一行说明（含义 + 输入格式 + 约束）。
var idlArgDocs = map[string]string{
	// ==================== system ====================

	"system.BootstrapEpochClock:start_timestamp_ms": "epoch 计时起点，Unix 毫秒时间戳（十进制数字）",
	"system.BootstrapEpochClock:min_duration_ms":    "epoch 最短时长，毫秒（十进制数字）",

	"system.BootstrapValidatorSetConfig:max_validator_set_size":    "验证人集合最大人数，十进制数字",
	"system.BootstrapValidatorSetConfig:group_threshold_ratio_num": "组阈值比例分子",
	"system.BootstrapValidatorSetConfig:group_threshold_ratio_den": "组阈值比例分母（分子 ≤ 分母）",
	"system.BootstrapValidatorSetConfig:block_threshold_ratio_num": "出块阈值比例分子",
	"system.BootstrapValidatorSetConfig:block_threshold_ratio_den": "出块阈值比例分母（分子 ≤ 分母）",
	"system.BootstrapValidatorSetConfig:batch_committee_ratio_num": "批次委员会比例分子",
	"system.BootstrapValidatorSetConfig:batch_committee_ratio_den": "批次委员会比例分母（分子 ≤ 分母）",

	"system.RegisterValidatorIdentity:validator":         "验证人地址，base58 字符串",
	"system.RegisterValidatorIdentity:consensus_account": "共识账户地址，须其私钥参与交易签名",
	"system.RegisterValidatorIdentity:consensus_pubkey":  "共识公钥，hex 字符串（可带 0x）",
	"system.RegisterValidatorIdentity:bls_pubkey":        "BLS 公钥，hex 字符串（48 字节）",
	"system.RegisterValidatorIdentity:ed25519_pubkey":    "ed25519 公钥，hex 字符串（32 字节）",

	"system.PrepareValidatorSet:target_epoch":       "目标 epoch 编号，十进制数字",
	"system.PrepareValidatorSet:validator_set_seed": "32 字节随机种子，64 位 hex 字符串（可带 0x）",

	// ==================== account ====================

	"account.Create:owner_pk":        "账户所有者公钥，0x 开头 66 位 hex（secp256k1 压缩）或 64 位 hex（ed25519），也接受 base58",
	"account.EnsureAccount:owner_pk": "账户所有者公钥，格式同 account.Create",

	"account.CreateMultisig:owner":     "多签账户地址，须其私钥参与交易签名",
	"account.CreateMultisig:signers":   "signer 公钥 JSON 数组（不含 owner 自身），每项为 hex 或 base58 公钥",
	"account.CreateMultisig:weights":   "与 signers 等长的权重数组，hex 字符串或数字数组，每项 1..=255",
	"account.CreateMultisig:threshold": "生效阈值（权重和达标线），十进制数字",

	"account.AddSigner:owner":     "多签账户地址，须其私钥参与交易签名",
	"account.AddSigner:signer_pk": "新 signer 公钥，hex 或 base58 字符串",
	"account.AddSigner:weight":    "该 signer 权重，十进制数字（1..=255）",

	"account.AddSigners:owner":     "多签账户地址，须其私钥参与交易签名",
	"account.AddSigners:signers":   "signer 公钥 JSON 数组，每项为 hex 或 base58 公钥",
	"account.AddSigners:weights":   "与 signers 等长的权重数组，hex 字符串或数字数组",
	"account.AddSigners:threshold": "添加后的新阈值，十进制数字",

	"account.RemoveSigner:owner":     "多签账户地址，须其私钥参与交易签名",
	"account.RemoveSigner:index":     "要移除的 signer 槽位号，十进制数字（0..63）",
	"account.RemoveSigner:threshold": "移除后的新阈值，十进制数字",

	"account.SetThreshold:owner":     "多签账户地址，须其私钥参与交易签名",
	"account.SetThreshold:threshold": "新阈值，十进制数字（不超过当前总权重）",

	"account.SetSignerWeight:owner":  "多签账户地址，须其私钥参与交易签名",
	"account.SetSignerWeight:index":  "signer 槽位号，十进制数字（0..63）",
	"account.SetSignerWeight:weight": "新权重，十进制数字（1..=255）",

	"account.VoteInit:owner":       "账户地址，任一有效 signer 签名即可（role=any_signer）",
	"account.VoteInit:intent_hash": "意图哈希，64 位 hex 字符串（32 字节），须与 proposal 内容匹配",
	"account.VoteInit:proposal":    "提案对象 JSON：{\"instructions\":[编码指令 bytes（hex）数组],\"auth_bit\":授权位图数字}，编码后 ≤1024 字节",
	"account.Vote:owner":           "账户地址，任一有效 signer 签名即可（role=any_signer）",
	"account.Vote:intent_hash":     "要投票的意图哈希，64 位 hex 字符串",

	"account.GetAccount:owner":       "账户地址，base58 字符串",
	"account.ListSigners:owner":      "账户地址，base58 字符串",
	"account.ResolveSigners:owner":   "账户地址，base58 字符串",
	"account.ResolveSigners:sig_bit": "signer 槽位位图，u64 十进制数字（bit i = 槽位 i，如 5 = 槽位 0 和 2）",
	"account.ResolveSigners:policy":  "校验策略 JSON：{\"min_signers\":最少人数（数字或 null）}",
	"account.GetVote:owner":          "账户地址，base58 字符串",
	"account.GetVote:intent_hash":    "要查询的意图哈希，64 位 hex 字符串",
	"account.ListActiveVotes:owner":  "账户地址，base58 字符串",

	// ==================== token ====================

	"token.Create:token":                              "新代币资源地址（由发币密钥对地址派生，base58），该私钥须参与交易签名",
	"token.Create:owner":                              "代币 owner 地址（管理权限人），base58 字符串",
	"token.Create:metadata":                           "元数据 JSON 对象：{\"name\":名称,\"symbol\":符号,\"decimals\":精度（常用 6）,\"icon\":图标 URL,\"uri\":URI}",
	"token.AbandonOwner:token":                        "代币资源地址，base58 字符串（原生 MIL 为 M11on 开头）",
	"token.TransferOwner:token":                       "代币资源地址，base58 字符串",
	"token.TransferOwner:to":                          "新 owner 地址，base58 字符串",
	"token.AbandonFreezer:token":                      "代币资源地址，base58 字符串",
	"token.TransferFreezer:token":                     "代币资源地址，base58 字符串",
	"token.TransferFreezer:to":                        "新 freezer 地址，base58 字符串",
	"token.Mint:token":                                "代币资源地址，base58 字符串",
	"token.Mint:to":                                   "接收地址，base58 字符串",
	"token.Mint:amount":                               "增发数量，十进制数字，单位为最小单位（6 位精度时 1 token = 10^6）",
	"token.MintBatch:token":                           "代币资源地址，base58 字符串",
	"token.MintBatch:to":                              "接收地址 JSON 数组，与 amount 等长",
	"token.MintBatch:amount":                          "数量 JSON 数组（最小单位），与 to 等长",
	"token.Burn:holder":                               "持有人地址，须其私钥参与交易签名",
	"token.Burn:token":                                "代币资源地址，base58 字符串",
	"token.Burn:amount":                               "销毁数量，十进制数字（最小单位）",
	"token.Transfer:from":                             "转出地址，须其私钥参与交易签名",
	"token.Transfer:token":                            "代币资源地址，base58 字符串",
	"token.Transfer:to":                               "转入地址，base58 字符串",
	"token.Transfer:amount":                           "转账数量，十进制数字（最小单位）",
	"token.TransferBatch:from":                        "转出地址，须其私钥参与交易签名",
	"token.TransferBatch:token":                       "代币资源地址，base58 字符串",
	"token.TransferBatch:to":                          "转入地址 JSON 数组，与 amount 等长",
	"token.TransferBatch:amount":                      "数量 JSON 数组（最小单位），与 to 等长",
	"token.Freeze:token":                              "代币资源地址，base58 字符串",
	"token.Freeze:holder":                             "被冻结地址，base58 字符串",
	"token.Freeze:amount":                             "冻结数量，十进制数字（最小单位）",
	"token.Unfreeze:token":                            "代币资源地址，base58 字符串",
	"token.Unfreeze:holder":                           "被解冻地址，base58 字符串",
	"token.Unfreeze:amount":                           "解冻数量，十进制数字（最小单位）",
	"token.Approve:owner":                             "代币持有人地址，须其私钥参与交易签名",
	"token.Approve:token":                             "代币资源地址，base58 字符串",
	"token.Approve:spender":                           "被授权地址，base58 字符串",
	"token.Approve:amount":                            "授权额度，十进制数字（最小单位）",
	"token.Revoke:owner":                              "代币持有人地址，须其私钥参与交易签名",
	"token.Revoke:token":                              "代币资源地址，base58 字符串",
	"token.Revoke:spender":                            "被撤销授权的地址，base58 字符串",
	"token.TransferFrom:spender":                      "被授权人地址，须其私钥参与交易签名",
	"token.TransferFrom:token":                        "代币资源地址，base58 字符串",
	"token.TransferFrom:from":                         "代币实际持有人地址，base58 字符串",
	"token.TransferFrom:amount":                       "划转数量，十进制数字（最小单位）",
	"token.SetIcon:token":                             "代币资源地址，base58 字符串",
	"token.SetIcon:icon_url":                          "图标 URL 字符串",
	"token.SetUri:token":                              "代币资源地址，base58 字符串",
	"token.SetUri:uri":                                "元数据 URI 字符串",
	"token.CreateWithCompliance:token":                "新代币资源地址（由发币密钥对地址派生，base58），该私钥须参与交易签名",
	"token.CreateWithCompliance:owner":                "代币 owner 地址，base58 字符串",
	"token.CreateWithCompliance:metadata":             "元数据 JSON 对象（同 token.Create）",
	"token.CreateWithCompliance:credential_id":        "必需凭证 schema ID 字符串（与 identity 组织注册声明的 schema 对应）",
	"token.SetComplianceMode:token":                   "代币资源地址，base58 字符串",
	"token.SetComplianceMode:mode":                    "合规模式枚举字符串：\"All\"（须全部凭证）或 \"Any\"（任一即可）",
	"token.AddComplianceRequirement:token":            "代币资源地址，base58 字符串",
	"token.AddComplianceRequirement:credential_id":    "要添加的凭证 schema ID 字符串",
	"token.RemoveComplianceRequirement:token":         "代币资源地址，base58 字符串",
	"token.RemoveComplianceRequirement:credential_id": "要移除的凭证 schema ID 字符串",
	"token.ClearComplianceRequirements:token":         "代币资源地址，base58 字符串",
	"token.ClaimFaucet:claimer":                       "领取者地址，须其私钥参与交易签名（每地址 24h 冷却，勿重试）",
	"token.BalanceOf:token":                           "代币资源地址，base58 字符串",
	"token.BalanceOf:account":                         "查询地址，base58 字符串",
	"token.FrozenOf:token":                            "代币资源地址，base58 字符串",
	"token.FrozenOf:account":                          "查询地址，base58 字符串",
	"token.ApprovalOf:token":                          "代币资源地址，base58 字符串",
	"token.ApprovalOf:owner":                          "代币持有人地址，base58 字符串",
	"token.ApprovalOf:spender":                        "被授权地址，base58 字符串",
	"token.TotalSupply:token":                         "代币资源地址，base58 字符串",
	"token.Metadata:token":                            "代币资源地址，base58 字符串",
	"token.Compliance:token":                          "代币资源地址，base58 字符串",
	"token.FaucetCooldownRemaining:account":           "查询地址，base58 字符串",

	// ==================== staking ====================

	"staking.CreateValidator:operator":                   "运营方地址，须其私钥参与交易签名",
	"staking.CreateValidator:validator":                  "验证人地址，base58 字符串（须与 operator、共识账户互不相同）",
	"staking.CreateValidator:consensus_account":          "共识账户地址，须其私钥参与交易签名",
	"staking.CreateValidator:consensus_pubkey":           "共识公钥，hex 字符串（可带 0x）",
	"staking.CreateValidator:bls_pubkey":                 "BLS 公钥，hex 字符串（48 字节）",
	"staking.CreateValidator:commission_rate_bps":        "佣金率，基点（万分比）数字，≤10000 即 ≤100%",
	"staking.SetStakingToken:token":                      "质押代币资源地址，base58 字符串（通常为 MIL）",
	"staking.InitializeEpochConfig:reward_per_validator": "每验证人每 epoch 奖励，十进制数字（质押代币最小单位）",
	"staking.JoinCandidatePool:operator":                 "验证人运营方地址，须其私钥参与交易签名",
	"staking.JoinCandidatePool:validator":                "验证人地址，base58 字符串",
	"staking.LeaveCandidatePool:operator":                "验证人运营方地址，须其私钥参与交易签名",
	"staking.LeaveCandidatePool:validator":               "验证人地址，base58 字符串",
	"staking.DeclareValidatorAvailability:operator":      "验证人运营方地址，须其私钥参与交易签名",
	"staking.DeclareValidatorAvailability:validator":     "验证人地址，base58 字符串",
	"staking.FundRewardTreasury:funder":                  "出资地址，须其私钥参与交易签名",
	"staking.FundRewardTreasury:amount":                  "存入金额，十进制数字（质押代币最小单位）",
	"staking.Stake:owner":                                "质押人地址，须其私钥参与交易签名",
	"staking.Stake:validator":                            "验证人地址，base58 字符串",
	"staking.Stake:amount":                               "质押数量，十进制数字（最小单位，不得低于最小质押额）",
	"staking.CancelPendingStake:owner":                   "质押人地址，须其私钥参与交易签名",
	"staking.CancelPendingStake:validator":               "验证人地址，base58 字符串",
	"staking.CancelPendingStake:amount":                  "撤销数量，十进制数字（最小单位，不得超过 pending 量）",
	"staking.ClaimRewards:owner":                         "质押人地址，须其私钥参与交易签名",
	"staking.ClaimRewards:validator":                     "验证人地址，base58 字符串",
	"staking.ClaimOperatorRewards:operator":              "验证人运营方地址，须其私钥参与交易签名",
	"staking.ClaimOperatorRewards:validator":             "验证人地址，base58 字符串",
	"staking.RequestUnstake:owner":                       "质押人地址，须其私钥参与交易签名",
	"staking.RequestUnstake:validator":                   "验证人地址，base58 字符串",
	"staking.RequestUnstake:shares":                      "解冻份额数量，十进制数字（注意是 shares 不是代币数量）",
	"staking.ValidatorProfile:validator":                 "验证人地址，base58 字符串",
	"staking.ValidatorPool:validator":                    "验证人地址，base58 字符串",
	"staking.StakePosition:owner":                        "质押人地址，base58 字符串",
	"staking.StakePosition:validator":                    "验证人地址，base58 字符串",
	"staking.PositionSummary:owner":                      "质押人地址，base58 字符串",
	"staking.PositionSummary:validator":                  "验证人地址，base58 字符串",
	"staking.EpochTransition:epoch":                      "epoch 编号，十进制数字",
	"staking.HeldPrincipal:owner":                        "查询地址，base58 字符串",
	"staking.ListDeclaredValidatorsForEpoch:epoch":       "epoch 编号，十进制数字",

	// ==================== identity ====================

	"identity.Create:subject":                                    "DID 主体地址，须其私钥参与交易签名",
	"identity.Create:doc":                                        "DID 文档 JSON：{\"subject_type\":\"Personal\"|\"Organization\",\"keys\":[{\"public_key\":公钥,\"label\":字符串或 null}],\"services\":[{\"label\":名称,\"service_endpoint\":URL}],\"avatar_uri\":字符串}",
	"identity.CreateWithAlias:subject":                           "DID 主体地址，须其私钥参与交易签名",
	"identity.CreateWithAlias:doc":                               "DID 文档 JSON（同 identity.Create）",
	"identity.CreateWithAlias:name":                              "名称对象 JSON：{\"alias\":别名字符串,\"suffix\":数字后缀}，全局唯一",
	"identity.AddKey:subject":                                    "DID 主体地址，须其私钥参与交易签名",
	"identity.AddKey:input":                                      "密钥对象 JSON：{\"public_key\":公钥 hex/base58,\"label\":字符串或 null}",
	"identity.UpdateKey:subject":                                 "DID 主体地址，须其私钥参与交易签名",
	"identity.UpdateKey:id":                                      "要更新的密钥 id，十进制数字",
	"identity.UpdateKey:input":                                   "新密钥对象 JSON（同 AddKey）",
	"identity.RemoveKey:subject":                                 "DID 主体地址，须其私钥参与交易签名",
	"identity.RemoveKey:id":                                      "要移除的密钥 id，十进制数字（不可移除最后一把）",
	"identity.AddService:subject":                                "DID 主体地址，须其私钥参与交易签名",
	"identity.AddService:input":                                  "服务对象 JSON：{\"label\":名称,\"service_endpoint\":URL 字符串（须绝对 URI）}",
	"identity.UpdateService:subject":                             "DID 主体地址，须其私钥参与交易签名",
	"identity.UpdateService:id":                                  "要更新的服务 id，十进制数字",
	"identity.UpdateService:input":                               "新服务对象 JSON（同 AddService）",
	"identity.RemoveService:subject":                             "DID 主体地址，须其私钥参与交易签名",
	"identity.RemoveService:id":                                  "要移除的服务 id，十进制数字",
	"identity.SetAvatarUri:subject":                              "DID 主体地址，须其私钥参与交易签名",
	"identity.SetAvatarUri:avatar_uri":                           "头像 URI 字符串，长度 1-512 字节",
	"identity.Deactivate:subject":                                "DID 主体地址，须其私钥参与交易签名",
	"identity.SetAlias:subject":                                  "DID 主体地址，须其私钥参与交易签名",
	"identity.SetAlias:name":                                     "名称对象 JSON：{\"alias\":别名字符串,\"suffix\":数字后缀}，全局唯一",
	"identity.RegisterOrganization:subject":                      "组织地址（须为 Organization 类型 DID），须其私钥参与交易签名",
	"identity.RegisterOrganization:roles":                        "角色字符串 JSON 数组：\"VcIssuer\"（VC 签发方）和/或 \"KycProvider\"（KYC 服务方）",
	"identity.RegisterOrganization:credential_schemas":           "凭证 schema 字符串 JSON 数组，如 [\"kyc_basic_v1\"]；声明任一角色时至少 1 个",
	"identity.UpdateOrganizationCapabilities:subject":            "组织地址，须其私钥参与交易签名",
	"identity.UpdateOrganizationCapabilities:roles":              "角色字符串 JSON 数组（同 RegisterOrganization）",
	"identity.UpdateOrganizationCapabilities:credential_schemas": "凭证 schema 字符串 JSON 数组（同 RegisterOrganization）",
	"identity.DeactivateOrganization:subject":                    "组织地址，须其私钥参与交易签名",
	"identity.DiscloseVcAttestation:subject":                     "凭证持有人地址，须其私钥参与交易签名",
	"identity.DiscloseVcAttestation:issuer":                      "凭证签发方地址，base58 字符串（其 DID 须存在且未停用）",
	"identity.DiscloseVcAttestation:issuer_key_id":               "签发方用于签名的密钥序号，十进制数字",
	"identity.DiscloseVcAttestation:credential_schema":           "凭证 schema 字符串（须与签发方注册声明一致）",
	"identity.DiscloseVcAttestation:credential_hash":             "凭证内容哈希，64 位 hex 字符串（32 字节，可带 0x）",
	"identity.DiscloseVcAttestation:valid_until_ms":              "失效时间，Unix 毫秒时间戳数字；永久有效传 null",
	"identity.DiscloseVcAttestation:issuer_signature":            "签发方对凭证内容的签名，hex 或 base58 字符串（链上验签）",
	"identity.RemoveVcDisclosure:subject":                        "凭证持有人地址，须其私钥参与交易签名",
	"identity.RemoveVcDisclosure:issuer":                         "凭证签发方地址，base58 字符串",
	"identity.RemoveVcDisclosure:credential_schema":              "要撤下的凭证 schema 字符串",
	"identity.RevokeVcAttestation:issuer":                        "凭证签发方地址，须其私钥参与交易签名（仅原签发方可撤销）",
	"identity.RevokeVcAttestation:subject":                       "凭证持有人地址，base58 字符串",
	"identity.RevokeVcAttestation:credential_schema":             "要撤销的凭证 schema 字符串",

	"identity.VcAttestationCore:subject":                   "凭证持有人地址，base58 字符串",
	"identity.VcAttestationCore:credential_schema":         "凭证 schema 字符串",
	"identity.VcAttestationCore:issuer":                    "凭证签发方地址，base58 字符串",
	"identity.VcAttestationLifecycle:subject":              "凭证持有人地址，base58 字符串",
	"identity.VcAttestationLifecycle:credential_schema":    "凭证 schema 字符串",
	"identity.VcAttestationLifecycle:issuer":               "凭证签发方地址，base58 字符串",
	"identity.AcceptedVcIssuerIndexMeta:subject":           "DID 主体地址，base58 字符串",
	"identity.AcceptedVcIssuerIndexMeta:credential_schema": "凭证 schema 字符串",
	"identity.AcceptedVcIssuers:subject":                   "DID 主体地址，base58 字符串",
	"identity.AcceptedVcIssuers:credential_schema":         "凭证 schema 字符串",
	"identity.DisclosedVcSchemas:subject":                  "DID 主体地址，base58 字符串",
	"identity.DisclosedVcs:subject":                        "DID 主体地址，base58 字符串",
	"identity.DisclosedVcs:offset":                         "分页起始偏移，十进制数字",
	"identity.DisclosedVcs:limit":                          "单页条数，十进制数字",
	"identity.HasValidVcFromIssuer:subject":                "DID 主体地址，base58 字符串",
	"identity.HasValidVcFromIssuer:issuer":                 "凭证签发方地址，base58 字符串",
	"identity.HasValidVcFromIssuer:credential_schema":      "凭证 schema 字符串",
	"identity.Core:subject":                                "DID 主体地址，base58 字符串",
	"identity.Document:subject":                            "DID 主体地址，base58 字符串",
	"identity.KeyIndex:subject":                            "DID 主体地址，base58 字符串",
	"identity.Keys:subject":                                "DID 主体地址，base58 字符串",
	"identity.Key:subject":                                 "DID 主体地址，base58 字符串",
	"identity.Key:id":                                      "密钥 id，十进制数字",
	"identity.ServiceIndex:subject":                        "DID 主体地址，base58 字符串",
	"identity.Services:subject":                            "DID 主体地址，base58 字符串",
	"identity.Service:subject":                             "DID 主体地址，base58 字符串",
	"identity.Service:id":                                  "服务 id，十进制数字",
	"identity.Alias:subject":                               "DID 主体地址，base58 字符串",
	"identity.Avatar:subject":                              "DID 主体地址，base58 字符串",
	"identity.UpdatedAt:subject":                           "DID 主体地址，base58 字符串",
	"identity.Deactivated:subject":                         "DID 主体地址，base58 字符串",
	"identity.NameBinding:name":                            "名称对象 JSON：{\"alias\":别名字符串,\"suffix\":数字后缀}",
	"identity.CredentialDefinition:credential_id":          "全局凭证 ID 字符串",
	"identity.OrganizationCapabilities:subject":            "组织地址，base58 字符串",
	"identity.OrganizationStatus:subject":                  "组织地址，base58 字符串",
	"identity.OrganizationUpdatedAt:subject":               "组织地址，base58 字符串",
	"identity.CredentialId:issuer":                         "凭证签发方地址，base58 字符串",
	"identity.CredentialId:credential_schema":              "凭证 schema 字符串",

	// ==================== sftoken ====================

	"sftoken.CreateSft:sft":                                 "SFT 资源账户地址，须其私钥参与交易签名",
	"sftoken.CreateSft:owner":                               "SFT 初始 owner 地址，base58 字符串",
	"sftoken.CreateSft:metadata":                            "集合元数据字符串（通常为 JSON 字符串，如 '{\"name\":\"…\",\"desc\":\"…\"}'）",
	"sftoken.CreateSft:royalty_bps":                         "版税基点（万分比），如 50 = 0.5%",
	"sftoken.CreateSlot:sft":                                "SFT 资源地址，base58 字符串",
	"sftoken.CreateSlot:creator":                            "SFT owner 地址，须其私钥参与交易签名（仅 owner 可建槽位）",
	"sftoken.CreateSlot:metadata":                           "槽位元数据字符串，如 \"Level-1 VIP Card\"",
	"sftoken.CreateSlot:attribute":                          "槽位属性字符串，如 \"level=1\"",
	"sftoken.CreateSlot:is_transferable":                    "true/false：槽位下 token 是否允许部分转移与合并",
	"sftoken.CreateSlot:slot_owner":                         "槽位 owner（即铸造权限人）地址，base58 字符串；不设则传 null",
	"sftoken.CreateSlot:update_author":                      "内容更新权限人地址，base58 字符串；不设则传 null",
	"sftoken.CreateSlot:freeze_authority":                   "冻结权限人地址，base58 字符串；不设则传 null",
	"sftoken.TransferOwner:sft":                             "SFT 资源地址，base58 字符串",
	"sftoken.TransferOwner:current_owner":                   "现任 SFT owner 地址，须其私钥参与交易签名",
	"sftoken.TransferOwner:new_owner":                       "新 owner 地址，base58 字符串",
	"sftoken.TransferSlotOwner:sft":                         "SFT 资源地址，base58 字符串",
	"sftoken.TransferSlotOwner:manager":                     "现任 slot_owner 地址，须其私钥参与交易签名",
	"sftoken.TransferSlotOwner:slot_id":                     "槽位编号，十进制数字（链上从 1 递增分配）",
	"sftoken.TransferSlotOwner:new_owner":                   "新 slot_owner 地址，base58 字符串",
	"sftoken.TransferSlotUpdateAuthor:sft":                  "SFT 资源地址，base58 字符串",
	"sftoken.TransferSlotUpdateAuthor:current_author":       "现任 update_author 地址，须其私钥参与交易签名",
	"sftoken.TransferSlotUpdateAuthor:slot_id":              "槽位编号，十进制数字",
	"sftoken.TransferSlotUpdateAuthor:new_author":           "新 update_author 地址，base58 字符串",
	"sftoken.TransferSlotFreezeAuthority:sft":               "SFT 资源地址，base58 字符串",
	"sftoken.TransferSlotFreezeAuthority:current_authority": "现任 freeze_authority 地址，须其私钥参与交易签名",
	"sftoken.TransferSlotFreezeAuthority:slot_id":           "槽位编号，十进制数字",
	"sftoken.TransferSlotFreezeAuthority:new_authority":     "新 freeze_authority 地址，base58 字符串",
	"sftoken.TransferRoyaltyRecipient:sft":                  "SFT 资源地址，base58 字符串",
	"sftoken.TransferRoyaltyRecipient:current_recipient":    "现任版税接收人地址，须其私钥参与交易签名",
	"sftoken.TransferRoyaltyRecipient:new_recipient":        "新版税接收人地址，base58 字符串",
	"sftoken.SetSlotTransferable:sft":                       "SFT 资源地址，base58 字符串",
	"sftoken.SetSlotTransferable:manager":                   "槽位管理权限人地址，须其私钥参与交易签名",
	"sftoken.SetSlotTransferable:slot_id":                   "槽位编号，十进制数字",
	"sftoken.SetSlotTransferable:is_transferable":           "true/false：是否允许部分转移与合并",
	"sftoken.FreezeSlot:sft":                                "SFT 资源地址，base58 字符串",
	"sftoken.FreezeSlot:freezer":                            "冻结权限人地址，须其私钥参与交易签名",
	"sftoken.FreezeSlot:slot_id":                            "槽位编号，十进制数字",
	"sftoken.UnfreezeSlot:sft":                              "SFT 资源地址，base58 字符串",
	"sftoken.UnfreezeSlot:freezer":                          "冻结权限人地址，须其私钥参与交易签名",
	"sftoken.UnfreezeSlot:slot_id":                          "槽位编号，十进制数字",
	"sftoken.Mint:sft":                                      "SFT 资源地址，base58 字符串",
	"sftoken.Mint:minter":                                   "槽位铸造权限人（slot_owner）地址，须其私钥参与交易签名",
	"sftoken.Mint:slot_id":                                  "目标槽位编号，十进制数字",
	"sftoken.Mint:to_address":                               "接收地址，base58 字符串",
	"sftoken.Mint:initial_value":                            "初始价值，十进制数字，必须 >0",
	"sftoken.Mint:metadata":                                 "token 元数据字符串，如 \"Gold Card #1\"",
	"sftoken.Mint:attribute":                                "token 属性字符串，如 \"grade=A\"",
	"sftoken.Burn:sft":                                      "SFT 资源地址，base58 字符串",
	"sftoken.Burn:owner":                                    "token 持有者地址，须其私钥参与交易签名",
	"sftoken.Burn:token_id":                                 "要销毁的 token 编号，十进制数字",
	"sftoken.Split:sft":                                     "SFT 资源地址，base58 字符串",
	"sftoken.Split:signer":                                  "源 token 持有者地址，须其私钥参与交易签名",
	"sftoken.Split:from_token_id":                           "源 token 编号，十进制数字",
	"sftoken.Split:to_address":                              "接收者地址，base58 字符串",
	"sftoken.Split:split_value":                             "拆出价值，十进制数字，必须 >0 且不超过源 token 价值",
	"sftoken.Merge:sft":                                     "SFT 资源地址，base58 字符串",
	"sftoken.Merge:signer":                                  "源 token 持有者地址，须其私钥参与交易签名",
	"sftoken.Merge:from_token_id":                           "被并入销毁的源 token 编号，十进制数字",
	"sftoken.Merge:to_token_id":                             "接收价值的目标 token 编号，十进制数字（须与源同槽位）",
	"sftoken.SetAttribute:sft":                              "SFT 资源地址，base58 字符串",
	"sftoken.SetAttribute:signer":                           "有更新权限的签名者地址（token owner / 槽位 update_author）",
	"sftoken.SetAttribute:token_id":                         "token 编号，十进制数字",
	"sftoken.SetAttribute:attribute":                        "新属性字符串",
	"sftoken.Approve:sft":                                   "SFT 资源地址，base58 字符串",
	"sftoken.Approve:owner":                                 "token 持有者地址，须其私钥参与交易签名",
	"sftoken.Approve:token_id":                              "token 编号，十进制数字",
	"sftoken.Approve:spender":                               "被授权地址，base58 字符串",
	"sftoken.Approve:amount":                                "授权额度，十进制数字",
	"sftoken.RevokeApproval:sft":                            "SFT 资源地址，base58 字符串",
	"sftoken.RevokeApproval:owner":                          "token 持有者地址，须其私钥参与交易签名",
	"sftoken.RevokeApproval:token_id":                       "token 编号，十进制数字",
	"sftoken.RevokeApproval:spender":                        "被撤销授权的地址，base58 字符串",
	"sftoken.Transfer:sft":                                  "SFT 资源地址，base58 字符串",
	"sftoken.Transfer:owner":                                "token 持有者地址，须其私钥参与交易签名",
	"sftoken.Transfer:token_id":                             "token 编号，十进制数字",
	"sftoken.Transfer:to":                                   "接收者地址，base58 字符串",
	"sftoken.Transfer:value":                                "转移价值，十进制数字，必须 >0（部分转移要求槽位可转移）",
	"sftoken.TransferFrom:sft":                              "SFT 资源地址，base58 字符串",
	"sftoken.TransferFrom:spender":                          "被授权方地址，须其私钥参与交易签名",
	"sftoken.TransferFrom:token_id":                         "token 编号，十进制数字",
	"sftoken.TransferFrom:to":                               "接收者地址，base58 字符串",
	"sftoken.TransferFrom:value":                            "转移价值，十进制数字，必须 >0 且不超过授权额度",

	"sftoken.SftInfo:sft":                 "SFT 资源地址，base58 字符串",
	"sftoken.SlotInfo:sft":                "SFT 资源地址，base58 字符串",
	"sftoken.SlotInfo:slot_id":            "槽位编号，十进制数字",
	"sftoken.RoyaltyInfo:sft":             "SFT 资源地址，base58 字符串",
	"sftoken.SlotOf:sft":                  "SFT 资源地址，base58 字符串",
	"sftoken.SlotOf:token_id":             "token 编号，十进制数字",
	"sftoken.Token:sft":                   "SFT 资源地址，base58 字符串",
	"sftoken.Token:token_id":              "token 编号，十进制数字",
	"sftoken.OwnerOf:sft":                 "SFT 资源地址，base58 字符串",
	"sftoken.OwnerOf:token_id":            "token 编号，十进制数字",
	"sftoken.ValueOf:sft":                 "SFT 资源地址，base58 字符串",
	"sftoken.ValueOf:token_id":            "token 编号，十进制数字",
	"sftoken.BalanceOf:sft":               "SFT 资源地址，base58 字符串",
	"sftoken.BalanceOf:token_id":          "token 编号，十进制数字（注意不是账户地址）",
	"sftoken.AttributeOf:sft":             "SFT 资源地址，base58 字符串",
	"sftoken.AttributeOf:token_id":        "token 编号，十进制数字",
	"sftoken.IsSlotFrozen:sft":            "SFT 资源地址，base58 字符串",
	"sftoken.IsSlotFrozen:slot_id":        "槽位编号，十进制数字",
	"sftoken.SlotUpdateAuthor:sft":        "SFT 资源地址，base58 字符串",
	"sftoken.SlotUpdateAuthor:slot_id":    "槽位编号，十进制数字",
	"sftoken.SlotFreezeAuthority:sft":     "SFT 资源地址，base58 字符串",
	"sftoken.SlotFreezeAuthority:slot_id": "槽位编号，十进制数字",
	"sftoken.SftOwnerOf:sft":              "SFT 资源地址，base58 字符串",
	"sftoken.SlotOwnerOf:sft":             "SFT 资源地址，base58 字符串",
	"sftoken.SlotOwnerOf:slot_id":         "槽位编号，十进制数字",
	"sftoken.ApprovalOf:sft":              "SFT 资源地址，base58 字符串",
	"sftoken.ApprovalOf:token_id":         "token 编号，十进制数字",
	"sftoken.ApprovalOf:spender":          "被授权地址，base58 字符串",

	// ==================== dex ====================

	"dex.CreateMarket:authority":                        "市场管理员地址，须其私钥参与交易签名",
	"dex.CreateMarket:base_token":                       "基础代币地址，base58 字符串（须与 quote 不同）",
	"dex.CreateMarket:quote_token":                      "计价代币地址，base58 字符串（须与 base 不同）",
	"dex.CreateMarket:base_lot_atoms":                   "每手（lot）对应的 base 代币原子数量，十进制数字，非 0",
	"dex.CreateMarket:quote_atoms_per_lot_tick":         "每个价格 tick 对应每手的 quote 原子数量（最小报价单位），非 0",
	"dex.CreateMarket:min_tick":                         "最小允许价格 tick，十进制数字，非 0",
	"dex.CreateMarket:max_tick":                         "最大允许价格 tick，十进制数字，非 0",
	"dex.CreateMarket:max_order_lots":                   "单笔订单最大手数，十进制数字，非 0",
	"dex.CreateMarket:max_fills_per_action":             "单笔动作最多撮合笔数，十进制数字，非 0",
	"dex.InitializeMarketDid:authority":                 "市场管理员地址，须其私钥参与交易签名",
	"dex.InitializeMarketDid:market_id":                 "市场地址，base58 字符串",
	"dex.InitializeMarketDid:doc":                       "DID 文档 JSON（同 identity.Create 的 doc），subject_type 须为 \"Organization\"",
	"dex.DiscloseMarketVcAttestation:authority":         "市场管理员地址，须其私钥参与交易签名",
	"dex.DiscloseMarketVcAttestation:market_id":         "市场地址，base58 字符串",
	"dex.DiscloseMarketVcAttestation:issuer":            "VC 签发方地址，base58 字符串",
	"dex.DiscloseMarketVcAttestation:issuer_key_id":     "签发方用于签名的密钥序号，十进制数字",
	"dex.DiscloseMarketVcAttestation:credential_schema": "凭证 schema 字符串",
	"dex.DiscloseMarketVcAttestation:credential_hash":   "凭证内容哈希，64 位 hex 字符串（32 字节）",
	"dex.DiscloseMarketVcAttestation:valid_until_ms":    "失效时间，Unix 毫秒时间戳数字；永久有效传 null",
	"dex.DiscloseMarketVcAttestation:issuer_signature":  "签发方签名，hex 或 base58 字符串",
	"dex.PlaceLimitOrder:owner":                         "下单者地址，须其私钥参与交易签名",
	"dex.PlaceLimitOrder:market_id":                     "市场地址，base58 字符串",
	"dex.PlaceLimitOrder:client_order_id":               "客户端自定义单号，十进制数字（幂等去重用）",
	"dex.PlaceLimitOrder:side":                          "方向枚举：\"Bid\"（买）或 \"Ask\"（卖）",
	"dex.PlaceLimitOrder:price_tick":                    "限价 tick，十进制数字，须在市场 [min_tick, max_tick] 内",
	"dex.PlaceLimitOrder:base_lots":                     "下单手数，十进制数字（≤ 市场 max_order_lots）",
	"dex.PlaceLimitOrder:time_in_force":                 "有效期枚举：\"GoodTilCancel\" | \"ImmediateOrCancel\"（立即成交否则取消）| \"AddLiquidityOnly\"（只挂单不吃单）",
	"dex.PlaceLimitOrder:self_trade_policy":             "自成交策略枚举：\"CancelTaker\" | \"CancelMaker\" | \"CancelBoth\"",
	"dex.PlaceLimitOrder:max_fills":                     "本笔最多成交次数，十进制数字",
	"dex.PlaceLimitOrder:channel":                       "返佣渠道地址，base58 字符串（无渠道可传零地址）",
	"dex.CancelOrder:owner":                             "订单本人地址，须其私钥参与交易签名",
	"dex.CancelOrder:market_id":                         "市场地址，base58 字符串",
	"dex.CancelOrder:order_id":                          "链上订单编号，十进制数字",
	"dex.CancelByClientOrderId:owner":                   "订单本人地址，须其私钥参与交易签名",
	"dex.CancelByClientOrderId:market_id":               "市场地址，base58 字符串",
	"dex.CancelByClientOrderId:client_order_id":         "下单时的客户端自定义单号，十进制数字",
	"dex.BatchCancel:owner":                             "订单本人地址，须其私钥参与交易签名",
	"dex.BatchCancel:market_id":                         "市场地址，base58 字符串",
	"dex.BatchCancel:order_ids":                         "订单编号 JSON 数组（非空、≤32 个、不重复）",
	"dex.SetMarketStatus:authority":                     "市场管理员地址，须其私钥参与交易签名",
	"dex.SetMarketStatus:market_id":                     "市场地址，base58 字符串",
	"dex.SetMarketStatus:status":                        "市场状态枚举：\"Active\"（正常）| \"CancelOnly\"（只可撤单）| \"Paused\"（暂停）",
	"dex.MarketInfo:market_id":                          "市场地址，base58 字符串",
	"dex.OrderInfoView:market_id":                       "市场地址，base58 字符串",
	"dex.OrderInfoView:order_id":                        "链上订单编号，十进制数字",
	"dex.BestBidAsk:market_id":                          "市场地址，base58 字符串",
	"dex.OrderbookDepth:market_id":                      "市场地址，base58 字符串",
	"dex.OrderbookDepth:depth":                          "档位数，十进制数字（1..=50）",
	"dex.OrdersAtLevel:market_id":                       "市场地址，base58 字符串",
	"dex.OrdersAtLevel:side":                            "方向枚举：\"Bid\" 或 \"Ask\"",
	"dex.OrdersAtLevel:tick":                            "价格档，十进制数字",
	"dex.OrdersAtLevel:cursor_order_id":                 "翻页游标：首页传 0，之后传上一页返回的 next_order_id",
	"dex.OrdersAtLevel:limit":                           "每页条数，十进制数字（1..=100）",
	"dex.VaultLiability:market_id":                      "市场地址，base58 字符串",
	"dex.VaultLiability:token":                          "查询的代币地址，base58 字符串",

	// ==================== keyless ====================

	"keyless.Authenticate:provider_id":                     "provider 标识，64 位 hex 字符串（内置 Google/Apple 有固定 ID，可由 ListProviders 查询）",
	"keyless.Authenticate:jwt":                             "JWT 字符串的 UTF-8 字节：hex 字符串（可带 0x）或数字数组；须为三段式 JWS 且未过期",
	"keyless.Authenticate:ephemeral_pk":                    "临时会话公钥，hex 或 base58 字符串（须与 JWT nonce 中的 commitment 一致）",
	"keyless.Bind:provider_id":                             "provider 标识，64 位 hex 字符串",
	"keyless.Bind:jwt":                                     "JWT 字符串的 UTF-8 字节（同 Authenticate）",
	"keyless.Bind:ephemeral_pk":                            "临时会话公钥，hex 或 base58 字符串",
	"keyless.Bind:owner":                                   "要绑定的链上账户地址，须其私钥参与交易签名",
	"keyless.Unbind:keyless_addr":                          "keyless 派生地址，base58 字符串",
	"keyless.Unbind:owner":                                 "绑定账户地址，须其私钥参与交易签名",
	"keyless.RevokeSession:session_id":                     "会话 ID，64 位 hex 字符串（可由 GetSessions 查询）",
	"keyless.RevokeSession:owner":                          "会话属主地址，须其私钥参与交易签名",
	"keyless.RegisterAudience:provider_id":                 "provider 标识，64 位 hex 字符串",
	"keyless.RegisterAudience:audience":                    "OIDC client ID 字符串，1..=256 字节",
	"keyless.RegisterAudience:registrant":                  "注册人地址，须其私钥参与交易签名",
	"keyless.CreateStaticProvider:display_name":            "provider 显示名，非空且 ≤64 字节",
	"keyless.CreateStaticProvider:issuer":                  "issuer URL 字符串，必须以 https:// 开头",
	"keyless.CreateStaticProvider:salt":                    "64 位 hex 随机盐（32 字节，与 issuer 共同派生 provider_id）",
	"keyless.CreateStaticProvider:keys":                    "验证公钥 JSON 数组（1..=8 项），每项 {\"kid\":字符串,\"algorithm\":\"Rs256\" 等,\"material\":{\"variant\":\"Rsa\",\"value\":{\"modulus\":hex,\"exponent\":hex}} 或 {\"variant\":\"P256\",\"value\":{\"x\":hex,\"y\":hex}}}",
	"keyless.CreateStaticProvider:owner":                   "provider 属主地址，须其私钥参与交易签名",
	"keyless.UpdateStaticProviderKeys:provider_id":         "provider 标识，64 位 hex 字符串",
	"keyless.UpdateStaticProviderKeys:keys":                "新验证公钥 JSON 数组（格式同 CreateStaticProvider）",
	"keyless.UpdateStaticProviderKeys:invalidate_sessions": "true/false：true 时旧会话全部立即失效",
	"keyless.UpdateStaticProviderKeys:owner":               "provider 属主地址，须其私钥参与交易签名",
	"keyless.GetSessions:owner":                            "账户地址，base58 字符串",
	"keyless.GetBinding:keyless_addr":                      "keyless 派生地址，base58 字符串",
	"keyless.GetProvider:provider_id":                      "provider 标识，64 位 hex 字符串",
	"keyless.GetProviderStatus:provider_id":                "provider 标识，64 位 hex 字符串",
	"keyless.GetAudience:provider_id":                      "provider 标识，64 位 hex 字符串",
	"keyless.GetAudience:audience":                         "要查询的 OIDC client ID 字符串",
	"keyless.IsKeylessAccount:owner":                       "账户地址，base58 字符串",

	// ==================== lucky_box ====================

	"lucky_box.CreateEqual:creator":      "创建者地址，须其私钥参与交易签名（资金从其账户托管）",
	"lucky_box.CreateEqual:box_id":       "盒子编号，十进制数字（创建者维度不可重复）",
	"lucky_box.CreateEqual:token":        "代币地址，base58 字符串",
	"lucky_box.CreateEqual:total_amount": "总金额，十进制数字（代币最小单位）",
	"lucky_box.CreateEqual:claim_count":  "可领取份数，十进制数字（≤10000）",
	"lucky_box.CreateEqual:rules":        "领取规则 JSON 或 null：{\"eligibility_bloom\":{\"bits\":hex 字节串（32..=32768 字节）,\"item_count\":数字},\"expires_at_ms\":毫秒时间戳或 null（默认 24 小时）}",
	"lucky_box.CreateLucky:creator":      "创建者地址，须其私钥参与交易签名（资金从其账户托管）",
	"lucky_box.CreateLucky:box_id":       "盒子编号，十进制数字（创建者维度不可重复）",
	"lucky_box.CreateLucky:token":        "代币地址，base58 字符串",
	"lucky_box.CreateLucky:total_amount": "总金额，十进制数字（代币最小单位）",
	"lucky_box.CreateLucky:claim_count":  "可领取份数，十进制数字（≤10000）",
	"lucky_box.CreateLucky:rules":        "领取规则 JSON 或 null（同 CreateEqual）",
	"lucky_box.Claim:claimant":           "领取者地址，须其私钥参与交易签名（每人限领一次）",
	"lucky_box.Claim:box_id":             "盒子编号，十进制数字",
	"lucky_box.Refund:box_id":            "盒子编号，十进制数字（盒子必须已过期）",
	"lucky_box.BoxView:box_id":           "盒子编号，十进制数字",
	"lucky_box.AssetPool:box_id":         "盒子编号，十进制数字",

	// ==================== social ====================

	"social.UpsertSocialProfile:subject":            "本人地址，须其私钥参与交易签名",
	"social.UpsertSocialProfile:uri":                "profile 数据 URI 字符串（内容存链下）",
	"social.RegisterSourceApp:owner":                "应用属主地址，须其私钥参与交易签名",
	"social.AddSourceAppPublisher:owner":            "应用属主地址，须其私钥参与交易签名",
	"social.AddSourceAppPublisher:source_app_id":    "源应用 ID，十进制数字（RegisterSourceApp 返回值）",
	"social.AddSourceAppPublisher:publisher":        "被授权为发布者的地址，base58 字符串",
	"social.RemoveSourceAppPublisher:owner":         "应用属主地址，须其私钥参与交易签名",
	"social.RemoveSourceAppPublisher:source_app_id": "源应用 ID，十进制数字",
	"social.RemoveSourceAppPublisher:publisher":     "要移除的发布者地址，base58 字符串",
	"social.TransferSourceAppOwner:owner":           "现任属主地址，须其私钥参与交易签名",
	"social.TransferSourceAppOwner:source_app_id":   "源应用 ID，十进制数字",
	"social.TransferSourceAppOwner:new_owner":       "新属主地址，base58 字符串",
	"social.DeactivateSourceApp:owner":              "应用属主地址，须其私钥参与交易签名",
	"social.DeactivateSourceApp:source_app_id":      "源应用 ID，十进制数字",
	"social.ReactivateSourceApp:owner":              "应用属主地址，须其私钥参与交易签名",
	"social.ReactivateSourceApp:source_app_id":      "源应用 ID，十进制数字",
	"social.UpsertPublicDataUri:publisher":          "发布者地址，须其私钥参与交易签名",
	"social.UpsertPublicDataUri:source_app_id":      "源应用 ID，十进制数字",
	"social.UpsertPublicDataUri:dataset_kind":       "数据类别枚举：\"Profile\" | \"Tags\" | \"Content\" | \"Relationship\" | \"Other\"",
	"social.UpsertPublicDataUri:scope_key":          "数据范围键字符串（如用户 ID）",
	"social.UpsertPublicDataUri:uri":                "数据 URI 字符串",
	"social.RetirePublicDataUri:publisher":          "发布者地址，须其私钥参与交易签名",
	"social.RetirePublicDataUri:source_app_id":      "源应用 ID，十进制数字",
	"social.RetirePublicDataUri:dataset_kind":       "数据类别枚举（同 UpsertPublicDataUri）",
	"social.RetirePublicDataUri:scope_key":          "数据范围键字符串",
	"social.RestorePublicDataUri:publisher":         "发布者地址，须其私钥参与交易签名",
	"social.RestorePublicDataUri:source_app_id":     "源应用 ID，十进制数字",
	"social.RestorePublicDataUri:dataset_kind":      "数据类别枚举（同 UpsertPublicDataUri）",
	"social.RestorePublicDataUri:scope_key":         "数据范围键字符串",
	"social.RestorePublicDataUri:uri":               "恢复后使用的数据 URI 字符串",
	"social.CreateCommunity:creator":                "创建者地址，须其私钥参与交易签名（每人最多创建 8 个社区）",
	"social.CreateCommunity:join_mode":              "加入模式枚举：\"Open\"（直接加入）| \"Approval\"（需审批）",
	"social.CreateCommunity:uri":                    "社区元数据 URI 字符串",
	"social.UpdateCommunityUri:owner":               "社区 owner 地址，须其私钥参与交易签名",
	"social.UpdateCommunityUri:community_id":        "社区 ID，十进制数字",
	"social.UpdateCommunityUri:uri":                 "新元数据 URI 字符串",
	"social.SetCommunityJoinMode:owner":             "社区 owner 地址，须其私钥参与交易签名",
	"social.SetCommunityJoinMode:community_id":      "社区 ID，十进制数字",
	"social.SetCommunityJoinMode:join_mode":         "加入模式枚举：\"Open\" | \"Approval\"",
	"social.TransferCommunityOwner:owner":           "社区 owner 地址，须其私钥参与交易签名",
	"social.TransferCommunityOwner:community_id":    "社区 ID，十进制数字",
	"social.TransferCommunityOwner:new_owner":       "新 owner 地址，base58 字符串",
	"social.DeactivateCommunity:owner":              "社区 owner 地址，须其私钥参与交易签名",
	"social.DeactivateCommunity:community_id":       "社区 ID，十进制数字",
	"social.GrantCommunityAdmin:owner":              "社区 owner 地址，须其私钥参与交易签名",
	"social.GrantCommunityAdmin:community_id":       "社区 ID，十进制数字",
	"social.GrantCommunityAdmin:admin":              "被授予管理员权限的地址，base58 字符串",
	"social.RevokeCommunityAdmin:owner":             "社区 owner 地址，须其私钥参与交易签名",
	"social.RevokeCommunityAdmin:community_id":      "社区 ID，十进制数字",
	"social.RevokeCommunityAdmin:admin":             "要撤销管理员的地址，base58 字符串",
	"social.JoinCommunity:member":                   "加入者地址，须其私钥参与交易签名",
	"social.JoinCommunity:community_id":             "社区 ID，十进制数字（仅 Open 模式）",
	"social.RequestJoinCommunity:member":            "申请人地址，须其私钥参与交易签名",
	"social.RequestJoinCommunity:community_id":      "社区 ID，十进制数字（仅 Approval 模式）",
	"social.CancelJoinRequest:member":               "申请人地址，须其私钥参与交易签名",
	"social.CancelJoinRequest:community_id":         "社区 ID，十进制数字",
	"social.ApproveJoinRequest:admin":               "管理员地址，须其私钥参与交易签名",
	"social.ApproveJoinRequest:community_id":        "社区 ID，十进制数字",
	"social.ApproveJoinRequest:member":              "申请人地址，base58 字符串",
	"social.RejectJoinRequest:admin":                "管理员地址，须其私钥参与交易签名",
	"social.RejectJoinRequest:community_id":         "社区 ID，十进制数字",
	"social.RejectJoinRequest:member":               "申请人地址，base58 字符串",
	"social.LeaveCommunity:member":                  "成员地址，须其私钥参与交易签名（owner 不能退出）",
	"social.LeaveCommunity:community_id":            "社区 ID，十进制数字",
	"social.RemoveCommunityMember:admin":            "管理员地址，须其私钥参与交易签名",
	"social.RemoveCommunityMember:community_id":     "社区 ID，十进制数字",
	"social.RemoveCommunityMember:member":           "要移除的成员地址，base58 字符串",
	"social.SocialProfile:subject":                  "查询地址，base58 字符串",
	"social.SourceApp:source_app_id":                "源应用 ID，十进制数字",
	"social.IsSourceAppPublisher:source_app_id":     "源应用 ID，十进制数字",
	"social.IsSourceAppPublisher:publisher":         "要判断的地址，base58 字符串",
	"social.PublicDataAnchor:source_app_id":         "源应用 ID，十进制数字",
	"social.PublicDataAnchor:dataset_kind":          "数据类别枚举（同 UpsertPublicDataUri）",
	"social.PublicDataAnchor:scope_key":             "数据范围键字符串",
	"social.Community:community_id":                 "社区 ID，十进制数字",
	"social.IsCommunityAdmin:community_id":          "社区 ID，十进制数字",
	"social.IsCommunityAdmin:admin":                 "要判断的地址，base58 字符串",
	"social.JoinRequest:community_id":               "社区 ID，十进制数字",
	"social.JoinRequest:member":                     "成员地址，base58 字符串",
	"social.CommunityMembership:community_id":       "社区 ID，十进制数字",
	"social.CommunityMembership:member":             "成员地址，base58 字符串",

	// ==================== demo ====================

	"demo.OpenOrder:operator":               "运营方地址，须其私钥参与交易签名（结算须同一人）",
	"demo.OpenOrder:order_id":               "自定义订单号字符串（不可重复）",
	"demo.PayOrder:payer":                   "付款人地址，须其私钥参与交易签名",
	"demo.PayOrder:order_id":                "订单号字符串",
	"demo.PayOrder:token":                   "代币地址，base58 字符串",
	"demo.PayOrder:amount":                  "支付数量，十进制数字（最小单位）",
	"demo.SettleOrder:operator":             "运营方地址，须其私钥参与交易签名（须与开单相同）",
	"demo.SettleOrder:order_id":             "订单号字符串",
	"demo.SettleOrder:token":                "代币地址，base58 字符串",
	"demo.SettleOrder:to":                   "收款地址，base58 字符串",
	"demo.SettleOrder:amount":               "结算数量，十进制数字（不得超过托管余额）",
	"demo.OrderBalance:order_id":            "订单号字符串",
	"demo.OrderBalance:token":               "代币地址，base58 字符串",
	"demo.OpenGasSponsorPool:pool":          "赞助池资源地址，base58 字符串（其管理员须与交易付款人一致）",
	"demo.ClaimSponsoredScore:claimer":      "领取者地址，须其私钥参与交易签名",
	"demo.ClaimSponsoredScore:pool":         "赞助池地址，base58 字符串",
	"demo.ClaimSponsoredScore:sponsor_seed": "赞助池编号，十进制数字（OpenGasSponsorPool 返回值）",
	"demo.ClaimSponsoredScore:amount":       "领取额度，十进制数字",
	"demo.SponsorPoolOf:sponsor_seed":       "赞助池编号，十进制数字",
	"demo.InitPool:pool":                    "池资源账户地址，须其私钥参与交易签名",
	"demo.InitPool:label":                   "标签文本字符串",
	"demo.InitDex:dex":                      "池资源账户地址，须其私钥参与交易签名",
	"demo.InitDex:label":                    "标签文本字符串",
	"demo.SetLabel:pool":                    "池地址，base58 字符串",
	"demo.SetLabel:label":                   "新标签文本字符串",
	"demo.BatchCredit:pool":                 "池地址，base58 字符串",
	"demo.BatchCredit:recipients":           "接收地址 JSON 数组（须已在池内登记）",
	"demo.BatchCredit:amount":               "每人加的积分额度，十进制数字",
	"demo.LabelOf:pool":                     "池地址，base58 字符串",
	"demo.ScoreOf:pool":                     "池地址，base58 字符串",
	"demo.ScoreOf:account":                  "查询地址，base58 字符串",
	"demo.SetTierCap:pool":                  "池地址，base58 字符串",
	"demo.SetTierCap:tier":                  "层级编号，十进制数字",
	"demo.SetTierCap:cap":                   "积分上限，十进制数字",
	"demo.TierCapOf:pool":                   "池地址，base58 字符串",
	"demo.TierCapOf:tier":                   "层级编号，十进制数字",
	"demo.EchoMode:mode":                    "枚举：\"One\" 或 {\"variant\":\"Two\",\"value\":{\"val\":数字}} 或 {\"variant\":\"Three\",\"value\":[数字,数字]}（tuple<u8,i16>）",
	"demo.LabelTotal:labels":                "JSON 对象，键为字符串、值为数字，如 {\"alpha\":10,\"beta\":20}",
	"demo.SpecialTypes:mode":                "枚举（同 EchoMode）",
	"demo.SpecialTypes:maybe_note":          "字符串或 null（option<String>）",
	"demo.SpecialTypes:tags":                "字符串 JSON 数组",
	"demo.SpecialTypes:labels":              "JSON 对象，键为字符串、值为数字",
	"demo.SpecialTypes:pair":                "两元素数组 [数字, 数字]（tuple<u8,i16>，第二个可为负）",
}
