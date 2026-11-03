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
  ['名称', '曲线', '地址', '私钥', '备注', ''].forEach(function (h) { hrow.appendChild(el('th', { text: h })); });
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

function initAccountBook() {
  if (!document.getElementById('accountBookCards')) return;
  var addBtn = document.getElementById('accountBookAddBtn');
  if (addBtn) addBtn.addEventListener('click', function () {
    var area = document.getElementById('accountBookImportArea');
    var submitBtn = document.getElementById('accountBookSubmitBtn');
    if (!area) return;
    area.style.display = '';
    if (submitBtn) submitBtn.style.display = '';
    if (!String(area.value || '').trim()) {
      area.value = JSON.stringify([{ name: '', keyType: 'secp256k1', privateKey: '', address: '', note: '' }], null, 2);
    }
    area.focus();
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
