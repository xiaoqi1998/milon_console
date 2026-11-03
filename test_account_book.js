// test_account_book.js — 账号本 UI 测试(源码直抽)
// 运行:node test_account_book.js
const fs = require('fs');
const path = require('path');
const ROOT = __dirname;
const jsSrc = fs.readFileSync(path.join(ROOT, 'static', 'js', 'account_book.js'), 'utf8');
const htmlSrc = fs.readFileSync(path.join(ROOT, 'static', 'index.html'), 'utf8');

let failed = 0;
function check(name, cond, detail) {
  if (cond) console.log('  ok  ' + name);
  else { failed++; console.log('FAIL  ' + name + (detail ? '\n      ' + detail : '')); }
}

// ---------- 1. 纯逻辑:三种导入形态归一(先于 mock 环境直抽) ----------
global.apiFetch = () => Promise.resolve({ json: async () => ({ success: true, data: [] }) });
eval(jsSrc);

const flat = bookNormalizeImport([{ name: 'u1', privateKey: 'aa', note: 'x' }]);
check('平铺数组', flat.length === 1 && flat[0].name === 'u1' && flat[0].note === 'x');

const wrap = bookNormalizeImport({ accounts: [{ name: 'u2', address: 'A' }] });
check('{accounts:[…]} 包装', wrap.length === 1 && wrap[0].name === 'u2');

const ds = bookNormalizeImport({
  owner: { privateKey: 'bc', role: 'gas payer' },
  users: [{ name: 'user1', address: 'B' }, { name: 'user2', address: 'C' }],
  newUsers: [{ name: 'new1', privateKey: 'dd' }],
});
check('sft_1000_testdata 三段式(owner 缺名补 owner)', ds.length === 4 &&
  ds[0].name === 'owner' && ds[1].name === 'user1' && ds[3].name === 'new1');

const single = bookNormalizeImport({ name: 'solo', address: 'D' });
check('单对象', single.length === 1 && single[0].name === 'solo');

let threw = '';
try { bookNormalizeImport('hello'); } catch (e) { threw = e.message; }
check('非法形态抛错', threw !== '');

// keyType 缺省 secp256k1
check('keyType 缺省', bookNormalizeImport([{ name: 'u1', privateKey: 'aa' }])[0].keyType === 'secp256k1');

// ---------- 2. HTML 锚点 ----------
for (const id of ['accountBookBox', 'accountBookAddBtn', 'accountBookImportBtn', 'accountBookSubmitBtn',
  'accountBookExportBtn', 'accountBookRefreshBtn', 'accountBookImportArea', 'accountBookCards']) {
  check('HTML 锚点 #' + id, htmlSrc.includes('id="' + id + '"'));
}
check('account_book.js 已引入', /account_book\.js\?v=\d+/.test(htmlSrc));

// ---------- 3. 关键函数 ----------
for (const fn of ['initAccountBook', 'refreshAccountBook', 'renderAccountBook', 'submitAccountBookImport']) {
  check('JS 函数 ' + fn, jsSrc.includes('function ' + fn));
}

// ---------- 4. mock DOM 渲染烟雾 ----------
function mockNode(tag) {
  return {
    tag, children: [], style: {}, dataset: {}, value: '', className: '',
    classList: { add() {}, remove() {}, toggle() {} },
    setAttribute() {}, addEventListener() {},
    appendChild(c) { this.children.push(c); },
    querySelectorAll() { return []; },
    set textContent(v) { this._txt = v; }, get textContent() { return this._txt || ''; },
    set innerHTML(v) { this._html = v; }, get innerHTML() { return this._html || ''; },
  };
}
const boxMock = mockNode('div');
global.document = {
  readyState: 'complete',
  createElement: (t) => mockNode(t),
  createTextNode: (t) => ({ text: t }),
  getElementById: (id) => (id === 'accountBookCards' ? boxMock : null),
  addEventListener() {},
};
global.el = (tag, attrs) => { const n = mockNode(tag); if (attrs && attrs.class) n.className = attrs.class; return n; };
global.showToast = () => {};
accountBookState.rows = [{ name: 'u1', address: 'A', privateKey: 'aabbccdd', keyType: 'secp256k1', note: '' }];
let threw2 = '';
try { renderAccountBook(); } catch (e) { threw2 = e.message; }
check('renderAccountBook 可执行', threw2 === '', threw2);
check('渲染产生表格节点', boxMock.children.length > 0);

console.log(failed === 0 ? '\nALL PASS' : `\n${failed} FAILED`);
process.exit(failed === 0 ? 0 : 1);
