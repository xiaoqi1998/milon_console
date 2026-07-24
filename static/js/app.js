const ENDPOINTS = [
  { id: 'net-list', method: 'GET', path: '/api/network/list', summary: '获取网络列表', group: '网络管理' },
  { id: 'net-current', method: 'GET', path: '/api/network/current', summary: '获取当前网络', group: '网络管理' },
  { id: 'net-switch', method: 'POST', path: '/api/network/switch', summary: '切换网络', group: '网络管理',
    bodyTemplate: JSON.stringify({ network: 'devNet' }, null, 2) },
  { id: 'health', method: 'GET', path: '/api/health', summary: '健康检查', group: '系统' },
  { id: 'chain-head', method: 'GET', path: '/api/chain-head', summary: '获取链头', group: '系统' },
  { id: 'acc-info', method: 'GET', path: '/api/accounts/:address', summary: '获取账户信息', group: '账户',
    pathParams: [{ name: 'address', ph: 'base58地址' }] },
  { id: 'acc-resources', method: 'GET', path: '/api/accounts/:address/resources', summary: '获取账户资源', group: '账户',
    pathParams: [{ name: 'address', ph: 'base58地址' }] },
  { id: 'acc-generate', method: 'POST', path: '/api/accounts/generate', summary: '生成账户', group: '账户',
    bodyTemplate: JSON.stringify({ keyType: 'secp256k1' }, null, 2) },
  { id: 'tx-hash', method: 'GET', path: '/api/transactions/:hash', summary: '按哈希查交易', group: '交易',
    pathParams: [{ name: 'hash', ph: 'hex或base58' }] },
  { id: 'tx-events', method: 'GET', path: '/api/transactions/:hash/events', summary: '获取交易事件', group: '交易',
    pathParams: [{ name: 'hash', ph: 'hex或base58' }],
    queryParams: [{ name: 'typeTag', ph: '可选' }] },
  { id: 'tx-wait', method: 'GET', path: '/api/transactions/:hash/wait', summary: '等待交易确认', group: '交易',
    pathParams: [{ name: 'hash', ph: 'hex或base58' }],
    queryParams: [{ name: 'timeoutSecs', ph: '60' }] },
  { id: 'tx-simulate', method: 'POST', path: '/api/transactions/simulate', summary: '底层模拟交易', group: '交易',
    bodyTemplate: JSON.stringify({ transactionPostcard: 'base64编码' }, null, 2) },
  { id: 'tx-submit', method: 'POST', path: '/api/transactions/submit', summary: '底层提交交易', group: '交易',
    bodyTemplate: JSON.stringify({ transactionPostcard: 'base64编码' }, null, 2) },
  { id: 'tx-inspect', method: 'POST', path: '/api/transactions/inspect', summary: '检测原始交易', group: '交易',
    bodyTemplate: JSON.stringify({ transactionPostcard: 'base64编码' }, null, 2) },
  { id: 'read', method: 'POST', path: '/api/read', summary: '读取视图函数', group: '合约',
    bodyTemplate: JSON.stringify({ appName: 'token', methodName: 'balance_of', args: { owner: 'base58地址' }, payerAddress: 'base58地址' }, null, 2) },
  { id: 'read-multi', method: 'POST', path: '/api/read/multi', summary: '多指令视图查询', group: '合约',
    bodyTemplate: JSON.stringify({ instructions: [{ appName: 'token', methodName: 'balance_of', args: { owner: 'base58地址' } }] }, null, 2) },
  { id: 'simulate', method: 'POST', path: '/api/simulate', summary: '模拟合约调用', group: '合约',
    bodyTemplate: JSON.stringify({ appName: 'token', methodName: 'transfer', args: { to: 'base58地址', amount: 1000 }, paymentMode: 'unified_payer_all', payerAddress: 'base58地址', signatureMode: { type: 'pubkey', publicKey: 'base58公钥' } }, null, 2) },
  { id: 'write', method: 'POST', path: '/api/write', summary: '写入交易', group: '合约',
    bodyTemplate: JSON.stringify({ appName: 'token', methodName: 'transfer', args: { to: 'base58地址', amount: 1000 }, paymentMode: 'unified_payer_all', payerPrivateKey: 'hex或base58私钥', payerAddress: 'base58地址', signatureMode: { type: 'pubkey', publicKey: 'base58公钥' } }, null, 2) },
  { id: 'write-multi', method: 'POST', path: '/api/write/multi-agent', summary: '多方签名写入', group: '合约',
    bodyTemplate: JSON.stringify({ appName: 'token', methodName: 'transfer', args: {}, paymentMode: 'unified_dual_sign', payerPrivateKey: '', payerAddress: '', ixPrivateKey: '', ixAddress: '', signatureMode: { type: 'pubkey', publicKey: '' } }, null, 2) },
  { id: 'write-multisig', method: 'POST', path: '/api/write/multisig', summary: '多签写入', group: '合约',
    bodyTemplate: JSON.stringify({ appName: 'token', methodName: 'transfer', args: {}, paymentMode: 'split', ownerPrivateKey: '', ownerAddress: '', signatureMode: { type: 'pubkey', publicKey: '' } }, null, 2) },
  { id: 'block', method: 'GET', path: '/api/rpc/blocks/:height', summary: '获取区块', group: 'RPC',
    pathParams: [{ name: 'height', ph: '区块高度' }] },
  { id: 'resource', method: 'GET', path: '/api/rpc/resources/:hash', summary: '获取资源', group: 'RPC',
    pathParams: [{ name: 'hash', ph: 'hex 18字节' }] },
  { id: 'access-value', method: 'POST', path: '/api/rpc/access-value', summary: '获取访问值', group: 'RPC',
    bodyTemplate: JSON.stringify({ blobHashes: ['hex 32字节'] }, null, 2) },
  { id: 'derive-addr', method: 'POST', path: '/api/util/address/derive', summary: '从公钥派生地址', group: '工具',
    bodyTemplate: JSON.stringify({ publicKey: 'hex或base58', keyType: 'secp256k1' }, null, 2) },
  { id: 'derive-pub', method: 'POST', path: '/api/util/key/derive-public', summary: '从私钥派生公钥', group: '工具',
    bodyTemplate: JSON.stringify({ privateKey: 'hex或base58', keyType: 'secp256k1' }, null, 2) },
  { id: 'sign', method: 'POST', path: '/api/util/sign', summary: '签名消息', group: '工具',
    bodyTemplate: JSON.stringify({ privateKey: 'hex或base58', message: 'hex编码', keyType: 'secp256k1' }, null, 2) },
  { id: 'verify', method: 'POST', path: '/api/util/verify', summary: '验签', group: '工具',
    bodyTemplate: JSON.stringify({ publicKey: 'hex或base58', message: 'hex编码', signature: 'hex', keyType: 'secp256k1' }, null, 2) },
  { id: 'faucet-claim', method: 'POST', path: '/api/faucet/claim', summary: '领取水龙头代币', group: '水龙头',
    bodyTemplate: JSON.stringify({ privateKey: 'hex或base58私钥', address: 'base58地址', signatureMode: { type: 'pubkey', publicKey: 'base58公钥' } }, null, 2) },
  { id: 'faucet-balance', method: 'GET', path: '/api/faucet/balance/:address', summary: '查询MIL余额', group: '水龙头',
    pathParams: [{ name: 'address', ph: 'base58地址' }] },
  { id: 'view-single', method: 'POST', path: '/api/view/single', summary: '底层单指令视图', group: '合约',
    bodyTemplate: JSON.stringify({ transactionPostcard: 'base64编码' }, null, 2) },
  { id: 'view-multi', method: 'POST', path: '/api/view/multi', summary: '底层多指令视图', group: '合约',
    bodyTemplate: JSON.stringify({ transactionPostcard: 'base64编码' }, null, 2) },
  { id: 'resource-path', method: 'GET', path: '/api/rpc/resource-paths/:hash', summary: '按哈希查资源路径', group: 'RPC',
    pathParams: [{ name: 'hash', ph: 'hex 18字节' }] },
];

const SDK_EXAMPLES = [
  { id: 'acc-info', title: '查询账户信息', desc: '通过地址查询账户余额和状态',
    go: `package main

import (
    "context"
    "fmt"
    "log"
)

func main() {
    client := milon.NewClient("https://api.milon.dev")
    
    ctx := context.Background()
    address := "0x1234...abcd"
    
    account, err := client.GetAccount(ctx, address)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("账户: %s\n", account.Address)
    fmt.Printf("余额: %d\n", account.Balance)
    fmt.Printf("序号: %d\n", account.SequenceNumber)
}`,
    python: `from milon_sdk import MilonClient

client = MilonClient("https://api.milon.dev")

address = "0x1234...abcd"
account = client.get_account(address)

print(f"账户: {account.address}")
print(f"余额: {account.balance}")
print(f"序号: {account.sequence_number}")
`,
    js: `import { MilonClient } from '@milon/sdk';

const client = new MilonClient('https://api.milon.dev');

const address = '0x1234...abcd';
const account = await client.getAccount(address);

console.log('账户:', account.address);
console.log('余额:', account.balance);
console.log('序号:', account.sequenceNumber);
` },
  { id: 'tx-transfer', title: '转账交易', desc: '发起一笔代币转账交易',
    go: `package main

import (
    "context"
    "fmt"
    "log"
)

func main() {
    client := milon.NewClient("https://api.milon.dev")
    senderKey := "0x...private_key..."
    
    ctx := context.Background()
    
    tx := milon.NewTransaction().
        WithAppName("token").
        WithMethod("transfer").
        WithArgs(map[string]interface{}{
            "to":     "0x5678...efgh",
            "amount": uint64(1000000),
        }).
        WithPayer(senderKey)
    
    resp, err := client.WriteTransaction(ctx, tx)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("交易哈希: %s\n", resp.Hash)
    fmt.Printf("状态: %s\n", resp.Status)
}`,
    python: `from milon_sdk import MilonClient, Transaction

client = MilonClient("https://api.milon.dev")
sender_key = "0x...private_key..."

tx = Transaction()
tx.app_name = "token"
tx.method = "transfer"
tx.args = {
    "to": "0x5678...efgh",
    "amount": 1000000,
}
tx.set_payer(sender_key)

resp = client.write_transaction(tx)
print(f"交易哈希: {resp.hash}")
print(f"状态: {resp.status}")
`,
    js: `import { MilonClient, Transaction } from '@milon/sdk';

const client = new MilonClient('https://api.milon.dev');
const senderKey = '0x...private_key...';

const tx = new Transaction()
  .setAppName('token')
  .setMethod('transfer')
  .setArgs({
    to: '0x5678...efgh',
    amount: 1000000,
  })
  .setPayer(senderKey);

const resp = await client.writeTransaction(tx);
console.log('交易哈希:', resp.hash);
console.log('状态:', resp.status);
` },
  { id: 'contract-read', title: '读取合约视图', desc: '调用合约的 view 函数读取数据',
    go: `package main

import (
    "context"
    "fmt"
    "log"
)

func main() {
    client := milon.NewClient("https://api.milon.dev")
    
    ctx := context.Background()
    
    result, err := client.Read(ctx, &milon.ReadRequest{
        AppName:     "token",
        MethodName:  "balance_of",
        Args: map[string]interface{}{
            "owner": "0x1234...abcd",
        },
        PayerAddress: "0x1234...abcd",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("余额: %d\n", result.Value)
}`,
    python: `from milon_sdk import MilonClient

client = MilonClient("https://api.milon.dev")

result = client.read(
    app_name="token",
    method_name="balance_of",
    args={"owner": "0x1234...abcd"},
    payer_address="0x1234...abcd",
)

print(f"余额: {result.value}")
`,
    js: `import { MilonClient } from '@milon/sdk';

const client = new MilonClient('https://api.milon.dev');

const result = await client.read({
  appName: 'token',
  methodName: 'balance_of',
  args: { owner: '0x1234...abcd' },
  payerAddress: '0x1234...abcd',
});

console.log('余额:', result.value);
` },
  { id: 'gen-account', title: '生成账户', desc: '生成新的密钥对和地址',
    go: `package main

import (
    "fmt"
    "log"
    
    "github.com/milon/milon-go-sdk/crypto"
)

func main() {
    keyPair, err := crypto.GenerateKeyPair(crypto.Secp256k1)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("私钥: %s\n", keyPair.PrivateKey.Hex())
    fmt.Printf("公钥: %s\n", keyPair.PublicKey.Base58())
    fmt.Printf("地址: %s\n", keyPair.Address)
}`,
    python: `from milon_sdk.crypto import generate_key_pair, KeyType

key_pair = generate_key_pair(KeyType.SECP256K1)

print(f"私钥: {key_pair.private_key.hex()}")
print(f"公钥: {key_pair.public_key.base58()}")
print(f"地址: {key_pair.address}")
`,
    js: `import { generateKeyPair, KeyType } from '@milon/sdk/crypto';

const keyPair = generateKeyPair(KeyType.Secp256k1);

console.log('私钥:', keyPair.privateKey.hex());
console.log('公钥:', keyPair.publicKey.base58());
console.log('地址:', keyPair.address);
` },
  { id: 'wait-tx', title: '等待交易确认', desc: '提交交易并等待确认',
    go: `package main

import (
    "context"
    "fmt"
    "log"
    "time"
)

func main() {
    client := milon.NewClient("https://api.milon.dev")
    
    ctx := context.Background()
    txHash := "0x...tx_hash..."
    
    resp, err := client.WaitForTransaction(ctx, txHash, 60*time.Second)
    if err != nil {
        log.Fatal(err)
    }
    
    if resp.Success {
        fmt.Println("交易已确认!")
        fmt.Printf("区块高度: %d\n", resp.BlockHeight)
    } else {
        fmt.Printf("交易失败: %s\n", resp.ErrorMessage)
    }
}`,
    python: `from milon_sdk import MilonClient

client = MilonClient("https://api.milon.dev")

tx_hash = "0x...tx_hash..."
resp = client.wait_for_transaction(tx_hash, timeout=60)

if resp.success:
    print("交易已确认!")
    print(f"区块高度: {resp.block_height}")
else:
    print(f"交易失败: {resp.error_message}")
`,
    js: `import { MilonClient } from '@milon/sdk';

const client = new MilonClient('https://api.milon.dev');

const txHash = '0x...tx_hash...';
const resp = await client.waitForTransaction(txHash, 60000);

if (resp.success) {
  console.log('交易已确认!');
  console.log('区块高度:', resp.blockHeight);
} else {
  console.log('交易失败:', resp.errorMessage);
}
` },
];

const ERROR_CODES = [
  { code: 0, name: 'SUCCESS', desc: '请求成功', solution: '操作已成功完成，无需额外处理。' },
  { code: 400, name: 'INVALID_ARGUMENT', desc: '请求参数无效或格式错误', solution: '请检查请求参数是否完整，格式是否正确。路径参数、查询参数和请求体都需要符合 API 规范。' },
  { code: 401, name: 'UNAUTHENTICATED', desc: '未认证，缺少有效的 API 密钥', solution: '请在请求头中添加有效的 Authorization 字段，或检查 API Key 是否正确。' },
  { code: 403, name: 'PERMISSION_DENIED', desc: '权限不足，无法访问该资源', solution: '请确认您的账户是否有访问该资源的权限，或联系管理员提升权限。' },
  { code: 404, name: 'NOT_FOUND', desc: '请求的资源不存在', solution: '请检查请求的 URL 和参数是否正确，确认资源确实存在。' },
  { code: 409, name: 'CONFLICT', desc: '资源冲突，例如账户已存在', solution: '资源已存在或状态冲突，请检查数据状态后重试。' },
  { code: 429, name: 'TOO_MANY_REQUESTS', desc: '请求频率超限', solution: 'API 调用次数已达到限制，请稍后重试或升级您的套餐。' },
  { code: 500, name: 'INTERNAL_ERROR', desc: '服务器内部错误', solution: '服务器出现未知错误，请稍后重试。如果问题持续存在，请联系技术支持。' },
  { code: 503, name: 'UNAVAILABLE', desc: '服务暂时不可用', solution: '服务正在维护或暂时不可用，请稍后重试。可查看状态页了解服务状态。' },
  { code: -32000, name: 'INVALID_PARAMS', desc: 'RPC 调用参数无效', solution: '检查 RPC 方法的参数是否正确，参数类型和数量需匹配方法定义。' },
  { code: -32601, name: 'METHOD_NOT_FOUND', desc: 'RPC 方法不存在', solution: '请检查 RPC 方法名是否正确，参考 API 文档确认方法名称。' },
  { code: -32602, name: 'INVALID_PARAMS_RPC', desc: 'RPC 参数格式错误', solution: 'RPC 请求参数格式不正确，请检查参数结构和类型。' },
];

const state = {
  currentEndpoint: null,
  currentView: 'console',
  activeRespTab: 'json',
  activeLang: 'go',
  currentSdkExample: null,
  currentDocId: null,
  history: [],
  loading: false,
  lastResponse: null,
  // IDL 方法 Tab 专用状态
  idlMetadata: [],
  idlLoaded: false,
  currentIdlApp: null,
  currentIdlMethod: null,
  idlExecMode: 'simulate', // entry 方法：simulate | submit
  idlActiveRespTab: 'idl-json',
  idlLastResponse: null,
};

const MAX_HISTORY = 50;
const HISTORY_STORAGE_KEY = 'milon_api_history';

function $(id) {
  return document.getElementById(id);
}

function el(tag, attrs) {
  var node = document.createElement(tag);
  if (attrs) {
    for (var k in attrs) {
      if (k === 'class') node.className = attrs[k];
      else if (k === 'text') node.textContent = attrs[k];
      else if (k === 'html') node.innerHTML = attrs[k];
      else if (k.indexOf('on') === 0 && typeof attrs[k] === 'function')
        node.addEventListener(k.slice(2).toLowerCase(), attrs[k]);
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
  return String(s).replace(/[&<>]/g, function (c) {
    return { '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c];
  });
}

function showToast(msg, type) {
  var container = $('toastContainer');
  var toast = el('div', { class: 'toast ' + (type || 'info') });
  var icons = { success: '\u2713', error: '\u2715', info: '\u2139', warning: '\u26a0' };
  toast.appendChild(el('span', { class: 'toast-icon', text: icons[type || 'info'] || '\u2139' }));
  toast.appendChild(el('span', { text: msg }));
  container.appendChild(toast);
  requestAnimationFrame(function () {
    toast.classList.add('show');
  });
  setTimeout(function () {
    toast.classList.remove('show');
    setTimeout(function () {
      if (toast.parentNode) toast.parentNode.removeChild(toast);
    }, 300);
  }, 2800);
}

function switchView(viewName) {
  state.currentView = viewName;
  document.querySelectorAll('.nav-tab').forEach(function (tab) {
    tab.classList.toggle('active', tab.getAttribute('data-view') === viewName);
  });
  document.querySelectorAll('.view').forEach(function (view) {
    view.classList.toggle('active', view.id === 'view-' + viewName);
  });
  // 切到 IDL Tab 时懒加载元数据
  if (viewName === 'idl' && !state.idlLoaded) {
    loadIDLMetadata();
  }
}

function renderEndpoints(filter) {
  var tree = $('endpointTree');
  tree.innerHTML = '';
  var keyword = (filter || '').trim().toLowerCase();
  var groups = {};
  ENDPOINTS.forEach(function (ep) {
    if (keyword) {
      var hay = (ep.method + ' ' + ep.path + ' ' + ep.summary + ' ' + ep.group).toLowerCase();
      if (hay.indexOf(keyword) < 0) return;
    }
    if (!groups[ep.group]) groups[ep.group] = [];
    groups[ep.group].push(ep);
  });
  var groupOrder = ['网络管理', '系统', '账户', '交易', '合约', 'RPC', '水龙头', '工具'];
  var keys = groupOrder
    .filter(function (g) {
      return groups[g];
    })
    .concat(
      Object.keys(groups).filter(function (g) {
        return groupOrder.indexOf(g) < 0;
      })
    );
  if (keys.length === 0) {
    tree.appendChild(el('div', { class: 'empty-state small', text: '无匹配端点' }));
    return;
  }
  var count = 0;
  keys.forEach(function (group) {
    var gw = el('div', { class: 'endpoint-group' });
    gw.appendChild(el('div', { class: 'group-title', text: group }));
    groups[group].forEach(function (ep) {
      count++;
      gw.appendChild(
        el(
          'div',
          {
            class: 'endpoint-item' + (state.currentEndpoint && state.currentEndpoint.id === ep.id ? ' active' : ''),
            'data-id': ep.id,
            onclick: function () {
              selectEndpoint(ep.id);
            },
          },
          el('span', { class: 'endpoint-method ' + ep.method, text: ep.method }),
          el(
            'div',
            { class: 'endpoint-text' },
            el('span', { class: 'endpoint-name', text: ep.path }),
            el('span', { class: 'endpoint-desc', text: ep.summary })
          )
        )
      );
    });
    tree.appendChild(gw);
  });
  $('endpointCount').textContent = String(count);
}

function selectEndpoint(id) {
  var ep = ENDPOINTS.find(function (e) {
    return e.id === id;
  });
  if (!ep) return;
  state.currentEndpoint = ep;
  renderEndpoints($('endpointSearch').value);
  var m = $('currentMethod');
  m.textContent = ep.method;
  m.className = 'method-badge ' + ep.method;
  $('currentPath').textContent = ep.path;
  $('currentSummary').textContent = ep.summary;
  renderParams(ep);
  if (window.innerWidth <= 768) $('endpointSidebar').classList.remove('open');
}

function renderParams(ep) {
  var body = $('editorBody');
  body.innerHTML = '';
  var hasContent = false;
  if (ep.pathParams && ep.pathParams.length) {
    hasContent = true;
    var sec = el('div', { class: 'param-section' });
    sec.appendChild(el('div', { class: 'param-section-title', text: '路径参数' }));
    ep.pathParams.forEach(function (p) {
      sec.appendChild(
        el(
          'div',
          { class: 'param-row' },
          el('label', { class: 'param-label', text: ':' + p.name }),
          el('input', {
            class: 'param-input',
            'data-pkind': 'path',
            'data-pname': p.name,
            placeholder: p.ph || '',
            type: 'text',
          })
        )
      );
    });
    body.appendChild(sec);
  }
  if (ep.queryParams && ep.queryParams.length) {
    hasContent = true;
    var qs = el('div', { class: 'param-section' });
    qs.appendChild(el('div', { class: 'param-section-title', text: '查询参数' }));
    ep.queryParams.forEach(function (p) {
      qs.appendChild(
        el(
          'div',
          { class: 'param-row' },
          el('label', { class: 'param-label', text: p.name }),
          el('input', {
            class: 'param-input',
            'data-pkind': 'query',
            'data-pname': p.name,
            placeholder: p.ph || '',
            type: 'text',
          })
        )
      );
    });
    body.appendChild(qs);
  }
  if (ep.method === 'POST') {
    hasContent = true;
    var bs = el('div', { class: 'param-section' });
    bs.appendChild(
      el(
        'div',
        { class: 'body-toolbar' },
        el('div', { class: 'param-section-title', text: '请求体 (JSON)' }),
        el(
          'div',
          { class: 'body-actions' },
          el('button', { class: 'body-format-btn', text: '格式化', onclick: formatBody }),
          el('button', { class: 'body-format-btn', text: '清空', onclick: clearBody })
        )
      )
    );
    var ta = el('textarea', {
      class: 'body-editor',
      id: 'bodyEditor',
      spellcheck: 'false',
      placeholder: '{\n  // JSON 请求体\n}',
    });
    ta.value = ep.bodyTemplate || '';
    bs.appendChild(ta);
    body.appendChild(bs);
  }
  if (!hasContent) {
    body.appendChild(
      el(
        'div',
        { class: 'empty-state' },
        el('div', { class: 'empty-icon-wrapper' }, el('div', { class: 'empty-glow' }), el('span', { class: 'empty-icon', text: '\u26a1' })),
        el('h3', { text: '无需参数' }),
        el('p', { text: '该端点无需参数\n直接点击「发送请求」按钮' })
      )
    );
  }
}

function formatBody() {
  var ta = $('bodyEditor');
  if (!ta) return;
  try {
    ta.value = JSON.stringify(JSON.parse(ta.value), null, 2);
    showToast('已格式化', 'success');
  } catch (e) {
    showToast('JSON 解析失败: ' + e.message, 'error');
  }
}

function clearBody() {
  var ta = $('bodyEditor');
  if (ta) ta.value = '';
  showToast('已清空', 'info');
}

function buildRequest() {
  var ep = state.currentEndpoint;
  if (!ep) return null;
  var url = ep.path;
  document.querySelectorAll('.param-input[data-pkind="path"]').forEach(function (inp) {
    var name = inp.getAttribute('data-pname');
    var val = inp.value.trim();
    url = url.replace(':' + name, encodeURIComponent(val));
  });
  var qs = [];
  document.querySelectorAll('.param-input[data-pkind="query"]').forEach(function (inp) {
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

function buildCurl(req) {
  var origin = window.location.origin;
  var cmd = "curl -X " + req.method + " '" + origin + req.url + "'";
  if (req.body) {
    cmd += " \\\n  -H 'Content-Type: application/json'";
    cmd += " \\\n  -d '" + req.body.replace(/'/g, "'\\''") + "'";
  }
  return cmd;
}

async function sendRequest() {
  if (!state.currentEndpoint) {
    showToast('请先选择端点', 'error');
    return;
  }
  var req = buildRequest();
  if (!req) return;
  var missing = req.url.match(/:[^/?]+/);
  if (missing) {
    showToast('路径参数未填写: ' + missing[0], 'error');
    return;
  }
  state.loading = true;
  setSendLoading(true);
  showResponseLoading();
  var start = performance.now();
  try {
    var opt = { method: req.method, headers: {} };
    if (req.body) {
      opt.headers['Content-Type'] = 'application/json';
      opt.body = req.body;
    }
    var resp = await fetch(req.url, opt);
    var duration = Math.round(performance.now() - start);
    var text = await resp.text();
    var size = new Blob([text]).size;
    var data;
    try {
      data = JSON.parse(text);
    } catch (e) {
      data = text;
    }
    displayResponse(data, resp.status, duration, resp.headers, text, size);
    addToHistory(state.currentEndpoint, req, resp.status, duration, data);
  } catch (err) {
    var d2 = Math.round(performance.now() - start);
    displayError(err, d2);
    addToHistory(state.currentEndpoint, req, 0, d2, { error: String(err) });
  } finally {
    state.loading = false;
    setSendLoading(false);
  }
}

function setSendLoading(loading) {
  var btn = $('sendBtn');
  btn.disabled = loading;
  btn.querySelector('span:last-child').textContent = loading ? '发送中...' : '发送请求';
}

function showResponseLoading() {
  $('statusBadge').className = 'status-badge loading';
  $('statusBadge').textContent = '请求中';
  $('respTime').textContent = '--';
  $('respSize').textContent = '--';
  var jp = $('tab-json');
  jp.innerHTML = '';
  var overlay = el('div', { class: 'loading-overlay' });
  overlay.appendChild(el('div', { class: 'spinner' }));
  jp.appendChild(overlay);
  $('tab-headers').innerHTML = '<div class="empty-state small"><p>请求中...</p></div>';
  $('tab-curl').innerHTML = '<div class="empty-state small"><p>请求中...</p></div>';
}

function extractGasInfo(data) {
  if (!data) return null;
  if (data.gasCharged !== undefined) return data.gasCharged;
  if (data.GasCharged !== undefined) return data.GasCharged;
  if (data.receipt && data.receipt.gasCharged !== undefined) return data.receipt.gasCharged;
  if (data.receipt && data.receipt.GasCharged !== undefined) return data.receipt.GasCharged;
  return null;
}

function queryTransactionStatus(txHash) {
  if (!txHash) return;
  selectEndpoint('tx-wait');
  var hashInput = document.querySelector('.param-input[data-pkind="path"][data-pname="hash"]');
  if (hashInput) hashInput.value = String(txHash);
  showToast('已切换到「等待交易确认」并填入交易哈希', 'success');
}

function displayResponse(data, statusCode, duration, headers, rawText, size) {
  var sc = $('statusBadge');
  var statusClass = statusCode >= 200 && statusCode < 300 ? 'success' : statusCode >= 400 ? 'error' : 'warning';
  sc.className = 'status-badge ' + statusClass;
  sc.textContent = String(statusCode);
  $('respTime').textContent = String(duration);
  $('respSize').textContent = formatSize(size || rawText.length);
  var jp = $('tab-json');
  jp.innerHTML = '';
  var gasValue = extractGasInfo(data);
  if (gasValue !== null) {
    jp.appendChild(el('div', {
      class: 'gas-info-banner',
      style: 'display:flex;align-items:center;gap:8px;padding:10px 14px;margin-bottom:12px;border-radius:8px;background:linear-gradient(135deg,rgba(124,92,255,0.18),rgba(34,211,238,0.18));border:1px solid rgba(124,92,255,0.45);color:#22d3ee;font-weight:600;font-size:14px;'
    },
      el('span', { text: '⛽' }),
      el('span', { text: 'Gas 费用: ' }),
      el('span', { style: 'color:#fff;font-weight:700;', text: String(gasValue) })
    ));
  }
  if (state.currentEndpoint && state.currentEndpoint.id === 'faucet-claim' && data && typeof data === 'object' && data.txHash) {
    jp.appendChild(el('div', {
      class: 'faucet-txhash-box',
      style: 'display:flex;align-items:center;flex-wrap:wrap;gap:8px;padding:10px 14px;margin-bottom:12px;border-radius:8px;background:rgba(34,211,238,0.08);border:1px solid rgba(34,211,238,0.35);'
    },
      el('span', { style: 'color:#22d3ee;font-weight:600;', text: '领水交易哈希:' }),
      el('code', { style: 'color:#fff;background:rgba(255,255,255,0.08);padding:2px 6px;border-radius:4px;word-break:break-all;', text: String(data.txHash) }),
      el('button', {
        class: 'btn btn-primary',
        style: 'padding:4px 12px;font-size:13px;',
        onclick: function () { queryTransactionStatus(data.txHash); }
      }, el('span', { text: '查询交易状态' }))
    ));
  }
  if (typeof data === 'string') {
    jp.appendChild(el('pre', { class: 'raw-viewer', text: data || '(空响应)' }));
  } else {
    var pre = el('pre', { class: 'json-viewer' });
    pre.innerHTML = formatJSON(data);
    jp.appendChild(pre);
  }
  var hp = $('tab-headers');
  hp.innerHTML = '';
  if (headers) {
    var tbl = el(
      'table',
      { class: 'headers-table' },
      el('thead', {}, el('tr', {}, el('th', { text: 'Header' }), el('th', { text: 'Value' })))
    );
    var tb = el('tbody', {});
    var seen = {};
    headers.forEach(function (val, key) {
      var lk = key.toLowerCase();
      if (seen[lk]) return;
      seen[lk] = true;
      tb.appendChild(el('tr', {}, el('td', { text: key }), el('td', { text: val })));
    });
    tbl.appendChild(tb);
    hp.appendChild(tbl);
  } else {
    hp.appendChild(el('div', { class: 'empty-state small', text: '无响应头' }));
  }
  var cp = $('tab-curl');
  cp.innerHTML = '';
  var req = buildRequest();
  if (req) {
    var curlCmd = buildCurl(req);
    cp.appendChild(el('pre', { class: 'raw-viewer', text: curlCmd }));
  }
  state.lastResponse = { data: data, rawText: rawText, statusCode: statusCode };
}

function displayError(err, duration) {
  var sc = $('statusBadge');
  sc.className = 'status-badge error';
  sc.textContent = 'ERR';
  $('respTime').textContent = String(duration);
  $('respSize').textContent = '--';
  var msg = err && err.message ? err.message : String(err);
  $('tab-json').innerHTML = '';
  $('tab-json').appendChild(
    el(
      'div',
      { class: 'error-box', text: '请求失败: ' + msg + '\n\n请检查:\n- 后端服务是否启动\n- 网络是否可达\n- 是否存在跨域问题' }
    )
  );
  $('tab-headers').innerHTML = '<div class="empty-state small"><p>无响应头</p></div>';
  $('tab-curl').innerHTML = '<div class="empty-state small"><p>无 cURL</p></div>';
}

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

function formatJSON(obj) {
  var json = JSON.stringify(obj, null, 2);
  return escapeHTML(json)
    .replace(
      /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g,
      function (match) {
        var cls = 'json-number';
        if (/^"/.test(match)) {
          cls = /:$/.test(match) ? 'json-key' : 'json-string';
        } else if (/true|false/.test(match)) {
          cls = 'json-boolean';
        } else if (/null/.test(match)) {
          cls = 'json-null';
        }
        return '<span class="' + cls + '">' + match + '</span>';
      }
    )
    .replace(/([{}\[\],])/g, '<span class="json-punct">$1</span>');
}

function copyCurl() {
  if (!state.currentEndpoint) {
    showToast('请先选择端点', 'error');
    return;
  }
  var req = buildRequest();
  if (!req) return;
  var cmd = buildCurl(req);
  copyToClipboard(cmd, function () {
    showToast('cURL 命令已复制', 'success');
  }, function () {
    showToast('复制失败', 'error');
  });
}

function copyResponse() {
  if (!state.lastResponse) {
    showToast('暂无响应数据', 'error');
    return;
  }
  var text = typeof state.lastResponse.data === 'string'
    ? state.lastResponse.data
    : JSON.stringify(state.lastResponse.data, null, 2);
  copyToClipboard(text, function () {
    showToast('响应已复制', 'success');
  }, function () {
    showToast('复制失败', 'error');
  });
}

function downloadResponse() {
  if (!state.lastResponse) {
    showToast('暂无响应数据', 'error');
    return;
  }
  var text = state.lastResponse.rawText || '';
  var blob = new Blob([text], { type: 'application/json' });
  var url = URL.createObjectURL(blob);
  var a = document.createElement('a');
  a.href = url;
  a.download = 'response.json';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
  showToast('响应已下载', 'success');
}

function copyToClipboard(text, onSuccess, onFail) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(onSuccess, function () {
      fallbackCopy(text, onSuccess, onFail);
    });
  } else {
    fallbackCopy(text, onSuccess, onFail);
  }
}

function fallbackCopy(text, onSuccess, onFail) {
  var ta = document.createElement('textarea');
  ta.value = text;
  document.body.appendChild(ta);
  ta.select();
  try {
    document.execCommand('copy');
    onSuccess();
  } catch (e) {
    onFail();
  }
  document.body.removeChild(ta);
}

function switchRespTab(name) {
  state.activeRespTab = name;
  document.querySelectorAll('.resp-tab').forEach(function (t) {
    t.classList.toggle('active', t.getAttribute('data-tab') === name);
  });
  document.querySelectorAll('.tab-pane').forEach(function (p) {
    p.classList.toggle('active', p.id === 'tab-' + name);
  });
}

async function loadNetworks() {
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
      ['networks', 'list', 'items', 'result'].forEach(function (k) {
        if (Array.isArray(data[k])) list = data[k];
      });
    }
    if (!list.length) {
      sel.innerHTML = '<option value="">（无网络）</option>';
      return;
    }
    list.forEach(function (n) {
      var name = typeof n === 'string' ? n : n.name || n.network || n.id || JSON.stringify(n);
      sel.appendChild(el('option', { value: name, text: name }));
    });
    try {
      var cur = await fetch('/api/network/current');
      var cd = await cur.json();
      var cn = '';
      if (typeof cd === 'string') cn = cd;
      else if (cd) {
        ['network', 'name', 'current', 'id'].forEach(function (k) {
          if (!cn && cd[k]) cn = cd[k];
        });
        if (!cn && cd.data) {
          ['network', 'name', 'current', 'id'].forEach(function (k) {
            if (!cn && cd.data[k]) cn = cd.data[k];
          });
        }
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
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ network: name }),
    });
    if (resp.ok) showToast('已切换到 ' + name, 'success');
    else {
      var t = await resp.text();
      showToast('切换失败: ' + (resp.status + ' ' + t).slice(0, 80), 'error');
    }
  } catch (err) {
    showToast('切换失败: ' + (err.message || err), 'error');
  }
}

async function checkHealth() {
  var dot = $('healthDot');
  var text = $('healthText');
  dot.className = 'health-dot loading';
  text.textContent = '检查中';
  try {
    var resp = await fetch('/api/health');
    if (resp.ok) {
      dot.className = 'health-dot healthy';
      text.textContent = '服务正常';
    } else {
      dot.className = 'health-dot unhealthy';
      text.textContent = '服务异常';
    }
  } catch (err) {
    dot.className = 'health-dot unhealthy';
    text.textContent = '连接失败';
  }
}

function loadHistory() {
  try {
    var stored = localStorage.getItem(HISTORY_STORAGE_KEY);
    if (stored) {
      state.history = JSON.parse(stored) || [];
    }
  } catch (e) {
    state.history = [];
  }
  renderHistory();
}

function saveHistory() {
  try {
    localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(state.history));
  } catch (e) {}
}

function addToHistory(endpoint, req, statusCode, duration, data) {
  state.history.unshift({
    id: Date.now(),
    endpoint: { id: endpoint.id, method: endpoint.method, path: endpoint.path, summary: endpoint.summary },
    req: req,
    statusCode: statusCode,
    duration: duration,
    time: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    timestamp: Date.now(),
    respData: data,
    respSize: data ? new Blob([JSON.stringify(data)]).size : 0,
  });
  if (state.history.length > MAX_HISTORY) state.history.length = MAX_HISTORY;
  saveHistory();
  renderHistory();
}

function renderHistory() {
  var panel = $('historyList');
  $('historyCount').textContent = String(state.history.length);
  panel.innerHTML = '';
  if (state.history.length === 0) {
    panel.appendChild(
      el(
        'div',
        { class: 'empty-state small' },
        el('div', { class: 'empty-icon-wrapper small' }, el('span', { class: 'empty-icon', text: '\ud83d\udcdc' })),
        el('h4', { text: '暂无历史' }),
        el('p', { text: '发送请求后，历史记录将保存在这里' })
      )
    );
    return;
  }
  state.history.forEach(function (h) {
    var sc = h.statusCode === 0 ? 'error' : h.statusCode >= 200 && h.statusCode < 300 ? 'success' : 'error';
    panel.appendChild(
      el(
        'div',
        {
          class: 'history-item',
          'data-id': h.id,
          onclick: function () {
            reloadHistory(h.id);
          },
        },
        el(
          'div',
          { class: 'history-item-header' },
          el('span', { class: 'h-method ' + h.endpoint.method, text: h.endpoint.method }),
          el('span', { class: 'h-path', text: h.req.url }),
          el('span', { class: 'h-status ' + sc, text: h.statusCode === 0 ? 'ERR' : String(h.statusCode) })
        ),
        el(
          'div',
          { class: 'history-item-footer' },
          el('span', { text: h.time }),
          el('span', { text: h.duration + 'ms' })
        )
      )
    );
  });
}

function reloadHistory(id) {
  var h = state.history.find(function (x) {
    return x.id === id;
  });
  if (!h) return;
  closeHistoryDrawer();

  // IDL 方法的历史记录单独处理
  if (h.endpoint.id && h.endpoint.id.indexOf('idl:') === 0) {
    reloadIDLHistory(h);
    return;
  }

  selectEndpoint(h.endpoint.id);

  // 恢复路径参数与查询参数
  if (h.req && h.req.url) {
    var urlStr = h.req.url;
    var queryStart = urlStr.indexOf('?');
    var pathPart = queryStart >= 0 ? urlStr.substring(0, queryStart) : urlStr;
    var queryPart = queryStart >= 0 ? urlStr.substring(queryStart + 1) : '';

    // 恢复路径参数
    if (h.endpoint.path) {
      var pathSegs = h.endpoint.path.split('/');
      var urlSegs = pathPart.split('/');
      for (var i = 0; i < pathSegs.length; i++) {
        if (pathSegs[i].charAt(0) === ':') {
          var paramName = pathSegs[i].substring(1);
          var paramVal = urlSegs[i] ? decodeURIComponent(urlSegs[i]) : '';
          var pathInput = document.querySelector('.param-input[data-pkind="path"][data-pname="' + paramName + '"]');
          if (pathInput) pathInput.value = paramVal;
        }
      }
    }

    // 恢复查询参数
    if (queryPart) {
      queryPart.split('&').forEach(function (pair) {
        var eqIdx = pair.indexOf('=');
        var qName = eqIdx >= 0 ? decodeURIComponent(pair.substring(0, eqIdx)) : decodeURIComponent(pair);
        var qVal = eqIdx >= 0 ? decodeURIComponent(pair.substring(eqIdx + 1)) : '';
        var queryInput = document.querySelector('.param-input[data-pkind="query"][data-pname="' + qName + '"]');
        if (queryInput) queryInput.value = qVal;
      });
    }
  }

  // 恢复请求体
  if (h.req && h.req.body) {
    var ta = $('bodyEditor');
    if (ta) ta.value = h.req.body;
  }

  // 恢复响应展示
  if (h.respData !== undefined) {
    var rawText = typeof h.respData === 'string' ? h.respData : JSON.stringify(h.respData, null, 2);
    displayResponse(h.respData, h.statusCode || 0, h.duration || 0, {}, rawText, h.respSize || rawText.length);
  }

  showToast('已恢复历史请求', 'success');
}

function clearHistory() {
  if (!state.history.length) return;
  if (!confirm('确定要清空所有历史记录吗？')) return;
  state.history = [];
  saveHistory();
  renderHistory();
  showToast('历史记录已清空', 'success');
}

function openHistoryDrawer() {
  $('historyDrawer').classList.add('open');
}

function closeHistoryDrawer() {
  $('historyDrawer').classList.remove('open');
}

function toggleHistoryDrawer() {
  $('historyDrawer').classList.toggle('open');
}

function renderSdkList(filter) {
  var list = $('sdkList');
  list.innerHTML = '';
  var keyword = (filter || '').trim().toLowerCase();
  SDK_EXAMPLES.forEach(function (ex) {
    if (keyword) {
      var hay = (ex.title + ' ' + ex.desc).toLowerCase();
      if (hay.indexOf(keyword) < 0) return;
    }
    list.appendChild(
      el(
        'div',
        {
          class: 'sdk-item' + (state.currentSdkExample && state.currentSdkExample.id === ex.id ? ' active' : ''),
          onclick: function () {
            selectSdkExample(ex.id);
          },
        },
        el('div', { class: 'sdk-item-title', text: ex.title }),
        el('div', { class: 'sdk-item-desc', text: ex.desc })
      )
    );
  });
}

function selectSdkExample(id) {
  var ex = SDK_EXAMPLES.find(function (e) {
    return e.id === id;
  });
  if (!ex) return;
  state.currentSdkExample = ex;
  renderSdkList($('sdkSearch').value);
  $('sdkTitle').textContent = ex.title;
  $('sdkSummary').textContent = ex.desc;
  renderSdkCode();
}

function renderSdkCode() {
  var ex = state.currentSdkExample;
  var code = $('sdkCode');
  if (!ex) {
    code.textContent = '// 选择左侧示例查看代码';
    return;
  }
  var lang = state.activeLang;
  code.textContent = ex[lang] || '// 暂无该语言的示例';
}

function switchLang(lang) {
  state.activeLang = lang;
  document.querySelectorAll('.lang-tab').forEach(function (t) {
    t.classList.toggle('active', t.getAttribute('data-lang') === lang);
  });
  renderSdkCode();
}

function renderErrorCodes(filter) {
  var grid = $('errorsGrid');
  grid.innerHTML = '';
  var keyword = (filter || '').trim().toLowerCase();
  var filtered = ERROR_CODES.filter(function (ec) {
    if (!keyword) return true;
    var hay = (ec.code + ' ' + ec.name + ' ' + ec.desc + ' ' + ec.solution).toLowerCase();
    return hay.indexOf(keyword) >= 0;
  });
  if (!filtered.length) {
    grid.appendChild(el('div', { class: 'empty-state small', style: 'grid-column: 1/-1;', text: '无匹配的错误码' }));
    return;
  }
  filtered.forEach(function (ec) {
    grid.appendChild(
      el(
        'div',
        { class: 'error-card' },
        el(
          'div',
          { class: 'error-card-header' },
          el('span', { class: 'error-code', text: String(ec.code) }),
          el('span', { class: 'error-name', text: ec.name })
        ),
        el('p', { class: 'error-desc', text: ec.desc }),
        el(
          'div',
          { class: 'error-solution', html: '<strong>解决方法:</strong> ' + escapeHTML(ec.solution) }
        )
      )
    );
  });
}

// ===== 接口文档数据（按端点 id 索引的详细说明）=====
var API_DOCS = {
  'net-list': {
    desc: '获取所有可用网络列表，返回每个网络的名称、链 ID、RPC 地址及是否为当前网络。',
    params: [],
    response: { success: true, code: 0, message: 'ok', data: [{ name: 'devNet', chainId: 900000001, rpcUrl: 'http://...', inxUrl: '', current: true }] },
  },
  'net-current': {
    desc: '获取当前正在使用的网络配置。',
    params: [],
    response: { success: true, code: 0, message: 'ok', data: { name: 'devNet', chainId: 900000001, rpcUrl: 'http://...', inxUrl: '', current: true } },
  },
  'net-switch': {
    desc: '切换当前网络。支持 devNet、localNet。',
    params: [{ name: 'network', type: 'string', required: true, desc: '目标网络名称（devNet / localNet）' }],
    response: { success: true, code: 0, message: 'ok', data: { name: 'devNet', chainId: 900000001, current: true } },
  },
  'health': {
    desc: '健康检查，返回当前链的最新区块高度与链 ID，可用于探活。',
    params: [],
    response: { success: true, code: 0, message: 'ok', data: { chainId: 900000001, blockHeight: 655 } },
  },
  'chain-head': {
    desc: '获取当前链头信息（最新区块高度等）。',
    params: [],
    response: { success: true, code: 0, message: 'ok', data: { chainId: 900000001, blockHeight: 655 } },
  },
  'acc-info': {
    desc: '按地址查询账户信息，包括资源、余额等链上状态。',
    params: [{ name: 'address', type: 'string', required: true, desc: 'Base58 编码的账户地址', in: 'path' }],
    response: { success: true, code: 0, message: 'ok', data: { address: '2L26F...', resources: [] } },
  },
  'acc-resources': {
    desc: '查询指定账户的资源列表（按地址过滤，非全局列表）。',
    params: [{ name: 'address', type: 'string', required: true, desc: 'Base58 编码的账户地址', in: 'path' }],
    response: { success: true, code: 0, message: 'ok', data: [{ resourceId: '...', typeTag: 1 }] },
  },
  'acc-generate': {
    desc: '生成新的账户密钥对与地址。支持 4 种密钥类型：secp256k1、ed25519、bls12381、fndsa512。',
    params: [{ name: 'keyType', type: 'string', required: false, desc: '密钥类型，默认 secp256k1。可选：secp256k1 / ed25519 / bls12381 / fndsa512' }],
    response: { success: true, code: 0, message: 'ok', data: { privateKey: '0x...', publicKey: '0x...', address: '2L26F...', keyType: 'secp256k1' } },
  },
  'tx-hash': {
    desc: '按交易哈希查询交易详情。回执中包含 gasCharged 字段（实际上链 gas 费用）。',
    params: [{ name: 'hash', type: 'string', required: true, desc: '交易哈希（hex 或 base58）', in: 'path' }],
    response: { success: true, code: 0, message: 'ok', data: { stamp: 123, payer: 0, receipt: { txHash: '...', state: 1, gasCharged: 0 } } },
  },
  'tx-events': {
    desc: '查询指定交易产生的所有事件，可按 typeTag 过滤。',
    params: [
      { name: 'hash', type: 'string', required: true, desc: '交易哈希', in: 'path' },
      { name: 'typeTag', type: 'uint64', required: false, desc: '事件类型标签过滤', in: 'query' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { events: [{ blockHeight: 655, txHash: '...', eventIndex: 0, data: { typeTag: 1, value: '0x...' } }] } },
  },
  'tx-wait': {
    desc: '等待指定交易被确认。回执中包含 gasCharged 字段。',
    params: [
      { name: 'hash', type: 'string', required: true, desc: '交易哈希', in: 'path' },
      { name: 'timeoutSecs', type: 'uint64', required: false, desc: '超时时间（秒），默认 60', in: 'query' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { stamp: 123, receipt: { state: 1, gasCharged: 0 } } },
  },
  'tx-simulate': {
    desc: '底层模拟接口，直接提交 base64 编码的 postcard 交易进行模拟。返回模拟回执，包含 gasCharged。',
    params: [{ name: 'transactionPostcard', type: 'string', required: true, desc: 'Base64 编码的 postcard 序列化交易' }],
    response: { success: true, code: 0, message: 'ok', data: { gasCharged: 0, state: 1 } },
  },
  'tx-submit': {
    desc: '底层提交接口，直接提交 base64 编码的 postcard 交易到链上。',
    params: [{ name: 'transactionPostcard', type: 'string', required: true, desc: 'Base64 编码的 postcard 序列化交易' }],
    response: { success: true, code: 0, message: 'ok', data: { txHash: 'abcdef...' } },
  },
  'tx-inspect': {
    desc: '检测原始交易（不提交），解析 base64 编码的 postcard 交易，返回交易哈希、指令哈希、付款人地址与校验结果。用于调试。',
    params: [{ name: 'transactionPostcard', type: 'string', required: true, desc: 'Base64 编码的 postcard 序列化交易' }],
    response: { success: true, code: 0, message: 'ok', data: { txHash: '...', ixHashes: ['...'], payer: '2L26F...', valid: true } },
  },
  'read': {
    desc: '读取合约视图函数（单返回值）。通过 appName + methodName 定位 IDL 方法，args 为参数键值对。',
    params: [
      { name: 'appName', type: 'string', required: true, desc: '应用名称（如 token）' },
      { name: 'methodName', type: 'string', required: true, desc: '方法名（如 balance_of）' },
      { name: 'args', type: 'object', required: false, desc: '方法参数键值对' },
      { name: 'payerAddress', type: 'string', required: false, desc: '付款人地址（视图查询可省略）' },
    ],
    response: { success: true, code: 0, message: 'ok', data: ['0x...'] },
  },
  'read-multi': {
    desc: '多指令视图查询，封装 SDK 的 BuildAndViewMultiIx，一次查询返回多个值。',
    params: [{ name: 'instructions', type: 'array', required: true, desc: '指令数组，每项含 appName/methodName/args' }],
    response: { success: true, code: 0, message: 'ok', data: [['0x...'], ['0x...']] },
  },
  'simulate': {
    desc: '模拟合约调用，支持 4 种支付模式。返回模拟回执，包含 gasCharged 字段。不实际修改链上状态。',
    params: [
      { name: 'appName', type: 'string', required: true, desc: '应用名称' },
      { name: 'methodName', type: 'string', required: true, desc: '方法名' },
      { name: 'args', type: 'object', required: false, desc: '方法参数' },
      { name: 'paymentMode', type: 'string', required: true, desc: '支付模式：unified_payer_all / unified_dual_sign / unified_payer_only_gas / split' },
      { name: 'payerAddress', type: 'string', required: true, desc: '付款人地址' },
      { name: 'signatureMode', type: 'object', required: true, desc: '签名模式：{"type":"pubkey","publicKey":"<base58>"}' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { gasCharged: 0, state: 1 } },
  },
  'write': {
    desc: '写入交易（实际上链），支持 4 种支付模式。需要付款人私钥签名。',
    params: [
      { name: 'appName', type: 'string', required: true, desc: '应用名称' },
      { name: 'methodName', type: 'string', required: true, desc: '方法名' },
      { name: 'args', type: 'object', required: false, desc: '方法参数' },
      { name: 'paymentMode', type: 'string', required: true, desc: '支付模式' },
      { name: 'payerPrivateKey', type: 'string', required: true, desc: '付款人私钥（hex 或 base58）' },
      { name: 'payerAddress', type: 'string', required: true, desc: '付款人地址' },
      { name: 'signatureMode', type: 'object', required: true, desc: '签名模式' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { txHash: 'abcdef...' } },
  },
  'write-multi': {
    desc: '多方签名写入，专用于 unified_dual_sign 模式。付款人付 gas，ix 签名者签名指令。',
    params: [
      { name: 'paymentMode', type: 'string', required: true, desc: '必须为 unified_dual_sign' },
      { name: 'payerPrivateKey', type: 'string', required: true, desc: '付款人私钥' },
      { name: 'payerAddress', type: 'string', required: true, desc: '付款人地址' },
      { name: 'ixPrivateKey', type: 'string', required: true, desc: '指令签名者私钥' },
      { name: 'ixAddress', type: 'string', required: true, desc: '指令签名者地址' },
      { name: 'signatureMode', type: 'object', required: true, desc: '付款人签名模式' },
      { name: 'ixSignatureMode', type: 'object', required: true, desc: '指令签名者签名模式' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { txHash: 'abcdef...' } },
  },
  'write-multisig': {
    desc: '多签写入，专用于 split 模式。owner 自付 gas 并签名指令。',
    params: [
      { name: 'paymentMode', type: 'string', required: true, desc: '必须为 split' },
      { name: 'ownerPrivateKey', type: 'string', required: true, desc: 'owner 私钥' },
      { name: 'ownerAddress', type: 'string', required: true, desc: 'owner 地址' },
      { name: 'signatureMode', type: 'object', required: true, desc: '签名模式' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { txHash: 'abcdef...' } },
  },
  'block': {
    desc: '按区块高度获取区块信息。',
    params: [{ name: 'height', type: 'uint64', required: true, desc: '区块高度', in: 'path' }],
    response: { success: true, code: 0, message: 'ok', data: { height: 1, hash: '...' } },
  },
  'resource': {
    desc: '按资源哈希获取资源内容。',
    params: [{ name: 'hash', type: 'string', required: true, desc: '资源哈希（hex 编码，18 字节）', in: 'path' }],
    response: { success: true, code: 0, message: 'ok', data: { hash: '...', data: '0x...' } },
  },
  'access-value': {
    desc: '批量获取访问值（按 blob 哈希数组查询）。',
    params: [{ name: 'blobHashes', type: 'array', required: true, desc: 'blob 哈希数组（hex 编码，每项 32 字节）' }],
    response: { success: true, code: 0, message: 'ok', data: [{ hash: '...', value: '0x...' }] },
  },
  'resource-path': {
    desc: '按哈希查询资源路径。',
    params: [{ name: 'hash', type: 'string', required: true, desc: '资源路径哈希（hex 编码，18 字节）', in: 'path' }],
    response: { success: true, code: 0, message: 'ok', data: { hash: '...', path: '...' } },
  },
  'faucet-claim': {
    desc: '从水龙头领取代币。领水交易本身消耗 gas（sponsored 交易，gasCharged=0）。成功后返回 txHash 可用于追踪交易状态。每个地址有 24 小时冷却期。',
    params: [
      { name: 'privateKey', type: 'string', required: true, desc: '领取者私钥（hex 或 base58）' },
      { name: 'address', type: 'string', required: true, desc: '领取者地址（base58）' },
      { name: 'signatureMode', type: 'object', required: true, desc: '签名模式：{"type":"pubkey","publicKey":"<base58>"}' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { address: '2L26F...', claimed: true, txHash: 'abcdef...' } },
  },
  'faucet-balance': {
    desc: '查询指定地址的 MIL 代币余额。',
    params: [{ name: 'address', type: 'string', required: true, desc: 'Base58 编码的账户地址', in: 'path' }],
    response: { success: true, code: 0, message: 'ok', data: { address: '2L26F...', balance: 9989324 } },
  },
  'derive-addr': {
    desc: '从公钥派生账户地址。',
    params: [
      { name: 'publicKey', type: 'string', required: true, desc: '公钥（hex 或 base58）' },
      { name: 'keyType', type: 'string', required: false, desc: '密钥类型，默认自动识别' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { address: '2L26F...', publicKey: '0x...', keyType: 'secp256k1' } },
  },
  'derive-pub': {
    desc: '从私钥派生公钥，支持 4 种密钥类型。',
    params: [
      { name: 'privateKey', type: 'string', required: true, desc: '私钥（hex 或 base58）' },
      { name: 'keyType', type: 'string', required: true, desc: '密钥类型：secp256k1 / ed25519 / bls12381 / fndsa512' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { publicKey: '0x...', keyType: 'secp256k1', privateKey: '0x...' } },
  },
  'sign': {
    desc: '使用私钥对消息签名。需服务端启用 ENABLE_UTIL_SIGN=true，否则返回 403。',
    params: [
      { name: 'privateKey', type: 'string', required: true, desc: '私钥（hex 或 base58）' },
      { name: 'message', type: 'string', required: true, desc: '消息（hex 编码）' },
      { name: 'keyType', type: 'string', required: true, desc: '密钥类型' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { signature: '0x...', publicKey: '0x...' } },
  },
  'verify': {
    desc: '验证签名是否有效。',
    params: [
      { name: 'publicKey', type: 'string', required: true, desc: '公钥（hex 或 base58）' },
      { name: 'message', type: 'string', required: true, desc: '原始消息（hex 编码）' },
      { name: 'signature', type: 'string', required: true, desc: '签名（hex 编码）' },
    ],
    response: { success: true, code: 0, message: 'ok', data: { valid: true } },
  },
  'view-single': {
    desc: '底层单指令视图查询，使用预构建的 base64 postcard。',
    params: [{ name: 'transactionPostcard', type: 'string', required: true, desc: 'Base64 编码的 postcard 序列化视图交易' }],
    response: { success: true, code: 0, message: 'ok', data: ['0x...'] },
  },
  'view-multi': {
    desc: '底层多指令视图查询，使用预构建的 base64 postcard。',
    params: [{ name: 'transactionPostcard', type: 'string', required: true, desc: 'Base64 编码的 postcard 序列化视图交易' }],
    response: { success: true, code: 0, message: 'ok', data: [['0x...'], ['0x...']] },
  },
};

function renderApiDocsNav(filter) {
  var nav = $('docsNav');
  nav.innerHTML = '';
  var keyword = (filter || '').trim().toLowerCase();
  var groups = {};
  ENDPOINTS.forEach(function (ep) {
    if (keyword) {
      var hay = (ep.method + ' ' + ep.path + ' ' + ep.summary + ' ' + ep.group).toLowerCase();
      if (hay.indexOf(keyword) < 0) return;
    }
    if (!groups[ep.group]) groups[ep.group] = [];
    groups[ep.group].push(ep);
  });
  var groupOrder = ['网络管理', '系统', '账户', '交易', '合约', 'RPC', '水龙头', '工具'];
  var keys = groupOrder
    .filter(function (g) { return groups[g]; })
    .concat(Object.keys(groups).filter(function (g) { return groupOrder.indexOf(g) < 0; }));
  if (keys.length === 0) {
    nav.appendChild(el('div', { class: 'empty-state small', text: '无匹配接口' }));
    return;
  }
  keys.forEach(function (group) {
    var gw = el('div', { class: 'docs-nav-group' });
    gw.appendChild(el('div', { class: 'docs-nav-group-title', text: group }));
    groups[group].forEach(function (ep) {
      gw.appendChild(
        el(
          'div',
          {
            class: 'docs-nav-item' + (state.currentDocId === ep.id ? ' active' : ''),
            'data-id': ep.id,
            onclick: function () { selectApiDoc(ep.id); },
          },
          el('span', { class: 'endpoint-method ' + ep.method, text: ep.method }),
          el('span', { class: 'docs-nav-item-text', text: ep.summary })
        )
      );
    });
    nav.appendChild(gw);
  });
}

function selectApiDoc(id) {
  state.currentDocId = id;
  var ep = ENDPOINTS.find(function (e) { return e.id === id; });
  if (!ep) return;
  renderApiDocsNav($('docsSearch').value);
  renderApiDocDetail(ep);
}

function renderApiDocDetail(ep) {
  var doc = API_DOCS[ep.id] || { desc: ep.summary, params: [], response: {} };
  $('docsTitle').textContent = ep.summary;
  $('docsSummary').textContent = ep.method + ' ' + ep.path;

  var body = $('docsBody');
  body.innerHTML = '';

  // 路径与方法
  body.appendChild(
    el('div', { class: 'docs-path-display' },
      el('span', { class: 'docs-method-badge ' + ep.method, text: ep.method }),
      el('span', { class: 'docs-path-text', text: ep.path })
    )
  );

  // 描述
  body.appendChild(el('p', { class: 'docs-desc', text: doc.desc }));

  // 请求参数
  var paramsSection = el('div', { class: 'docs-section' });
  paramsSection.appendChild(el('div', { class: 'docs-section-title', text: '请求参数' }));
  if (doc.params && doc.params.length > 0) {
    var table = el('table', { class: 'docs-table' });
    table.appendChild(
      el('tr', {},
        el('th', { text: '参数名' }),
        el('th', { text: '位置' }),
        el('th', { text: '类型' }),
        el('th', { text: '必填' }),
        el('th', { text: '说明' })
      )
    );
    doc.params.forEach(function (p) {
      table.appendChild(
        el('tr', {},
          el('td', { text: p.name }),
          el('td', { text: p.in || 'body' }),
          el('td', { text: p.type }),
          el('td', { class: p.required ? 'required' : 'optional', text: p.required ? '是' : '否' }),
          el('td', { text: p.desc })
        )
      );
    });
    paramsSection.appendChild(table);
  } else {
    paramsSection.appendChild(el('div', { class: 'docs-empty', text: '该接口无需请求参数' }));
  }
  body.appendChild(paramsSection);

  // 请求示例
  if (ep.bodyTemplate) {
    var reqSection = el('div', { class: 'docs-section' });
    reqSection.appendChild(el('div', { class: 'docs-section-title', text: '请求体示例' }));
    reqSection.appendChild(
      el('div', { class: 'docs-code-block' },
        el('pre', { text: ep.bodyTemplate })
      )
    );
    body.appendChild(reqSection);
  }

  // curl 示例
  var curlSection = el('div', { class: 'docs-section' });
  curlSection.appendChild(el('div', { class: 'docs-section-title', text: 'cURL 示例' }));
  var curlCmd = buildDocCurl(ep);
  curlSection.appendChild(
    el('div', { class: 'docs-code-block' },
      el('pre', { text: curlCmd })
    )
  );
  body.appendChild(curlSection);

  // 响应示例
  var respSection = el('div', { class: 'docs-section' });
  respSection.appendChild(el('div', { class: 'docs-section-title', text: '响应示例' }));
  respSection.appendChild(
    el('div', { class: 'docs-code-block' },
      el('pre', { text: JSON.stringify(doc.response, null, 2) })
    )
  );
  body.appendChild(respSection);
}

function buildDocCurl(ep) {
  var base = 'http://localhost:8080';
  var url = base + ep.path;
  if (ep.method === 'GET') {
    var queryParts = (ep.queryParams || []).map(function (q) {
      return q.name + '=' + (q.ph || 'value');
    });
    if (queryParts.length > 0) url += '?' + queryParts.join('&');
    return 'curl ' + url;
  }
  var body = ep.bodyTemplate ? ep.bodyTemplate : '{}';
  return 'curl -X POST ' + url + ' \\\n  -H "Content-Type: application/json" \\\n  -d \'' + body.replace(/\n/g, '\n  ') + '\'';
}

// ==================== IDL 方法 Tab ====================

// 标量类型集合：渲染为单行输入框；其余类型（含 vec/option/map/tuple/自定义结构）渲染为 JSON 文本域
var IDL_SCALAR_TYPES = {
  'u8': '0', 'u16': '0', 'u32': '0', 'u64': '0', 'u128': '0',
  'i8': '0', 'i16': '0', 'i32': '0', 'i64': '0',
  'bool': 'false', 'boolean': 'false',
  'String': '', 'string': '',
  'Address': '', 'PublicKey': '', 'Signer': '', 'AnySigner': '',
  'B96': '', 'B144': '', 'B160': '', 'B256': '', 'Bitmap64': '', 'bytes': ''
};

// 支付模式定义：每种模式对应的额外字段（submit 模式才需要 privateKey）
var IDL_PAYMENT_MODES = [
  { value: 'unified_payer_all', label: '统一付款（payer 签全部）', needIx: false },
  { value: 'unified_payer_only_gas', label: '统一付款（仅付 gas）', needIx: false },
  { value: 'unified_dual_sign', label: '双重签名（payer + ix）', needIx: true },
  { value: 'split', label: '拆分签名（owner 自付）', needIx: false },
];

function idlIsScalarType(typeStr) {
  return Object.prototype.hasOwnProperty.call(IDL_SCALAR_TYPES, typeStr);
}

async function loadIDLMetadata() {
  var tree = $('idlTree');
  tree.innerHTML = '';
  tree.appendChild(el('div', { class: 'empty-state small', text: '加载 IDL 元数据中...' }));
  try {
    var resp = await fetch('/api/idl/metadata');
    var data = await resp.json();
    var apps = data && data.data ? data.data : [];
    state.idlMetadata = apps;
    state.idlLoaded = true;
    renderIDLAppList();
  } catch (err) {
    tree.innerHTML = '';
    tree.appendChild(el('div', { class: 'empty-state small', text: '加载失败: ' + (err.message || String(err)) }));
  }
}

function renderIDLAppList(filter) {
  var tree = $('idlTree');
  tree.innerHTML = '';
  var keyword = (filter || $('idlSearch').value || '').trim().toLowerCase();
  var totalCount = 0;
  var hasAny = false;

  state.idlMetadata.forEach(function (app) {
    var matched = app.instructions.filter(function (ix) {
      if (!keyword) return true;
      var hay = (app.name + ' ' + ix.name + ' ' + ix.handler + ' ' + ix.kind).toLowerCase();
      return hay.indexOf(keyword) >= 0;
    });
    if (matched.length === 0) return;
    hasAny = true;
    totalCount += matched.length;

    // 按 kind 分组：entry 在前，view 在后
    var entries = matched.filter(function (ix) { return ix.kind === 'entry'; });
    var views = matched.filter(function (ix) { return ix.kind === 'view'; });

    var gw = el('div', { class: 'endpoint-group idl-app-group' });
    gw.appendChild(el('div', { class: 'group-title', text: app.name + ' (app_id=' + app.appId + ')' }));

    if (entries.length) {
      gw.appendChild(el('div', { class: 'idl-kind-label', text: 'entry (' + entries.length + ')' }));
      entries.forEach(function (ix) { gw.appendChild(buildIDLMethodItem(app, ix)); });
    }
    if (views.length) {
      gw.appendChild(el('div', { class: 'idl-kind-label', text: 'view (' + views.length + ')' }));
      views.forEach(function (ix) { gw.appendChild(buildIDLMethodItem(app, ix)); });
    }
    tree.appendChild(gw);
  });

  if (!hasAny) {
    tree.appendChild(el('div', { class: 'empty-state small', text: '无匹配方法' }));
  }
  $('idlMethodCount').textContent = String(totalCount);
}

function buildIDLMethodItem(app, ix) {
  var isActive = state.currentIdlMethod &&
    state.currentIdlApp === app.name &&
    state.currentIdlMethod.name === ix.name;
  return el(
    'div',
    {
      class: 'endpoint-item idl-method-item' + (isActive ? ' active' : ''),
      onclick: function () { selectIDLMethod(app.name, ix.name); },
    },
    el('span', { class: 'idl-kind-badge ' + ix.kind, text: ix.kind }),
    el(
      'div',
      { class: 'endpoint-text' },
      el('span', { class: 'endpoint-name', text: ix.name }),
      el('span', { class: 'endpoint-desc', text: ix.handler + (ix.sponsor ? ' · sponsored' : '') })
    )
  );
}

function selectIDLMethod(appName, methodName) {
  var app = state.idlMetadata.find(function (a) { return a.name === appName; });
  if (!app) return;
  var ix = app.instructions.find(function (i) { return i.name === methodName; });
  if (!ix) return;
  state.currentIdlApp = appName;
  state.currentIdlMethod = ix;
  renderIDLAppList();
  renderIDLHeader(app, ix);
  renderIDLForm(ix);
  if (window.innerWidth <= 768) $('idlSidebar').classList.remove('open');
}

function renderIDLHeader(app, ix) {
  var badge = $('idlKindBadge');
  badge.textContent = ix.kind;
  badge.className = 'method-badge idl-kind-badge ' + ix.kind;
  $('idlMethodTitle').textContent = ix.name;
  $('idlAppLabel').textContent = app.name;
  $('idlMethodPath').textContent = '::' + ix.name + (ix.returns ? ' → ' + ix.returns.type : '');
}

function renderIDLForm(ix) {
  var body = $('idlEditorBody');
  body.innerHTML = '';

  // 方法说明区
  var infoSec = el('div', { class: 'param-section' });
  infoSec.appendChild(el('div', { class: 'param-section-title', text: '方法信息' }));
  var infoRows = [
    ['app', state.currentIdlApp],
    ['handler', ix.handler],
    ['kind', ix.kind],
    ['discriminator', String(ix.discriminator)],
  ];
  if (ix.returns) infoRows.push(['returns', ix.returns.type]);
  if (ix.sponsor) infoRows.push(['sponsor', 'true（gas 由赞助者代付）']);
  infoRows.forEach(function (r) {
    infoSec.appendChild(el('div', { class: 'idl-info-row' },
      el('span', { class: 'idl-info-key', text: r[0] }),
      el('span', { class: 'idl-info-val', text: r[1] })
    ));
  });
  body.appendChild(infoSec);

  // input 参数
  var inputArgs = ix.args.filter(function (a) { return a.role === 'input'; });
  var signerArgs = ix.args.filter(function (a) { return a.role === 'signer' || a.role === 'any_signer'; });

  if (inputArgs.length) {
    var argSec = el('div', { class: 'param-section' });
    argSec.appendChild(el('div', { class: 'param-section-title', text: '参数 (args)' }));
    inputArgs.forEach(function (a) { argSec.appendChild(buildIDLArgInput(a)); });
    body.appendChild(argSec);
  } else {
    body.appendChild(el('div', { class: 'param-section' },
      el('div', { class: 'param-section-title', text: '参数 (args)' }),
      el('div', { class: 'empty-state small', text: '该方法无 input 参数' })
    ));
  }

  // signer 参数提示
  if (signerArgs.length) {
    var hintSec = el('div', { class: 'param-section' });
    hintSec.appendChild(el('div', { class: 'param-section-title', text: '签名者参数（由支付模式提供）' }));
    signerArgs.forEach(function (a) {
      hintSec.appendChild(el('div', { class: 'idl-signer-hint' },
        el('span', { class: 'idl-arg-name', text: a.name }),
        el('span', { class: 'idl-arg-type', text: a.type }),
        el('span', { class: 'idl-signer-note', text: 'role=' + a.role + '，由下方支付模式字段提供' })
      ));
    });
    body.appendChild(hintSec);
  }

  // entry 方法：支付模式 + 执行模式
  if (ix.kind === 'entry') {
    body.appendChild(buildIDLPaymentSection());
  }

  // 默认执行模式
  state.idlExecMode = 'simulate';
}

function buildIDLArgInput(arg) {
  var isScalar = idlIsScalarType(arg.type);
  var row = el('div', { class: 'param-row idl-arg-row' },
    el('label', { class: 'param-label' },
      el('span', { class: 'idl-arg-name', text: arg.name }),
      el('span', { class: 'idl-arg-type', text: arg.type })
    )
  );
  if (isScalar) {
    var inp = el('input', {
      class: 'param-input',
      'data-argname': arg.name,
      placeholder: IDL_SCALAR_TYPES[arg.type] !== '' ? '如 ' + IDL_SCALAR_TYPES[arg.type] : arg.type,
      type: 'text',
    });
    inp.value = IDL_SCALAR_TYPES[arg.type];
    row.appendChild(inp);
  } else {
    var ta = el('textarea', {
      class: 'body-editor idl-arg-editor',
      'data-argname': arg.name,
      spellcheck: 'false',
      placeholder: 'JSON，如 ' + idlDefaultComplexValue(arg.type),
    });
    ta.value = idlDefaultComplexValue(arg.type);
    row.appendChild(ta);
  }
  return row;
}

function idlDefaultComplexValue(typeStr) {
  if (typeStr.indexOf('vec<') === 0) return '[]';
  if (typeStr.indexOf('option<') === 0) return 'null';
  if (typeStr.indexOf('map<') === 0) return '{}';
  if (typeStr.indexOf('tuple<') === 0) return '[]';
  // 自定义 struct/enum：返回空对象
  return '{}';
}

function buildIDLPaymentSection() {
  var sec = el('div', { class: 'param-section' });
  sec.appendChild(el('div', { class: 'param-section-title', text: '执行配置' }));

  // 执行模式切换
  var modeRow = el('div', { class: 'param-row idl-exec-mode-row' },
    el('label', { class: 'param-label', text: '执行模式' })
  );
  var simulateRadio = el('label', { class: 'idl-radio-label' },
    el('input', { type: 'radio', name: 'idlExecMode', value: 'simulate', checked: 'checked', onchange: function () { onIDLExecModeChange('simulate'); } }),
    el('span', { text: ' 模拟（不上链）' })
  );
  var submitRadio = el('label', { class: 'idl-radio-label' },
    el('input', { type: 'radio', name: 'idlExecMode', value: 'submit', onchange: function () { onIDLExecModeChange('submit'); } }),
    el('span', { text: ' 上链提交' })
  );
  modeRow.appendChild(simulateRadio);
  modeRow.appendChild(submitRadio);
  sec.appendChild(modeRow);

  // 支付模式选择
  var pmRow = el('div', { class: 'param-row' },
    el('label', { class: 'param-label', text: 'paymentMode' })
  );
  var pmSelect = el('select', { class: 'param-input', id: 'idlPaymentMode', onchange: renderIDLPaymentFields });
  IDL_PAYMENT_MODES.forEach(function (m) {
    pmSelect.appendChild(el('option', { value: m.value, text: m.label }));
  });
  pmRow.appendChild(pmSelect);
  sec.appendChild(pmRow);

  // 动态字段容器
  sec.appendChild(el('div', { id: 'idlPaymentFields' }));
  // 初始渲染
  setTimeout(renderIDLPaymentFields, 0);
  return sec;
}

function onIDLExecModeChange(mode) {
  state.idlExecMode = mode;
  renderIDLPaymentFields();
}

function renderIDLPaymentFields() {
  var container = $('idlPaymentFields');
  if (!container) return;
  container.innerHTML = '';
  var pm = $('idlPaymentMode') ? $('idlPaymentMode').value : 'unified_payer_all';
  var isSubmit = state.idlExecMode === 'submit';
  var modeDef = IDL_PAYMENT_MODES.find(function (m) { return m.value === pm; }) || IDL_PAYMENT_MODES[0];

  // 通用：payer/owner 地址
  container.appendChild(buildIDLFieldRow('payerAddress', '付款/所有者地址 (base58)', '', 'text', 'data-field', 'payerAddress'));

  if (pm === 'split') {
    // split 模式用 owner 概念，复用 payerAddress 作为 owner 地址
    if (isSubmit) {
      container.appendChild(buildIDLFieldRow('ownerPrivateKey', 'owner 私钥 (hex/base58)', '', 'password', 'data-field', 'ownerPrivateKey'));
    }
  } else {
    if (isSubmit) {
      container.appendChild(buildIDLFieldRow('payerPrivateKey', 'payer 私钥 (hex/base58)', '', 'password', 'data-field', 'payerPrivateKey'));
    }
  }

  // dual_sign 需要 ix 字段
  if (modeDef && modeDef.needIx) {
    container.appendChild(buildIDLFieldRow('ixAddress', 'ix 签名者地址 (base58)', '', 'text', 'data-field', 'ixAddress'));
    if (isSubmit) {
      container.appendChild(buildIDLFieldRow('ixPrivateKey', 'ix 私钥 (hex/base58)', '', 'password', 'data-field', 'ixPrivateKey'));
    }
  }

  // signatureMode（JSON）
  container.appendChild(buildIDLFieldRow('signatureMode', 'signatureMode (JSON)', '{\n  "type": "pubkey",\n  "publicKey": "base58公钥"\n}', 'textarea', 'data-field', 'signatureMode'));
  if (modeDef && modeDef.needIx) {
    container.appendChild(buildIDLFieldRow('ixSignatureMode', 'ixSignatureMode (JSON)', '{\n  "type": "pubkey",\n  "publicKey": "base58公钥"\n}', 'textarea', 'data-field', 'ixSignatureMode'));
  }
}

function buildIDLFieldRow(name, label, value, inputType, attrKey, attrVal) {
  var row = el('div', { class: 'param-row' },
    el('label', { class: 'param-label', text: label })
  );
  if (inputType === 'textarea') {
    var ta = el('textarea', { class: 'body-editor idl-field-editor', spellcheck: 'false' });
    ta.setAttribute(attrKey, attrVal);
    ta.value = value;
    row.appendChild(ta);
  } else {
    var inp = el('input', { class: 'param-input', type: inputType || 'text' });
    inp.setAttribute(attrKey, attrVal);
    inp.value = value;
    row.appendChild(inp);
  }
  return row;
}

function buildIDLRequest() {
  var ix = state.currentIdlMethod;
  if (!ix) return null;
  var appName = state.currentIdlApp;

  // 收集 input 参数
  var args = {};
  var argInputs = document.querySelectorAll('#idlEditorBody [data-argname]');
  argInputs.forEach(function (inp) {
    var name = inp.getAttribute('data-argname');
    var raw = inp.value.trim();
    if (raw === '') return;
    // 标量尝试原值，复合类型解析 JSON
    var typeStr = '';
    if (ix.args) {
      var def = ix.args.find(function (a) { return a.name === name; });
      if (def) typeStr = def.type;
    }
    if (idlIsScalarType(typeStr)) {
      // 数值与布尔尝试转换
      if (typeStr === 'bool' || typeStr === 'boolean') {
        args[name] = raw === 'true' || raw === '1';
      } else if (typeStr.indexOf('u') === 0 || typeStr.indexOf('i') === 0 || typeStr === 'Bitmap64' || typeStr === 'bytes') {
        args[name] = raw;
      } else {
        args[name] = raw;
      }
    } else {
      try { args[name] = JSON.parse(raw); }
      catch (e) { args[name] = raw; }
    }
  });

  if (ix.kind === 'view') {
    // view: POST /api/read
    return {
      method: 'POST',
      url: '/api/read',
      body: JSON.stringify({ appName: appName, methodName: ix.name, args: args }, null, 2),
    };
  }

  // entry: simulate 或 submit
  var pm = $('idlPaymentMode') ? $('idlPaymentMode').value : 'unified_payer_all';
  var isSubmit = state.idlExecMode === 'submit';
  var payload = { appName: appName, methodName: ix.name, args: args, paymentMode: pm };

  function readField(field) {
    var node = document.querySelector('#idlPaymentFields [' + 'data-field' + '="' + field + '"]');
    return node ? node.value.trim() : '';
  }

  payload.payerAddress = readField('payerAddress');

  if (pm === 'split') {
    if (isSubmit) {
      var ownerSk = readField('ownerPrivateKey');
      if (ownerSk) payload.ownerPrivateKey = ownerSk;
    }
  } else {
    if (isSubmit) {
      var payerSk = readField('payerPrivateKey');
      if (payerSk) payload.payerPrivateKey = payerSk;
    }
  }

  var modeDef = IDL_PAYMENT_MODES.find(function (m) { return m.value === pm; });
  if (modeDef && modeDef.needIx) {
    payload.ixAddress = readField('ixAddress');
    if (isSubmit) {
      var ixSk = readField('ixPrivateKey');
      if (ixSk) payload.ixPrivateKey = ixSk;
    }
    var ixSig = readField('ixSignatureMode');
    if (ixSig) { try { payload.ixSignatureMode = JSON.parse(ixSig); } catch (e) {} }
  }

  var sigMode = readField('signatureMode');
  if (sigMode) { try { payload.signatureMode = JSON.parse(sigMode); } catch (e) {} }

  var url = isSubmit ? '/api/write' : '/api/simulate';
  return { method: 'POST', url: url, body: JSON.stringify(payload, null, 2) };
}

async function sendIDLRequest() {
  if (!state.currentIdlMethod) {
    showToast('请先选择 IDL 方法', 'error');
    return;
  }
  var req = buildIDLRequest();
  if (!req) return;
  state.loading = true;
  setIDLSendLoading(true);
  showIDLResponseLoading();
  var start = performance.now();
  try {
    var opt = { method: req.method, headers: { 'Content-Type': 'application/json' }, body: req.body };
    var resp = await fetch(req.url, opt);
    var duration = Math.round(performance.now() - start);
    var text = await resp.text();
    var size = new Blob([text]).size;
    var data;
    try { data = JSON.parse(text); } catch (e) { data = text; }
    displayIDLResponse(data, resp.status, duration, resp.headers, text, size);
    addIDLToHistory(req, resp.status, duration, data);
  } catch (err) {
    var d2 = Math.round(performance.now() - start);
    displayIDLError(err, d2);
    addIDLToHistory(req, 0, d2, { error: String(err) });
  } finally {
    state.loading = false;
    setIDLSendLoading(false);
  }
}

function setIDLSendLoading(loading) {
  var btn = $('idlSendBtn');
  btn.disabled = loading;
  btn.querySelector('span:last-child').textContent = loading ? '发送中...' : '发送请求';
}

function showIDLResponseLoading() {
  var sc = $('idlStatusBadge');
  sc.className = 'status-badge loading';
  sc.textContent = '请求中';
  $('idlRespTime').textContent = '--';
  $('idlRespSize').textContent = '--';
  var jp = $('tab-idl-json');
  jp.innerHTML = '';
  var overlay = el('div', { class: 'loading-overlay' });
  overlay.appendChild(el('div', { class: 'spinner' }));
  jp.appendChild(overlay);
  $('tab-idl-headers').innerHTML = '<div class="empty-state small"><p>请求中...</p></div>';
  $('tab-idl-curl').innerHTML = '<div class="empty-state small"><p>请求中...</p></div>';
}

function displayIDLResponse(data, statusCode, duration, headers, rawText, size) {
  var sc = $('idlStatusBadge');
  var statusClass = statusCode >= 200 && statusCode < 300 ? 'success' : statusCode >= 400 ? 'error' : 'warning';
  sc.className = 'status-badge ' + statusClass;
  sc.textContent = String(statusCode);
  $('idlRespTime').textContent = String(duration);
  $('idlRespSize').textContent = formatSize(size || rawText.length);

  var jp = $('tab-idl-json');
  jp.innerHTML = '';
  // Gas 信息横幅
  var gasValue = extractGasInfo(data);
  if (gasValue !== null) {
    jp.appendChild(el('div', {
      class: 'gas-info-banner',
      style: 'display:flex;align-items:center;gap:8px;padding:10px 14px;margin-bottom:12px;border-radius:8px;background:linear-gradient(135deg,rgba(124,92,255,0.18),rgba(34,211,238,0.18));border:1px solid rgba(124,92,255,0.45);color:#22d3ee;font-weight:600;font-size:14px;'
    },
      el('span', { text: '⛽' }),
      el('span', { text: 'Gas 费用: ' }),
      el('span', { style: 'color:#fff;font-weight:700;', text: String(gasValue) })
    ));
  }
  if (typeof data === 'string') {
    jp.appendChild(el('pre', { class: 'raw-viewer', text: data || '(空响应)' }));
  } else {
    var pre = el('pre', { class: 'json-viewer' });
    pre.innerHTML = formatJSON(data);
    jp.appendChild(pre);
  }

  // Headers
  var hp = $('tab-idl-headers');
  hp.innerHTML = '';
  if (headers) {
    var tbl = el('table', { class: 'headers-table' },
      el('thead', {}, el('tr', {}, el('th', { text: 'Header' }), el('th', { text: 'Value' })))
    );
    var tb = el('tbody', {});
    var seen = {};
    headers.forEach(function (val, key) {
      var lk = key.toLowerCase();
      if (seen[lk]) return;
      seen[lk] = true;
      tb.appendChild(el('tr', {}, el('td', { text: key }), el('td', { text: val })));
    });
    tbl.appendChild(tb);
    hp.appendChild(tbl);
  } else {
    hp.appendChild(el('div', { class: 'empty-state small', text: '无响应头' }));
  }

  // cURL
  var cp = $('tab-idl-curl');
  cp.innerHTML = '';
  var req = buildIDLRequest();
  if (req) {
    cp.appendChild(el('pre', { class: 'raw-viewer', text: buildCurl(req) }));
  }

  state.idlLastResponse = { data: data, rawText: rawText, statusCode: statusCode };
}

function displayIDLError(err, duration) {
  var sc = $('idlStatusBadge');
  sc.className = 'status-badge error';
  sc.textContent = 'ERR';
  $('idlRespTime').textContent = String(duration);
  $('idlRespSize').textContent = '--';
  var msg = err && err.message ? err.message : String(err);
  $('tab-idl-json').innerHTML = '';
  $('tab-idl-json').appendChild(
    el('div', { class: 'error-box', text: '请求失败: ' + msg + '\n\n请检查:\n- 后端服务是否启动\n- 网络是否可达\n- 参数是否正确' })
  );
  $('tab-idl-headers').innerHTML = '<div class="empty-state small"><p>无响应头</p></div>';
  $('tab-idl-curl').innerHTML = '<div class="empty-state small"><p>无 cURL</p></div>';
}

function switchIDLRespTab(name) {
  state.idlActiveRespTab = name;
  // 仅在 IDL 视图内切换
  var idlView = $('view-idl');
  idlView.querySelectorAll('.resp-tab').forEach(function (t) {
    t.classList.toggle('active', t.getAttribute('data-tab') === name);
  });
  idlView.querySelectorAll('.tab-pane').forEach(function (p) {
    p.classList.toggle('active', p.id === 'tab-' + name);
  });
}

function copyIDLCurl() {
  if (!state.currentIdlMethod) { showToast('请先选择方法', 'error'); return; }
  var req = buildIDLRequest();
  if (!req) return;
  copyToClipboard(buildCurl(req), function () { showToast('cURL 命令已复制', 'success'); }, function () { showToast('复制失败', 'error'); });
}

function copyIDLResponse() {
  if (!state.idlLastResponse) { showToast('暂无响应数据', 'error'); return; }
  var text = typeof state.idlLastResponse.data === 'string'
    ? state.idlLastResponse.data
    : JSON.stringify(state.idlLastResponse.data, null, 2);
  copyToClipboard(text, function () { showToast('响应已复制', 'success'); }, function () { showToast('复制失败', 'error'); });
}

function downloadIDLResponse() {
  if (!state.idlLastResponse) { showToast('暂无响应数据', 'error'); return; }
  var text = state.idlLastResponse.rawText || '';
  var blob = new Blob([text], { type: 'application/json' });
  var url = URL.createObjectURL(blob);
  var a = document.createElement('a');
  a.href = url;
  a.download = 'idl-response.json';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
  showToast('响应已下载', 'success');
}

function resetIDLForm() {
  if (state.currentIdlMethod) {
    renderIDLForm(state.currentIdlMethod);
    showToast('参数已重置', 'success');
  }
}

// IDL 历史记录：用合成 endpoint 对象，id 以 "idl:" 前缀标识
function addIDLToHistory(req, statusCode, duration, data) {
  var ix = state.currentIdlMethod;
  var appName = state.currentIdlApp;
  var epId = 'idl:' + appName + ':' + ix.name;
  state.history.unshift({
    id: Date.now(),
    endpoint: { id: epId, method: req.method, path: req.url, summary: appName + '::' + ix.name },
    req: req,
    statusCode: statusCode,
    duration: duration,
    time: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    timestamp: Date.now(),
    respData: data,
    respSize: data ? new Blob([JSON.stringify(data)]).size : 0,
  });
  if (state.history.length > MAX_HISTORY) state.history.length = MAX_HISTORY;
  saveHistory();
  renderHistory();
}

// 从历史记录恢复 IDL 方法调用
function reloadIDLHistory(h) {
  var parts = h.endpoint.id.split(':');
  if (parts.length < 3) return;
  var appName = parts[1];
  var methodName = parts[2];
  switchView('idl');
  // 等待元数据加载后选中方法并恢复表单
  function restore() {
    if (!state.idlLoaded || !state.idlMetadata.length) {
      setTimeout(restore, 100);
      return;
    }
    selectIDLMethod(appName, methodName);
    // 恢复请求体到表单
    if (h.req && h.req.body) {
      try {
        var payload = JSON.parse(h.req.body);
        // 恢复 args
        if (payload.args) {
          Object.keys(payload.args).forEach(function (k) {
            var inp = document.querySelector('#idlEditorBody [data-argname="' + k + '"]');
            if (inp) {
              inp.value = typeof payload.args[k] === 'string' ? payload.args[k] : JSON.stringify(payload.args[k], null, 2);
            }
          });
        }
        // 恢复支付字段
        if (payload.paymentMode && $('idlPaymentMode')) {
          $('idlPaymentMode').value = payload.paymentMode;
          renderIDLPaymentFields();
          // 延迟填充动态生成的字段
          setTimeout(function () {
            ['payerAddress', 'payerPrivateKey', 'ownerPrivateKey', 'ixAddress', 'ixPrivateKey', 'signatureMode', 'ixSignatureMode'].forEach(function (f) {
              if (payload[f] !== undefined) {
                var node = document.querySelector('#idlPaymentFields [data-field="' + f + '"]');
                if (node) node.value = typeof payload[f] === 'string' ? payload[f] : JSON.stringify(payload[f], null, 2);
              }
            });
            // 恢复执行模式
            if (h.req.url === '/api/write') {
              var submitRadio = document.querySelector('input[name="idlExecMode"][value="submit"]');
              if (submitRadio) { submitRadio.checked = true; onIDLExecModeChange('submit'); }
            }
          }, 50);
        }
      } catch (e) {}
    }
    // 恢复响应展示
    if (h.respData !== undefined) {
      var rawText = typeof h.respData === 'string' ? h.respData : JSON.stringify(h.respData, null, 2);
      displayIDLResponse(h.respData, h.statusCode || 0, h.duration || 0, {}, rawText, h.respSize || rawText.length);
    }
    showToast('已恢复历史请求', 'success');
  }
  restore();
}

function initApp() {
  $('endpointCount').textContent = String(ENDPOINTS.length);
  $('docsCount').textContent = String(ENDPOINTS.length);
  renderEndpoints();
  renderApiDocsNav();
  document.querySelectorAll('.nav-tab').forEach(function (tab) {
    tab.addEventListener('click', function () {
      switchView(tab.getAttribute('data-view'));
    });
  });
  $('endpointSearch').addEventListener('input', function (e) {
    renderEndpoints(e.target.value);
  });
  $('docsSearch').addEventListener('input', function (e) {
    renderApiDocsNav(e.target.value);
  });
  $('sdkSearch').addEventListener('input', function (e) {
    renderSdkList(e.target.value);
  });
  $('errorSearch').addEventListener('input', function (e) {
    renderErrorCodes(e.target.value);
  });
  $('sendBtn').addEventListener('click', sendRequest);
  $('resetBtn').addEventListener('click', function () {
    if (state.currentEndpoint) {
      renderParams(state.currentEndpoint);
      showToast('参数已重置', 'success');
    }
  });
  $('copyCurlBtn').addEventListener('click', copyCurl);
  $('copyRespBtn').addEventListener('click', copyResponse);
  $('downloadRespBtn').addEventListener('click', downloadResponse);
  document.querySelectorAll('.resp-tab').forEach(function (t) {
    t.addEventListener('click', function () {
      // IDL 视图内的 resp-tab 单独处理
      if (t.closest('#view-idl')) {
        switchIDLRespTab(t.getAttribute('data-tab'));
      } else {
        switchRespTab(t.getAttribute('data-tab'));
      }
    });
  });
  $('refreshBtn').addEventListener('click', function () {
    loadNetworks();
    checkHealth();
  });
  $('networkSelect').addEventListener('change', function (e) {
    switchNetwork(e.target.value);
  });
  $('historyToggle').addEventListener('click', toggleHistoryDrawer);
  $('drawerOverlay').addEventListener('click', closeHistoryDrawer);
  $('clearHistoryBtn').addEventListener('click', clearHistory);
  document.querySelectorAll('.lang-tab').forEach(function (t) {
    t.addEventListener('click', function () {
      switchLang(t.getAttribute('data-lang'));
    });
  });
  // IDL Tab 事件绑定
  $('idlSearch').addEventListener('input', function (e) {
    renderIDLAppList(e.target.value);
  });
  $('idlSendBtn').addEventListener('click', sendIDLRequest);
  $('idlResetBtn').addEventListener('click', resetIDLForm);
  $('idlCopyCurlBtn').addEventListener('click', copyIDLCurl);
  $('idlCopyRespBtn').addEventListener('click', copyIDLResponse);
  $('idlDownloadRespBtn').addEventListener('click', downloadIDLResponse);
  document.addEventListener('keydown', function (e) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      if (state.currentView === 'idl') {
        sendIDLRequest();
      } else {
        sendRequest();
      }
    }
    if (e.key === 'Escape') {
      if ($('historyDrawer').classList.contains('open')) {
        closeHistoryDrawer();
      }
    }
  });
  loadHistory();
  loadNetworks();
  checkHealth();
  renderSdkList();
  renderErrorCodes();
  if (SDK_EXAMPLES.length > 0) {
    selectSdkExample(SDK_EXAMPLES[0].id);
  }
  if (ENDPOINTS.length > 0) {
    selectEndpoint(ENDPOINTS[0].id);
    selectApiDoc(ENDPOINTS[0].id);
  }
  setInterval(checkHealth, 30000);
}

document.addEventListener('DOMContentLoaded', initApp);
