// ==================== 保存指令（Saved Instructions）前端层 ====================
// 依赖 app.js 的全局：state / $ / el / showToast / renderIDLAppList / renderEndpoints /
// buildIDLRequest / displayIDLResponse / displayResponse / getCurrentAccount
// 展示设计：
//  1) IDL 方法页：侧栏方法条目带 ★N 徽标；方法详情面板顶部「保存的指令」区，
//     一个方法可挂多条指令，卡片式列出，支持 执行/载入表单/编辑/删除；
//  2) 控制台侧栏：动态「保存指令」分组，点条目弹出执行弹窗（read/simulate/send）；
//  3) 控制台 read/simulate/write 请求体工具栏：「存为指令」按钮，把当前 JSON 存成指令。

state.savedInstructions = [];
state.savedLoaded = false;
state.idlPrefill = null; // 载入指令到 IDL 表单时的预填数据

var SAVED_CONSOLE_ENDPOINTS = ['read', 'simulate', 'write'];
var SAVED_PAYMENT_MODES = [
  { value: 'unified_payer_all', label: 'unified_payer_all（payer 签 ix+gas）' },
  { value: 'unified_dual_sign', label: 'unified_dual_sign（payer/ix 双账户）' },
  { value: 'unified_payer_only_gas', label: 'unified_payer_only_gas（payer 只签 gas）' },
  { value: 'split', label: 'split（owner 自付）' },
  { value: 'sponsored', label: 'sponsored（代付）' },
  { value: 'multi_signer', label: 'multi_signer（多签）' },
];

// ---------- 数据层 ----------

async function loadSavedInstructions() {
  try {
    var resp = await fetch('/api/saved-instructions');
    var data = await resp.json();
    state.savedInstructions = (data && data.data) ? data.data : [];
  } catch (e) {
    state.savedInstructions = [];
  }
  state.savedLoaded = true;
}

async function refreshSavedViews() {
  await loadSavedInstructions();
  if (state.idlLoaded) renderIDLAppList($('idlSearch') ? $('idlSearch').value : '');
  if (!state.idlBatchMode && state.currentIdlMethod) {
    var sec = $('idlSavedSection');
    if (sec) fillSavedSection(sec, state.currentIdlMethod);
  }
  if (state.currentView === 'console') {
    renderEndpoints($('endpointSearch') ? $('endpointSearch').value : '');
  }
}

function savedForMethod(appName, methodName) {
  return state.savedInstructions.filter(function (s) {
    return s.appName === appName && s.methodName === methodName;
  });
}

function savedById(id) {
  return state.savedInstructions.find(function (s) { return s.id === id; }) || null;
}

// ---------- IDL 侧栏徽标 ----------

function savedBadgeFor(appName, methodName) {
  var n = savedForMethod(appName, methodName).length;
  if (!n) return null;
  return el('span', {
    class: 'saved-count-badge',
    text: '\u2605' + n,
    title: n + ' 条保存的指令',
  });
}

// ---------- IDL 方法详情：保存的指令区 ----------

function renderSavedSectionForIDL(body, ix) {
  var sec = el('div', { class: 'param-section saved-section', id: 'idlSavedSection' });
  body.appendChild(sec);
  fillSavedSection(sec, ix);
}

function fillSavedSection(sec, ix) {
  sec.innerHTML = '';
  var head = el('div', { class: 'body-toolbar' },
    el('div', { class: 'param-section-title', text: '保存的指令' }),
    el('div', { class: 'body-actions' },
      el('button', {
        class: 'body-format-btn',
        text: '+ 存当前表单为指令',
        onclick: function () { saveIDLFormAsInstruction(ix); },
      })
    )
  );
  sec.appendChild(head);

  var list = savedForMethod(state.currentIdlApp, ix.name);
  if (!list.length) {
    sec.appendChild(el('div', { class: 'empty-state small', text: '该方法暂无保存的指令；填好参数后点「存当前表单为指令」即可复用' }));
    return;
  }
  list.forEach(function (item) { sec.appendChild(buildSavedCard(item, 'idl')); });
}

function buildSavedCard(item, ctx) {
  var card = el('div', { class: 'saved-card' });
  var titleRow = el('div', { class: 'saved-card-head' },
    el('span', { class: 'saved-card-name', text: item.name || '(未命名)' }),
    el('span', { class: 'idl-kind-badge ' + item.kind, text: item.kind }),
    item.paymentMode ? el('span', { class: 'saved-badge pm', text: item.paymentMode }) : null,
    el('span', { class: 'saved-badge time', text: fmtSavedTime(item.updatedAt || item.createdAt) })
  );
  card.appendChild(titleRow);
  if (item.description) {
    card.appendChild(el('div', { class: 'saved-card-desc', text: item.description }));
  }
  card.appendChild(el('div', { class: 'saved-card-args', text: 'args: ' + safeJSON(item.args) }));

  var actions = el('div', { class: 'saved-card-actions' });
  if (item.kind === 'view') {
    actions.appendChild(el('button', { class: 'btn btn-small btn-primary', text: '执行 read', onclick: function () { runSaved(item, 'read', ctx); } }));
  } else {
    actions.appendChild(el('button', { class: 'btn btn-small btn-primary', text: 'simulate', onclick: function () { runSaved(item, 'simulate', ctx); } }));
    actions.appendChild(el('button', { class: 'btn btn-small btn-danger', text: 'send 上链', onclick: function () { runSaved(item, 'send', ctx); } }));
  }
  if (ctx === 'idl') {
    actions.appendChild(el('button', { class: 'btn btn-small', text: '载入表单', onclick: function () { loadSavedIntoIDLForm(item); } }));
  }
  actions.appendChild(el('button', { class: 'btn btn-small', text: '编辑', onclick: function () { openSavedEditor(item); } }));
  actions.appendChild(el('button', {
    class: 'btn btn-small',
    text: '删除',
    onclick: function () { deleteSaved(item); },
  }));
  card.appendChild(actions);
  return card;
}

function fmtSavedTime(ms) {
  if (!ms) return '';
  var d = new Date(ms);
  function p(n) { return n < 10 ? '0' + n : String(n); }
  return p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
}

function safeJSON(v) {
  try { return JSON.stringify(v); } catch (e) { return String(v); }
}

// ---------- 执行 ----------

async function runSaved(item, mode, ctx) {
  var start = performance.now();
  try {
    var resp = await fetch('/api/saved-instructions/' + item.id + '/execute?mode=' + encodeURIComponent(mode), { method: 'POST' });
    var dur = Math.round(performance.now() - start);
    var text = await resp.text();
    var size = new Blob([text]).size;
    var data;
    try { data = JSON.parse(text); } catch (e) { data = text; }
    var target = ctx || state.currentView;
    if (target === 'idl') {
      displayIDLResponse(data, resp.status, dur, resp.headers, text, size);
    } else {
      displayResponse(data, resp.status, dur, resp.headers, text, size);
    }
    showToast('已执行 ' + item.name + '（' + mode + '）', resp.ok ? 'success' : 'error');
  } catch (err) {
    showToast('执行失败: ' + (err.message || String(err)), 'error');
  }
}

// ---------- 控制台侧栏分组 ----------

function appendSavedGroupToConsole(tree, keyword) {
  var kw = (keyword || '').trim().toLowerCase();
  var items = state.savedInstructions.filter(function (s) {
    if (!kw) return true;
    var hay = (s.name + ' ' + s.appName + ' ' + s.methodName + ' ' + s.kind).toLowerCase();
    return hay.indexOf(kw) >= 0;
  });
  if (!items.length) return 0;
  var gw = el('div', { class: 'endpoint-group' });
  gw.appendChild(el('div', { class: 'group-title', text: '保存指令 (' + items.length + ')' }));
  items.forEach(function (s) {
    gw.appendChild(el('div', {
      class: 'endpoint-item saved-item',
      onclick: function () { openSavedExecModal(s.id); },
    },
      el('span', { class: 'endpoint-method ' + (s.kind === 'view' ? 'GET' : 'POST'), text: s.kind === 'view' ? 'VIEW' : 'EXEC' }),
      el('div', { class: 'endpoint-text' },
        el('span', { class: 'endpoint-name', text: s.name || '(未命名)' }),
        el('span', { class: 'endpoint-desc', text: s.appName + '.' + s.methodName + (s.paymentMode ? ' · ' + s.paymentMode : '') })
      )
    ));
  });
  tree.appendChild(gw);
  return items.length;
}

function openSavedExecModal(id) {
  var item = savedById(id);
  if (!item) { showToast('指令不存在（可能已删除）', 'error'); refreshSavedViews(); return; }
  var modes = item.kind === 'view' ? ['read'] : ['simulate', 'send'];
  var picked = modes[0];
  var modal = buildModalShell('执行指令：' + (item.name || item.id));
  var info = el('div', { class: 'si-modal-desc', text: item.appName + '.' + item.methodName + ' · ' + item.kind + (item.paymentMode ? ' · ' + item.paymentMode : '') });
  modal.body.appendChild(info);
  modal.body.appendChild(el('div', { class: 'si-modal-desc args', text: 'args: ' + safeJSON(item.args) }));

  var modeRow = el('div', { class: 'si-modal-row' });
  modes.forEach(function (m, i) {
    var label = el('label', { class: 'idl-radio-label' },
      el('input', {
        type: 'radio', name: 'siExecMode', value: m, checked: i === 0 ? 'checked' : null,
        onchange: function () { picked = m; },
      }),
      el('span', { text: ' ' + (m === 'send' ? 'send（上链）' : m) })
    );
    modeRow.appendChild(label);
  });
  modal.body.appendChild(modeRow);
  if (item.kind === 'entry') {
    modal.body.appendChild(el('div', { class: 'si-modal-hint', text: 'send 会真实上链并默认等待确认，请确认参数与账户。' }));
  }

  modal.footer.appendChild(el('button', {
    class: 'btn btn-primary',
    text: '执行',
    onclick: function () {
      modal.close();
      switchView('console');
      runSaved(item, picked, 'console');
    },
  }));
  modal.footer.appendChild(el('button', { class: 'btn', text: '编辑', onclick: function () { modal.close(); openSavedEditor(item); } }));
  modal.footer.appendChild(el('button', { class: 'btn', text: '删除', onclick: function () { modal.close(); deleteSaved(item); } }));
  modal.open();
}

// ---------- 删除 ----------

async function deleteSaved(item) {
  var ok = await uiConfirm('删除指令「' + (item.name || item.id) + '」？该操作不可恢复。', { danger: true });
  if (!ok) return;
  try {
    var resp = await fetch('/api/saved-instructions/' + item.id, { method: 'DELETE' });
    var data = await resp.json();
    if (data && data.success) {
      showToast('已删除', 'success');
      refreshSavedViews();
    } else {
      showToast('删除失败: ' + (data && data.message), 'error');
    }
  } catch (e) {
    showToast('删除失败: ' + (e.message || String(e)), 'error');
  }
}

// ---------- 载入到 IDL 表单 ----------

function loadSavedIntoIDLForm(item) {
  state.idlPrefill = item;
  renderIDLForm(state.currentIdlMethod);
  // 标量参数按 data-argname 回填
  setTimeout(function () {
    var args = item.args || {};
    Object.keys(args).forEach(function (name) {
      var node = document.querySelector('#idlEditorBody [data-argname="' + name + '"]');
      if (!node) return;
      var v = args[name];
      node.value = (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') ? String(v) : JSON.stringify(v);
    });
    // 支付/签名配置回填（renderIDLPaymentFields 是 setTimeout 异步渲染，这里再延后一点）
    if (item.paymentMode && $('idlPaymentMode')) $('idlPaymentMode').value = item.paymentMode;
    if (typeof renderIDLPaymentFields === 'function') renderIDLPaymentFields();
    setTimeout(function () {
      setPayField('payerAddress', item.payerAddress);
      setPayField('payerPrivateKey', item.payerPrivateKey);
      setPayField('ownerAddress', item.ownerAddress);
      setPayField('ownerPrivateKey', item.ownerPrivateKey);
      setPayField('ixAddress', item.ixAddress);
      setPayField('ixPrivateKey', item.ixPrivateKey);
      if (item.signatureMode) setPayField('signatureMode', typeof item.signatureMode === 'string' ? item.signatureMode : JSON.stringify(item.signatureMode, null, 2));
      if (item.ixSignatureMode) setPayField('ixSignatureMode', typeof item.ixSignatureMode === 'string' ? item.ixSignatureMode : JSON.stringify(item.ixSignatureMode, null, 2));
      state.idlPrefill = null;
    }, 20);
  }, 10);
  showToast('已载入指令「' + (item.name || item.id) + '」到表单', 'success');
}

function setPayField(field, value) {
  if (!value) return;
  var node = document.querySelector('#idlPaymentFields [data-field="' + field + '"]');
  if (node) node.value = value;
}

// ---------- 保存入口：IDL 表单 ----------

function saveIDLFormAsInstruction(ix) {
  var payload;
  try {
    var req = buildIDLRequest();
    if (!req) return;
    payload = JSON.parse(req.body);
  } catch (e) {
    showToast('当前表单无法解析: ' + (e.message || String(e)), 'error');
    return;
  }
  payload.appName = state.currentIdlApp;
  payload.methodName = ix.name;
  openSavedEditor(null, payload);
}

// ---------- 保存入口：控制台请求体 ----------

function saveConsoleBodyAsInstruction(ep) {
  var ta = $('bodyEditor');
  if (!ta) return;
  var body;
  try {
    body = JSON.parse(ta.value);
  } catch (e) {
    showToast('请求体不是合法 JSON', 'error');
    return;
  }
  if (!body.appName || !body.methodName) {
    showToast('请求体需包含 appName 与 methodName', 'error');
    return;
  }
  openSavedEditor(null, body);
}

// ---------- 编辑器弹窗 ----------

function openSavedEditor(existing, prefill) {
  var src = existing || prefill || {};
  var modal = buildModalShell(existing ? '编辑指令' : '保存为指令');

  function row(label, node) {
    modal.body.appendChild(el('div', { class: 'si-modal-row' },
      el('label', { class: 'si-modal-label', text: label }), node));
    return node;
  }

  var nameInp = row('名称', el('input', { class: 'param-input', value: src.name || '' }));
  var descInp = row('备注', el('input', { class: 'param-input', value: src.description || '' }));
  var argsTa = row('args (JSON)', el('textarea', { class: 'si-modal-ta', rows: '6' }));
  argsTa.value = safeJSON(src.args || {});

  var pmSel = el('select', { class: 'param-input' });
  pmSel.appendChild(el('option', { value: '', text: '（view 方法无需）' }));
  SAVED_PAYMENT_MODES.forEach(function (m) {
    pmSel.appendChild(el('option', { value: m.value, text: m.label }));
  });
  pmSel.value = src.paymentMode || '';
  row('paymentMode', pmSel);

  var payerAddr = row('payerAddress', el('input', { class: 'param-input', value: src.payerAddress || '' }));
  var payerSk = row('payerPrivateKey', el('input', { class: 'param-input', type: 'password', value: src.payerPrivateKey || '' }));
  var ownerAddr = row('ownerAddress (split)', el('input', { class: 'param-input', value: src.ownerAddress || '' }));
  var ownerSk = row('ownerPrivateKey (split)', el('input', { class: 'param-input', type: 'password', value: src.ownerPrivateKey || '' }));
  var sigTa = row('signatureMode (JSON)', el('textarea', { class: 'si-modal-ta', rows: '3' }));
  sigTa.value = src.signatureMode ? (typeof src.signatureMode === 'string' ? src.signatureMode : JSON.stringify(src.signatureMode, null, 2)) : '';

  modal.footer.appendChild(el('button', {
    class: 'btn btn-primary',
    text: existing ? '保存修改' : '创建',
    onclick: async function () {
      var payload = {
        name: nameInp.value.trim(),
        description: descInp.value.trim(),
        appName: src.appName,
        methodName: src.methodName,
        args: parseJSONField(argsTa.value, 'args'),
        paymentMode: pmSel.value,
        payerAddress: payerAddr.value.trim(),
        payerPrivateKey: payerSk.value.trim(),
        ownerAddress: ownerAddr.value.trim(),
        ownerPrivateKey: ownerSk.value.trim(),
        signatureMode: parseJSONField(sigTa.value, 'signatureMode'),
      };
      if (payload.args === PARSE_FAIL || payload.signatureMode === PARSE_FAIL) return;
      if (!payload.name) { showToast('名称不能为空', 'error'); return; }
      // 保留编辑器未暴露的字段（signers/gasPayer/ix*）
      if (existing) {
        ['ixAddress', 'ixPrivateKey', 'ixSignatureMode', 'signers', 'gasPayer'].forEach(function (k) {
          if (existing[k] !== undefined) payload[k] = existing[k];
        });
      } else {
        ['ixAddress', 'ixPrivateKey', 'ixSignatureMode', 'signers', 'gasPayer'].forEach(function (k) {
          if (src[k] !== undefined) payload[k] = src[k];
        });
      }
      try {
        var url = existing ? '/api/saved-instructions/' + existing.id : '/api/saved-instructions';
        var resp = await fetch(url, {
          method: existing ? 'PUT' : 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        var data = await resp.json();
        if (data && data.success) {
          showToast(existing ? '已更新' : '已保存指令', 'success');
          modal.close();
          refreshSavedViews();
        } else {
          showToast('保存失败: ' + (data && data.message), 'error');
        }
      } catch (e) {
        showToast('保存失败: ' + (e.message || String(e)), 'error');
      }
    },
  }));
  modal.open();
}

var PARSE_FAIL = {};
function parseJSONField(text, label) {
  if (!text || !text.trim()) return undefined;
  try {
    return JSON.parse(text);
  } catch (e) {
    showToast(label + ' 不是合法 JSON', 'error');
    return PARSE_FAIL;
  }
}

// ---------- 弹窗外壳 ----------

function buildModalShell(title) {
  var overlay = el('div', { class: 'si-modal-overlay' });
  var box = el('div', { class: 'si-modal' });
  var head = el('div', { class: 'si-modal-head' },
    el('span', { class: 'si-modal-title', text: title }),
    el('button', { class: 'si-modal-close', text: '\u00d7', onclick: function () { close(); } })
  );
  var body = el('div', { class: 'si-modal-body' });
  var footer = el('div', { class: 'si-modal-footer' },
    el('button', { class: 'btn', text: '取消', onclick: function () { close(); } }));
  box.appendChild(head);
  box.appendChild(body);
  box.appendChild(footer);
  overlay.appendChild(box);
  overlay.addEventListener('click', function (e) { if (e.target === overlay) close(); });
  function close() {
    if (overlay.parentNode) overlay.parentNode.removeChild(overlay);
  }
  return {
    body: body,
    footer: footer,
    open: function () { document.body.appendChild(overlay); },
    close: close,
  };
}

// ---------- 初始化 ----------

function initSavedInstructions() {
  loadSavedInstructions().then(function () {
    if (state.currentView === 'console') {
      renderEndpoints($('endpointSearch') ? $('endpointSearch').value : '');
    }
  });
}
