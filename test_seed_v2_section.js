// test_seed_v2_section.js — 播种器 v2 控制台页测试(源码直抽)
// 运行:node test_seed_v2_section.js
const fs = require('fs');
const path = require('path');
const ROOT = __dirname;
const htmlSrc = fs.readFileSync(path.join(ROOT, 'static', 'index.html'), 'utf8');
const jsSrc = fs.readFileSync(path.join(ROOT, 'static', 'js', 'seed.js'), 'utf8');

let failed = 0;
function check(name, cond, detail) {
  if (cond) console.log('  ok  ' + name);
  else { failed++; console.log('FAIL  ' + name + (detail ? '\n      ' + detail : '')); }
}

// 1. HTML 锚点:模板条 + 四组卡片 + 预览/执行区
const seedView = htmlSrc.slice(htmlSrc.indexOf('id="view-seed"'),
  htmlSrc.indexOf('id="view-', htmlSrc.indexOf('id="view-seed"') + 10) > 0
    ? htmlSrc.indexOf('</section>', htmlSrc.indexOf('id="view-seed"')) : undefined);
for (const id of ['seedTemplateBtns', 'seedAcctCount', 'seedCurvesChips', 'seedTiersChips',
  'seedAcctFaucet', 'seedAcctDid', 'seedAcctVc', 'seedMsCount', 'seedMsSigners',
  'seedMsThreshold', 'seedTokCount', 'seedDecimalsChips', 'seedCompCount',
  'seedSftCount', 'seedSftSlots', 'seedSftTokens', 'seedScenariosChips', 'seedPostOps',
  'seedPhrase', 'seedPreview', 'seedSummary', 'seedStartBtn', 'seedLog', 'seedAddrTable']) {
  check('HTML 锚点 #' + id, seedView.includes(id));
}

// 2. seed.js 关键函数存在
for (const fn of ['buildSeedCfg', 'applySeedTemplate', 'refreshSeedPreview', 'startSeedJob']) {
  check('JS 函数 ' + fn, jsSrc.includes('function ' + fn));
}

// 3. 防抖 500ms 调预览
check('预览防抖 500', /setTimeout[^;]{0,80}500/.test(jsSrc) && jsSrc.includes('/api/tool/seed/plan'));

// 4. 四端点路径
for (const ep of ['/api/tool/seed/templates', '/api/tool/seed/plan', '/api/tool/seed"', '/api/tool/seed/status/']) {
  check('端点 ' + ep.trim(), jsSrc.includes(ep.replace('"', '')));
}

// 5. buildSeedCfg 纯逻辑可抽测:模拟 DOM 环境构造 SeedConfig
const dom = {};
global.document = {
  getElementById: (id) => dom[id] || null,
  querySelectorAll: () => [],
};
global.localStorage = { getItem: () => null, setItem: () => {} };
dom['seedAcctCount'] = { value: '5' };
dom['seedTokCount'] = { value: '2' };
dom['seedMsCount'] = { value: '1' };
dom['seedMsSigners'] = { value: '3' };
dom['seedMsThreshold'] = { value: '2' };
dom['seedSftCount'] = { value: '1' };
dom['seedSftSlots'] = { value: '2' };
dom['seedSftTokens'] = { value: '3' };
dom['seedCompCount'] = { value: '0' };
dom['seedPhrase'] = { value: '' };
// chips: 选中态通过 data-key 与 .on class 表达
global.__chips = { seedDecimalsChips: [{ dataset: { key: '0' }, classList: { contains: () => true } },
                                     { dataset: { key: '6' }, classList: { contains: () => false } }] };
try {
  eval(jsSrc);
  const cfg = buildSeedCfg();
  check('buildSeedCfg 账号数', cfg.accounts.count === 5, JSON.stringify(cfg.accounts));
  check('buildSeedCfg 精度 chips', Array.isArray(cfg.tokens.decimals) && cfg.tokens.decimals[0] === 0,
    JSON.stringify(cfg.tokens.decimals));
  check('buildSeedCfg 多签', cfg.multisig.count === 1 && cfg.multisig.signers === 3 && cfg.multisig.threshold === 2);
} catch (e) {
  check('buildSeedCfg 可执行', false, e.message);
}

// 6. 静态版本号已 bump
check('版本 bump 20261019', htmlSrc.includes('seed.js?v=20261019'));

console.log(failed === 0 ? '\nALL PASS' : `\n${failed} FAILED`);
process.exit(failed === 0 ? 0 : 1);
