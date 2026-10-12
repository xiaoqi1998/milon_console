// test_js_escape.js — 前端注入安全测试(直接从 static/js/app.js 抽取真实源码)
// 运行:node test_js_escape.js
// 覆盖:
//   1. formatJSON txHash 按钮:功能存活(正则与高亮 span 结构匹配)
//   2. data-hash 属性引号逃逸(属性注入 → XSS)
//   3. tx_hash 变体键名
//   4. 语法高亮回归守卫(防止有人"顺手"给 escapeHTML 全局加引号转义打断高亮)
//   5. 元素内容上下文的 < > 转义不回归
const fs = require('fs');
const path = require('path');

const APP_JS = path.join(__dirname, 'static', 'js', 'app.js');
const src = fs.readFileSync(APP_JS, 'utf8');

// 标记切分抽取(函数体内正则含 {} 字面量,brace-matching 不可靠,用相邻函数作界标)
function sliceBetween(startMarker, endMarker) {
  const s = src.indexOf(startMarker);
  if (s < 0) throw new Error('marker not found: ' + startMarker);
  const e = src.indexOf(endMarker, s);
  if (e < 0) throw new Error('end marker not found: ' + endMarker);
  return src.slice(s, e);
}

const escapeHTMLSrc = sliceBetween('function escapeHTML', '// 线性 SVG 图标');
const formatJSONSrc = sliceBetween('function formatJSON', 'function copyCurl');

// 组装被测作用域:escapeHTML + formatJSON 同域(formatJSON 依赖 escapeHTML)
const scope = new Function(escapeHTMLSrc + '\n' + formatJSONSrc + '\n return formatJSON;');
const formatJSON = scope();

// 按钮标签的安全形状:只有 class/data-hash/title 三个属性,data-hash 值内无裸引号。
// 若 payload 引号逃逸出属性,标签里会多出 onmouseover= 之类属性,该严格模式即失配。
const SAFE_BUTTON_TAG = /^<button class="txhash-convert-btn" data-hash="[^"]*" title="转换为 base58">$/;

function buttonTag(html) {
  const m = html.match(/<button[^>]*>/);
  return m ? m[0] : null;
}

let failed = 0;
function check(name, cond, detail) {
  if (cond) {
    console.log('  PASS ' + name);
  } else {
    failed++;
    console.log('  FAIL ' + name + (detail ? '\n       ' + detail : ''));
  }
}

console.log('== formatJSON txHash 按钮:功能与注入安全 ==');

// 1) 恶意 payload:引号必须实体化,逃不出 data-hash 属性
{
  const payload = 'x" onmouseover="alert(1)" data-x="y';
  const out = formatJSON({ success: true, data: { txHash: payload } });
  const tag = buttonTag(out);
  check('txHash 按钮已生成(功能存活)', tag !== null,
    '按钮未生成:高亮 span 结构与按钮正则不匹配(功能死代码)');
  if (tag) {
    check('data-hash 属性无引号逃逸(严格标签形状)', SAFE_BUTTON_TAG.test(tag),
      '实际标签: ' + tag);
    check('标签内无注入的事件处理器属性', !/\son\w+\s*=\s*"[^"]*"\s+(?:data-x|title)/.test(tag) && !/ data-x="/.test(tag),
      '实际标签: ' + tag);
  }
}

// 2) tx_hash 变体键名同样覆盖
{
  const out = formatJSON({ data: { tx_hash: 'a" onclick="evil()' } });
  const tag = buttonTag(out);
  check('tx_hash 键按钮已生成', tag !== null);
  if (tag) {
    check('tx_hash 按钮无引号逃逸', SAFE_BUTTON_TAG.test(tag), '实际标签: ' + tag);
  }
}

console.log('== 高亮与既有转义回归守卫 ==');

// 3) 语法高亮不被破坏(引号必须以原样存活到 token 匹配阶段)
{
  const out = formatJSON({ a: 'b' });
  check('json-key span 仍生成', out.includes('json-key'));
  check('json-string span 仍生成', out.includes('json-string'));
  check('json-punct span 仍生成', out.includes('json-punct'));
}

// 4) 元素内容上下文的 HTML 转义保持
{
  const out = formatJSON({ x: '<img src=x onerror=y>' });
  check('< > 仍被转义(防标签注入)', out.includes('&lt;img') && !out.includes('<img src=x onerror'));
}

// 5) 常规 hex txHash:按钮正常生成且属性值干净(不带 JSON 引号)
{
  const out = formatJSON({ txHash: 'deadbeef' });
  const tag = buttonTag(out);
  check('正常 hex txHash 按钮生成', tag !== null && /data-hash="deadbeef"/.test(tag),
    tag ? '实际标签: ' + tag : '按钮未生成');
}

console.log(failed === 0 ? '\nALL PASS' : '\n' + failed + ' FAILED');
process.exit(failed === 0 ? 0 : 1);
