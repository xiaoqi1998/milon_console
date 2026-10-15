// test_mcp_accounts_section.js — MCP 接入页「配置操作账户」小节测试
// （直接从 static/index.html 与 static/js/app.js 抽取真实源码）
// 运行:node test_mcp_accounts_section.js
// 覆盖:
//   1. HTML 锚点:小节标题/参数表/交互生成器五元素 ID 齐备
//   2. buildAccountsConfig:合法数组 → 完整 mcpServers 配置(headers 内 X-Milon-Accounts=base64(JSON))
//   3. base64 编码正确性(解码回原文)与 UTF-8 安全(中文 name)
//   4. 坏 JSON → 错误信息、不产出配置
//   5. 非数组 / 缺 address / 缺 privateKey → 逐条定位错误
//   6. fndsa512 缺 publicKey → 明确警告文案
//   7. 超上限 64 条 → 拒绝
//   8. 静态资源版本号已 bump(防缓存回归)
const fs = require('fs');
const path = require('path');

const ROOT = __dirname;
const APP_JS = path.join(ROOT, 'static', 'js', 'app.js');
const INDEX_HTML = path.join(ROOT, 'static', 'index.html');

const appSrc = fs.readFileSync(APP_JS, 'utf8');
const htmlSrc = fs.readFileSync(INDEX_HTML, 'utf8');

let failed = 0;
function check(name, cond, detail) {
  if (cond) {
    console.log('  ok  ' + name);
  } else {
    failed++;
    console.log('FAIL  ' + name + (detail ? '\n      ' + detail : ''));
  }
}

function sliceBetween(src, startMarker, endMarker) {
  const s = src.indexOf(startMarker);
  if (s < 0) throw new Error('marker not found: ' + startMarker);
  const e = src.indexOf(endMarker, s + startMarker.length);
  if (e < 0) throw new Error('end marker not found: ' + endMarker);
  return src.slice(s, e);
}

// ---------- 1. HTML 锚点 ----------
console.log('[1] index.html 锚点');
check('小节标题存在', htmlSrc.includes('配置操作账户'));
check('说明提及「只传地址、免传私钥」', htmlSrc.includes('免传私钥'));
check('参数表表头(字段)存在', /配置操作账户[\s\S]*?<th[^>]*>\s*字段/.test(htmlSrc));
check('参数表提及 fndsa512', /配置操作账户[\s\S]{0,4000}fndsa512/.test(htmlSrc));
check('示范配置块 mcpAccountsSample 存在', htmlSrc.includes('id="mcpAccountsSample"'));
check('输入框 mcpAccountsInput 存在', htmlSrc.includes('id="mcpAccountsInput"'));
check('生成按钮 mcpAccountsGenBtn 存在', htmlSrc.includes('id="mcpAccountsGenBtn"'));
check('预览块 mcpAccountsPreview 存在', htmlSrc.includes('id="mcpAccountsPreview"'));
check('复制按钮 mcpAccountsCopyBtn 存在', htmlSrc.includes('id="mcpAccountsCopyBtn"'));
check('错误提示 mcpAccountsErr 存在', htmlSrc.includes('id="mcpAccountsErr"'));
check('base64 命令段提及 PowerShell', /配置操作账户[\s\S]{0,6000}PowerShell/.test(htmlSrc));
check('安全边界:零持久化', /配置操作账户[\s\S]{0,8000}零持久化/.test(htmlSrc));
check('安全边界:能带 header 即能签名', /配置操作账户[\s\S]{0,8000}能签名/.test(htmlSrc));

// ---------- 2-7. buildAccountsConfig 行为 ----------
console.log('[2] buildAccountsConfig 行为');
let buildAccountsConfig;
try {
  const fnSrc = sliceBetween(appSrc, 'function buildAccountsConfig', 'function initMcpAccountsGen');
  buildAccountsConfig = new Function(fnSrc + '\n return buildAccountsConfig;')();
  check('函数源码抽取成功', true);
} catch (e) {
  check('函数源码抽取成功', false, e.message);
}

if (buildAccountsConfig) {
  const okArr = '[{"name":"main","address":"AddrMain","privateKey":"privhex01"},{"name":"q","address":"AddrQ","privateKey":"privQ","keyType":"fndsa512","publicKey":"pubQ"}]';

  const ok = buildAccountsConfig(okArr);
  check('合法数组 → 产出配置对象', ok && ok.ok === true, JSON.stringify(ok));
  if (ok && ok.ok) {
    check('配置含 mcpServers.milon', !!(ok.config && ok.config.mcpServers && ok.config.mcpServers.milon));
    const hdr = ok.config && ok.config.mcpServers && ok.config.mcpServers.milon &&
      ok.config.mcpServers.milon.headers && ok.config.mcpServers.milon.headers['X-Milon-Accounts'];
    check('headers 带 X-Milon-Accounts', typeof hdr === 'string' && hdr.length > 0);
    if (hdr) {
      const decoded = Buffer.from(hdr, 'base64').toString('utf8');
      const back = JSON.parse(decoded);
      check('base64 解码回原文(数组保真)', Array.isArray(back) && back.length === 2 && back[0].address === 'AddrMain');
      // UTF-8 安全:中文 name
      const cn = buildAccountsConfig('[{"name":"主账户","address":"A1","privateKey":"p1"}]');
      if (cn && cn.ok) {
        const back2 = JSON.parse(Buffer.from(cn.config.mcpServers.milon.headers['X-Milon-Accounts'], 'base64').toString('utf8'));
        check('UTF-8 安全(中文 name 编码回读)', back2[0].name === '主账户');
      } else {
        check('UTF-8 安全(中文 name 编码回读)', false, JSON.stringify(cn));
      }
    }
  }

  const badJson = buildAccountsConfig('{not json');
  check('坏 JSON → ok=false 且报错含 JSON', badJson && badJson.ok === false && /JSON/.test(badJson.error));

  const notArr = buildAccountsConfig('{"address":"x"}');
  check('非数组 → ok=false 且报错含数组', notArr && notArr.ok === false && /数组/.test(notArr.error));

  const missAddr = buildAccountsConfig('[{"privateKey":"p"}]');
  check('缺 address → 定位 [0]', missAddr && missAddr.ok === false && /\[0\]/.test(missAddr.error) && /address/.test(missAddr.error));

  const missPriv = buildAccountsConfig('[{"address":"A"},{"address":"B","privateKey":"p"}]');
  check('缺 privateKey → 定位 [0]', missPriv && missPriv.ok === false && /\[0\]/.test(missPriv.error) && /privateKey/.test(missPriv.error));

  const fndsaNoPk = buildAccountsConfig('[{"address":"A","privateKey":"p","keyType":"fndsa512"}]');
  check('fndsa512 缺 publicKey → 明确报错', fndsaNoPk && fndsaNoPk.ok === false && /publicKey/.test(fndsaNoPk.error));

  const badType = buildAccountsConfig('[{"address":"A","privateKey":"p","keyType":"rsa"}]');
  check('非法 keyType → 报错含四曲线', badType && badType.ok === false && /secp256k1/.test(badType.error));

  const dup = buildAccountsConfig('[{"address":"A","privateKey":"p"},{"address":"A","privateKey":"p2"}]');
  check('地址重复 → 拒绝', dup && dup.ok === false && /重复/.test(dup.error));

  const many = JSON.stringify(Array.from({ length: 65 }, (_, i) => ({ address: 'A' + i, privateKey: 'p' + i })));
  const over = buildAccountsConfig(many);
  check('65 条超上限 → 拒绝', over && over.ok === false && /64/.test(over.error));

  const empty = buildAccountsConfig('  ');
  check('空输入 → ok=false(不动作)', empty && empty.ok === false);
}

// ---------- 8. 版本号 bump ----------
console.log('[3] 静态资源版本号');
const versionMatch = htmlSrc.match(/app\.js\?v=([0-9a-z]+)/);
check('app.js 版本号新于 20261013b', !!versionMatch && versionMatch[1] !== '20261013b',
  versionMatch ? 'got ' + versionMatch[1] : 'no match');
check('css 版本号同步', !/style\.css\?v=20261013b/.test(htmlSrc));

console.log('');
if (failed > 0) {
  console.log('FAILED: ' + failed);
  process.exit(1);
} else {
  console.log('ALL PASS');
}
