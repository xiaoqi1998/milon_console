// test_seed_overview.js — 总览数据层测试(源码直抽,无浏览器依赖)
// 运行:node test_seed_overview.js
const fs = require('fs');
const path = require('path');
const ROOT = __dirname;
const jsSrc = fs.readFileSync(path.join(ROOT, 'static', 'js', 'seed_overview.js'), 'utf8');

let failed = 0;
function check(name, cond, detail) {
  if (cond) console.log('  ok  ' + name);
  else { failed++; console.log('FAIL  ' + name + (detail ? '\n      ' + detail : '')); }
}

// 直抽环境:apiFetch 由用例注入
global.apiFetch = async () => { throw new Error('not stubbed'); };
eval(jsSrc);

// 1. sha256 标准向量(NIST FIPS 180-4)
check('sha256("")', seedSha256('') === 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855');
check('sha256("abc")', seedSha256('abc') === 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad');
check('sha256(448bit 向量)',
  seedSha256('abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq') ===
  '248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1');

// 2. 与 Go SeedDeriveKey 对拍:privkey = sha256(短语:角色:名字)
const VECTORS = [
  ['milon-devnet-seed-v1', 'admin', 'admin', 'f7c4b028d203b290184688a08f5e726c8db284e4e7c9fc30d0a842801fc143b9'],
  ['milon-devnet-seed-v1', 'user', 'test01', 'f620a61f2638deb0c9935f5ff01bb3f69b94274a4e716cfa0ae8785d80826e97'],
  ['milon-devnet-seed-v1', 'issuer', 'kycorg', 'a3b33ba569c980a38140cb2a496340b2579507f3057891c0790c14559c973267'],
  ['milon-devnet-seed-v1', 'multisig', 'ms01', '75b9263371cbb8b4149003ee5b75f05d8dfe79007837bf6192a2d01c67527967'],
  ['milon-devnet-seed-v1', 'token', 'ST01', 'b89e42fd257e9ee4d451066106de865f93804633afa21ce041df6c2316d94b38'],
  ['milon-devnet-seed-v1', 'sft', 'set01', 'bd3e70895a71bce3678fe4d157f00861a3567ffe270722d9d738e3ad81b7a895'],
];
for (const [phrase, role, name, want] of VECTORS) {
  check('privkey ' + role + ':' + name, seedPrivKey(phrase, role, name) === want, seedPrivKey(phrase, role, name));
}

// 3. seedAttachKeys:按行 role/name 自描述补私钥,不改入参
const rows = [
  { name: 'admin', role: 'admin', curve: 'secp256k1', address: 'A1' },
  { name: 'test01', role: 'user', curve: 'secp256k1', address: 'A2' },
  { name: 'ST01', role: 'token', curve: 'secp256k1', address: 'A3' },
  { name: 'set01', role: 'sft', curve: 'ed25519', address: 'A4' },
];
const enriched = seedAttachKeys(rows, 'milon-devnet-seed-v1');
check('attachKeys 行数不变', enriched.length === 4);
check('attachKeys admin 私钥', enriched[0].privateKey === VECTORS[0][3]);
check('attachKeys token 私钥', enriched[2].privateKey === VECTORS[4][3]);
check('attachKeys 入参不被修改', rows[0].privateKey === undefined);

// 4. seedGroupRows:按角色分组(admin/users/issuers/multisig/tokens/sfts)
const grouped = seedGroupRows(rows);
check('分组 admin', grouped.admin.length === 1 && grouped.admin[0].name === 'admin');
check('分组 users', grouped.users.length === 1);
check('分组 tokens', grouped.tokens.length === 1);
check('分组 sfts', grouped.sfts.length === 1);
check('分组 issuers 空', grouped.issuers.length === 0 && Array.isArray(grouped.issuers));
check('分组 multisig 空', grouped.multisig.length === 0);

// 5. seedDatasetArtifacts:从数据集命名空间提取代币/SFT 地址与 slot 产物
const dataset = {
  configHash: '2b6cca834a4e1d06',
  modules: {
    accounts: { faucet: { count: 4 } },
    tokens: { 'create:ST01': { address: 'AddrTok' }, 'mint:ST01': { holders: 3 } },
    sft: {
      'create:set01': { address: 'AddrSft', recreated: true },
      'slot:set01:transferable': { slotId: 1, tokenIds: [1, 2, 3], transferable: true },
    },
  },
  stageCount: 5,
};
const art = seedDatasetArtifacts(dataset);
check('产物 代币地址', art.tokens.ST01 === 'AddrTok');
check('产物 SFT 地址', art.sfts.set01.address === 'AddrSft');
check('产物 slot/tokenIds', art.sfts.set01.slots.transferable && art.sfts.set01.slots.transferable.slotId === 1 &&
  art.sfts.set01.slots.transferable.tokenIds.join() === '1,2,3');
check('产物 阶段数', art.stageCount === 5);

// 5.5 发证书 + 三卡链上实查:纯函数
const issuePayload = vcIssuePayload(
  { seedPhrase: 'p', accounts: { count: 3 } }, 'kycorg', 'test02', 'SeedKyc', 2);
check('vcIssuePayload 字段', issuePayload.issuerName === 'kycorg' && issuePayload.subjectName === 'test02' &&
  issuePayload.schemaPrefix === 'SeedKyc' && issuePayload.count === 2 &&
  issuePayload.config.seedPhrase === 'p', JSON.stringify(issuePayload));

const hv = vcHoldingsView('3R4xzbWqd5N9VKUUeCvxFHKj5mTv');
check('DisclosedVcs 视图调用', hv.appName === 'identity' && hv.methodName === 'DisclosedVcs' &&
  hv.args.subject === '3R4xzbWqd5N9VKUUeCvxFHKj5mTv' && hv.args.limit === 100);

const tb = tokenBalanceView('TokAddr', 'HoldAddr');
check('token.BalanceOf 视图调用', tb.appName === 'token' && tb.methodName === 'BalanceOf' &&
  tb.args.token === 'TokAddr' && tb.args.account === 'HoldAddr');

const so = sftOwnerView('SftAddr', 7);
check('sftoken.SftOwnerOf 视图调用', so.appName === 'sftoken' && so.methodName === 'SftOwnerOf' &&
  so.args.sft === 'SftAddr' && so.args.token_id === 7);

check('DisclosedVcs 归一:数组', JSON.stringify(normalizeDisclosedVcs([{ credential_schema: 'SeedKyc1' }, { credential_schema: 'SeedKyc2' }])) ===
  JSON.stringify(['SeedKyc1', 'SeedKyc2']));
check('DisclosedVcs 归一:失败→null', normalizeDisclosedVcs(undefined) === null);
check('u64 归一', normalizeU64View(1234) === 1234 && normalizeU64View('0') === 0 && normalizeU64View(null) === null);

// SFT 持仓任务清单:dataset 产物 → (场景, tokenId) 平铺
const sftJobs = sftHoldingJobs({ sfts: { set01: { address: 'A', slots: {
  transferable: { slotId: 1, tokenIds: [1, 2] },
  burn: { slotId: 2, tokenIds: [3] },
} } } }, 'set01');
check('SFT 持仓任务平铺', sftJobs.length === 3 && sftJobs[0].scenario === 'transferable' &&
  sftJobs[2].tokenId === 3 && sftJobs.every(j => j.sft === 'A'));

// 6. 短语一致性:localStorage seedLastPhrase 判定(账号固定保护)
global.localStorage = {
  store: {},
  getItem(k) { return this.store[k] || null; },
  setItem(k, v) { this.store[k] = String(v); },
  removeItem(k) { delete this.store[k]; },
};
check('无记录不警告', seedPhraseMismatch('milon-devnet-seed-v1') === false);
localStorage.setItem('seedLastPhrase', 'milon-devnet-seed-v1');
check('同短语不警告', seedPhraseMismatch('milon-devnet-seed-v1') === false);
check('异短语警告', seedPhraseMismatch('8rHKRGW4irtL') === true);
check('空短语不警告', seedPhraseMismatch('') === false);

// 7. 数据层取数封装:POST accounts / GET dataset
let captured = null;
global.apiFetch = (url, init) => Promise.resolve({
  json: async () => {
    captured = { url, init };
    return { success: true, data: { accounts: rows, seedPhrase: 'milon-devnet-seed-v1', configHash: 'h' } };
  },
});
seedFetchAccounts({ accounts: { count: 3 } }).then((res) => {
  check('fetch accounts URL', captured.url === '/api/tool/seed/accounts');
  check('fetch accounts body', JSON.parse(captured.init.body).config.accounts.count === 3);
  check('fetch accounts 返回含派生私钥', res.accounts && res.accounts[0] &&
    res.accounts[0].privateKey === VECTORS[0][3], JSON.stringify(res.accounts && res.accounts[0]));
  return seedFetchDataset();
}).then(() => {
  check('fetch dataset URL', captured.url === '/api/tool/seed/dataset');
  domChecks();
}).catch((e) => {
  check('取数封装可执行', false, e.message);
  console.log(`${failed} FAILED`);
  process.exit(1);
});

// ---------- 8+. 渲染层 / 接线 / 版本(重新以干净 mock 环境直抽) ----------
function domChecks() {
  const htmlSrc = fs.readFileSync(path.join(ROOT, 'static', 'index.html'), 'utf8');
  const seedSrc = fs.readFileSync(path.join(ROOT, 'static', 'js', 'seed.js'), 'utf8');

  // 8. index.html 锚点
  for (const id of ['seedOverviewBox', 'seedPhraseWarn', 'seedDatasetSel', 'seedDatasetInfo',
    'seedOverviewRefresh', 'seedOverviewCopy', 'seedOverviewCards']) {
    check('HTML 锚点 #' + id, htmlSrc.includes('id="' + id + '"'));
  }

  // 9. 渲染层函数存在
  for (const fn of ['initSeedOverview', 'refreshSeedOverview', 'seedRenderOverview',
    'seedBuildCardTable', 'seedPreferredPhrase', 'openVcIssuePanel', 'submitVcIssue',
    'fetchVcHoldings', 'fetchTokenHoldings', 'fetchSftHoldings']) {
    check('JS 函数 ' + fn, jsSrc.includes('function ' + fn));
  }
  // 9.5 账户卡带"发证书"操作列;证书卡带链上实查按钮;代币/SFT 卡带持仓按钮
  check('账户卡操作列表头', jsSrc.includes("'发证书'") && jsSrc.includes("'操作'"));
  check('证书卡链上按钮', jsSrc.includes('查链上证书'));
  check('代币卡持仓按钮', jsSrc.includes('查链上持仓'));
  check('发证书走 vc-issue 端点', jsSrc.includes('/api/tool/vc-issue'));
  check('链上读走 /api/read', jsSrc.includes("'/api/read'"));

  // 10. 账号固定保护接线(seed.js)
  check('seed.js 记录上次成功短语', seedSrc.includes('seedRememberPhrase'));
  check('seed.js 回填优先上次短语', seedSrc.includes('seedPreferredPhrase'));
  check('seed.js 播种完成刷新总览', seedSrc.includes('refreshSeedOverview'));
  check('seed.js 计划缓存供 VC 卡', seedSrc.includes('seedOverviewState.plan'));

  // 11. 版本 bump
  check('seed.js 版本 bump 20261103', htmlSrc.includes('seed.js?v=20261103'));
  check('seed_overview.js 已引入', /seed_overview\.js\?v=\d+/.test(htmlSrc));
  check('style.css 版本 bump 20261103', htmlSrc.includes('style.css?v=20261103'));
  check('.seed-warn 样式存在', fs.readFileSync(path.join(ROOT, 'static', 'css', 'style.css'), 'utf8').includes('.seed-warn'));

  // 11.5 账号固定保护补强:孤儿配置(configHash 不在数据集)也须警告
  check('孤儿配置警告逻辑', jsSrc.includes('knownDataset') && jsSrc.includes('没有对应的链上数据集'));
  const bookSrc2 = fs.readFileSync(path.join(ROOT, 'static', 'js', 'account_book.js'), 'utf8');
  const addIdx = bookSrc2.indexOf("getElementById('accountBookAddBtn')");
  const addBlock = bookSrc2.slice(addIdx, addIdx + 700);
  check('添加一条 同时亮出提交按钮', addBlock.includes('accountBookSubmitBtn'));

  // 12. mock DOM 烟雾:渲染卡片,私钥默认遮蔽
  const made = [];
  function mockNode(tag) {
    return {
      tag, children: [], style: {}, dataset: {}, value: '',
      classList: { add() {}, remove() {}, toggle() {} },
      setAttribute() {}, addEventListener() {}, appendChild(c) { made.push(c); this.children.push(c); },
      querySelectorAll() { return []; }, get textContent() { return ''; }, set textContent(v) { this._txt = v; },
    };
  }
  global.document = {
    readyState: 'complete',
    createElement: (t) => mockNode(t),
    createTextNode: (t) => ({ text: t }),
    getElementById: (id) => (id === 'seedOverviewCards' ? boxMock : null),
    addEventListener() {},
  };
  const boxMock = mockNode('div');
  global.el = (tag, attrs) => { const n = mockNode(tag); if (attrs) { if (attrs.class) n.className = attrs.class; if (attrs.text != null) n.textContent = attrs.text; } return n; };
  global.showToast = () => {};
  eval(jsSrc); // 重新求值:带渲染函数的完整文件
  seedOverviewState.accounts = seedAttachKeys(rows, 'milon-devnet-seed-v1');
  seedOverviewState.datasets = [dataset];
  seedOverviewState.configHash = '2b6cca834a4e1d06';
  let threw = '';
  try { seedRenderOverview(); } catch (e) { threw = e.message; }
  check('seedRenderOverview 可执行', threw === '', threw);
  check('渲染产生卡片节点', made.length > 0);

  // 13. 三卡链上实查渲染闭环(mock DOM 走同一渲染代码路径)
  seedOverviewState.vcHoldings = { 'em4tZpYeYYrjjxcyopCWL4ZxACg': ['SeedKyc1'] };
  seedOverviewState.tokenHoldings = [{ symbol: 'ST01', holderName: 'test01', holderAddr: 'A1', balance: 134 }];
  seedOverviewState.sftHoldings = [{ key: 'set01', scenario: 'transferable', tokenId: 1, ownerName: 'test01', ownerAddr: 'A1' }];
  threw = '';
  try { seedRenderOverview(); } catch (e) { threw = e.message; }
  check('实查数据渲染可执行', threw === '', threw);
  const allText = JSON.stringify(made);
  check('证书表含 test02 持证', allText.includes('SeedKyc1'));
  check('代币持仓卡渲染', allText.includes('代币持仓') && allText.includes('134'));
  check('SFT 持仓卡渲染', allText.includes('SFT 持仓') && allText.includes('transferable'));

  console.log(failed === 0 ? '\nALL PASS' : `\n${failed} FAILED`);
  process.exit(failed === 0 ? 0 : 1);
}
