// ==================== 播种总览:数据层 + 渲染层 ====================
// 依赖 app.js 的全局:apiFetch / el / showToast;依赖 seed.js 的全局:buildSeedCfg(运行期)。
// 职责:1) 纯 JS sha256 → 前端本地派生私钥(= Go SeedDeriveKey:sha256(短语:角色:名字)),
//         服务端 /api/tool/seed/accounts 只出地址/公钥,"永不返回私钥"不变量不破;
//       2) 行分组 / 数据集产物提取 / 短语一致性判定(账号固定保护);
//       3) 取数封装:seedFetchAccounts / seedFetchDataset;
//       4) 总览卡片渲染:账户/多签/代币/SFT/VC,私钥默认遮蔽、单点复制、余额懒加载。
// node 直抽可加载:无顶层 await、无顶层 fetch;初始化由 DOMContentLoaded 守卫 + 元素存在检查。

var SEED_LAST_PHRASE_KEY = 'seedLastPhrase';

// ---------- sha256(纯 JS,UTF-8;node 直抽与浏览器同源) ----------

var SEED_SHA256_K = [
  0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
  0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
  0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
  0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
  0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
  0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
  0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
  0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
];

function seedSha256Rotr(x, n) { return (x >>> n) | (x << (32 - n)); }

// bytes: 0..255 字节数组 → 64 hex
function seedSha256Bytes(bytes) {
  var H = [0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19];
  var bitLen = bytes.length * 8;
  var padded = bytes.concat([0x80]);
  while (padded.length % 64 !== 56) padded.push(0);
  var hi = Math.floor(bitLen / 0x100000000), lo = bitLen % 0x100000000;
  padded.push((hi >>> 24) & 255, (hi >>> 16) & 255, (hi >>> 8) & 255, hi & 255,
    (lo >>> 24) & 255, (lo >>> 16) & 255, (lo >>> 8) & 255, lo & 255);
  var w = new Array(64);
  for (var b = 0; b < padded.length; b += 64) {
    for (var t = 0; t < 16; t++) {
      w[t] = (padded[b + 4 * t] << 24) | (padded[b + 4 * t + 1] << 16) | (padded[b + 4 * t + 2] << 8) | padded[b + 4 * t + 3];
    }
    for (var t2 = 16; t2 < 64; t2++) {
      var s0 = seedSha256Rotr(w[t2 - 15], 7) ^ seedSha256Rotr(w[t2 - 15], 18) ^ (w[t2 - 15] >>> 3);
      var s1 = seedSha256Rotr(w[t2 - 2], 17) ^ seedSha256Rotr(w[t2 - 2], 19) ^ (w[t2 - 2] >>> 10);
      w[t2] = (w[t2 - 16] + s0 + w[t2 - 7] + s1) | 0;
    }
    var a = H[0], b2 = H[1], c = H[2], d = H[3], e = H[4], f = H[5], g = H[6], h = H[7];
    for (var t3 = 0; t3 < 64; t3++) {
      var S1 = seedSha256Rotr(e, 6) ^ seedSha256Rotr(e, 11) ^ seedSha256Rotr(e, 25);
      var ch = (e & f) ^ (~e & g);
      var temp1 = (h + S1 + ch + SEED_SHA256_K[t3] + w[t3]) | 0;
      var S0 = seedSha256Rotr(a, 2) ^ seedSha256Rotr(a, 13) ^ seedSha256Rotr(a, 22);
      var maj = (a & b2) ^ (a & c) ^ (b2 & c);
      var temp2 = (S0 + maj) | 0;
      h = g; g = f; f = e; e = (d + temp1) | 0; d = c; c = b2; b2 = a; a = (temp1 + temp2) | 0;
    }
    H[0] = (H[0] + a) | 0; H[1] = (H[1] + b2) | 0; H[2] = (H[2] + c) | 0; H[3] = (H[3] + d) | 0;
    H[4] = (H[4] + e) | 0; H[5] = (H[5] + f) | 0; H[6] = (H[6] + g) | 0; H[7] = (H[7] + h) | 0;
  }
  var out = '';
  for (var i = 0; i < 8; i++) out += ('00000000' + ((H[i] >>> 0).toString(16))).slice(-8);
  return out;
}

function seedSha256(str) {
  // UTF-8 编码后按字节摘要(unescape/encodeURIComponent 是 ES5 环境的 UTF-8 标准手法)
  var utf8 = unescape(encodeURIComponent(String(str)));
  var bytes = [];
  for (var i = 0; i < utf8.length; i++) bytes.push(utf8.charCodeAt(i) & 0xff);
  return seedSha256Bytes(bytes);
}

// 与 Go handler.SeedDeriveKey 逐字节一致
function seedPrivKey(phrase, role, name) {
  return seedSha256(phrase + ':' + role + ':' + name);
}

// ---------- 数据变换(纯函数,直测) ----------

// 给端点行补私钥(行自带 role/name,自描述;返回副本不改入参)
function seedAttachKeys(accounts, phrase) {
  return (accounts || []).map(function (r) {
    var copy = {};
    for (var k in r) copy[k] = r[k];
    copy.privateKey = seedPrivKey(phrase, r.role, r.name);
    return copy;
  });
}

// 按角色分组:admin/users/issuers/multisig/tokens/sfts(其余入 others)
function seedGroupRows(rows) {
  var grouped = { admin: [], users: [], issuers: [], multisig: [], tokens: [], sfts: [], others: [] };
  (rows || []).forEach(function (r) {
    switch (r.role) {
      case 'admin': grouped.admin.push(r); break;
      case 'user': grouped.users.push(r); break;
      case 'issuer': grouped.issuers.push(r); break;
      case 'multisig': grouped.multisig.push(r); break;
      case 'token': grouped.tokens.push(r); break;
      case 'sft': grouped.sfts.push(r); break;
      default: grouped.others.push(r);
    }
  });
  return grouped;
}

// 数据集命名空间 → 展示产物:tokens.{SYM}.address、sfts.{KEY}.{address,slots.{scenario}.{slotId,tokenIds}}
function seedDatasetArtifacts(dataset) {
  var out = { configHash: dataset && dataset.configHash || '', tokens: {}, sfts: {}, stageCount: (dataset && dataset.stageCount) || 0 };
  var mods = (dataset && dataset.modules) || {};
  Object.keys(mods.tokens || {}).forEach(function (stage) {
    if (stage.indexOf('create:') === 0 && mods.tokens[stage] && mods.tokens[stage].address) {
      out.tokens[stage.slice(7)] = mods.tokens[stage].address;
    }
  });
  Object.keys(mods.sft || {}).forEach(function (stage) {
    var payload = mods.sft[stage] || {};
    if (stage.indexOf('create:') === 0 && payload.address) {
      var key = stage.slice(7);
      if (!out.sfts[key]) out.sfts[key] = { address: payload.address, slots: {} };
      else out.sfts[key].address = payload.address;
    } else if (stage.indexOf('slot:') === 0) {
      // 形如 slot:set01:transferable
      var rest = stage.slice(5);
      var sep = rest.indexOf(':');
      if (sep > 0) {
        var key2 = rest.slice(0, sep), scenario = rest.slice(sep + 1);
        if (!out.sfts[key2]) out.sfts[key2] = { address: '', slots: {} };
        out.sfts[key2].slots[scenario] = payload;
      }
    }
  });
  return out;
}

// ---------- 发证书 + 三卡链上实查(纯函数) ----------

// 发证书请求体(config = 总览当前旋钮,决定派生命名空间)
function vcIssuePayload(cfg, issuerName, subjectName, schemaPrefix, count) {
  return { config: cfg, issuerName: issuerName, subjectName: subjectName,
    schemaPrefix: schemaPrefix, count: Number(count) || 1 };
}

// 三类链上读(POST /api/read 的 app/method/args;口径与播种器/flowCallView 一致)
function vcHoldingsView(subjectAddr) {
  return { appName: 'identity', methodName: 'DisclosedVcs', args: { subject: subjectAddr, offset: 0, limit: 100 } };
}
function tokenBalanceView(tokenAddr, holderAddr) {
  return { appName: 'token', methodName: 'BalanceOf', args: { token: tokenAddr, account: holderAddr } };
}
function sftOwnerView(sftAddr, tokenId) {
  return { appName: 'sftoken', methodName: 'SftOwnerOf', args: { sft: sftAddr, token_id: tokenId } };
}

// 归一:链端 Err 载荷/解码降级 → null;成功 → schema 数组 / 数值
function normalizeDisclosedVcs(value) {
  if (!Array.isArray(value)) return null;
  return value.map(function (row) {
    if (row && typeof row === 'object') return String(row.credential_schema || row.schema || '?');
    return String(row);
  });
}
function normalizeU64View(value) {
  if (typeof value === 'number' && isFinite(value)) return value;
  if (typeof value === 'string' && value !== '' && !isNaN(Number(value))) return Number(value);
  return null;
}

// dataset 产物 → SFT 持仓任务清单(集合地址, 场景, tokenId)平铺
function sftHoldingJobs(art, key) {
  var out = [];
  var s = art && art.sfts && art.sfts[key];
  if (!s) return out;
  Object.keys(s.slots || {}).forEach(function (sc) {
    (s.slots[sc].tokenIds || []).forEach(function (tid) {
      out.push({ sft: s.address, key: key, scenario: sc, tokenId: tid });
    });
  });
  return out;
}

// ---------- 账号固定保护:短语一致性 ----------

// localStorage 记录"上次成功播种的短语";不一致 = 本次开始将生成全新账号集 → 页面黄条警告
function seedPhraseMismatch(phrase) {
  if (!phrase) return false;
  var last = null;
  try { last = localStorage.getItem(SEED_LAST_PHRASE_KEY); } catch (e) { return false; }
  return !!last && last !== phrase;
}

function seedRememberPhrase(phrase) {
  if (!phrase) return;
  try { localStorage.setItem(SEED_LAST_PHRASE_KEY, phrase); } catch (e) { /* 隐私模式不阻塞 */ }
}

// 上次成功播种的短语(账号固定保护的回填源)
function seedPreferredPhrase() {
  try { return localStorage.getItem(SEED_LAST_PHRASE_KEY) || ''; } catch (e) { return ''; }
}

// ---------- 取数封装 ----------

// config: SeedConfig(与 /api/tool/seed/plan 同构);返回 {seedPhrase, configHash, accounts(含本地派生私钥)}
function seedFetchAccounts(cfg) {
  return apiFetch('/api/tool/seed/accounts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ config: cfg }),
  }).then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) throw new Error(body.message || '账户总表拉取失败');
      var d = body.data || {};
      return {
        seedPhrase: d.seedPhrase || '',
        configHash: d.configHash || '',
        accounts: seedAttachKeys(d.accounts || [], d.seedPhrase || ''),
      };
    });
}

// 返回 datasets 数组(configHash 升序)+ updatedAt
function seedFetchDataset() {
  return apiFetch('/api/tool/seed/dataset')
    .then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) throw new Error(body.message || '数据集拉取失败');
      return { datasets: body.data.datasets || [], updatedAt: body.data.updatedAt || 0 };
    });
}

// ---------- 渲染层(总览卡片) ----------

// 总览状态:plan 由 seed.js 的计划预览顺手缓存(seedPreviewNow),供多签/代币/SFT/VC 卡补充规格信息
var seedOverviewState = {
  accounts: [], datasets: [], configHash: '', seedPhrase: '',
  plan: null, balances: {}, datasetUpdated: 0,
  vcHoldings: null,      // addr → schema 数组 | null(查询失败)
  tokenHoldings: null,   // [{symbol, holderName, holderAddr, balance}]
  sftHoldings: null,     // [{key, scenario, tokenId, ownerName, ownerAddr}]
  vcPanelFor: null,      // 发证书面板当前目标行(账户名)
  selectedDataset: '',   // 展示/取数用的命名空间(孤儿配置时 fallback 到最后一个;警告判定仍用 configHash)
};

var SEED_ROLE_LABELS = {
  admin: '管理员', user: '用户', issuer: '签发方', multisig: '多签', token: '代币', sft: 'SFT',
};

function seedCopyText(text, okMsg) {
  if (!text) { if (typeof showToast === 'function') showToast('内容为空', 'warning'); return; }
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    navigator.clipboard.writeText(text).then(function () {
      if (typeof showToast === 'function') showToast(okMsg || '已复制', 'success');
    });
  } else if (typeof showToast === 'function') {
    showToast('浏览器不支持剪贴板', 'warning');
  }
}

// 行 → tr:名称/角色/曲线/地址(复制)/私钥(遮蔽→显示/复制)/余额/[操作]/备注
function seedOverviewRow(r, noteFor, rowAction) {
  var tr = el('tr', null);
  tr.appendChild(el('td', { class: 'seed-mono', text: r.name }));
  tr.appendChild(el('td', { text: SEED_ROLE_LABELS[r.role] || r.role || '' }));
  tr.appendChild(el('td', { class: 'seed-mono', text: r.curve || '' }));
  // 地址 + 复制
  var addrTd = el('td', null);
  if (r.address) {
    addrTd.appendChild(el('span', { class: 'seed-mono seed-addr', text: r.address }));
    addrTd.appendChild(el('button', {
      class: 'btn btn-secondary btn-sm', text: '复制', title: '复制地址',
      onclick: function () { seedCopyText(r.address, '地址已复制: ' + r.name); },
    }));
  } else {
    addrTd.appendChild(el('span', { text: '—' }));
  }
  tr.appendChild(addrTd);
  // 私钥:默认遮蔽,点击显示/隐藏;独立复制按钮
  var keyTd = el('td', null);
  var keySpan = el('span', { class: 'seed-mono seed-key', text: String(r.privateKey || '').slice(0, 10) + '…' });
  var revealed = false;
  keyTd.appendChild(keySpan);
  keyTd.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', text: '显示',
    onclick: function () {
      revealed = !revealed;
      keySpan.textContent = revealed ? r.privateKey : String(r.privateKey || '').slice(0, 10) + '…';
      this.textContent = revealed ? '隐藏' : '显示';
    },
  }));
  keyTd.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', text: '复制', title: '复制私钥',
    onclick: function () { seedCopyText(r.privateKey, '私钥已复制: ' + r.name); },
  }));
  tr.appendChild(keyTd);
  // 余额(懒加载填充)
  var balTd = el('td', null);
  balTd.appendChild(el('span', {
    class: 'seed-balance',
    text: seedOverviewState.balances[r.address] != null ? String(seedOverviewState.balances[r.address]) : '—',
  }));
  tr.appendChild(balTd);
  // 操作列(可选,如 发证书)
  if (rowAction) {
    var opTd = el('td', null);
    opTd.appendChild(rowAction(r));
    tr.appendChild(opTd);
  }
  // 备注(角色附加信息)
  var note = noteFor ? noteFor(r) : '';
  tr.appendChild(el('td', { text: note || '' }));
  return tr;
}

// 卡片骨架:标题 + 描述 + 表格 + 卡片级按钮(复制整卡 / 查链上余额 / opts.extraButtons)
// opts.rowAction: 行级操作(如 账户卡发证书);opts.extraButtons: 追加页脚按钮
function seedBuildCardTable(title, desc, rows, noteFor, emptyHint, opts) {
  opts = opts || {};
  var card = el('div', { class: 'vc-card seed-ov-card' });
  var header = el('div', { class: 'vc-card-header' });
  header.appendChild(el('h3', { text: title }));
  header.appendChild(el('span', { class: 'vc-card-desc', text: desc }));
  card.appendChild(header);
  if (!rows || !rows.length) {
    card.appendChild(el('div', { class: 'vc-card-desc', text: emptyHint || '(本类无账户)' }));
    return card;
  }
  var wrap = el('div', { style: 'overflow-x:auto' });
  var tbl = el('table', { class: 'seed-table' });
  var thead = el('thead', null);
  var hrow = el('tr', null);
  var heads = ['名称', '角色', '曲线', '地址', '私钥', '余额'];
  if (opts.rowAction) heads.push('操作');
  heads.push('备注');
  heads.forEach(function (h) { hrow.appendChild(el('th', { text: h })); });
  thead.appendChild(hrow);
  tbl.appendChild(thead);
  var tbody = el('tbody', null);
  rows.forEach(function (r) { tbody.appendChild(seedOverviewRow(r, noteFor, opts.rowAction)); });
  tbl.appendChild(tbody);
  wrap.appendChild(tbl);
  card.appendChild(wrap);
  var foot = el('div', { style: 'display:flex;gap:8px;margin-top:8px' });
  foot.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', text: '复制本卡(JSON)',
    onclick: function () { seedCopyText(JSON.stringify(rows, null, 2), title + ' 已复制'); },
  }));
  foot.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', text: '查链上余额',
    onclick: function () { seedFetchBalances(rows); },
  }));
  (opts.extraButtons || []).forEach(function (b) {
    foot.appendChild(el('button', { class: 'btn btn-secondary btn-sm', text: b.text, onclick: b.onclick }));
  });
  card.appendChild(foot);
  return card;
}

// 懒加载余额:逐地址查 faucet,结果入 state 后整表重渲
// (新账户可能 512 无余额档案:失败显示"查询失败",不阻塞其余地址)
function seedFetchBalances(rows) {
  var addrs = {};
  (rows || []).forEach(function (r) { if (r.address) addrs[r.address] = true; });
  var keys = Object.keys(addrs);
  if (!keys.length) { if (typeof showToast === 'function') showToast('本卡无地址可查', 'warning'); return; }
  if (typeof showToast === 'function') showToast('查询 ' + keys.length + ' 个地址余额…', 'success');
  keys.forEach(function (addr) {
    apiFetch('/api/faucet/balance/' + encodeURIComponent(addr))
      .then(function (resp) { return resp.json(); })
      .then(function (body) {
        seedOverviewState.balances[addr] = body.success ? body.data.balance : '查询失败';
        seedRenderOverview();
      })
      .catch(function () { seedOverviewState.balances[addr] = '查询失败'; seedRenderOverview(); });
  });
}

// 角色附加备注(noteFor):
// 多签 → 签名者/阈值(plan.multisig);代币 → 精度/合规 + 链上地址 + owner=admin;SFT → 链上地址/slot/场景
function seedOverviewNoteFns() {
  var plan = seedOverviewState.plan || {};
  var art = null;
  var ds = seedOverviewState.datasets;
  for (var i = 0; i < ds.length; i++) {
    if (ds[i].configHash === seedOverviewState.configHash) { art = seedDatasetArtifacts(ds[i]); break; }
  }
  var tokenMeta = {};
  (plan.tokens || []).forEach(function (t) { tokenMeta[t.symbol] = t; });
  (plan.compliance || []).forEach(function (c) { tokenMeta[c.symbol] = { decimals: c.decimals, compliance: c }; });
  var msMeta = {};
  (plan.multisig || []).forEach(function (m) { msMeta[m.name] = m; });
  var sftMeta = {};
  (plan.sfts || []).forEach(function (s) { sftMeta[s.key] = s; });
  return {
    multisig: function (r) {
      var m = msMeta[r.name];
      return m ? ('签名者 ' + (m.signerNames || []).join(',') + ' 阈值 ' + m.threshold + '/' + (m.signerNames || []).length) : '';
    },
    token: function (r) {
      var parts = [];
      var m = tokenMeta[r.name];
      if (m && m.decimals != null) parts.push('精度 ' + m.decimals);
      if (m && m.compliance) parts.push('合规@' + m.compliance.issuerName + ' mode=' + m.compliance.mode);
      if (art && art.tokens[r.name]) parts.push('链上 ' + art.tokens[r.name]);
      parts.push('owner=admin');
      return parts.join(' · ');
    },
    sft: function (r) {
      var parts = [];
      var m = sftMeta[r.name];
      if (m) parts.push('symbol ' + m.symbol);
      if (art && art.sfts[r.name]) {
        if (art.sfts[r.name].address) parts.push('链上 ' + art.sfts[r.name].address);
        var slotKeys = Object.keys(art.sfts[r.name].slots || {});
        slotKeys.forEach(function (sc) {
          var s = art.sfts[r.name].slots[sc];
          parts.push('slot' + (s.slotId != null ? s.slotId : '?') + '/' + sc + '(' + ((s.tokenIds || []).length) + '枚)');
        });
      }
      return parts.join(' · ');
    },
  };
}

function seedRenderOverview() {
  var box = document.getElementById('seedOverviewCards');
  if (!box) return;
  var st = seedOverviewState;
  // 账号固定保护:黄条警告。两种情形:
  // a) 短语 ≠ 上次成功短语(localStorage 基准);b) 当前配置的 configHash 不在任何已有数据集里
  //    ——说明页面展示的派生账号与链上现存数据集无关(如 localStorage 残留旧短语),正是"账号不要变"要防的场景。
  var warn = document.getElementById('seedPhraseWarn');
  if (warn) {
    var mismatch = seedPhraseMismatch(st.seedPhrase);
    var knownDataset = (st.datasets || []).some(function (d) { return d.configHash === st.configHash; });
    var orphan = (st.datasets || []).length > 0 && st.configHash && !knownDataset;
    warn.style.display = (mismatch || orphan) ? '' : 'none';
    if (mismatch) {
      warn.textContent = '⚠ 当前 seed 短语与上次成功播种的数据集不一致:开始播种将生成一整套全新账号(地址全变)。要沿用固定账号请改回:' + seedPreferredPhrase();
    } else if (orphan) {
      warn.textContent = '⚠ 当前 seed 短语/旋钮组合没有对应的链上数据集(' + st.datasets.length + ' 个已有数据集属于其他配置:' +
        st.datasets.map(function (d) { return d.configHash || '遗留'; }).join(', ') +
        ')。总览展示的是按当前短语的全新派生,与链上现存账号可能对不上——要沿用固定账号请核对 seed 短语(固定数据集常用 milon-devnet-seed-v1)。';
    }
  }
  // 数据集选择器 + 信息
  var sel = document.getElementById('seedDatasetSel');
  if (sel) {
    sel.innerHTML = '';
    if (!st.datasets.length) {
      sel.appendChild(el('option', { value: '', text: '(尚无数据集:未播种过)' }));
    } else {
      st.datasets.forEach(function (d) {
        var label = (d.configHash || '遗留(无前缀)') + ' · ' + d.stageCount + ' 阶段';
        sel.appendChild(el('option', { value: d.configHash, text: label }));
      });
      sel.value = st.configHash;
      if (sel.value !== st.configHash) sel.value = st.datasets[st.datasets.length - 1].configHash;
      st.selectedDataset = sel.value;
    }
  }
  var info = document.getElementById('seedDatasetInfo');
  if (info) {
    var cur = null;
    st.datasets.forEach(function (d) { if (d.configHash === st.configHash) cur = d; });
    if (!cur && st.datasets.length) cur = st.datasets[st.datasets.length - 1];
    info.textContent = cur
      ? '生效命名空间 ' + (cur.configHash || '遗留') + ' · 模块 ' + Object.keys(cur.modules || {}).length + ' · 阶段 ' + cur.stageCount +
        (st.datasetUpdated ? ' · 状态文件更新 ' + new Date(st.datasetUpdated).toLocaleString() : '')
      : '状态文件为空(尚未播种或已 --reset)';
  }
  // 卡片
  box.innerHTML = '';
  var rows = st.accounts;
  if (!rows.length) {
    box.appendChild(el('div', { class: 'vc-card-desc', text: '总览为空:点「刷新总览」按当前旋钮派生;或播种完成后自动刷新' }));
    return;
  }
  var grouped = seedGroupRows(rows);
  var notes = seedOverviewNoteFns();
  var allAcct = grouped.admin.concat(grouped.users, grouped.issuers);
  box.appendChild(seedBuildCardTable('账户', '管理员 + 用户 + 签发方;私钥由 seed 短语本地派生(' + (st.seedPhrase || '-') + ');「发证书」按当前短语派生双方密钥签发',
    allAcct, function (r) { return r.role === 'user' ? (r.alias || '') : (r.subjectType ? r.subjectType + (r.alias ? ' · ' + r.alias : '') : ''); },
    null, { rowAction: seedIssueButtonFor }));
  box.appendChild(seedBuildCardTable('多签', '多签账户本体;签名者为普通账号', grouped.multisig, notes.multisig));
  box.appendChild(seedBuildCardTable('代币', '代币/合规代币账户(owner 均为 admin);「查链上持仓」按 dataset 地址实查全员余额', grouped.tokens, notes.token,
    null, { extraButtons: [{ text: '查链上持仓', onclick: fetchTokenHoldings }] }));
  box.appendChild(seedBuildCardTable('SFT', 'SFT 集合(mint authority)与 slot 产物;「查链上持仓」逐 tokenId 实查持有人', grouped.sfts, notes.sft,
    null, { extraButtons: [{ text: '查链上持仓', onclick: fetchSftHoldings }] }));
  // VC 证书卡:链上实查(懒加载)+ 计划参考
  box.appendChild(seedRenderVcCard());
  // 持仓明细卡(查过才出现)
  if (st.tokenHoldings) box.appendChild(seedRenderTokenHoldingsCard());
  if (st.sftHoldings) box.appendChild(seedRenderSftHoldingsCard());
}

// 账户卡行操作:发证书按钮
function seedIssueButtonFor(r) {
  return el('button', {
    class: 'btn btn-secondary btn-sm', text: '发证书',
    onclick: function () { openVcIssuePanel(r); },
  });
}

// 地址 → 账户名(派生账户表内查找;找不到回显地址)
function seedNameFor(addr) {
  var rows = seedOverviewState.accounts || [];
  for (var i = 0; i < rows.length; i++) {
    if (rows[i].address === addr) return rows[i].name;
  }
  return addr;
}

// ---------- 发证书 + 三卡链上实查(交互) ----------

// /api/read 通用视图调用(服务端按 IDL 解码返回 JSON)
function seedViewCall(desc) {
  return apiFetch('/api/read', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(desc),
  }).then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) throw new Error(body.message || '视图调用失败');
      return body.data;
    });
}

// 当前生效命名空间的 dataset 产物(tokens/sfts 地址与 tokenIds)
function seedCurrentArt() {
  var st = seedOverviewState;
  var target = st.selectedDataset || st.configHash;
  for (var i = 0; i < st.datasets.length; i++) {
    if (st.datasets[i].configHash === target) return seedDatasetArtifacts(st.datasets[i]);
  }
  return null;
}

// 发证书面板:插在目标行下方,不整表重渲(避免输入态丢失)
function openVcIssuePanel(r, srcEl) {
  var old = document.querySelector('.seed-issue-panel');
  if (old) old.parentNode.removeChild(old);
  var tr = srcEl && srcEl.closest ? srcEl.closest('tr') : null;
  if (!tr || !tr.parentNode) { if (typeof showToast === 'function') showToast('找不到目标行', 'error'); return; }
  var panel = el('tr', { class: 'seed-issue-panel' });
  var td = el('td', { colspan: '9', style: 'background:var(--bg-tertiary)' });
  td.appendChild(el('span', { class: 'vc-card-desc', text: '给 ' + r.name + ' 发证书:' }));
  td.appendChild(el('input', { class: 'seed-issue-issuer', value: 'kycorg', title: '签发方账户名(须为 issuer)' }));
  td.appendChild(el('input', { class: 'seed-issue-prefix', value: 'SeedKyc', title: 'schema 前缀 → 签发 前缀1..N' }));
  td.appendChild(el('input', { class: 'seed-issue-count', value: '1', type: 'number', min: '1', max: '10', title: '张数 1..10' }));
  td.appendChild(el('button', {
    class: 'btn btn-primary btn-sm', text: '签发',
    onclick: function () { submitVcIssue(r, panel, this); },
  }));
  td.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', text: '取消',
    onclick: function () { if (panel.parentNode) panel.parentNode.removeChild(panel); },
  }));
  tr.parentNode.insertBefore(panel, tr.nextSibling);
  if (typeof showToast === 'function') showToast('发证书面板已展开(幂等,重复签发自动跳过)', 'success');
}

// 提交发证书;成功后移除面板并刷新已加载的链上证书数据
function submitVcIssue(r, panel, btn) {
  var q = function (cls) { return panel.querySelector ? panel.querySelector('.' + cls) : null; };
  var issuer = q('seed-issue-issuer'), prefix = q('seed-issue-prefix'), count = q('seed-issue-count');
  if (!issuer || !prefix || !count) return;
  if (typeof buildSeedCfg !== 'function') return;
  var payload = vcIssuePayload(buildSeedCfg(), String(issuer.value || '').trim(), r.name,
    String(prefix.value || '').trim(), parseInt(count.value, 10) || 1);
  btn.textContent = '签发中…';
  btn.setAttribute('disabled', 'disabled');
  apiFetch('/api/tool/vc-issue', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  }).then(function (resp) { return resp.json(); })
    .then(function (body) {
      btn.textContent = '签发';
      btn.removeAttribute('disabled');
      if (!body.success) throw new Error(body.message || '签发失败');
      var creds = (body.data && body.data.credentials) || [];
      var schemas = creds.map(function (cr) { return cr.schema + (cr.valid ? '✓' : '(已冻结/撤销)'); });
      showToast('证书已上链: ' + schemas.join(', '), 'success');
      if (panel.parentNode) panel.parentNode.removeChild(panel);
      if (seedOverviewState.vcHoldings) fetchVcHoldings();
    })
    .catch(function (e) {
      btn.textContent = '签发';
      btn.removeAttribute('disabled');
      if (typeof showToast === 'function') showToast('签发失败: ' + (e && e.message || e), 'error');
    });
}

// 证书卡:链上实查区 + 计划参考
function seedRenderVcCard() {
  var st = seedOverviewState;
  var card = el('div', { class: 'vc-card seed-ov-card' });
  var header = el('div', { class: 'vc-card-header' });
  header.appendChild(el('h3', { text: 'VC 证书' }));
  header.appendChild(el('span', { class: 'vc-card-desc', text: '链上实查(identity.DisclosedVcs,懒加载)+ 计划参考;发证书入口在账户卡行内「操作」列' }));
  card.appendChild(header);
  var foot = el('div', { style: 'display:flex;gap:8px;margin-top:8px' });
  foot.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', text: '查链上证书',
    onclick: function () { fetchVcHoldings(); },
  }));
  foot.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', text: '复制实查(JSON)',
    onclick: function () {
      seedCopyText(JSON.stringify(st.vcHoldings || {}, null, 2), '证书实查已复制');
    },
  }));
  card.appendChild(foot);
  // 链上实查结果
  if (st.vcHoldings) {
    var keys = Object.keys(st.vcHoldings);
    if (!keys.length) {
      card.appendChild(el('div', { class: 'vc-card-desc', text: '查询中…或名单内无人持证' }));
    } else {
      var wrap = el('div', { style: 'overflow-x:auto;margin-top:8px' });
      var tbl = el('table', { class: 'seed-table' });
      var thead = el('thead', null);
      var hrow = el('tr', null);
      ['账户', '持证 schema', '凭证数'].forEach(function (h) { hrow.appendChild(el('th', { text: h })); });
      thead.appendChild(hrow);
      tbl.appendChild(thead);
      var tbody = el('tbody', null);
      keys.forEach(function (addr) {
        var schemas = st.vcHoldings[addr];
        var tr = el('tr', null);
        tr.appendChild(el('td', { class: 'seed-mono', text: seedNameFor(addr) }));
        tr.appendChild(el('td', { text: schemas ? schemas.join(', ') : '查询失败' }));
        tr.appendChild(el('td', { text: schemas ? String(schemas.length) : '—' }));
        tbody.appendChild(tr);
      });
      tbl.appendChild(tbody);
      wrap.appendChild(tbl);
      card.appendChild(wrap);
    }
  }
  // 计划参考
  var plan = st.plan;
  if (plan && (plan.vcIssuers || []).length) {
    (plan.vcIssuers || []).forEach(function (v) {
      card.appendChild(el('div', { class: 'vc-card-desc', style: 'margin:4px 0',
        text: '计划参考: ' + v.issuerName + ' · ' + v.schemaPrefix + '1~' + v.schemaCount + ' → 持证:' + (v.certifies || []).join(',') }));
    });
    (plan.vcLifecycle || []).forEach(function (l) {
      card.appendChild(el('div', { class: 'vc-card-desc', style: 'margin:2px 0', text: '生命周期: ' + l.issuerName + ' ' + l.kind + ' → ' + l.user }));
    });
  }
  return card;
}

// 懒加载链上证书:逐账户 identity.DisclosedVcs
function fetchVcHoldings() {
  var st = seedOverviewState;
  var addrs = {};
  (st.accounts || []).forEach(function (r) { if (r.address) addrs[r.address] = true; });
  var keys = Object.keys(addrs);
  if (!keys.length) { if (typeof showToast === 'function') showToast('总览暂无账户,先刷新总览', 'warning'); return; }
  st.vcHoldings = {};
  seedRenderOverview();
  var pending = keys.map(function (addr) {
    return seedViewCall(vcHoldingsView(addr))
      .then(function (data) { st.vcHoldings[addr] = normalizeDisclosedVcs(data); })
      .catch(function () { st.vcHoldings[addr] = null; });
  });
  Promise.all(pending).then(function () { seedRenderOverview(); });
}

// 代币持仓:dataset 代币地址 × 全员 token.BalanceOf(非零才入表)
function fetchTokenHoldings() {
  var st = seedOverviewState;
  var art = seedCurrentArt();
  var tokens = art && art.tokens;
  if (!tokens || !Object.keys(tokens).length) {
    if (typeof showToast === 'function') showToast('当前数据集没有代币地址(先播种代币模块)', 'warning');
    return;
  }
  var holders = (st.accounts || []).filter(function (r) { return r.address; });
  if (!holders.length) return;
  st.tokenHoldings = [];
  seedRenderOverview();
  var jobs = [];
  Object.keys(tokens).forEach(function (sym) {
    holders.forEach(function (h) {
      jobs.push({ symbol: sym, tokenAddr: tokens[sym], holderName: h.name, holderAddr: h.address });
    });
  });
  return Promise.all(jobs.map(function (j) {
    return seedViewCall(tokenBalanceView(j.tokenAddr, j.holderAddr))
      .then(function (data) {
        var bal = normalizeU64View(data);
        if (bal != null && bal > 0) {
          st.tokenHoldings.push({ symbol: j.symbol, holderName: j.holderName, holderAddr: j.holderAddr, balance: bal });
        }
      })
      .catch(function () { /* 单点失败跳过,不阻塞其余 */ });
  })).then(function () {
    st.tokenHoldings.sort(function (a, b) { return a.symbol < b.symbol ? -1 : (a.symbol > b.symbol ? 1 : 0); });
    seedRenderOverview();
  });
}

// SFT 持仓:dataset tokenIds × sftoken.SftOwnerOf
function fetchSftHoldings() {
  var st = seedOverviewState;
  var art = seedCurrentArt();
  if (!art || !Object.keys(art.sfts || {}).length) {
    if (typeof showToast === 'function') showToast('当前数据集没有 SFT 地址(先播种 SFT 模块)', 'warning');
    return;
  }
  var jobs = [];
  Object.keys(art.sfts).forEach(function (key) {
    jobs = jobs.concat(sftHoldingJobs(art, key));
  });
  if (!jobs.length) {
    if (typeof showToast === 'function') showToast('dataset 里没有 tokenId 产物(旧播种?', 'warning');
    return;
  }
  st.sftHoldings = [];
  seedRenderOverview();
  return Promise.all(jobs.map(function (j) {
    return seedViewCall(sftOwnerView(j.sft, j.tokenId))
      .then(function (data) {
        var owner = null;
        if (typeof data === 'string') owner = data;
        else if (data && typeof data === 'object' && typeof data.address === 'string') owner = data.address;
        if (owner) {
          st.sftHoldings.push({ key: j.key, scenario: j.scenario, tokenId: j.tokenId,
            ownerAddr: owner, ownerName: seedNameFor(owner) });
        }
      })
      .catch(function () { /* 单点失败跳过 */ });
  })).then(function () {
    st.sftHoldings.sort(function (a, b) { return a.tokenId - b.tokenId; });
    seedRenderOverview();
  });
}

// 持仓明细卡(查过才出现)
function seedRenderTokenHoldingsCard() {
  var st = seedOverviewState;
  var card = el('div', { class: 'vc-card seed-ov-card' });
  var header = el('div', { class: 'vc-card-header' });
  header.appendChild(el('h3', { text: '代币持仓' }));
  header.appendChild(el('span', { class: 'vc-card-desc', text: 'dataset 代币 × 全员余额(非零;token.BalanceOf)' }));
  card.appendChild(header);
  var rows = st.tokenHoldings || [];
  if (!rows.length) {
    card.appendChild(el('div', { class: 'vc-card-desc', text: '查询中…或全部为零' }));
    return card;
  }
  var wrap = el('div', { style: 'overflow-x:auto' });
  var tbl = el('table', { class: 'seed-table' });
  var thead = el('thead', null);
  var hrow = el('tr', null);
  ['代币', '持有人', '余额'].forEach(function (h) { hrow.appendChild(el('th', { text: h })); });
  thead.appendChild(hrow);
  tbl.appendChild(thead);
  var tbody = el('tbody', null);
  rows.forEach(function (r) {
    var tr = el('tr', null);
    tr.appendChild(el('td', { class: 'seed-mono', text: r.symbol }));
    tr.appendChild(el('td', { text: r.holderName }));
    tr.appendChild(el('td', { class: 'seed-mono', text: String(r.balance) }));
    tbody.appendChild(tr);
  });
  tbl.appendChild(tbody);
  wrap.appendChild(tbl);
  card.appendChild(wrap);
  card.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', style: 'margin-top:8px', text: '复制明细(JSON)',
    onclick: function () { seedCopyText(JSON.stringify(rows, null, 2), '代币持仓已复制'); },
  }));
  return card;
}

function seedRenderSftHoldingsCard() {
  var st = seedOverviewState;
  var card = el('div', { class: 'vc-card seed-ov-card' });
  var header = el('div', { class: 'vc-card-header' });
  header.appendChild(el('h3', { text: 'SFT 持仓' }));
  header.appendChild(el('span', { class: 'vc-card-desc', text: 'dataset tokenId × 持有人(SftOwnerOf)' }));
  card.appendChild(header);
  var rows = st.sftHoldings || [];
  if (!rows.length) {
    card.appendChild(el('div', { class: 'vc-card-desc', text: '查询中…或 tokenId 不可读' }));
    return card;
  }
  var wrap = el('div', { style: 'overflow-x:auto' });
  var tbl = el('table', { class: 'seed-table' });
  var thead = el('thead', null);
  var hrow = el('tr', null);
  ['集合', '场景', 'tokenId', '持有人'].forEach(function (h) { hrow.appendChild(el('th', { text: h })); });
  thead.appendChild(hrow);
  tbl.appendChild(thead);
  var tbody = el('tbody', null);
  rows.forEach(function (r) {
    var tr = el('tr', null);
    tr.appendChild(el('td', { class: 'seed-mono', text: r.key }));
    tr.appendChild(el('td', { text: r.scenario }));
    tr.appendChild(el('td', { class: 'seed-mono', text: String(r.tokenId) }));
    tr.appendChild(el('td', { text: r.ownerName + (r.ownerName === r.ownerAddr ? '' : ' (' + r.ownerAddr + ')') }));
    tbody.appendChild(tr);
  });
  tbl.appendChild(tbody);
  wrap.appendChild(tbl);
  card.appendChild(wrap);
  card.appendChild(el('button', {
    class: 'btn btn-secondary btn-sm', style: 'margin-top:8px', text: '复制明细(JSON)',
    onclick: function () { seedCopyText(JSON.stringify(rows, null, 2), 'SFT 持仓已复制'); },
  }));
  return card;
}

// 刷新总览:按当前旋钮派生账户 + 读数据集,渲染
function refreshSeedOverview() {
  var box = document.getElementById('seedOverviewCards');
  if (!box) return;
  if (typeof buildSeedCfg !== 'function') return;
  var cfg = buildSeedCfg();
  Promise.all([seedFetchAccounts(cfg), seedFetchDataset()])
    .then(function (rs) {
      var acc = rs[0], ds = rs[1];
      seedOverviewState.accounts = acc.accounts;
      seedOverviewState.seedPhrase = acc.seedPhrase;
      seedOverviewState.configHash = acc.configHash;
      seedOverviewState.datasets = ds.datasets;
      seedOverviewState.datasetUpdated = ds.updatedAt;
      seedRenderOverview();
    })
    .catch(function (e) {
      seedOverviewState.accounts = [];
      seedRenderOverview();
      box.appendChild(el('div', { class: 'vc-card-desc', text: '总览加载失败: ' + (e && e.message || e) }));
    });
}

function initSeedOverview() {
  if (!document.getElementById('seedOverviewCards')) return;
  var btn = document.getElementById('seedOverviewRefresh');
  if (btn) btn.addEventListener('click', refreshSeedOverview);
  var copyBtn = document.getElementById('seedOverviewCopy');
  if (copyBtn) copyBtn.addEventListener('click', function () {
    var st = seedOverviewState;
    var ds = null;
    st.datasets.forEach(function (d) { if (d.configHash === st.configHash) ds = d; });
    seedCopyText(JSON.stringify({
      seedPhrase: st.seedPhrase, configHash: st.configHash,
      accounts: st.accounts, dataset: ds,
    }, null, 2), '总览已复制(JSON)');
  });
  var sel = document.getElementById('seedDatasetSel');
  if (sel) sel.addEventListener('change', function () {
    seedOverviewState.configHash = sel.value;
    seedRenderOverview();
  });
  refreshSeedOverview();
}

if (typeof document !== 'undefined' && typeof document.getElementById === 'function' && document.getElementById('seedOverviewCards')) {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initSeedOverview);
  } else {
    initSeedOverview();
  }
}
