// ==================== 账号本:自定义测试账号 UI ====================
// 依赖 app.js 的全局:apiFetch / el / showToast;复用 seed_overview.js 的 seedCopyText / seed-table 样式。
// 服务端:/api/tool/account-book(GET 列表 / POST upsert / DELETE /:name),
// 存 data/custom_accounts.json;导入兼容三种形态(平铺数组 / {accounts} / sft_1000_testdata 三段式)。
// node 直抽可加载:无顶层 await、无顶层 fetch;初始化由 DOMContentLoaded 守卫 + 元素存在检查。

var accountBookState = { rows: [] };

// 客户端预检归一(与服务端 accountBookNormalize 同构;服务端仍权威):
// 平铺数组 | {accounts:[…]} | {owner, users, newUsers} | 单对象
function bookNormalizeImport(parsed) {
  var flat = null;
  if (Array.isArray(parsed)) {
    flat = parsed;
  } else if (parsed && typeof parsed === 'object') {
    if (Array.isArray(parsed.accounts) || parsed.owner || Array.isArray(parsed.users) || Array.isArray(parsed.newUsers)) {
      flat = [];
      if (parsed.owner) {
        // sft_1000_testdata 实档 owner 无 name 字段:缺名默认 "owner"(服务端同款)
        if (!parsed.owner.name) parsed.owner.name = 'owner';
        flat.push(parsed.owner);
      }
      if (Array.isArray(parsed.accounts)) flat = flat.concat(parsed.accounts);
      if (Array.isArray(parsed.users)) flat = flat.concat(parsed.users);
      if (Array.isArray(parsed.newUsers)) flat = flat.concat(parsed.newUsers);
    } else if (parsed.name) {
      flat = [parsed];
    }
  }
  if (!flat) throw new Error('无法识别的导入格式:支持 数组 / {accounts:[…]} / {owner,users,newUsers} / 单对象');
  return flat.map(function (e) {
    if (!e || !e.name) throw new Error('条目缺 name(name 即 ID,同名覆盖)');
    if (!e.privateKey && !e.address) throw new Error(e.name + ': 私钥与地址至少给一');
    return {
      name: e.name, address: e.address || '', privateKey: e.privateKey || '',
      publicKey: e.publicKey || '', keyType: e.keyType || 'secp256k1',
      note: e.note || '',
    };
  });
}

function refreshAccountBook() {
  return apiFetch('/api/tool/account-book')
    .then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) throw new Error(body.message || '账号本拉取失败');
      accountBookState.rows = body.data || [];
      renderAccountBook();
    })
    .catch(function (e) {
      if (typeof showToast === 'function') showToast('账号本加载失败: ' + (e && e.message || e), 'error');
    });
}

function renderAccountBook() {
  var box = document.getElementById('accountBookCards');
  if (!box) return;
  box.innerHTML = '';
  var rows = accountBookState.rows || [];
  if (!rows.length) {
    box.appendChild(el('div', { class: 'vc-card-desc', text: '账号本为空:「添加一条」或粘贴导入(sft_1000_testdata.json 可直接粘)' }));
    return;
  }
  var wrap = el('div', { style: 'overflow-x:auto' });
  var tbl = el('table', { class: 'seed-table' });
  var thead = el('thead', null);
  var hrow = el('tr', null);
  ['名称', '曲线', '地址', '私钥', '备注', '操作'].forEach(function (h) { hrow.appendChild(el('th', { text: h })); });
  thead.appendChild(hrow);
  tbl.appendChild(thead);
  var tbody = el('tbody', null);
  rows.forEach(function (r) {
    var tr = el('tr', null);
    tr.appendChild(el('td', { class: 'seed-mono', text: r.name }));
    tr.appendChild(el('td', { class: 'seed-mono', text: r.keyType || '' }));
    var addrTd = el('td', null);
    if (r.address) {
      addrTd.appendChild(el('span', { class: 'seed-mono seed-addr', text: r.address }));
      addrTd.appendChild(el('button', {
        class: 'btn btn-secondary btn-sm', text: '复制',
        onclick: function () { seedCopyText(r.address, '地址已复制: ' + r.name); },
      }));
    } else {
      addrTd.appendChild(el('span', { text: '—' }));
    }
    tr.appendChild(addrTd);
    var keyTd = el('td', null);
    if (r.privateKey) {
      var keySpan = el('span', { class: 'seed-mono seed-key', text: String(r.privateKey).slice(0, 10) + '…' });
      var revealed = false;
      keyTd.appendChild(keySpan);
      keyTd.appendChild(el('button', {
        class: 'btn btn-secondary btn-sm', text: '显示',
        onclick: function () {
          revealed = !revealed;
          keySpan.textContent = revealed ? r.privateKey : String(r.privateKey).slice(0, 10) + '…';
          this.textContent = revealed ? '隐藏' : '显示';
        },
      }));
      keyTd.appendChild(el('button', {
        class: 'btn btn-secondary btn-sm', text: '复制',
        onclick: function () { seedCopyText(r.privateKey, '私钥已复制: ' + r.name); },
      }));
    } else {
      keyTd.appendChild(el('span', { text: '(仅地址)' }));
    }
    tr.appendChild(keyTd);
    tr.appendChild(el('td', { text: r.note || '' }));
    var opTd = el('td', null);
    // 证书展开(复用 seed_overview.js 的共用面板;仅地址可查时可用)
    if (r.address) {
      opTd.appendChild(el('button', {
        class: 'btn btn-secondary btn-sm', text: '证书',
        onclick: function () { seedToggleCertsPanel({ address: r.address, name: r.name }, this); },
      }));
      opTd.appendChild(el('span', { text: ' ' }));
    }
    opTd.appendChild(el('button', {
      class: 'btn btn-secondary btn-sm', text: '删除',
      onclick: function () {
        if (!window.confirm('删除账号本条目 ' + r.name + '?')) return;
        apiFetch('/api/tool/account-book/' + encodeURIComponent(r.name), { method: 'DELETE' })
          .then(function (resp) { return resp.json(); })
          .then(function (body) {
            if (!body.success) throw new Error(body.message || '删除失败');
            showToast('已删除 ' + r.name, 'success');
            refreshAccountBook();
          })
          .catch(function (e) { showToast('删除失败: ' + (e && e.message || e), 'error'); });
      },
    }));
    tr.appendChild(opTd);
    tbody.appendChild(tr);
  });
  tbl.appendChild(tbody);
  wrap.appendChild(tbl);
  box.appendChild(wrap);
}

// 提交导入:textarea JSON → 归一 → POST upsert(服务端补派生地址)→ 刷新
function submitAccountBookImport() {
  var area = document.getElementById('accountBookImportArea');
  if (!area) return;
  var raw = String(area.value || '').trim();
  if (!raw) { showToast('先粘贴 JSON 再提交', 'warning'); return; }
  var parsed;
  try { parsed = JSON.parse(raw); } catch (e) { showToast('JSON 解析失败: ' + e.message, 'error'); return; }
  var rows;
  try { rows = bookNormalizeImport(parsed); } catch (e) { showToast(e.message, 'error'); return; }
  apiFetch('/api/tool/account-book', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(rows),
  }).then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) throw new Error(body.message || '导入失败');
      accountBookState.rows = body.data || [];
      renderAccountBook();
      area.value = '';
      area.style.display = 'none';
      var submitBtn = document.getElementById('accountBookSubmitBtn');
      if (submitBtn) submitBtn.style.display = 'none';
      showToast('账号本已导入 ' + rows.length + ' 条(同名覆盖)', 'success');
    })
    .catch(function (e) { showToast('导入失败: ' + (e && e.message || e), 'error'); });
}

// ---------- 表单录入(不想拼 JSON 就用这个) ----------

var BOOK_KEY_TYPES = ['fndsa512', 'secp256k1', 'ed25519', 'bls12381'];

function hideAccountBookJsonImport() {
  var area = document.getElementById('accountBookImportArea');
  var submitBtn = document.getElementById('accountBookSubmitBtn');
  if (area) area.style.display = 'none';
  if (submitBtn) submitBtn.style.display = 'none';
}

function accountBookFormPanel() {
  var p = document.getElementById('accountBookFormPanel');
  if (p) p.innerHTML = '';
  return p;
}

// keyType 启发:私钥形状 → 缺省曲线(可改)。2562 hex = fndsa512 完整私钥。
function accountBookGuessKeyType(priv) {
  var s = String(priv || '').trim();
  if (/^[0-9a-fA-F]{2562}$/.test(s)) return 'fndsa512';
  if (/^[1-9A-HJ-NP-Za-km-z]{340,400}$/.test(s)) return 'fndsa512'; // base58 长串
  return 'secp256k1';
}

// 单条表单:name + 私钥 + 曲线 + 备注;地址留空由服务端派生。
function renderAccountBookForm() {
  var panel = accountBookFormPanel();
  if (!panel) return;
  var mk = function (labelText, input) {
    var row = el('div', { style: 'display:flex;gap:8px;align-items:center;margin:4px 0' });
    var lab = el('label', { text: labelText, style: 'min-width:70px' });
    row.appendChild(lab);
    row.appendChild(input);
    return row;
  };
  var nameIn = el('input', { type: 'text', placeholder: '名称(name 即 ID,同名覆盖)', style: 'flex:1' });
  var keySel = el('select', null);
  BOOK_KEY_TYPES.forEach(function (k) {
    var o = el('option', { text: k + (k === 'fndsa512' ? '(抗量子,缺省)' : '') });
    o.value = k;
    if (k === 'fndsa512') o.selected = true;
    keySel.appendChild(o);
  });
  var privIn = el('textarea', { placeholder: '私钥(hex/base58);fndsa512 完整私钥 2562 位 hex 也可直接粘贴;留空则只建地址簿条目', style: 'min-height:60px;width:100%;font-family:monospace' });
  var addrIn = el('input', { type: 'text', placeholder: '地址(选填;留空由服务端从私钥派生)', style: 'flex:1' });
  var noteIn = el('input', { type: 'text', placeholder: '备注(选填)', style: 'flex:1' });
  privIn.addEventListener('input', function () {
    var g = accountBookGuessKeyType(privIn.value);
    for (var i = 0; i < keySel.options.length; i++) {
      if (keySel.options[i].value === g) keySel.selectedIndex = i;
    }
  });
  panel.appendChild(mk('名称', nameIn));
  panel.appendChild(mk('曲线', keySel));
  panel.appendChild(mk('私钥', privIn));
  panel.appendChild(mk('地址', addrIn));
  panel.appendChild(mk('备注', noteIn));
  var submit = el('button', { class: 'btn btn-sm', text: '保存到账号本' });
  submit.addEventListener('click', function () {
    var entry = {
      name: String(nameIn.value || '').trim(),
      keyType: keySel.value,
      privateKey: String(privIn.value || '').trim(),
      address: String(addrIn.value || '').trim(),
      note: String(noteIn.value || '').trim(),
    };
    if (!entry.name) { showToast('先填名称', 'warning'); return; }
    if (!entry.privateKey && !entry.address) { showToast('私钥与地址至少填一', 'warning'); return; }
    apiFetch('/api/tool/account-book', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify([entry]),
    }).then(function (r) { return r.json(); })
      .then(function (body) {
        if (!body.success) throw new Error(body.message || '保存失败');
        accountBookState.rows = body.data || [];
        renderAccountBook();
        panel.innerHTML = '';
        showToast('已保存 ' + entry.name + (entry.privateKey ? '(地址已派生)' : '(仅地址)'), 'success');
      })
      .catch(function (e) { showToast('保存失败: ' + (e && e.message || e), 'error'); });
  });
  panel.appendChild(submit);
  panel.style.display = '';
  nameIn.focus();
}

// 从账户管理导入:列 localStorage 账户(milon_accounts),勾选 + 曲线下拉后批量导入。
function renderAccountBookFromAccounts() {
  var panel = accountBookFormPanel();
  if (!panel) return;
  var stored = [];
  try { stored = JSON.parse(localStorage.getItem('milon_accounts') || '[]') || []; } catch (e) { stored = []; }
  if (!stored.length) {
    panel.appendChild(el('div', { class: 'vc-card-desc', text: '账户管理里还没有账户:先在「账户管理」添加或导入,再回来勾选导入' }));
    panel.style.display = '';
    return;
  }
  var sel = {};
  var keySelects = {};
  stored.forEach(function (a) {
    var row = el('div', { style: 'display:flex;gap:8px;align-items:center;margin:3px 0' });
    var cb = el('input', { type: 'checkbox' });
    cb.checked = false;
    cb.addEventListener('change', function () { sel[a.address] = this.checked; });
    row.appendChild(cb);
    row.appendChild(el('span', { class: 'seed-mono', text: a.label || a.address, style: 'min-width:140px' }));
    row.appendChild(el('span', { class: 'seed-mono seed-addr', text: a.address }));
    var ksel = el('select', null);
    var guess = accountBookGuessKeyType(a.privateKey);
    BOOK_KEY_TYPES.forEach(function (k) {
      var o = el('option', { text: k });
      o.value = k;
      if (k === guess) o.selected = true;
      ksel.appendChild(o);
    });
    keySelects[a.address] = ksel;
    row.appendChild(ksel);
    if (guess === 'fndsa512' && !a.publicKey) {
      row.appendChild(el('span', { class: 'vc-card-desc', text: '⚠ fndsa512 须带公钥,该账户缺 publicKey,导入会失败' }));
    }
    panel.appendChild(row);
  });
  var submit = el('button', { class: 'btn btn-sm', text: '导入勾选账户' });
  submit.addEventListener('click', function () {
    var entries = [];
    stored.forEach(function (a) {
      if (!sel[a.address]) return;
      entries.push({
        name: a.label || a.address.slice(0, 8),
        keyType: keySelects[a.address].value,
        privateKey: a.privateKey || '',
        publicKey: a.publicKey || '',
        address: a.address || '',
        note: '来自账户管理',
      });
    });
    if (!entries.length) { showToast('先勾选要导入的账户', 'warning'); return; }
    apiFetch('/api/tool/account-book', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(entries),
    }).then(function (r) { return r.json(); })
      .then(function (body) {
        if (!body.success) throw new Error(body.message || '导入失败');
        accountBookState.rows = body.data || [];
        renderAccountBook();
        panel.innerHTML = '';
        showToast('已从账户管理导入 ' + entries.length + ' 条', 'success');
      })
      .catch(function (e) { showToast('导入失败: ' + (e && e.message || e), 'error'); });
  });
  panel.appendChild(submit);
  panel.style.display = '';
}

function initAccountBook() {
  if (!document.getElementById('accountBookCards')) return;
  var addBtn = document.getElementById('accountBookAddBtn');
  if (addBtn) addBtn.addEventListener('click', function () {
    hideAccountBookJsonImport();
    renderAccountBookForm();
  });
  var fromAccBtn = document.getElementById('accountBookFromAccountsBtn');
  if (fromAccBtn) fromAccBtn.addEventListener('click', function () {
    hideAccountBookJsonImport();
    renderAccountBookFromAccounts();
  });
  var importBtn = document.getElementById('accountBookImportBtn');
  if (importBtn) importBtn.addEventListener('click', function () {
    var area = document.getElementById('accountBookImportArea');
    var submitBtn = document.getElementById('accountBookSubmitBtn');
    if (!area) return;
    var show = area.style.display === 'none';
    area.style.display = show ? '' : 'none';
    if (submitBtn) submitBtn.style.display = show ? '' : 'none';
  });
  var submitBtn = document.getElementById('accountBookSubmitBtn');
  if (submitBtn) submitBtn.addEventListener('click', submitAccountBookImport);
  var exportBtn = document.getElementById('accountBookExportBtn');
  if (exportBtn) exportBtn.addEventListener('click', function () {
    seedCopyText(JSON.stringify(accountBookState.rows, null, 2), '账号本已导出(JSON)');
  });
  var refreshBtn = document.getElementById('accountBookRefreshBtn');
  if (refreshBtn) refreshBtn.addEventListener('click', refreshAccountBook);
  refreshAccountBook();
}

if (typeof document !== 'undefined' && typeof document.getElementById === 'function' && document.getElementById('accountBookCards')) {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAccountBook);
  } else {
    initAccountBook();
  }
}
