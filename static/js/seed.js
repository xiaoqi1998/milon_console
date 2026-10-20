// ==================== 测试数据构造器 v2(模板 + 全局旋钮)====================
// 依赖 app.js 的全局:apiFetch / $ / el / showToast
// 数据流:DOM → buildSeedCfg() → POST /api/tool/seed/plan(防抖 500ms)→ 计划预览
//        → POST /api/tool/seed → GET /api/tool/seed/status/:id 轮询 → 地址表
// 配置结构与 handler.SeedConfig JSON tag 逐字段对齐(seedPhrase/accounts/multisig/tokens/sft)。
// node 直抽可加载:无顶层 await、无顶层 fetch;初始化由 DOMContentLoaded 守卫 + 元素存在检查。

var SEED_CFG_KEY = 'seedV2Cfg';
var SEED_DEFAULT_PHRASE = 'milon-devnet-seed-v1';
var seedState = { jobId: null, pollTimer: null };
var seedPreviewTimer = null;
var seedTemplateCache = null;

// chips 容器 → 字段映射(data-key 为配置原值,.on 为选中态)
var SEED_CHIPS = {
  seedCurvesChips: { list: ['secp256k1', 'ed25519', 'fndsa512'], field: 'curves' },
  seedTiersChips: { list: ['hundred', 'thousand', 'ten_k'], field: 'balanceTiers' },
  seedDecimalsChips: { list: ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9'], field: 'decimals' },
  seedScenariosChips: { list: ['transferable', 'frozen', 'merge', 'burn', 'reject_transfer'], field: 'scenarios' },
};

var SEED_CHIP_LABELS = {
  hundred: '百 MIL', thousand: '千 MIL', ten_k: '万 MIL',
  transferable: '可转移', frozen: '冻结', merge: '合并', burn: '销毁', reject_transfer: '无证拒收',
};

var SEED_MODULE_LABELS = {
  accounts: '账户', multisig: '多签', tokens: '代币', sft: 'SFT',
  vc: 'VC', compliance: '合规', txs: '交易流',
};

// 初始默认 = 服务端 standard 模板内容(handler.SeedConfigDefaults)
var SEED_DEFAULT_CFG = {
  seedPhrase: SEED_DEFAULT_PHRASE,
  accounts: { count: 8, curves: ['secp256k1', 'ed25519'], faucet: true, balanceTiers: ['thousand'], did: true, vcCert: true },
  multisig: { count: 1, signers: 5, threshold: 3, faucet: true, did: true },
  tokens: { count: 3, decimals: [0, 6, 9], complianceCount: 1, freezeDemo: true },
  sft: { count: 2, slotsPerSft: 2, tokensPerSlot: 3, scenarios: ['transferable', 'frozen', 'merge', 'burn', 'reject_transfer'], postOps: true },
};

// ---------- DOM 读取 ----------

// 本地 getElementById 封装:不依赖 app.js 的 $ —— node 直抽 eval 环境下仅 mock document
function seedEl(id) {
  return document.getElementById(id);
}

function seedInt(id, fallback) {
  var node = seedEl(id);
  var v = node ? parseInt(node.value, 10) : NaN;
  return isNaN(v) ? fallback : v;
}

function seedBool(id) {
  var node = seedEl(id);
  return !!(node && node.checked);
}

// chip 元素来源:浏览器走容器 querySelectorAll;node 直抽环境回退 __chips[id] 注入
function seedChipNodes(id) {
  var box = document.getElementById(id);
  if (box && box.querySelectorAll) {
    return box.querySelectorAll('.chip');
  }
  if (typeof __chips === 'object' && __chips && __chips[id]) {
    return __chips[id];
  }
  return [];
}

function seedChipsVals(id) {
  return Array.prototype.filter.call(seedChipNodes(id), function (n) {
    return n.classList && n.classList.contains('on');
  }).map(function (n) { return n.dataset.key; });
}

// DOM → SeedConfig(字段名与 handler.SeedConfig JSON tag 一致)
function buildSeedCfg() {
  var phrase = seedEl('seedPhrase') ? String(seedEl('seedPhrase').value || '').trim() : '';
  return {
    seedPhrase: phrase || SEED_DEFAULT_PHRASE,
    accounts: {
      count: seedInt('seedAcctCount', 8),
      curves: seedChipsVals('seedCurvesChips'),
      faucet: seedBool('seedAcctFaucet'),
      balanceTiers: seedChipsVals('seedTiersChips'),
      did: seedBool('seedAcctDid'),
      vcCert: seedBool('seedAcctVc'),
    },
    multisig: {
      count: seedInt('seedMsCount', 0),
      signers: seedInt('seedMsSigners', 5),
      threshold: seedInt('seedMsThreshold', 3),
      faucet: seedBool('seedMsFaucet'),
      did: seedBool('seedMsDid'),
    },
    tokens: {
      count: seedInt('seedTokCount', 3),
      decimals: seedChipsVals('seedDecimalsChips').map(Number),
      complianceCount: seedInt('seedCompCount', 0),
      freezeDemo: seedBool('seedFreezeDemo'),
    },
    sft: {
      count: seedInt('seedSftCount', 2),
      slotsPerSft: seedInt('seedSftSlots', 2),
      tokensPerSlot: seedInt('seedSftTokens', 3),
      scenarios: seedChipsVals('seedScenariosChips'),
      postOps: seedBool('seedPostOps'),
    },
  };
}

// ---------- DOM 写入(模板/回填) ----------

function seedSetChips(id, values) {
  var box = seedEl(id);
  if (!box) return;
  var set = {};
  (values || []).forEach(function (v) { set[String(v)] = true; });
  Array.prototype.forEach.call(box.querySelectorAll('.chip'), function (chip) {
    chip.classList.toggle('on', !!set[chip.dataset.key]);
  });
}

// SeedConfig 对象 → 填充全部旋钮
function seedFillForm(cfg) {
  if (!cfg) return;
  var a = cfg.accounts || {}, m = cfg.multisig || {}, t = cfg.tokens || {}, s = cfg.sft || {};
  if (seedEl('seedPhrase')) seedEl('seedPhrase').value = cfg.seedPhrase || '';
  if (seedEl('seedAcctCount')) seedEl('seedAcctCount').value = a.count != null ? a.count : 8;
  seedSetChips('seedCurvesChips', a.curves);
  if (seedEl('seedAcctFaucet')) seedEl('seedAcctFaucet').checked = !!a.faucet;
  seedSetChips('seedTiersChips', a.balanceTiers);
  if (seedEl('seedAcctDid')) seedEl('seedAcctDid').checked = !!a.did;
  if (seedEl('seedAcctVc')) seedEl('seedAcctVc').checked = !!a.vcCert;
  if (seedEl('seedMsCount')) seedEl('seedMsCount').value = m.count != null ? m.count : 0;
  if (seedEl('seedMsSigners')) seedEl('seedMsSigners').value = m.signers != null ? m.signers : 5;
  if (seedEl('seedMsThreshold')) seedEl('seedMsThreshold').value = m.threshold != null ? m.threshold : 3;
  if (seedEl('seedMsFaucet')) seedEl('seedMsFaucet').checked = !!m.faucet;
  if (seedEl('seedMsDid')) seedEl('seedMsDid').checked = !!m.did;
  if (seedEl('seedTokCount')) seedEl('seedTokCount').value = t.count != null ? t.count : 3;
  seedSetChips('seedDecimalsChips', seedDecimalsToArr(t.decimals));
  if (seedEl('seedCompCount')) seedEl('seedCompCount').value = t.complianceCount != null ? t.complianceCount : 0;
  if (seedEl('seedFreezeDemo')) seedEl('seedFreezeDemo').checked = !!t.freezeDemo;
  if (seedEl('seedSftCount')) seedEl('seedSftCount').value = s.count != null ? s.count : 2;
  if (seedEl('seedSftSlots')) seedEl('seedSftSlots').value = s.slotsPerSft != null ? s.slotsPerSft : 2;
  if (seedEl('seedSftTokens')) seedEl('seedSftTokens').value = s.tokensPerSlot != null ? s.tokensPerSlot : 3;
  seedSetChips('seedScenariosChips', s.scenarios);
  if (seedEl('seedPostOps')) seedEl('seedPostOps').checked = !!s.postOps;
}

// Go []uint8 经 encoding/json 序列化为 base64 字符串(如 [0,6] → "AAY="):
// 服务端下发的模板/回显 config 须还原为数字数组再填 chips
function seedDecimalsToArr(v) {
  if (Array.isArray(v)) return v;
  if (typeof v === 'string' && v.length) {
    try {
      var bin = atob(v);
      var out = [];
      for (var i = 0; i < bin.length; i++) out.push(bin.charCodeAt(i));
      return out;
    } catch (e) { return []; }
  }
  return [];
}

function seedRenderChips() {
  Object.keys(SEED_CHIPS).forEach(function (id) {
    var box = seedEl(id);
    if (!box) return;
    box.innerHTML = '';
    SEED_CHIPS[id].list.forEach(function (key) {
      var chip = el('button', { class: 'chip', type: 'button', 'data-key': key, text: SEED_CHIP_LABELS[key] || key });
      chip.addEventListener('click', function () {
        chip.classList.toggle('on');
        refreshSeedPreview();
      });
      box.appendChild(chip);
    });
  });
}

// ---------- 模板条 ----------

function seedRenderTemplateBtns(templates) {
  var box = seedEl('seedTemplateBtns');
  if (!box) return;
  box.innerHTML = '';
  (templates || []).forEach(function (t) {
    var btn = el('button', { class: 'btn btn-secondary btn-sm', type: 'button', title: t.description, text: t.name });
    btn.addEventListener('click', function () { applySeedTemplate(t.key); });
    box.appendChild(btn);
  });
}

function seedLoadTemplates() {
  return apiFetch('/api/tool/seed/templates')
    .then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) { showToast(body.message || '模板拉取失败', 'error'); return null; }
      seedTemplateCache = body.data || [];
      seedRenderTemplateBtns(seedTemplateCache);
      return seedTemplateCache;
    })
    .catch(function (e) { showToast('模板拉取失败: ' + e, 'error'); return null; });
}

// GET templates → 填充 DOM → 立即刷新预览
function applySeedTemplate(key) {
  var use = function (list) {
    var tpl = null;
    (list || []).forEach(function (t) { if (t.key === key) tpl = t; });
    if (!tpl) { showToast('未找到模板: ' + key, 'error'); return; }
    seedFillForm(tpl.config);
    showToast('已应用模板「' + tpl.name + '」', 'success');
    refreshSeedPreview();
  };
  if (seedTemplateCache) { use(seedTemplateCache); return; }
  seedLoadTemplates().then(function (list) { if (list) use(list); });
}

// ---------- 计划预览(防抖 500ms) ----------

function refreshSeedPreview() {
  if (seedPreviewTimer) clearTimeout(seedPreviewTimer);
  seedPreviewTimer = setTimeout(seedPreviewNow, 500);
}

function seedShowErrors(message, errs) {
  var box = seedEl('seedPreview');
  var sumBox = seedEl('seedSummary');
  if (sumBox) sumBox.textContent = '✗ ' + message;
  if (!box) return;
  box.classList.add('is-error');
  var lines = Array.isArray(errs) && errs.length
    ? errs.map(function (e, i) { return (i + 1) + '. ' + e; })
    : [message];
  box.textContent = '✗ ' + message + (Array.isArray(errs) && errs.length ? '(' + errs.length + ' 项)' : '') +
    '\n' + lines.join('\n');
}

function seedRenderSummary(s) {
  var sumBox = seedEl('seedSummary');
  if (!sumBox || !s) return;
  var line = '账号 ' + s.accounts + ' · 多签 ' + s.multisig + ' · 代币 ' + s.tokens + '(合规 ' + s.tokensCompliance + ')' +
    ' · SFT ' + s.sftSets + ' 套 / ' + s.sftSlots + ' slot / ' + s.sftTokens + ' 枚 · 预估交易 ≈ ' + s.txEstimate;
  var warns = s.warnings || [];
  if (warns.length) line += '\n⚠ ' + warns.join('\n⚠ ');
  sumBox.textContent = line;
}

function seedRenderPlanTree(plan) {
  var box = seedEl('seedPreview');
  if (!box || !plan) return;
  var L = [];
  L.push('configHash: ' + (plan.configHash || '-') + '   seed 短语: ' + (plan.seedPhrase || ''));
  var accts = ['admin(' + (plan.admin ? plan.admin.curve : '-') + ')'];
  (plan.users || []).forEach(function (u) { accts.push(u.name + '(' + u.curve + ')'); });
  (plan.issuers || []).forEach(function (u) { accts.push(u.name + '(' + u.curve + ')'); });
  L.push('账号(' + accts.length + '): ' + accts.join(', '));
  (plan.multisig || []).forEach(function (m) {
    L.push('多签 ' + m.name + ': 签名者 ' + (m.signerNames || []).join(',') +
      ' 阈值 ' + m.threshold + '/' + (m.signerNames || []).length +
      (m.faucet ? ' 领水✓' : '') + (m.did ? ' DID✓' : ''));
  });
  (plan.tokens || []).forEach(function (t) {
    L.push('代币 ' + t.symbol + '(' + t.name + ') 精度 ' + t.decimals + (t.freezeDemoUser ? ' 冻结演示→' + t.freezeDemoUser : ''));
  });
  (plan.sfts || []).forEach(function (s) {
    var parts = (s.scenarios || []).map(function (sc) {
      return sc.key + '(' + (sc.dists || []).length + ' 枚' + ((sc.postOps || []).length ? ',' + sc.postOps.length + ' 后置' : '') + ')';
    });
    L.push('SFT ' + s.key + '(' + s.symbol + '): ' + parts.join(' / '));
  });
  (plan.vcIssuers || []).forEach(function (v) {
    L.push('VC ' + v.issuerName + ': ' + v.schemaPrefix + '1~' + v.schemaCount + ' → ' + (v.certifies || []).length + ' 人持证');
  });
  if ((plan.vcLifecycle || []).length) {
    L.push('VC 生命周期: ' + plan.vcLifecycle.length + ' 笔(冻结/吊销)');
  }
  (plan.compliance || []).forEach(function (c) {
    L.push('合规 ' + c.symbol + '@' + c.issuerName + ' schema=' + c.schema + ' mode=' + c.mode +
      ';持证 ' + c.certifiedReceiver + ' 可收,无证 ' + c.uncertifiedVictim + ' 拦截实测');
  });
  var txs = plan.txs || {};
  L.push('交易 ' + ((txs.milTransfers || []).length) + ' 笔互转 + 批量分发 + ' + ((txs.failures || []).length) + ' 笔预期失败');
  box.textContent = L.join('\n');
}

function seedPreviewNow() {
  seedPreviewTimer = null;
  var cfg = buildSeedCfg();
  try { localStorage.setItem(SEED_CFG_KEY, JSON.stringify(cfg)); } catch (e) { /* 隐私模式/配额满不阻塞 */ }
  apiFetch('/api/tool/seed/plan', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ config: cfg }),
  }).then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) {
        seedShowErrors(body.message || '计划生成失败', body.data);
        return;
      }
      var box = seedEl('seedPreview');
      if (box) box.classList.remove('is-error');
      seedRenderSummary(body.data.summary);
      seedRenderPlanTree(body.data.plan);
    })
    .catch(function (e) { seedShowErrors('预览请求失败: ' + e, null); });
}

// ---------- 启动 + 轮询 ----------

function seedBeginJob(jobId) {
  seedState.jobId = jobId;
  if (seedEl('seedJobId')) seedEl('seedJobId').textContent = jobId;
  if (seedEl('seedStatus')) seedEl('seedStatus').textContent = 'running';
  if (seedEl('seedStages')) seedEl('seedStages').textContent = '启动中…';
  var log = seedEl('seedLog');
  if (log) { log.textContent = ''; log.classList.remove('is-error'); }
  if (seedEl('seedAddrBox')) seedEl('seedAddrBox').style.display = 'none';
  seedPoll();
}

function seedPoll() {
  if (!seedState.jobId) return;
  apiFetch('/api/tool/seed/status/' + encodeURIComponent(seedState.jobId))
    .then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) {
        showToast(body.message || '进度查询失败', 'error');
        seedState.jobId = null;
        return;
      }
      var d = body.data;
      if (seedEl('seedStatus')) seedEl('seedStatus').textContent = d.status || '';
      var stageBox = seedEl('seedStages');
      if (stageBox) {
        stageBox.textContent = Object.keys(d.stages || {}).sort().map(function (m) {
          var names = d.stages[m] || [];
          return (SEED_MODULE_LABELS[m] || m) + ': ' + names.length + ' 阶段 — ' + names.join(' / ');
        }).join('\n') || '(尚未落盘)';
      }
      var logBox = seedEl('seedLog');
      if (logBox) {
        var logs = (d.logs || []).join('\n');
        if (logBox.textContent !== logs) {
          logBox.textContent = logs;
          logBox.scrollTop = logBox.scrollHeight;
        }
      }
      if (d.status === 'completed' || d.status === 'failed') {
        seedState.jobId = null;
        if (seedEl('seedJobId')) seedEl('seedJobId').textContent = '—';
        if (d.status === 'failed') {
          showToast('播种失败: ' + (d.error || ''), 'error');
          if (logBox) logBox.classList.add('is-error');
        } else {
          showToast('播种完成 ✓', 'success');
          seedRenderAddrTable(d.accounts || {});
        }
        return;
      }
      seedState.pollTimer = setTimeout(seedPoll, 2000);
    })
    .catch(function () { seedState.pollTimer = setTimeout(seedPoll, 3000); });
}

function seedRenderAddrTable(accounts) {
  var box = seedEl('seedAddrTable');
  var card = seedEl('seedAddrBox');
  if (!box || !card) return;
  var keys = Object.keys(accounts);
  box.textContent = keys.length
    ? keys.map(function (k) { return String(k).padEnd(24, ' ') + ' ' + accounts[k]; }).join('\n')
    : '(无地址返回)';
  card.style.display = '';
}

function seedCopyTable() {
  var box = seedEl('seedAddrTable');
  var text = box ? box.textContent : '';
  if (!text || text === '-' || text === '(无地址返回)') { showToast('暂无地址表', 'warning'); return; }
  if (navigator.clipboard) {
    navigator.clipboard.writeText(text).then(function () { showToast('已复制', 'success'); });
  } else {
    showToast('浏览器不支持剪贴板', 'warning');
  }
}

// 先 GET plan 校验(失败显示错误列表);txEstimate 超阈值弹确认;POST 启动 + 轮询
// 阈值 100:实测内置模板预估 极简 19 / 标准 87 / 全量 136——须让全量触发确认
function startSeedJob() {
  if (seedState.jobId) { showToast('已有任务在跑: ' + seedState.jobId, 'warning'); return; }
  var cfg = buildSeedCfg();
  apiFetch('/api/tool/seed/plan', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ config: cfg }),
  }).then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) {
        seedShowErrors(body.message || '配置校验失败', body.data);
        showToast('配置校验失败,详见计划预览错误列表', 'error');
        return null;
      }
      var summary = (body.data && body.data.summary) || {};
      if ((summary.txEstimate || 0) > 100 &&
          !window.confirm('预计 ' + summary.txEstimate + ' 笔链上交易,写入数据量大且耗时较长。确认开始播种?')) {
        return null;
      }
      var reset = seedBool('seedReset');
      var payload = { config: cfg };
      if (reset) payload.reset = true;
      return apiFetch('/api/tool/seed', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      }).then(function (r) { return r.json().then(function (b) { return { code: r.status, body: b }; }); })
        .then(function (res) {
          if (res.code === 202 && res.body.success) {
            seedBeginJob(res.body.data.jobId);
            showToast('播种任务已启动', 'success');
          } else {
            showToast(res.body.message || '启动失败', 'error');
            if (res.body.data && res.body.data.jobId) {
              // 409:已有任务在跑,直接接管轮询
              seedBeginJob(res.body.data.jobId);
            } else if (Array.isArray(res.body.data) && res.body.data.length) {
              seedShowErrors(res.body.message || '启动失败', res.body.data);
            }
          }
        });
    })
    .catch(function (e) { showToast('启动失败: ' + e, 'error'); });
}

// ---------- localStorage 保存/回填 ----------

function seedRestoreLast() {
  var raw = null;
  try { raw = localStorage.getItem(SEED_CFG_KEY); } catch (e) { return false; }
  if (!raw) return false;
  try {
    var cfg = JSON.parse(raw);
    if (cfg && cfg.accounts && cfg.tokens && cfg.sft) {
      seedFillForm(cfg);
      return true;
    }
  } catch (e) { /* 坏 JSON 走默认 */ }
  return false;
}

// ---------- 初始化(DOMContentLoaded 守卫,node eval 下无锚点则不绑定) ----------

function initSeedView() {
  seedRenderChips();
  if (!seedRestoreLast()) seedFillForm(SEED_DEFAULT_CFG);
  seedLoadTemplates();
  // 数字/短语输入 → 防抖预览
  ['seedAcctCount', 'seedMsCount', 'seedMsSigners', 'seedMsThreshold',
    'seedTokCount', 'seedCompCount', 'seedSftCount', 'seedSftSlots', 'seedSftTokens', 'seedPhrase'
  ].forEach(function (id) {
    var node = seedEl(id);
    if (node) node.addEventListener('input', refreshSeedPreview);
  });
  // 开关 → 防抖预览
  ['seedAcctFaucet', 'seedAcctDid', 'seedAcctVc', 'seedMsFaucet', 'seedMsDid', 'seedFreezeDemo', 'seedPostOps', 'seedReset']
    .forEach(function (id) {
      var node = seedEl(id);
      if (node) node.addEventListener('change', refreshSeedPreview);
    });
  if (seedEl('seedStartBtn')) seedEl('seedStartBtn').addEventListener('click', startSeedJob);
  if (seedEl('seedAddrCopy')) seedEl('seedAddrCopy').addEventListener('click', seedCopyTable);
  refreshSeedPreview();
}

if (typeof document !== 'undefined' && seedEl('seedStartBtn')) {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initSeedView);
  } else {
    initSeedView();
  }
}
