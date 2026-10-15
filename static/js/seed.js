// ==================== 一键测试数据播种（/api/tool/seed）====================
// 依赖 app.js 的全局：apiFetch / $ / showToast
// 流程:规模/模块/seed 短语 → (可选)计划预览 → 确认 → 启动异步任务 →
//       2s 轮询进度(阶段+日志) → 完成展示关键地址表。
// 私钥永不返回:地址表仅作引用,完整地址/私钥在 data/seed_state.json。

var seedState = { jobId: null, pollTimer: null };

var SEED_MODULE_LABELS = {
  accounts: '账户(领水+DID)',
  tokens: '代币(多精度+冻结)',
  sft: 'SFT(集合+场景slot)',
  vc: 'VC 证书(组织+发放)',
  compliance: '合规代币(挂证)',
  txs: '交易流(互转+失败实测)',
};

function seedSelectedModules() {
  var out = [];
  document.querySelectorAll('.seed-mod-check:checked').forEach(function (box) {
    out.push(box.value);
  });
  return out;
}

function seedFillPreview(plan) {
  var box = $('seedPreview');
  if (!box) return;
  var lines = [];
  lines.push('规模: ' + plan.scale + '   seed 短语: ' + plan.seedPhrase);
  lines.push('账户: admin 1 + 用户 ' + (plan.users || []).length + ' + issuer ' + (plan.issuers || []).length);
  var tokens = plan.tokens || [];
  var decimals = tokens.map(function (t) { return t.decimals; }).join(', ');
  lines.push('代币: ' + tokens.length + ' 个(精度 ' + decimals + ')');
  (plan.sfts || []).forEach(function (s) {
    lines.push('SFT ' + s.key + '(' + s.symbol + '): ' + (s.scenarios || []).length + ' 场景');
  });
  (plan.vcIssuers || []).forEach(function (v) {
    lines.push('VC: ' + v.issuerName + ' 签发 ' + v.schemaPrefix + '1~' + v.schemaCount + ' → ' + (v.certifies || []).length + ' 人');
  });
  var c = plan.compliance || {};
  if (c.symbol) {
    lines.push('合规: ' + c.symbol + ' 挂证 ' + c.schema + '@' + c.issuerName + ' mode=' + c.mode +
      ';持证 ' + c.certifiedReceiver + ' 可收,无证 ' + c.uncertifiedVictim + ' 拦截实测');
  }
  var txs = plan.txs || {};
  lines.push('交易流: ' + ((txs.milTransfers || []).length) + ' 笔互转 + 批量 + ' +
    ((txs.failures || []).length) + ' 笔预期失败');
  box.textContent = lines.join('\n');
}

function seedPreview() {
  var scale = $('seedScale').value;
  apiFetch('/api/tool/seed/plan?scale=' + encodeURIComponent(scale))
    .then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) { showToast(body.message || '预览失败', 'error'); return; }
      seedFillPreview(body.data);
    })
    .catch(function (e) { showToast('预览失败: ' + e, 'error'); });
}

function seedStart() {
  if (seedState.jobId) { showToast('已有任务在跑: ' + seedState.jobId, 'warning'); return; }
  var scale = $('seedScale').value;
  var phrase = ($('seedPhrase').value || '').trim() || 'milon-devnet-seed-v1';
  var reset = !!$('seedReset').checked;
  var modules = seedSelectedModules();
  var desc = '将向当前网络写入大量链上数据:\n' +
    '规模 ' + scale + ' / 模块 ' + (modules.length ? modules.join(', ') : '全部') +
    '\n重新创世后地址不变;同链重复执行幂等续跑。确认开始?';
  if (!window.confirm(desc)) return;

  var payload = { scale: scale, seedPhrase: phrase, reset: reset };
  if (modules.length) payload.modules = modules;

  apiFetch('/api/tool/seed', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  }).then(function (r) { return r.json().then(function (b) { return { code: r.status, body: b }; }); })
    .then(function (res) {
      if (res.code === 202 && res.body.success) {
        seedState.jobId = res.body.data.jobId;
        $('seedJobId').textContent = seedState.jobId;
        $('seedProgressBox').style.display = '';
        $('seedAccountsBox').style.display = 'none';
        $('seedLog').textContent = '';
        showToast('播种任务已启动', 'success');
        seedPoll();
      } else {
        showToast(res.body.message || '启动失败', 'error');
        if (res.body.data && res.body.data.jobId) {
          // 409:展示在跑任务
          seedState.jobId = res.body.data.jobId;
          $('seedJobId').textContent = seedState.jobId;
          $('seedProgressBox').style.display = '';
          seedPoll();
        }
      }
    })
    .catch(function (e) { showToast('启动失败: ' + e, 'error'); });
}

function seedPoll() {
  if (!seedState.jobId) return;
  apiFetch('/api/tool/seed/status/' + encodeURIComponent(seedState.jobId))
    .then(function (r) { return r.json(); })
    .then(function (body) {
      if (!body.success) { showToast(body.message || '进度查询失败', 'error'); return; }
      var d = body.data;
      $('seedStatus').textContent = d.status || '';
      var logs = (d.logs || []).join('\n');
      var logBox = $('seedLog');
      if (logBox.textContent !== logs) {
        logBox.textContent = logs;
        logBox.scrollTop = logBox.scrollHeight;
      }
      var stageLines = [];
      var stages = d.stages || {};
      Object.keys(stages).sort().forEach(function (m) {
        stageLines.push(m + ': ' + (stages[m] || []).length + ' 阶段');
      });
      $('seedStages').textContent = stageLines.join('\n') || '(尚未落盘)';
      if (d.status === 'completed' || d.status === 'failed') {
        clearInterval(seedState.pollTimer);
        seedState.pollTimer = null;
        seedState.jobId = null;
        $('seedJobId').textContent = '';
        if (d.status === 'failed') {
          showToast('播种失败: ' + (d.error || ''), 'error');
        } else {
          showToast('播种完成 ✓', 'success');
        }
        if (d.accounts) {
          var box = $('seedAccounts');
          var rows = Object.keys(d.accounts).map(function (k) {
            return k + '  ' + d.accounts[k];
          });
          box.textContent = rows.join('\n');
          $('seedAccountsBox').style.display = '';
        }
        return;
      }
      seedState.pollTimer = setTimeout(seedPoll, 2000);
    })
    .catch(function () { seedState.pollTimer = setTimeout(seedPoll, 3000); });
}

function seedCopyAccounts() {
  var text = $('seedAccounts').textContent || '';
  if (!text) return;
  if (navigator.clipboard) {
    navigator.clipboard.writeText(text).then(function () { showToast('已复制', 'success'); });
  } else {
    showToast('浏览器不支持剪贴板', 'warning');
  }
}

function initSeedView() {
  $('seedPreviewBtn').addEventListener('click', seedPreview);
  $('seedStartBtn').addEventListener('click', seedStart);
  $('seedCopyBtn').addEventListener('click', seedCopyAccounts);
  seedPreview();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initSeedView);
} else {
  initSeedView();
}
