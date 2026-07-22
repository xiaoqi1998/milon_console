/* Milon API Console - app.js (native JS, no framework) */
const ENDPOINTS = [
  {id:'net-list', method:'GET', path:'/api/network/list', summary:'获取网络列表', group:'网络管理'},
  {id:'net-current', method:'GET', path:'/api/network/current', summary:'获取当前网络', group:'网络管理'},
  {id:'net-switch', method:'POST', path:'/api/network/switch', summary:'切换网络', group:'网络管理', bodyTemplate: JSON.stringify({network:'devNet'},null,2)},
  {id:'health', method:'GET', path:'/api/health', summary:'健康检查', group:'系统'},
  {id:'chain-head', method:'GET', path:'/api/chain-head', summary:'获取链头', group:'系统'},
  {id:'acc-info', method:'GET', path:'/api/accounts/:address', summary:'获取账户信息', group:'账户', pathParams:[{name:'address',ph:'base58地址'}]},
  {id:'acc-resources', method:'GET', path:'/api/accounts/:address/resources', summary:'获取账户资源', group:'账户', pathParams:[{name:'address',ph:'base58地址'}]},
  {id:'acc-generate', method:'POST', path:'/api/accounts/generate', summary:'生成账户', group:'账户'},
  {id:'tx-hash', method:'GET', path:'/api/transactions/:hash', summary:'按哈希查交易', group:'交易', pathParams:[{name:'hash',ph:'hex或base58'}]},
  {id:'tx-events', method:'GET', path:'/api/transactions/:hash/events', summary:'获取交易事件', group:'交易', pathParams:[{name:'hash',ph:'hex或base58'}], queryParams:[{name:'typeTag',ph:'可选'}]},
  {id:'tx-wait', method:'GET', path:'/api/transactions/:hash/wait', summary:'等待交易确认', group:'交易', pathParams:[{name:'hash',ph:'hex或base58'}], queryParams:[{name:'timeoutSecs',ph:'60'}]},  {id:'tx-simulate', method:'POST', path:'/api/transactions/simulate', summary:'底层模拟', group:'交易', bodyTemplate: JSON.stringify({transactionPostcard:'base64编码'},null,2)},
  {id:'tx-submit', method:'POST', path:'/api/transactions/submit', summary:'底层提交', group:'交易', bodyTemplate: JSON.stringify({transactionPostcard:'base64编码'},null,2)},
  {id:'read', method:'POST', path:'/api/read', summary:'读取视图函数', group:'合约', bodyTemplate: JSON.stringify({appName:'token',methodName:'balance_of',args:{owner:'base58地址'},payerAddress:'base58地址'},null,2)},
  {id:'simulate', method:'POST', path:'/api/simulate', summary:'模拟合约调用', group:'合约', bodyTemplate: JSON.stringify({appName:'token',methodName:'transfer',args:{to:'base58地址',amount:1000},paymentMode:'unified_payer_all',payerAddress:'base58地址',signatureMode:{type:'pubkey',publicKey:'base58公钥'}},null,2)},
  {id:'write', method:'POST', path:'/api/write', summary:'写入交易', group:'合约', bodyTemplate: JSON.stringify({appName:'token',methodName:'transfer',args:{to:'base58地址',amount:1000},paymentMode:'unified_payer_all',payerPrivateKey:'hex或base58私钥',payerAddress:'base58地址',signatureMode:{type:'pubkey',publicKey:'base58公钥'}},null,2)},  {id:'write-multi', method:'POST', path:'/api/write/multi-agent', summary:'多方签名写入', group:'合约', bodyTemplate: JSON.stringify({appName:'token',methodName:'transfer',args:{},paymentMode:'unified_dual_sign',payerPrivateKey:'',payerAddress:'',ixPrivateKey:'',ixAddress:'',signatureMode:{type:'pubkey',publicKey:''}},null,2)},
  {id:'write-multisig', method:'POST', path:'/api/write/multisig', summary:'多签写入', group:'合约', bodyTemplate: JSON.stringify({appName:'token',methodName:'transfer',args:{},paymentMode:'split',ownerPrivateKey:'',ownerAddress:'',signatureMode:{type:'pubkey',publicKey:''}},null,2)},
  {id:'block', method:'GET', path:'/api/rpc/blocks/:height', summary:'获取区块', group:'RPC', pathParams:[{name:'height',ph:'区块高度'}]},
  {id:'resource', method:'GET', path:'/api/rpc/resources/:hash', summary:'获取资源', group:'RPC', pathParams:[{name:'hash',ph:'hex 18字节'}]},
  {id:'access-value', method:'POST', path:'/api/rpc/access-value', summary:'获取访问值', group:'RPC', bodyTemplate: JSON.stringify({blobHashes:['hex 32字节']},null,2)},
  {id:'derive-addr', method:'POST', path:'/api/util/address/derive', summary:'从公钥派生地址', group:'工具', bodyTemplate: JSON.stringify({publicKey:'hex或base58',keyType:'secp256k1'},null,2)},
  {id:'derive-pub', method:'POST', path:'/api/util/key/derive-public', summary:'从私钥派生公钥', group:'工具', bodyTemplate: JSON.stringify({privateKey:'hex或base58',keyType:'secp256k1'},null,2)},
  {id:'sign', method:'POST', path:'/api/util/sign', summary:'签名消息', group:'工具', bodyTemplate: JSON.stringify({privateKey:'hex或base58',message:'hex编码',keyType:'secp256k1'},null,2)},
  {id:'verify', method:'POST', path:'/api/util/verify', summary:'验签', group:'工具', bodyTemplate: JSON.stringify({publicKey:'hex或base58',message:'hex编码',signature:'hex'},null,2)},
];
const state = { currentEndpoint: null, history: [], activeTab: 'json', loading: false };
const MAX_HISTORY = 20;function $(id) { return document.getElementById(id); }
function el(tag, attrs) {
  var node = document.createElement(tag);
  if (attrs) {
    for (var k in attrs) {
      if (k === 'class') node.className = attrs[k];
      else if (k === 'text') node.textContent = attrs[k];
      else if (k.indexOf('on') === 0 && typeof attrs[k] === 'function') node.addEventListener(k.slice(2).toLowerCase(), attrs[k]);
      else if (attrs[k] != null) node.setAttribute(k, attrs[k]);
    }
  }
  for (var i = 2; i < arguments.length; i++) {
    var c = arguments[i];
    if (c == null) continue;
    node.appendChild(typeof c === 'string' ? document.createTextNode(c) : c);
  }
  return node;
}
function escapeHTML(s) {
  return String(s).replace(/[&<>]/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;'}[c]; });
}
var toastTimer = null;
function showToast(msg, type) {
  var t = $('toast');
  t.textContent = msg;
  t.className = 'toast show' + (type ? ' ' + type : '');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(function(){ t.className = 'toast'; }, 2400);
}function renderEndpoints(filter) {
  var tree = $('endpointTree');
  tree.innerHTML = '';
  var keyword = (filter || '').trim().toLowerCase();
  var groups = {};
  ENDPOINTS.forEach(function(ep){
    if (keyword) {
      var hay = (ep.method + ' ' + ep.path + ' ' + ep.summary + ' ' + ep.group).toLowerCase();
      if (hay.indexOf(keyword) < 0) return;
    }
    if (!groups[ep.group]) groups[ep.group] = [];
    groups[ep.group].push(ep);
  });
  var groupOrder = ['网络管理','系统','账户','交易','合约','RPC','工具'];
  var keys = groupOrder.filter(function(g){ return groups[g]; })
    .concat(Object.keys(groups).filter(function(g){ return groupOrder.indexOf(g) < 0; }));
  if (keys.length === 0) {
    tree.appendChild(el('div', {class:'history-empty', text:'无匹配端点'}));
    return;
  }
  keys.forEach(function(group){
    var gw = el('div', {class:'endpoint-group'}, el('div', {class:'group-title', text: group}));
    groups[group].forEach(function(ep){
      gw.appendChild(el('div', {
        class: 'endpoint-item' + (state.currentEndpoint && state.currentEndpoint.id === ep.id ? ' active' : ''),
        'data-id': ep.id,
        onclick: function(){ selectEndpoint(ep.id); }
      },
        el('span', {class:'endpoint-method ' + ep.method, text: ep.method}),
        el('div', {class:'endpoint-text'},
          el('span', {class:'endpoint-name', text: ep.path}),
          el('span', {class:'endpoint-desc', text: ep.summary})
        )
      ));
    });
    tree.appendChild(gw);
  });
}function selectEndpoint(id) {
  var ep = ENDPOINTS.find(function(e){ return e.id === id; });
  if (!ep) return;
  state.currentEndpoint = ep;
  renderEndpoints($('endpointSearch').value);
  var m = $('currentMethod');
  m.textContent = ep.method;
  m.className = 'method-badge ' + ep.method;
  $('currentPath').textContent = ep.path;
  $('currentSummary').textContent = ep.summary;
  renderParams(ep);
  if (window.innerWidth <= 768) $('sidebar').classList.add('collapsed');
}
function renderParams(ep) {
  var body = $('editorBody');
  body.innerHTML = '';
  if (ep.pathParams && ep.pathParams.length) {
    var sec = el('div', {class:'param-section'}, el('div', {class:'param-section-title', text:'路径参数'}));
    ep.pathParams.forEach(function(p){
      sec.appendChild(el('div', {class:'param-row'},
        el('label', {class:'param-label', text: ':' + p.name}),
        el('input', {class:'param-input', 'data-pkind':'path','data-pname':p.name, placeholder:p.ph || '', type:'text'})
      ));
    });
    body.appendChild(sec);
  }
  if (ep.queryParams && ep.queryParams.length) {
    var qs = el('div', {class:'param-section'}, el('div', {class:'param-section-title', text:'查询参数'}));
    ep.queryParams.forEach(function(p){
      qs.appendChild(el('div', {class:'param-row'},
        el('label', {class:'param-label', text: p.name}),
        el('input', {class:'param-input', 'data-pkind':'query','data-pname':p.name, placeholder:p.ph || '', type:'text'})
      ));
    });
    body.appendChild(qs);
  }
  if (ep.method === 'POST') {
    var bs = el('div', {class:'param-section'});
    bs.appendChild(el('div', {class:'body-toolbar'},
      el('div', {class:'param-section-title', text:'请求体 (JSON)', style:'margin-bottom:0;border-bottom:none'}),
      el('div', {style:'display:flex;gap:6px;'},
        el('button', {class:'body-format-btn', text:'格式化', onclick: formatBody}),
        el('button', {class:'body-format-btn', text:'清空', onclick: clearBody})
      )
    ));
    var ta = el('textarea', {class:'body-editor', id:'bodyEditor', spellcheck:'false', placeholder:'{ }'});
    ta.value = ep.bodyTemplate || '';
    bs.appendChild(ta);
    body.appendChild(bs);
  }
  if (!body.children.length) {
    body.appendChild(el('div', {class:'history-empty', text:'该端点无需参数，直接点击「发送」'}));
  }
}function formatBody() {
  var ta = $('bodyEditor');
  if (!ta) return;
  try { ta.value = JSON.stringify(JSON.parse(ta.value), null, 2); showToast('已格式化', 'success'); }
  catch (e) { showToast('JSON 解析失败: ' + e.message, 'error'); }
}
function clearBody() { var ta = $('bodyEditor'); if (ta) ta.value = ''; }
function buildRequest() {
  var ep = state.currentEndpoint;
  if (!ep) return null;
  var url = ep.path;
  document.querySelectorAll('.param-input[data-pkind="path"]').forEach(function(inp){
    var name = inp.getAttribute('data-pname');
    var val = inp.value.trim();
    url = url.replace(':' + name, encodeURIComponent(val));
  });
  var qs = [];
  document.querySelectorAll('.param-input[data-pkind="query"]').forEach(function(inp){
    var name = inp.getAttribute('data-pname');
    var val = inp.value.trim();
    if (val !== '') qs.push(encodeURIComponent(name) + '=' + encodeURIComponent(val));
  });
  if (qs.length) url += '?' + qs.join('&');
  var body = null;
  if (ep.method === 'POST') {
    var ta = $('bodyEditor');
    if (ta && ta.value.trim()) body = ta.value.trim();
  }
  return { method: ep.method, url: url, body: body };
}
async function sendRequest() {
  if (!state.currentEndpoint) { showToast('请先选择端点', 'error'); return; }
  var req = buildRequest();
  if (!req) return;
  var missing = req.url.match(/:[^/?]+/);
  if (missing) { showToast('路径参数未填写: ' + missing[0], 'error'); return; }
  state.loading = true;
  setSendLoading(true);
  showResponseLoading();
  var start = performance.now();
  try {
    var opt = { method: req.method, headers: {} };
    if (req.body) { opt.headers['Content-Type'] = 'application/json'; opt.body = req.body; }
    var resp = await fetch(req.url, opt);
    var duration = Math.round(performance.now() - start);
    var text = await resp.text();
    var data; try { data = JSON.parse(text); } catch (e) { data = text; }
    displayResponse(data, resp.status, duration, resp.headers, text);
    addToHistory(state.currentEndpoint, req, resp.status, duration, data);
  } catch (err) {
    var d2 = Math.round(performance.now() - start);
    displayError(err, d2);
    addToHistory(state.currentEndpoint, req, 0, d2, { error: String(err) });
  } finally {
    state.loading = false; setSendLoading(false);
  }
}function setSendLoading(loading) {
  var btn = $('sendBtn');
  btn.disabled = loading;
  btn.querySelector('span').textContent = loading ? '发送中...' : '发送';
}
function showResponseLoading() {
  $('responseMeta').innerHTML = '<span class="meta-placeholder">请求中...</span>';
  $('tab-json').innerHTML = '';
  $('tab-json').appendChild(el('div', {class:'loading-overlay'}, el('div', {class:'spinner'})));
  $('tab-headers').innerHTML = '<div class="response-empty">请求中...</div>';
  $('tab-raw').innerHTML = '<div class="response-empty">请求中...</div>';
}
function displayResponse(data, statusCode, duration, headers, rawText) {
  var meta = $('responseMeta');
  meta.innerHTML = '';
  var sc = statusCode >= 200 && statusCode < 300 ? 'success' : (statusCode >= 400 ? 'error' : 'warn');
  meta.appendChild(el('span', {class:'status-code ' + sc, text: String(statusCode)}));
  meta.appendChild(el('span', {text: ' · ' + duration + 'ms'}));
  meta.appendChild(el('span', {text: ' · ' + (state.currentEndpoint ? state.currentEndpoint.method : '')}));
  var jp = $('tab-json');
  jp.innerHTML = '';
  if (typeof data === 'string') {
    jp.appendChild(el('div', {class:'raw-viewer', text: data || '(空响应)'}));
  } else {
    var pre = el('pre', {class:'json-viewer'});
    pre.innerHTML = formatJSON(data);
    jp.appendChild(pre);
  }
  var hp = $('tab-headers');
  hp.innerHTML = '';
  if (headers) {
    var tbl = el('table', {class:'headers-table'},
      el('thead', {}, el('tr', {}, el('th', {text:'Header'}), el('th', {text:'Value'})))
    );
    var tb = el('tbody', {});
    var seen = {};
    headers.forEach(function(val, key){
      var lk = key.toLowerCase();
      if (seen[lk]) return;
      seen[lk] = true;
      tb.appendChild(el('tr', {}, el('td', {text: key}), el('td', {text: val})));
    });
    tbl.appendChild(tb);
    hp.appendChild(tbl);
  } else {
    hp.appendChild(el('div', {class:'response-empty', text:'无响应头'}));
  }
  var rp = $('tab-raw');
  rp.innerHTML = '';
  rp.appendChild(el('pre', {class:'raw-viewer', text: rawText || '(空响应)'}));
}function displayError(err, duration) {
  var meta = $('responseMeta');
  meta.innerHTML = '';
  meta.appendChild(el('span', {class:'status-code error', text:'ERR'}));
  meta.appendChild(el('span', {text: ' · ' + duration + 'ms'}));
  var msg = (err && err.message) ? err.message : String(err);
  $('tab-json').innerHTML = '';
  $('tab-json').appendChild(el('div', {class:'error-box', text:'请求失败: ' + msg + '\n\n请检查:\n- 后端服务是否启动\n- 网络是否可达\n- 是否存在跨域问题'}));
  $('tab-headers').innerHTML = '<div class="response-empty">无响应头</div>';
  $('tab-raw').innerHTML = '<div class="response-empty">无原始响应</div>';
}
function formatJSON(obj) {
  var json = JSON.stringify(obj, null, 2);
  return escapeHTML(json).replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g,
    function(match){
      var cls = 'json-number';
      if (/^"/.test(match)) { cls = /:$/.test(match) ? 'json-key' : 'json-string'; }
      else if (/true|false/.test(match)) { cls = 'json-boolean'; }
      else if (/null/.test(match)) { cls = 'json-null'; }
      return '<span class="' + cls + '">' + match + '</span>';
    }
  ).replace(/([{}\[\],])/g, '<span class="json-punct">$1</span>');
}
function copyCurl() {
  var ep = state.currentEndpoint;
  if (!ep) { showToast('请先选择端点', 'error'); return; }
  var req = buildRequest();
  if (!req) return;
  var origin = window.location.origin;
  var cmd = 'curl -X ' + req.method + " '" + origin + req.url + "'";
  if (req.body) {
    cmd += ' \\\n  -H \'Content-Type: application/json\'';
    cmd += ' \\\n  -d \'' + req.body.replace(/'/g, "'\\''") + "'";
  }
  function done(){ showToast('curl 命令已复制', 'success'); }
  function fail(){ showToast('复制失败', 'error'); }
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(cmd).then(done, function(){
      var ta = document.createElement('textarea'); ta.value = cmd; document.body.appendChild(ta); ta.select();
      try { document.execCommand('copy'); done(); } catch (e) { fail(); }
      document.body.removeChild(ta);
    });
  } else { fail(); }
}async function loadNetworks() {
  var sel = $('networkSelect');
  sel.innerHTML = '<option value="">加载中...</option>';
  try {
    var resp = await fetch('/api/network/list');
    var data = await resp.json();
    sel.innerHTML = '';
    var list = [];
    if (Array.isArray(data)) list = data;
    else if (data && Array.isArray(data.networks)) list = data.networks;
    else if (data && Array.isArray(data.data)) list = data.data;
    else if (data && typeof data === 'object') {
      ['networks','list','items','result'].forEach(function(k){ if (Array.isArray(data[k])) list = data[k]; });
    }
    if (!list.length) { sel.innerHTML = '<option value="">（无网络）</option>'; return; }
    list.forEach(function(n){
      var name = typeof n === 'string' ? n : (n.name || n.network || n.id || JSON.stringify(n));
      sel.appendChild(el('option', {value: name, text: name}));
    });
    try {
      var cur = await fetch('/api/network/current');
      var cd = await cur.json();
      var cn = '';
      if (typeof cd === 'string') cn = cd;
      else if (cd) {
        ['network','name','current','id'].forEach(function(k){ if (!cn && cd[k]) cn = cd[k]; });
        if (!cn && cd.data) { ['network','name','current','id'].forEach(function(k){ if (!cn && cd.data[k]) cn = cd.data[k]; }); }
      }
      if (cn) sel.value = cn;
    } catch (e) {}
  } catch (err) {
    sel.innerHTML = '<option value="">加载失败</option>';
    showToast('网络列表加载失败', 'error');
  }
}
async function switchNetwork(name) {
  if (!name) return;
  try {
    var resp = await fetch('/api/network/switch', {
      method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({network: name})
    });
    if (resp.ok) showToast('已切换到 ' + name, 'success');
    else { var t = await resp.text(); showToast('切换失败: ' + (resp.status + ' ' + t).slice(0,80), 'error'); }
  } catch (err) { showToast('切换失败: ' + (err.message || err), 'error'); }
}
function addToHistory(endpoint, req, statusCode, duration, data) {
  state.history.unshift({
    id: Date.now(), endpoint: endpoint, req: req, statusCode: statusCode, duration: duration,
    time: new Date().toLocaleTimeString('zh-CN', {hour12:false})
  });
  if (state.history.length > MAX_HISTORY) state.history.length = MAX_HISTORY;
  renderHistory();
}function renderHistory() {
  var panel = $('historyPanel');
  $('historyCount').textContent = String(state.history.length);
  panel.innerHTML = '';
  if (state.history.length === 0) {
    panel.appendChild(el('div', {class:'history-empty', text:'暂无历史记录'}));
    return;
  }
  state.history.forEach(function(h){
    var sc = h.statusCode === 0 ? 'error' : (h.statusCode >= 200 && h.statusCode < 300 ? 'success' : 'error');
    panel.appendChild(el('div', {
      class:'history-item', 'data-id': h.id,
      onclick: function(){ reloadHistory(h.id); }
    },
      el('span', {class:'h-method ' + h.endpoint.method, text: h.endpoint.method}),
      el('span', {class:'h-path', text: h.req.url}),
      el('span', {class:'h-status ' + sc, text: h.statusCode === 0 ? 'ERR' : String(h.statusCode)}),
      el('span', {class:'h-time', text: h.time + ' · ' + h.duration + 'ms'})
    ));
  });
}
function reloadHistory(id) {
  var h = state.history.find(function(x){ return x.id === id; });
  if (!h) return;
  selectEndpoint(h.endpoint.id);
  $('historyBar').classList.remove('expanded');
  showToast('已加载历史端点', 'success');
}
function switchTab(name) {
  state.activeTab = name;
  document.querySelectorAll('.tab').forEach(function(t){
    t.classList.toggle('active', t.getAttribute('data-tab') === name);
  });
  document.querySelectorAll('.tab-panel').forEach(function(p){
    p.classList.toggle('active', p.id === 'tab-' + name);
  });
}
function initApp() {
  renderEndpoints();
  $('sendBtn').addEventListener('click', sendRequest);
  $('resetBtn').addEventListener('click', function(){
    if (state.currentEndpoint) { renderParams(state.currentEndpoint); showToast('参数已重置', 'success'); }
  });
  $('copyCurlBtn').addEventListener('click', copyCurl);
  document.querySelectorAll('.tab').forEach(function(t){
    t.addEventListener('click', function(){ switchTab(t.getAttribute('data-tab')); });
  });
  $('sidebarToggle').addEventListener('click', function(){ $('sidebar').classList.toggle('collapsed'); });
  $('endpointSearch').addEventListener('input', function(e){ renderEndpoints(e.target.value); });
  $('networkSelect').addEventListener('change', function(e){ switchNetwork(e.target.value); });
  $('refreshBtn').addEventListener('click', loadNetworks);
  $('historyToggle').addEventListener('click', function(){ $('historyBar').classList.toggle('expanded'); });
  document.addEventListener('keydown', function(e){
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); sendRequest(); }
  });
  renderHistory();
  loadNetworks();
  if (window.innerWidth <= 768) $('sidebar').classList.add('collapsed');
  if (ENDPOINTS.length > 0) selectEndpoint(ENDPOINTS[0].id);
}
document.addEventListener('DOMContentLoaded', initApp);