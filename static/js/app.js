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
  idlCollapsedApps: {}, // appName -> true 表示折叠；默认全部折叠
  idlTypes: {},       // appName -> Map<typeName, typeDef>（struct/enum 定义缓存）
  idlConstants: {},   // appName -> Map<constName, value>（常量缓存）
  idlErrors: {},      // appName -> Map<code, errorMeta>（错误码缓存）
  // 账户管理
  accounts: [],
  currentAccountId: null,
  accountExpanded: {},   // accountId -> true 表示展开详情
  accountKeyVisible: {}, // accountId -> true 表示私钥明文显示
};

const MAX_HISTORY = 50;
const HISTORY_STORAGE_KEY = 'milon_api_history';
const ACCOUNTS_STORAGE_KEY = 'milon_accounts';
const CURRENT_ACCOUNT_KEY = 'milon_current_account';

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
    if (Array.isArray(c)) {
      for (var j = 0; j < c.length; j++) {
        var cc = c[j];
        if (cc == null) continue;
        node.appendChild(typeof cc === 'string' ? document.createTextNode(cc) : cc);
      }
    } else {
      node.appendChild(typeof c === 'string' ? document.createTextNode(c) : c);
    }
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
  var activeAcc = getCurrentAccount();
  if (ep.pathParams && ep.pathParams.length) {
    hasContent = true;
    var sec = el('div', { class: 'param-section' });
    sec.appendChild(el('div', { class: 'param-section-title', text: '路径参数' }));
    ep.pathParams.forEach(function (p) {
      var prefilled = '';
      if (activeAcc && /address/i.test(p.name)) {
        prefilled = activeAcc.address;
      }
      var input = el('input', {
        class: 'param-input',
        'data-pkind': 'path',
        'data-pname': p.name,
        placeholder: p.ph || '',
        type: 'text',
        value: prefilled,
      });
      var rowChildren = [
        el('label', { class: 'param-label', text: ':' + p.name }),
        input,
      ];
      if (/hash/i.test(p.name)) {
        rowChildren.push(makeBase58ToggleBtn(input));
      }
      sec.appendChild(
        el('div', { class: 'param-row' }, rowChildren)
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
    var bodyTpl = ep.bodyTemplate || '';
    if (activeAcc && bodyTpl) {
      if (activeAcc.address) bodyTpl = bodyTpl.split('base58地址').join(activeAcc.address);
      if (activeAcc.privateKey) bodyTpl = bodyTpl.split('hex或base58私钥').join(activeAcc.privateKey);
      if (activeAcc.publicKey) bodyTpl = bodyTpl.split('base58公钥').join(activeAcc.publicKey);
    }
    ta.value = bodyTpl;
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

// hexToBase58 将 hex 编码的字节（可带 0x 前缀）转换为 Bitcoin 标准 base58 字符串。
function hexToBase58(hexStr) {
  if (!hexStr) return '';
  var hex = String(hexStr).trim();
  if (hex.startsWith('0x') || hex.startsWith('0X')) hex = hex.slice(2);
  hex = hex.replace(/[^0-9a-fA-F]/g, '');
  if (hex.length === 0) return '';
  // 前导 0 字节 -> 前导 '1'
  var leadingZeros = 0;
  for (var i = 0; i < hex.length; i += 2) {
    if (hex.substr(i, 2) === '00') leadingZeros++;
    else break;
  }
  var digits = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
  var bytes = [];
  for (var j = 0; j < hex.length; j += 2) {
    bytes.push(parseInt(hex.substr(j, 2), 16));
  }
  var b58 = [];
  var num = BigInt('0x' + hex);
  if (num === 0n) {
    return '1'.repeat(bytes.length);
  }
  while (num > 0n) {
    var rem = num % 58n;
    num = num / 58n;
    b58.unshift(digits[Number(rem)]);
  }
  var result = '1'.repeat(leadingZeros) + b58.join('');
  return result;
}

// base58ToHex 将 base58 字符串解码回 hex（带 0x 前缀）。
function base58ToHex(b58Str) {
  if (!b58Str) return '';
  var s = String(b58Str).trim();
  var digits = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
  var val = 0n;
  for (var i = 0; i < s.length; i++) {
    var c = s[i];
    var d = digits.indexOf(c);
    if (d < 0) return ''; // 非法 base58 字符
    val = val * 58n + BigInt(d);
  }
  // 前导 '1' 表示前导 0 字节
  var leadingOnes = 0;
  for (var j = 0; j < s.length; j++) {
    if (s[j] === '1') leadingOnes++;
    else break;
  }
  if (val === 0n) {
    return '0x' + '00'.repeat(leadingOnes);
  }
  var hex = val.toString(16);
  if (hex.length % 2) hex = '0' + hex;
  return '0x' + '00'.repeat(leadingOnes) + hex;
}

// looksLikeHex 判断字符串是否像 hex（允许 0x 前缀，其余全为 hex 字符）。
function looksLikeHex(str) {
  var s = String(str).trim();
  if (s.startsWith('0x') || s.startsWith('0X')) s = s.slice(2);
  s = s.replace(/[^0-9a-fA-F]/g, '');
  return s.length > 0 && s === String(str).trim().replace(/^0x/i, '');
}

// makeBase58ToggleBtn 创建一个按钮，点击后把关联输入框的 hex/base58 互转。
function makeBase58ToggleBtn(input) {
  var btn = el('button', {
    class: 'base58-toggle-btn',
    type: 'button',
    title: 'hex ⇄ base58 互转',
    text: '⇄ base58',
  });
  btn.addEventListener('click', function () {
    var v = (input.value || '').trim();
    if (!v) {
      showToast('输入框为空', 'error');
      return;
    }
    var out;
    if (looksLikeHex(v)) {
      out = hexToBase58(v);
    } else {
      out = base58ToHex(v);
    }
    if (!out) {
      showToast('无法识别该格式（需为 hex 或 base58）', 'error');
      return;
    }
    input.value = out;
    showToast('已转换：' + out, 'success');
  });
  return btn;
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
    .replace(/([{}\[\],])/g, '<span class="json-punct">$1</span>')
    .replace(
      /(<span class="json-key">"(?:txHash|tx_hash)"<\/span>:\s*<span class="json-string">)([^<]*)(<\/span>)/g,
      function (_m, pre, val, post) {
        return pre + val + post +
          ' <button class="txhash-convert-btn" data-hash="' + val + '" title="转换为 base58">⇄ base58</button>';
      }
    );
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

// ===== 账户管理 =====
function loadAccounts() {
  try {
    var stored = localStorage.getItem(ACCOUNTS_STORAGE_KEY);
    if (stored) {
      state.accounts = JSON.parse(stored) || [];
    }
  } catch (e) { state.accounts = []; }
}

function saveAccounts() {
  try {
    localStorage.setItem(ACCOUNTS_STORAGE_KEY, JSON.stringify(state.accounts));
  } catch (e) {}
}

function loadCurrentAccount() {
  try {
    state.currentAccountId = localStorage.getItem(CURRENT_ACCOUNT_KEY) || null;
  } catch (e) { state.currentAccountId = null; }
}

function saveCurrentAccount() {
  try {
    if (state.currentAccountId) {
      localStorage.setItem(CURRENT_ACCOUNT_KEY, state.currentAccountId);
    } else {
      localStorage.removeItem(CURRENT_ACCOUNT_KEY);
    }
  } catch (e) {}
}

function getCurrentAccount() {
  if (!state.currentAccountId) return null;
  return state.accounts.find(function (a) { return a.id === state.currentAccountId; }) || null;
}

function genAccountId() {
  return 'acc_' + Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
}

function updateAccountLabel() {
  var node = $('accountLabel');
  if (!node) return;
  var acc = getCurrentAccount();
  node.textContent = acc ? acc.label : '未选择';
}

function openAccountModal() {
  renderAccountModal();
  $('accountModal').classList.add('open');
}

function closeAccountModal() {
  $('accountModal').classList.remove('open');
}

function renderAccountModal() {
  var body = $('accountModalBody');
  if (!body) return;
  body.innerHTML = '';

  // 账户列表
  var listTitle = el('div', { class: 'account-form-title', text: '已保存账户' });
  body.appendChild(listTitle);

  if (state.accounts.length === 0) {
    body.appendChild(el('div', { class: 'account-empty-hint', text: '暂无账户，在下方添加' }));
  } else {
    var list = el('div', { class: 'account-list' });
    state.accounts.forEach(function (acc) {
      var isActive = acc.id === state.currentAccountId;
      var isExpanded = !!state.accountExpanded[acc.id];
      var rowWrap = el('div', { class: 'account-row-wrap' });
      var row = el('div', { class: 'account-row' + (isActive ? ' active' : '') });

      // 展开/折叠切换按钮
      var toggleBtn = el('button', {
        class: 'account-expand-btn' + (isExpanded ? ' open' : ''),
        type: 'button',
        title: isExpanded ? '收起详情' : '展开私钥/公钥',
        text: isExpanded ? '▾' : '▸',
      });
      toggleBtn.addEventListener('click', function () {
        state.accountExpanded[acc.id] = !state.accountExpanded[acc.id];
        renderAccountModal();
      });
      row.appendChild(toggleBtn);

      var info = el('div', { class: 'account-info' },
        el('div', { class: 'account-name', text: acc.label }),
        el('div', { class: 'account-addr', text: acc.address })
      );
      var actions = el('div', { class: 'account-actions' });
      if (isActive) {
        actions.appendChild(el('span', { class: 'account-badge', text: '活跃' }));
      } else {
        var setActiveBtn = el('button', { class: 'btn btn-secondary btn-sm', text: '设为活跃' });
        setActiveBtn.addEventListener('click', function () { setCurrentAccount(acc.id); });
        actions.appendChild(setActiveBtn);
      }
      var delBtn = el('button', { class: 'btn btn-ghost btn-sm', text: '删除' });
      delBtn.addEventListener('click', function () {
        if (confirm('确认删除账户 "' + acc.label + '"？')) {
          removeAccount(acc.id);
        }
      });
      actions.appendChild(delBtn);
      row.appendChild(info);
      row.appendChild(actions);
      rowWrap.appendChild(row);

      // 展开详情面板：私钥（遮蔽+眼睛切换）+ 公钥（明文）
      if (isExpanded) {
        var detail = el('div', { class: 'account-detail' });

        // 私钥行
        var skVisible = !!state.accountKeyVisible[acc.id];
        detail.appendChild(buildAccountKeyRow(
          '私钥',
          acc.privateKey,
          true,
          skVisible,
          function (next) { state.accountKeyVisible[acc.id] = next; }
        ));

        // 公钥行
        detail.appendChild(buildAccountKeyRow('公钥', acc.publicKey || '', false, true, null));

        // 地址行（可复制）
        detail.appendChild(buildAccountKeyRow('地址', acc.address, false, true, null));

        rowWrap.appendChild(detail);
      }

      list.appendChild(rowWrap);
    });
    body.appendChild(list);
  }

  // 批量导入区
  var importSection = el('div', { class: 'account-import-section' });
  importSection.appendChild(el('div', { class: 'account-form-title', text: '批量导入（粘贴 JSON 数组）' }));

  var importHint = el('div', { class: 'account-import-hint', text: '每项格式：{"address":"...","privateKey":"..."}，label 自动生成，同地址覆盖旧账户' });
  importSection.appendChild(importHint);

  var importTa = el('textarea', {
    class: 'body-editor account-import-ta',
    spellcheck: 'false',
    placeholder: '[\n  {"address":"2Gqw...","privateKey":"..."},\n  {"address":"...","privateKey":"..."}\n]'
  });
  importSection.appendChild(importTa);

  var importActions = el('div', { class: 'account-import-actions' });
  var importBtn = el('button', { class: 'btn btn-primary btn-sm', text: '导入' });
  importBtn.addEventListener('click', function () {
    var text = importTa.value.trim();
    if (!text) { showToast('请先粘贴 JSON 数组文本', 'warning'); return; }
    var r = importAccounts(text);
    if (!r) return;
    if (r.added === 0 && r.updated === 0 && r.skipped === 0) {
      showToast('导入完成：无可导入的有效账户', 'warning');
      return;
    }
    saveAccounts();
    importTa.value = '';
    var parts = [];
    if (r.added > 0) parts.push('新增 ' + r.added + ' 个');
    if (r.updated > 0) parts.push('覆盖 ' + r.updated + ' 个');
    if (r.skipped > 0) parts.push('跳过 ' + r.skipped + ' 个非法项');
    showToast('导入完成：' + parts.join('，'), 'success');
    renderAccountModal();
  });
  importActions.appendChild(importBtn);
  importSection.appendChild(importActions);
  body.appendChild(importSection);

  // 新增表单
  var form = el('div', { class: 'account-form' });
  form.appendChild(el('div', { class: 'account-form-title', text: '添加账户' }));

  var labelRow = el('div', { class: 'param-row' },
    el('label', { class: 'param-label', text: '标签' })
  );
  var labelInput = el('input', { class: 'param-input', type: 'text', placeholder: '如：测试账户1' });
  labelRow.appendChild(labelInput);
  form.appendChild(labelRow);

  var addrRow = el('div', { class: 'param-row' },
    el('label', { class: 'param-label', text: '地址 (base58)' })
  );
  var addrInput = el('input', { class: 'param-input', type: 'text', placeholder: '2Gqw...' });
  addrRow.appendChild(addrInput);
  form.appendChild(addrRow);

  var skRow = el('div', { class: 'param-row' },
    el('label', { class: 'param-label', text: '私钥 (hex/base58)' })
  );
  var skInput = el('input', { class: 'param-input', type: 'password', placeholder: 'hex 或 base58' });
  skRow.appendChild(skInput);
  form.appendChild(skRow);

  var pkRow = el('div', { class: 'param-row' },
    el('label', { class: 'param-label', text: '公钥 (可选)' })
  );
  var pkInput = el('input', { class: 'param-input', type: 'text', placeholder: 'base58 公钥' });
  pkRow.appendChild(pkInput);
  form.appendChild(pkRow);

  var actions = el('div', { class: 'account-form-actions' });
  var genBtn = el('button', { class: 'btn btn-secondary btn-sm', text: '调用生成接口' });
  genBtn.addEventListener('click', function () {
    genBtn.disabled = true;
    genBtn.textContent = '生成中...';
    fetch('/api/accounts/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ keyType: 'secp256k1' })
    }).then(function (r) { return r.json(); }).then(function (resp) {
      genBtn.disabled = false;
      genBtn.textContent = '调用生成接口';
      if (resp && resp.success && resp.data) {
        var d = resp.data;
        if (d.address) addrInput.value = d.address;
        if (d.privateKey) skInput.value = d.privateKey;
        if (d.publicKey) pkInput.value = d.publicKey;
        if (!labelInput.value) labelInput.value = '生成账户 ' + new Date().toLocaleTimeString();
        showToast('账户已生成，确认后请点保存', 'success');
      } else {
        showToast('生成失败：' + (resp && resp.message ? resp.message : '未知错误'), 'error');
      }
    }).catch(function (err) {
      genBtn.disabled = false;
      genBtn.textContent = '调用生成接口';
      showToast('请求失败：' + err.message, 'error');
    });
  });
  actions.appendChild(genBtn);

  var saveBtn = el('button', { class: 'btn btn-primary btn-sm', text: '保存账户' });
  saveBtn.addEventListener('click', function () {
    var label = labelInput.value.trim();
    var address = addrInput.value.trim();
    var privateKey = skInput.value.replace(/\s/g, '');
    var publicKey = pkInput.value.trim();
    if (!label) { showToast('请填写标签', 'warning'); return; }
    if (!address) { showToast('请填写地址', 'warning'); return; }
    if (!privateKey) { showToast('请填写私钥', 'warning'); return; }
    addAccount({ label: label, address: address, privateKey: privateKey, publicKey: publicKey });
  });
  actions.appendChild(saveBtn);
  form.appendChild(actions);

  body.appendChild(form);
}

// 构建账户详情中的一行密钥信息（label + value + 可选眼睛切换 + 复制按钮）
// mask: 是否默认遮蔽；visible: 初始是否明文；onToggle: 眼睛切换回调（可选）
function buildAccountKeyRow(label, value, mask, visible, onToggle) {
  var row = el('div', { class: 'account-key-row' });
  row.appendChild(el('span', { class: 'account-key-label', text: label }));

  var valText = value || '';
  var masked = mask && !visible;
  var displayText = masked ? '••••••••••••••••••••' : valText;

  var valEl = el('span', {
    class: 'account-key-value' + (masked ? ' masked' : ''),
    text: displayText,
    title: masked ? '点击眼睛图标显示明文' : valText,
  });
  row.appendChild(valEl);

  if (mask) {
    var eyeBtn = el('button', {
      class: 'account-eye-btn',
      type: 'button',
      title: masked ? '显示明文' : '隐藏明文',
      text: masked ? '👁' : '🙈',
    });
    eyeBtn.addEventListener('click', function () {
      var next = !visible;
      if (onToggle) onToggle(next);
      // 局部更新当前行的显示，避免整表重绘
      valEl.textContent = next ? valText : '••••••••••••••••••••';
      valEl.classList.toggle('masked', !next);
      valEl.title = next ? valText : '点击眼睛图标显示明文';
      eyeBtn.textContent = next ? '🙈' : '👁';
      eyeBtn.title = next ? '隐藏明文' : '显示明文';
      visible = next;
    });
    row.appendChild(eyeBtn);
  }

  var copyBtn = el('button', {
    class: 'account-copy-btn',
    type: 'button',
    title: '复制' + label,
    text: '复制',
  });
  copyBtn.addEventListener('click', function () {
    if (!valText) { showToast(label + '为空', 'warning'); return; }
    copyToClipboard(valText,
      function () { showToast(label + '已复制', 'success'); },
      function () { showToast('复制失败', 'error'); });
  });
  row.appendChild(copyBtn);

  return row;
}

function addAccount(data) {
  var acc = {
    id: genAccountId(),
    label: data.label,
    address: data.address,
    privateKey: data.privateKey,
    publicKey: data.publicKey || '',
    createdAt: Date.now()
  };
  state.accounts.push(acc);
  saveAccounts();
  showToast('账户已保存', 'success');
  renderAccountModal();
}

// 计算现有账户中「导入账户N」的最大序号，返回下一个可用序号。
function nextImportLabelSeq() {
  var max = 0;
  state.accounts.forEach(function (a) {
    var m = /^导入账户(\d+)$/.exec(a.label);
    if (m) {
      var n = parseInt(m[1], 10);
      if (n > max) max = n;
    }
  });
  return max + 1;
}

// 粘贴 JSON 文本批量导入账户。
// 每项仅需 {address, privateKey}；label 自动生成，publicKey 留空；同地址覆盖旧账户。
// 返回 {added, updated, skipped}；解析失败或非数组返回 null。
function importAccounts(jsonText) {
  var arr;
  try {
    arr = JSON.parse(jsonText);
  } catch (e) {
    showToast('导入失败：JSON 解析错误', 'error');
    return null;
  }
  if (!Array.isArray(arr)) {
    showToast('导入失败：请粘贴一个 JSON 数组（如 [{"address":"...","privateKey":"..."}]）', 'error');
    return null;
  }

  var result = { added: 0, updated: 0, skipped: 0 };
  var labelSeq = nextImportLabelSeq();

  arr.forEach(function (item) {
    if (!item || typeof item !== 'object') { result.skipped++; return; }
    var address = String(item.address || '').trim();
    var privateKey = String(item.privateKey || '').replace(/\s/g, '');
    if (!address || !privateKey) { result.skipped++; return; }

    var label = '导入账户' + labelSeq;
    labelSeq++;

    var existing = null;
    for (var i = 0; i < state.accounts.length; i++) {
      if (state.accounts[i].address === address) { existing = state.accounts[i]; break; }
    }

    if (existing) {
      // 同地址覆盖：保留原 id 与 createdAt，更新 label/address/privateKey，publicKey 清空
      existing.label = label;
      existing.address = address;
      existing.privateKey = privateKey;
      existing.publicKey = '';
      result.updated++;
    } else {
      state.accounts.push({
        id: genAccountId(),
        label: label,
        address: address,
        privateKey: privateKey,
        publicKey: '',
        createdAt: Date.now()
      });
      result.added++;
    }
  });

  return result;
}

function removeAccount(id) {
  state.accounts = state.accounts.filter(function (a) { return a.id !== id; });
  if (state.currentAccountId === id) {
    state.currentAccountId = null;
    saveCurrentAccount();
    updateAccountLabel();
  }
  saveAccounts();
  showToast('账户已删除', 'info');
  renderAccountModal();
}

function setCurrentAccount(id) {
  state.currentAccountId = id;
  saveCurrentAccount();
  updateAccountLabel();
  closeAccountModal();
  showToast('已设为活跃账户', 'success');
  // 若当前在 IDL 视图，重新预填执行配置（signatureMode / payer 地址等带入新账户）
  if (state.view === 'idl' && state.currentIdlMethod) {
    renderIDLPaymentFields();
  }
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
  { value: 'multi_signer', label: '多 signer（bit0 多签）', needIx: false, needSigners: true },
  { value: 'sponsored', label: '赞助交易（sponsor pool 付 gas）', needIx: false },
];

// IDL 方法实例参数映射表：键为 appName.MethodName，值为该方法的 args 示例对象
// 数据来源：D:\pprojiect\Auto_test_new\test_cases\milon\ 下 6 个模块的测试用例
var IDL_EXAMPLE_ARGS = {
  // ==================== token 模块（app_id=2，30 个方法）====================
  'token.Create': {
    token: 'M11on1111111111111111111111',
    owner: '48A2Th5n4LoQ5LuwzxF7T27VYDZU',
    metadata: { name: 'IndependentToken', symbol: 'IND', decimals: 6, icon: 'https://example.com/ind.png' }
  },
  'token.AbandonOwner': { token: 'M11on1111111111111111111111' },
  'token.TransferOwner': { token: 'M11on1111111111111111111111', to: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'token.AbandonFreezer': { token: 'M11on1111111111111111111111' },
  'token.TransferFreezer': { token: 'M11on1111111111111111111111', to: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'token.Mint': { token: 'M11on1111111111111111111111', to: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', amount: 1000000000 },
  'token.MintBatch': {
    token: 'M11on1111111111111111111111',
    to: ['2T2u6f4znq3ps3XvBPQYUtNH4DKx', '3tamDhFSgAdAAFZP7pwoCpNAZzFH'],
    amount: [500000000, 300000000]
  },
  'token.Burn': { holder: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', token: 'M11on1111111111111111111111', amount: 1000000 },
  'token.Transfer': { from: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', token: 'M11on1111111111111111111111', to: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', amount: 500000000 },
  'token.TransferWithTag': { from: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', token: 'M11on1111111111111111111111', to: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', amount: 500000000, _tag: 1001 },
  'token.TransferBatch': {
    from: '2T2u6f4znq3ps3XvBPQYUtNH4DKx',
    token: 'M11on1111111111111111111111',
    to: ['2T2u6f4znq3ps3XvBPQYUtNH4DKx', '3tamDhFSgAdAAFZP7pwoCpNAZzFH'],
    amount: [1000000, 2000000]
  },
  'token.Freeze': { token: 'M11on1111111111111111111111', holder: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', amount: 1000000 },
  'token.Unfreeze': { token: 'M11on1111111111111111111111', holder: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', amount: 500000 },
  'token.Approve': { owner: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', token: 'M11on1111111111111111111111', spender: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', amount: 1000000000 },
  'token.Revoke': { owner: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', token: 'M11on1111111111111111111111', spender: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'token.TransferFrom': { spender: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', token: 'M11on1111111111111111111111', from: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', amount: 500000000 },
  'token.SetIcon': { token: 'M11on1111111111111111111111', icon_url: 'https://example.com/new_icon.png' },
  'token.CreateWithCompliance': {
    token: 'M11on1111111111111111111111',
    owner: '48A2Th5n4LoQ5LuwzxF7T27VYDZU',
    metadata: { name: 'ComplianceToken', symbol: 'CMP', decimals: 6, icon: 'https://example.com/cmp.png' },
    credential_id: 'schema_id_xxx'
  },
  'token.SetComplianceMode': { token: 'M11on1111111111111111111111', mode: 'Any' },
  'token.AddComplianceRequirement': { token: 'M11on1111111111111111111111', credential_id: 'schema_id_xxx' },
  'token.RemoveComplianceRequirement': { token: 'M11on1111111111111111111111', credential_id: 'schema_id_xxx' },
  'token.ClearComplianceRequirements': { token: 'M11on1111111111111111111111' },
  'token.ClaimFaucet': { claimer: 'gKzpjpfWVvwgDs26DTCFFA9eRxb' },
  'token.BalanceOf': { token: 'M11on1111111111111111111111', account: '2T2u6f4znq3ps3XvBPQYUtNH4DKx' },
  'token.FrozenOf': { token: 'M11on1111111111111111111111', account: '2T2u6f4znq3ps3XvBPQYUtNH4DKx' },
  'token.ApprovalOf': { token: 'M11on1111111111111111111111', owner: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', spender: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'token.TotalSupply': { token: 'M11on1111111111111111111111' },
  'token.Metadata': { token: 'M11on1111111111111111111111' },
  'token.Compliance': { token: 'M11on1111111111111111111111' },
  'token.FaucetCooldownRemaining': { account: '48A2Th5n4LoQ5LuwzxF7T27VYDZU' },

  // ==================== account 模块（app_id=1，15 个方法）====================
  'account.Create': { owner_pk: '0x03a34b2f5d9c0e1a8e4f2b1d6a3c5e7d9f0b2a4c6e8d0f1a3c5e7d9f0b2a4c6e' },
  'account.EnsureAccount': { owner_pk: '0x03a34b2f5d9c0e1a8e4f2b1d6a3c5e7d9f0b2a4c6e8d0f1a3c5e7d9f0b2a4c6e' },
  'account.CreateMultisig': {
    owner: '0x1234567890abcdef1234567890abcdef12345678',
    signers: ['0x03a34b2f5d9c0e1a8e4f2b1d6a3c5e7d9f0b2a4c6e8d0f1a3c5e7d9f0b2a4c6e', '0x03b45c3f6e0d1f2b9e5a3c7b8d4e6f0a2c4b6d8e0f2a4c6b8d0e2f4a6c8b0d2e4f'],
    weights: [1, 1],
    threshold: 2
  },
  'account.AddSigner': { owner: '0x1234567890abcdef1234567890abcdef12345678', signer_pk: '0x03a34b2f5d9c0e1a8e4f2b1d6a3c5e7d9f0b2a4c6e8d0f1a3c5e7d9f0b2a4c6e', weight: 1 },
  'account.AddSigners': {
    owner: '0x1234567890abcdef1234567890abcdef12345678',
    signers: ['0x03a34b2f5d9c0e1a8e4f2b1d6a3c5e7d9f0b2a4c6e8d0f1a3c5e7d9f0b2a4c6e', '0x03c56d4f7e1f2a3b0c4d6e8f0a2b4c6d8e0f2a4b6c8d0e2f4a6b8c0d2e4f6a0b2c'],
    weights: [1, 1],
    threshold: 2
  },
  'account.RemoveSigner': { owner: '0x1234567890abcdef1234567890abcdef12345678', index: 2, threshold: 2 },
  'account.SetThreshold': { owner: '0x1234567890abcdef1234567890abcdef12345678', threshold: 3 },
  'account.SetSignerWeight': { owner: '0x1234567890abcdef1234567890abcdef12345678', index: 2, weight: 2 },
  'account.VoteInit': {
    owner: '0x1234567890abcdef1234567890abcdef12345678',
    intent_hash: '0x0000000000000000000000000000000000000000000000000000000000000001',
    proposal: { instructions: ['00'], auth_bit: '1' }
  },
  'account.Vote': { owner: '0x1234567890abcdef1234567890abcdef12345678', intent_hash: '0x0000000000000000000000000000000000000000000000000000000000000001' },
  'account.GetAccount': { owner: '0x1234567890abcdef1234567890abcdef12345678' },
  'account.ListSigners': { owner: '0x1234567890abcdef1234567890abcdef12345678' },
  'account.ResolveSigners': { owner: '0x1234567890abcdef1234567890abcdef12345678', sig_bit: '1', policy: { min_signers: 1 } },
  'account.GetVote': { owner: '0x1234567890abcdef1234567890abcdef12345678', intent_hash: '0x0000000000000000000000000000000000000000000000000000000000000001' },
  'account.ListActiveVotes': { owner: '0x1234567890abcdef1234567890abcdef12345678' },

  // ==================== demo 模块（app_id=255，18 个方法：10 entry + 8 view）====================
  'demo.OpenOrder': { operator: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19', order_id: 'order-a1b2c3d4e5f6' },
  'demo.PayOrder': { payer: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19', order_id: 'order-a1b2c3d4e5f6', token: 'M11on1111111111111111111111', amount: 1000 },
  'demo.SettleOrder': { operator: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19', order_id: 'order-a1b2c3d4e5f6', token: 'M11on1111111111111111111111', to: 'SYsZAfKGTthBZBLXrZSXVWvSgLo', amount: 400 },
  'demo.OrderBalance': { order_id: 'order-a1b2c3d4e5f6', token: 'M11on1111111111111111111111' },
  'demo.OpenGasSponsorPool': { pool: '3v2p43BUhFjE9fwouSsgcaySL9ej' },
  'demo.ClaimSponsoredScore': { claimer: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19', pool: '3v2p43BUhFjE9fwouSsgcaySL9ej', sponsor_seed: 42, amount: 500 },
  'demo.SponsorPoolOf': { sponsor_seed: 42 },
  'demo.InitPool': { pool: '3v2p43BUhFjE9fwouSsgcaySL9ej', label: 'demo-pool-label' },
  'demo.InitDex': { dex: '3v2p43BUhFjE9fwouSsgcaySL9ej', label: 'demo-dex-label' },
  'demo.SetLabel': { pool: '3v2p43BUhFjE9fwouSsgcaySL9ej', label: 'updated-pool-label' },
  'demo.BatchCredit': { pool: '3v2p43BUhFjE9fwouSsgcaySL9ej', recipients: ['SYsZAfKGTthBZBLXrZSXVWvSgLo'], amount: 1000 },
  'demo.LabelOf': { pool: '3v2p43BUhFjE9fwouSsgcaySL9ej' },
  'demo.ScoreOf': { pool: '3v2p43BUhFjE9fwouSsgcaySL9ej', account: 'SYsZAfKGTthBZBLXrZSXVWvSgLo' },
  'demo.SetTierCap': { pool: '3v2p43BUhFjE9fwouSsgcaySL9ej', tier: 1, cap: 1000 },
  'demo.TierCapOf': { pool: '3v2p43BUhFjE9fwouSsgcaySL9ej', tier: 1 },
  'demo.EchoMode': { mode: { variant: 'Two', value: { val: 32 } } },
  'demo.LabelTotal': { labels: { alpha: 10, beta: 20, gamma: 30 } },
  'demo.SpecialTypes': {
    mode: { variant: 'Two', value: { val: 32 } },
    maybe_note: 'optional-note-string',
    tags: ['tag1', 'tag2', 'tag3'],
    labels: { alpha: 10, beta: 20 },
    pair: [1, 2]
  },

  // ==================== identity 模块（app_id=4，39 个方法）====================
  'identity.Create': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    doc: {
      subject_type: 'Personal',
      keys: [{ public_key: '0x02a3b4c5d6e7f8091a2b3c4d5e6f7081920a3b4c5d6e7f8091a2b3c4d5e6f70819', label: null }],
      services: [],
      avatar_uri: 'https://milon.test/avatar.png'
    }
  },
  'identity.CreateWithAlias': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    doc: {
      subject_type: 'Personal',
      keys: [{ public_key: '0x02a3b4c5d6e7f8091a2b3c4d5e6f7081920a3b4c5d6e7f8091a2b3c4d5e6f70819', label: null }],
      services: [],
      avatar_uri: 'https://milon.test/avatar.png'
    },
    name: { alias: 'newuser3847', suffix: 2002 }
  },
  'identity.AddKey': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    input: { public_key: '0x02a3b4c5d6e7f8091a2b3c4d5e6f7081920a3b4c5d6e7f8091a2b3c4d5e6f70819', label: 'test-key-1' }
  },
  'identity.UpdateKey': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    id: 1,
    input: { public_key: '0x02a3b4c5d6e7f8091a2b3c4d5e6f7081920a3b4c5d6e7f8091a2b3c4d5e6f70819', label: 'test-key-1-updated' }
  },
  'identity.RemoveKey': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz', id: 1 },
  'identity.AddService': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    input: { label: 'service-1', service_endpoint: 'https://milon.test/service/hub' }
  },
  'identity.UpdateService': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    id: 0,
    input: { label: 'service-1-updated', service_endpoint: 'https://milon.test/service/vault' }
  },
  'identity.RemoveService': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz', id: 0 },
  'identity.SetAvatarUri': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz', avatar_uri: 'https://milon.test/avatar-v2.png' },
  'identity.Deactivate': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.SetAlias': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    name: { alias: 'testuser3847', suffix: 1001 }
  },
  'identity.RegisterOrganization': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    roles: ['VcIssuer'],
    credential_schemas: ['OrgSchemaV1']
  },
  'identity.UpdateOrganizationCapabilities': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    roles: ['VcIssuer', 'KycProvider'],
    credential_schemas: ['OrgSchemaV1', 'OrgSchemaV2']
  },
  'identity.DeactivateOrganization': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.DiscloseVcAttestation': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    issuer: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19',
    issuer_key_id: 0,
    credential_schema: 'TestSchemaV1',
    credential_hash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    valid_until_ms: 1893456000000,
    issuer_signature: '0x9a8b7c6d5e4f30291a2b3c4d5e6f7081920a3b4c5d6e7f8091a2b3c4d5e6f7081a2b3c4d5e6f7081920a3b4c5d6e7f8091a2b3c4d5e6f7081920a3b4c5d6e7f8091'
  },
  'identity.RemoveVcDisclosure': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    issuer: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19',
    credential_schema: 'TestSchemaV1'
  },
  'identity.RevokeVcAttestation': {
    issuer: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19',
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    credential_schema: 'TestSchemaV1'
  },
  'identity.Core': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.Document': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.KeyIndex': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.Keys': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.Key': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz', id: 1 },
  'identity.ServiceIndex': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.Services': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.Service': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz', id: 0 },
  'identity.Alias': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.Avatar': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.UpdatedAt': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.Deactivated': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.NameBinding': { name: { alias: 'testuser', suffix: 1001 } },
  'identity.CredentialDefinition': { credential_id: 'TestSchemaV1' },
  'identity.OrganizationCapabilities': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.OrganizationStatus': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.OrganizationUpdatedAt': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz' },
  'identity.AcceptedVcIssuerIndexMeta': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz', credential_schema: 'TestSchemaV1' },
  'identity.AcceptedVcIssuers': { subject: '2cb9FUuEobGuwF9wukU59ytKNdjz', credential_schema: 'TestSchemaV1' },
  'identity.HasValidVcFromIssuer': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    issuer: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19',
    credential_schema: 'TestSchemaV1',
    now_ms: 1723382400000
  },
  'identity.VcAttestationCore': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    credential_schema: 'TestSchemaV1',
    issuer: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19'
  },
  'identity.VcAttestationLifecycle': {
    subject: '2cb9FUuEobGuwF9wukU59ytKNdjz',
    credential_schema: 'TestSchemaV1',
    issuer: '2MKpJ2Zzi8Fetx7t3TFi2jNGvv19'
  },

  // ==================== nft 模块（app_id=5，20 个方法）====================
  'nft.CreateCollection': {
    collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx',
    owner: '48A2Th5n4LoQ5LuwzxF7T27VYDZU',
    metadata: { name: 'TestNFTCollection', symbol: 'TNFT', base_uri: 'https://milon.test/nft/' }
  },
  'nft.SetCollectionMetadata': {
    collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx',
    metadata: { name: 'UpdatedCollectionName', symbol: 'UPD', base_uri: 'https://milon.test/nft/v2/' }
  },
  'nft.SetMetadata': {
    collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx',
    mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH',
    metadata: {
      name: 'TestNFT #1', symbol: 'TNFT1', uri: 'https://milon.test/nft/v2.json',
      external_url: 'https://milon.test/nft/1'
    }
  },
  'nft.SetAttributes': {
    collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH',
    attributes: [{ trait_type: 'Color', value: 'Blue' }, { trait_type: 'Rarity', value: 'Legendary' }]
  },
  'nft.SetProperties': {
    collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH',
    properties: [{ key: 'rarity', value: 'rare' }, { key: 'edition', value: 'genesis' }]
  },
  'nft.CreateUnique': {
    collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', to: 'gKzpjpfWVvwgDs26DTCFFA9eRxb',
    metadata: {
      name: 'TestNFT #1', symbol: 'TNFT1', uri: 'https://milon.test/nft/1.json',
      external_url: 'https://milon.test/nft/1'
    },
    attributes: [{ trait_type: 'Color', value: 'Blue' }],
    properties: [{ key: 'rarity', value: 'rare' }],
    royalty: { recipient: '48A2Th5n4LoQ5LuwzxF7T27VYDZU', bps: 500 }
  },
  'nft.CreateBatch': {
    collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH',
    to: ['gKzpjpfWVvwgDs26DTCFFA9eRxb', '48A2Th5n4LoQ5LuwzxF7T27VYDZU'], amounts: [1, 2], max_supply: 10,
    metadata: {
      name: 'TestNFT #1', symbol: 'TNFT1', uri: 'https://milon.test/nft/1.json',
      external_url: 'https://milon.test/nft/1'
    },
    attributes: [{ trait_type: 'Color', value: 'Blue' }],
    properties: [{ key: 'rarity', value: 'rare' }],
    royalty: { recipient: '48A2Th5n4LoQ5LuwzxF7T27VYDZU', bps: 500 }
  },
  'nft.MintBatch': { collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', to: ['gKzpjpfWVvwgDs26DTCFFA9eRxb'], amounts: [1] },
  'nft.Transfer': { from: 'gKzpjpfWVvwgDs26DTCFFA9eRxb', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', to: '48A2Th5n4LoQ5LuwzxF7T27VYDZU', amount: 1 },
  'nft.Burn': { owner: '48A2Th5n4LoQ5LuwzxF7T27VYDZU', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', amount: 1 },
  'nft.SetRoyalty': { collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', royalty: { recipient: '48A2Th5n4LoQ5LuwzxF7T27VYDZU', bps: 1000 } },
  'nft.TransferRoyaltyRecipient': { recipient: '48A2Th5n4LoQ5LuwzxF7T27VYDZU', mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', new_recipient: 'gKzpjpfWVvwgDs26DTCFFA9eRxb' },
  'nft.CollectionMetadata': { collection: '2T2u6f4znq3ps3XvBPQYUtNH4DKx' },
  'nft.MetadataUri': { mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'nft.Attributes': { mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'nft.Properties': { mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'nft.MintConfigView': { mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'nft.TotalSupply': { mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },
  'nft.BalanceOf': { mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH', owner: 'gKzpjpfWVvwgDs26DTCFFA9eRxb' },
  'nft.RoyaltyInfo': { mint: '3tamDhFSgAdAAFZP7pwoCpNAZzFH' },

  // ==================== staking 模块（app_id=3，30 个方法）====================
  'staking.CreateValidator': {
    operator: '0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b',
    validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e',
    consensus_account: '0x7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b',
    consensus_pubkey: 'fndsa512_public_key_hex_string',
    bls_pubkey: 'bls12381_public_key_hex_string',
    network_address: '3030303030303030303030303030303030303030303030303030303030303030',
    commission_rate_bps: 100
  },
  'staking.JoinCandidatePool': { operator: '0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.LeaveCandidatePool': { operator: '0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.FundRewardTreasury': { funder: '0x9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b', amount: 1000 },
  'staking.Stake': { owner: '0x3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e', amount: 1000 },
  'staking.CancelPendingStake': { owner: '0x3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e', amount: 500 },
  'staking.ClaimRewards': { owner: '0x3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.ClaimOperatorRewards': { operator: '0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.RequestUnstake': { owner: '0x3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e', shares: 500 },
  'staking.ValidatorProfile': { validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.ValidatorPool': { validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.StakePosition': { owner: '0x3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.PositionSummary': { owner: '0x3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d', validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.CandidatePool': {},
  'staking.ActiveSetSnapshot': { epoch: 0 },
  'staking.ActiveSetHash': { epoch: 0 },
  'staking.CurrentActiveSetSnapshot': {},
  'staking.CurrentActiveSetHash': {},
  'staking.EpochTransition': { epoch: 0 },
  'staking.EpochConfig': {},
  'staking.EpochState': {},
  'staking.RewardTreasury': {},
  'staking.HeldPrincipal': { owner: '0x3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d' },
  'staking.EpochTransitionAttempt': { epoch: 0 },
  'staking.ConsensusActiveSet': { epoch: 0 },
  'staking.CurrentConsensusActiveSet': {},
  'staking.ConsensusActiveValidator': { epoch: 0, index: 0 },
  'staking.ConsensusActiveValidatorIndex': { epoch: 0, validator: '0x4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e' },
  'staking.ConsensusActiveValidatorIndexByPubkey': { epoch: 0, consensus_pubkey: 'fndsa512_public_key_hex_string' },
  'staking.ConsensusActiveValidatorsByBitmap': { epoch: 0, bitmap: 1 }
};

// IDL Entry 方法支付配置映射表：键为 appName.MethodName
// paymentMode: 测试用例使用的支付模式；payerRole: 付款角色提示；signerHint: 多签名场景提示
var IDL_EXAMPLE_PAYMENT = {
  // token 模块（23 entry）
  'token.Create': { paymentMode: 'unified_payer_all', payerRole: 'token' },
  'token.AbandonOwner': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.TransferOwner': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.AbandonFreezer': { paymentMode: 'unified_payer_all', payerRole: 'freezer' },
  'token.TransferFreezer': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.Mint': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.MintBatch': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.Burn': { paymentMode: 'unified_payer_all', payerRole: 'holder' },
  'token.Transfer': { paymentMode: 'unified_payer_all', payerRole: 'from' },
  'token.TransferWithTag': { paymentMode: 'unified_payer_all', payerRole: 'from' },
  'token.TransferBatch': { paymentMode: 'unified_payer_all', payerRole: 'from' },
  'token.Freeze': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.Unfreeze': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.Approve': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.Revoke': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.TransferFrom': { paymentMode: 'unified_payer_all', payerRole: 'spender' },
  'token.SetIcon': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.CreateWithCompliance': { paymentMode: 'unified_payer_all', payerRole: 'token' },
  'token.SetComplianceMode': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.AddComplianceRequirement': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.RemoveComplianceRequirement': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.ClearComplianceRequirements': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'token.ClaimFaucet': { paymentMode: 'unified_payer_all', payerRole: 'claimer' },

  // account 模块（10 entry）
  'account.Create': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'account.EnsureAccount': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'account.CreateMultisig': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'account.AddSigner': { paymentMode: 'split', payerRole: 'owner（多签账户地址）' },
  'account.AddSigners': { paymentMode: 'split', payerRole: 'owner（多签账户地址）' },
  'account.RemoveSigner': { paymentMode: 'split', payerRole: 'owner（多签账户地址）' },
  'account.SetThreshold': { paymentMode: 'split', payerRole: 'owner（多签账户地址）' },
  'account.SetSignerWeight': { paymentMode: 'split', payerRole: 'owner（多签账户地址）' },
  'account.VoteInit': { paymentMode: 'split', payerRole: 'owner（多签账户地址）' },
  'account.Vote': { paymentMode: 'split', payerRole: 'owner（多签账户地址）' },

  // demo 模块（10 entry）
  'demo.OpenOrder': { paymentMode: 'unified_payer_all', payerRole: 'operator' },
  'demo.PayOrder': { paymentMode: 'unified_payer_all', payerRole: 'payer' },
  'demo.SettleOrder': { paymentMode: 'unified_payer_all', payerRole: 'operator' },
  'demo.OpenGasSponsorPool': { paymentMode: 'unified_payer_all', payerRole: 'pool' },
  'demo.ClaimSponsoredScore': { paymentMode: 'unified_payer_all', payerRole: 'claimer' },
  'demo.InitPool': { paymentMode: 'unified_payer_all', payerRole: 'pool' },
  'demo.InitDex': { paymentMode: 'unified_payer_all', payerRole: 'dex' },
  'demo.SetLabel': { paymentMode: 'unified_payer_all', payerRole: 'pool' },
  'demo.BatchCredit': { paymentMode: 'unified_payer_all', payerRole: 'pool' },
  'demo.SetTierCap': { paymentMode: 'unified_payer_all', payerRole: 'pool' },

  // identity 模块（17 entry）
  'identity.Create': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.CreateWithAlias': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.AddKey': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.UpdateKey': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.RemoveKey': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.AddService': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.UpdateService': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.RemoveService': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.SetAvatarUri': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.Deactivate': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.SetAlias': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.RegisterOrganization': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.UpdateOrganizationCapabilities': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.DeactivateOrganization': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.DiscloseVcAttestation': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.RemoveVcDisclosure': { paymentMode: 'unified_payer_all', payerRole: 'subject' },
  'identity.RevokeVcAttestation': { paymentMode: 'unified_payer_all', payerRole: 'issuer' },

  // nft 模块（12 entry）
  'nft.CreateCollection': { paymentMode: 'unified_payer_all', payerRole: 'collection' },
  'nft.SetCollectionMetadata': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'nft.SetMetadata': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'nft.SetAttributes': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'nft.SetProperties': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'nft.CreateUnique': { paymentMode: 'multi_signer', payerRole: 'mint', signerHint: '该方法需 mint + owner 双签 bit0，使用 multi_signer 模式通过 /api/simulate 或 /api/write 调用' },
  'nft.CreateBatch': { paymentMode: 'multi_signer', payerRole: 'mint', signerHint: '该方法需 mint + owner 双签 bit0，使用 multi_signer 模式通过 /api/simulate 或 /api/write 调用' },
  'nft.MintBatch': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'nft.Transfer': { paymentMode: 'unified_payer_all', payerRole: 'from' },
  'nft.Burn': { paymentMode: 'unified_payer_all', payerRole: 'owner（NFT 持有者）' },
  'nft.SetRoyalty': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'nft.TransferRoyaltyRecipient': { paymentMode: 'unified_payer_all', payerRole: 'recipient' },

  // staking 模块（9 entry）
  'staking.CreateValidator': { paymentMode: 'multi_signer', payerRole: 'operator', signerHint: '该方法需 operator + consensus_account 双签，使用 multi_signer 模式通过 /api/simulate 或 /api/write 调用。注：consensus_pubkey/bls_pubkey/network_address 为 bytes 类型，JSON REST API 可能无法正确序列化' },
  'staking.JoinCandidatePool': { paymentMode: 'unified_payer_all', payerRole: 'operator' },
  'staking.LeaveCandidatePool': { paymentMode: 'unified_payer_all', payerRole: 'operator' },
  'staking.FundRewardTreasury': { paymentMode: 'unified_payer_all', payerRole: 'funder' },
  'staking.Stake': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'staking.CancelPendingStake': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'staking.ClaimRewards': { paymentMode: 'unified_payer_all', payerRole: 'owner' },
  'staking.ClaimOperatorRewards': { paymentMode: 'unified_payer_all', payerRole: 'operator' },
  'staking.RequestUnstake': { paymentMode: 'unified_payer_all', payerRole: 'owner' }
};

function idlIsScalarType(typeStr) {
  return Object.prototype.hasOwnProperty.call(IDL_SCALAR_TYPES, typeStr);
}

// 根据加载的元数据构建类型/常量/错误缓存，供表单渲染与默认值填充使用。
function buildIDLCaches(apps) {
  state.idlTypes = {};
  state.idlConstants = {};
  state.idlErrors = {};
  apps.forEach(function (app) {
    var typeMap = new Map();
    (app.types || []).forEach(function (t) { typeMap.set(t.name, t); });
    state.idlTypes[app.name] = typeMap;

    var constMap = new Map();
    (app.constants || []).forEach(function (c) {
      // 解析常量值：字符串值去除首尾引号，数字/布尔保持原类型
      var v = c.value;
      if (typeof v === 'string') {
        var s = v.trim();
        if ((s[0] === '"' && s[s.length - 1] === '"') || (s[0] === "'" && s[s.length - 1] === "'")) {
          v = s.slice(1, -1);
        }
      }
      constMap.set(c.name, v);
    });
    state.idlConstants[app.name] = constMap;

    var errMap = new Map();
    (app.errors || []).forEach(function (e) { errMap.set(e.code, e); });
    state.idlErrors[app.name] = errMap;
  });
}

// 解析类型字符串，返回 { base, generic }。
// 如 "vec<Address>" -> { base: "vec", generic: "Address" }；"Address" -> { base: "Address", generic: "" }
function idlParseType(typeStr) {
  var s = (typeStr || '').trim();
  var lt = s.indexOf('<');
  if (lt > 0 && s[s.length - 1] === '>') {
    return { base: s.slice(0, lt), generic: s.slice(lt + 1, -1) };
  }
  return { base: s, generic: '' };
}

// 根据 app 名称查询类型定义（struct/enum），返回 typeDef 或 null。
function idlGetTypeDef(appName, typeName) {
  if (!appName || !typeName) return null;
  var map = state.idlTypes[appName];
  if (!map) return null;
  return map.get(typeName) || null;
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
    // 默认全部折叠
    state.idlCollapsedApps = {};
    apps.forEach(function (a) { state.idlCollapsedApps[a.name] = true; });
    buildIDLCaches(apps);
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

    // 搜索时自动展开；否则按折叠状态
    var collapsed = !keyword && state.idlCollapsedApps[app.name];

    // 按 kind 分组：entry 在前，view 在后
    var entries = matched.filter(function (ix) { return ix.kind === 'entry'; });
    var views = matched.filter(function (ix) { return ix.kind === 'view'; });

    var gw = el('div', { class: 'endpoint-group idl-app-group' + (collapsed ? ' collapsed' : '') });
    var title = el('div', {
      class: 'group-title idl-app-title' + (collapsed ? '' : ' expanded'),
      text: app.name + ' (app_id=' + app.appId + ' · ' + matched.length + ')',
      onclick: function () { toggleIDLApp(app.name); },
    });
    gw.appendChild(title);

    if (!collapsed) {
      if (entries.length) {
        gw.appendChild(el('div', { class: 'idl-kind-label', text: 'entry (' + entries.length + ')' }));
        entries.forEach(function (ix) { gw.appendChild(buildIDLMethodItem(app, ix)); });
      }
      if (views.length) {
        gw.appendChild(el('div', { class: 'idl-kind-label', text: 'view (' + views.length + ')' }));
        views.forEach(function (ix) { gw.appendChild(buildIDLMethodItem(app, ix)); });
      }
    }
    tree.appendChild(gw);
  });

  if (!hasAny) {
    tree.appendChild(el('div', { class: 'empty-state small', text: '无匹配方法' }));
  }
  $('idlMethodCount').textContent = String(totalCount);
}

function toggleIDLApp(appName) {
  state.idlCollapsedApps[appName] = !state.idlCollapsedApps[appName];
  renderIDLAppList();
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
  // 选中方法时自动展开对应 app
  state.idlCollapsedApps[appName] = false;
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

  // 中文备注区（来自 IDL 数据里的 description）
  if (ix.description) {
    var descSec = el('div', { class: 'param-section idl-doc-section' });
    descSec.appendChild(el('div', { class: 'param-section-title', text: '说明' }));
    descSec.appendChild(el('div', { class: 'idl-doc-desc', text: ix.description || '' }));
    body.appendChild(descSec);
  }

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

  // IDL 编码参数。后端 pd.NormalizeArgs / Encode 会校验完整 args，
  // 所以 signer / any_signer 也需要作为 args 提交。
  var inputArgs = ix.args.filter(function (a) {
    return a.role === 'input' || a.role === 'signer' || a.role === 'any_signer';
  });
  var signerArgs = ix.args.filter(function (a) { return a.role === 'signer' || a.role === 'any_signer'; });

  if (inputArgs.length) {
    var argSec = el('div', { class: 'param-section' });
    argSec.appendChild(el('div', { class: 'param-section-title', text: '参数 (args，含 signer 地址参数)' }));
    inputArgs.forEach(function (a) { argSec.appendChild(buildIDLArgInput(a, state.currentIdlApp, ix.name)); });
    body.appendChild(argSec);
  } else {
    body.appendChild(el('div', { class: 'param-section' },
      el('div', { class: 'param-section-title', text: '参数 (args)' }),
      el('div', { class: 'empty-state small', text: '该方法无 input 参数' })
    ));
  }

  // signer 参数提示（结合 signerLookups 展示签名者角色与参数映射）
  if (signerArgs.length) {
    var hintSec = el('div', { class: 'param-section' });
    hintSec.appendChild(el('div', { class: 'param-section-title', text: '签名者说明' }));
    signerArgs.forEach(function (a) {
      hintSec.appendChild(el('div', { class: 'idl-signer-hint' },
        el('span', { class: 'idl-arg-name', text: a.name }),
        el('span', { class: 'idl-arg-type', text: a.type }),
        el('span', { class: 'idl-signer-note', text: 'role=' + a.role + '，需要在 args 中传地址，同时在下方签名配置里提供对应签名' })
      ));
    });
    // 展示 signerLookups 的角色 -> 参数映射
    if (ix.signerLookups) {
      Object.keys(ix.signerLookups).forEach(function (role) {
        var lk = ix.signerLookups[role];
        hintSec.appendChild(el('div', { class: 'idl-signer-hint lookup' },
          el('span', { class: 'idl-signer-note', text: '签名者角色「' + role + '」→ 取参数「' + (lk.arg || '—') + '」(' + (lk.type || 'Address') + ')' + (lk.res ? '，资源 res=' + lk.res : '') })
        ));
      });
    }
    body.appendChild(hintSec);
  }

  // entry 方法：支付模式 + 执行模式
  if (ix.kind === 'entry') {
    body.appendChild(buildIDLPaymentSection());
  }

  // 默认执行模式
  state.idlExecMode = 'simulate';
}

function buildIDLArgInput(arg, appName, methodName) {
  var typeDef = idlGetTypeDef(appName, arg.type);
  var paramDesc = arg.description;
  var row = el('div', { class: 'param-row idl-arg-row' },
    el('label', { class: 'param-label' },
      el('span', { class: 'idl-arg-name', text: arg.name }),
      el('span', { class: 'idl-arg-type', text: arg.type }),
      paramDesc ? el('span', { class: 'idl-arg-cn', text: paramDesc }) : null
    )
  );

  // 枚举类型：渲染为下拉选择
  if (typeDef && typeDef.kind === 'enum') {
    row.appendChild(buildIDLEnumInput(arg, appName, methodName, typeDef));
    return row;
  }

  // struct 类型：渲染为字段级子表单
  if (typeDef && typeDef.kind === 'struct') {
    row.appendChild(buildIDLStructInput(arg, appName, methodName, typeDef));
    return row;
  }

  var isScalar = idlIsScalarType(arg.type);
  if (isScalar) {
    var defaultVal = IDL_SCALAR_TYPES[arg.type];
    var activeAcc = getCurrentAccount();
    var activeAddr = (activeAcc && activeAcc.address) ? activeAcc.address : '';
    var exampleVal = idlArgExampleValue(appName, methodName, arg);
    var hasExample = exampleVal !== undefined;
    var val;
    if (arg.type === 'Address' || arg.type === 'Signer' || arg.type === 'AnySigner') {
      // 地址类参数：优先填活跃账户地址；无活跃账户时退回示例值。
      // 注意：不要盲目用示例值覆盖（示例值可能是假数据或固定合约地址，会导致链端报 requires signer）。
      val = activeAddr || (hasExample ? String(exampleVal) : defaultVal);
    } else {
      // 非地址类：优先常量默认值 -> 示例值 -> 类型默认
      var constVal = idlConstantForArg(appName, arg);
      if (constVal !== undefined) {
        val = String(constVal);
      } else if (hasExample) {
        val = String(exampleVal);
      } else {
        val = defaultVal;
      }
    }
    var inp = el('input', {
      class: 'param-input',
      'data-argname': arg.name,
      placeholder: (hasExample && exampleVal !== '' ? '如 ' + exampleVal + '（可修改）' : '如 ' + defaultVal),
      type: 'text',
    });
    inp.value = val;
    row.appendChild(inp);
    if (arg.type === 'Address' || arg.type === 'Signer' || arg.type === 'AnySigner' || /hash/i.test(arg.name)) {
      row.appendChild(makeBase58ToggleBtn(inp));
    }
    return row;
  }

  // 其它复合类型（vec/map/tuple/option/未知 struct）：JSON textarea
  var parsed = idlParseType(arg.type);
  var genericHint = parsed.generic ? '元素类型 ' + parsed.generic : '';
  var defaultComplex = idlDefaultComplexValue(arg.type);
  var complexVal;
  var exampleComplex = idlArgExampleValue(appName, methodName, arg);
  if (exampleComplex !== undefined) {
    // 复合类型：注入当前活跃账户的真实公钥，替换示例里的假/占位公钥
    var activeAccForComplex = getCurrentAccount();
    var injected = idlInjectAccountPublicKey(exampleComplex, activeAccForComplex);
    complexVal = JSON.stringify(injected, null, 2);
  } else {
    complexVal = defaultComplex;
  }
  var ta = el('textarea', {
    class: 'body-editor idl-arg-editor',
    'data-argname': arg.name,
    spellcheck: 'false',
    placeholder: (genericHint ? genericHint + '；' : '') + 'JSON，如 ' + defaultComplex,
  });
  ta.value = complexVal;
  row.appendChild(ta);
  return row;
}

// 获取某个参数在 IDL_EXAMPLE_ARGS 中的示例值（优先），否则返回 undefined。
function idlArgExampleValue(appName, methodName, arg) {
  var exampleArgs = IDL_EXAMPLE_ARGS[appName + '.' + methodName];
  if (exampleArgs && exampleArgs[arg.name] !== undefined) {
    return exampleArgs[arg.name];
  }
  return undefined;
}

// 从 IDL constants 中推断参数的默认值（按参数名模糊匹配常量名）。
function idlConstantForArg(appName, arg) {
  var constMap = state.idlConstants[appName];
  if (!constMap || !constMap.size) return undefined;
  var name = arg.name.toLowerCase();
  var candidates = [
    arg.name.toUpperCase(),
    arg.name.toUpperCase() + '_ADDRESS',
    arg.name.toUpperCase() + '_ID',
    name + '_address',
    name + '_id',
  ];
  for (var i = 0; i < candidates.length; i++) {
    if (constMap.has(candidates[i])) {
      var v = constMap.get(candidates[i]);
      if (typeof v === 'string' && /^M11on/i.test(v)) return v;
    }
  }
  return undefined;
}

// 枚举参数：渲染为 select，选项值为合法的 JSON 编码（unit variant 用字符串，带字段 variant 用对象）。
function buildIDLEnumInput(arg, appName, methodName, typeDef) {
  var exampleVal = idlArgExampleValue(appName, methodName, arg);
  var selectedVariant = '';
  if (typeof exampleVal === 'string') {
    selectedVariant = exampleVal;
  } else if (exampleVal && typeof exampleVal === 'object' && exampleVal.variant) {
    selectedVariant = exampleVal.variant;
  }

  var sel = el('select', { class: 'param-input idl-enum-select', 'data-argname': arg.name });
  // 空占位项
  sel.appendChild(el('option', { value: '', text: '— 选择枚举值 —' }));

  typeDef.variants.forEach(function (v) {
    var optVal;
    if (v.kind === 'unit' || (v.fields && v.fields.length === 0)) {
      // unit 变体：直接是字符串
      optVal = JSON.stringify(v.name);
    } else if (v.kind === 'tuple') {
      // tuple 变体：{"variant":"X","value":[...]}
      optVal = JSON.stringify({ variant: v.name, value: v.fields.map(function () { return null; }) });
    } else {
      // struct 变体：{"variant":"X","value":{...}}
      var fieldsObj = {};
      v.fields.forEach(function (f) {
        fieldsObj[f.name] = idlDefaultForType(appName, f.type);
      });
      optVal = JSON.stringify({ variant: v.name, value: fieldsObj });
    }
    var opt = el('option', { value: optVal, text: v.name });
    sel.appendChild(opt);
  });

  if (selectedVariant) {
    for (var i = 0; i < sel.options.length; i++) {
      var o = sel.options[i];
      if (o.textContent === selectedVariant) { sel.selectedIndex = i; break; }
    }
  }
  return sel;
}

// struct 参数：渲染为字段级子表单，隐藏 textarea 保存完整 JSON，字段变化时同步。
function buildIDLStructInput(arg, appName, methodName, typeDef) {
  var container = el('div', { class: 'idl-struct-input' });

  // 初始对象：优先示例值，否则按字段类型生成默认值
  var exampleVal = idlArgExampleValue(appName, methodName, arg);
  var initial = (exampleVal && typeof exampleVal === 'object' && !Array.isArray(exampleVal))
    ? exampleVal
    : {};
  if (!exampleVal || typeof exampleVal !== 'object') {
    typeDef.fields.forEach(function (f) { initial[f.name] = idlDefaultForType(appName, f.type); });
  }
  if (Object.keys(initial).length === 0) {
    typeDef.fields.forEach(function (f) { initial[f.name] = idlDefaultForType(appName, f.type); });
  }

  // 隐藏 textarea 承载完整 JSON，供 buildIDLRequest 统一收集
  var hidden = el('textarea', {
    class: 'idl-struct-hidden',
    'data-argname': arg.name,
    style: 'display:none;',
  });
  hidden.value = JSON.stringify(initial, null, 2);
  container.appendChild(hidden);

  function sync() {
    hidden.value = JSON.stringify(initial, null, 2);
  }

  var fieldsBox = el('div', { class: 'idl-struct-fields' });
  typeDef.fields.forEach(function (f) {
    var fTypeDef = idlGetTypeDef(appName, f.type);
    var fRow = el('div', { class: 'idl-struct-field-row' },
      el('label', { class: 'idl-struct-field-label' },
        el('span', { class: 'idl-arg-name', text: f.name }),
        el('span', { class: 'idl-arg-type', text: f.type })
      )
    );

    if (fTypeDef && fTypeDef.kind === 'enum') {
      var sel = el('select', { class: 'param-input idl-enum-select' });
      sel.appendChild(el('option', { value: '', text: '— 选择 —' }));
      fTypeDef.variants.forEach(function (v) {
        var optVal;
        if (v.kind === 'unit' || (v.fields && v.fields.length === 0)) {
          optVal = JSON.stringify(v.name);
        } else {
          var fieldsObj = {};
          v.fields.forEach(function (vf) { fieldsObj[vf.name] = idlDefaultForType(appName, vf.type); });
          optVal = JSON.stringify({ variant: v.name, value: fieldsObj });
        }
        sel.appendChild(el('option', { value: optVal, text: v.name }));
      });
      // 选中当前值对应的变体
      var cur = initial[f.name];
      if (cur !== undefined && cur !== null && cur !== '') {
        var curName = typeof cur === 'string' ? cur : cur.variant;
        for (var si = 0; si < sel.options.length; si++) {
          if (sel.options[si].textContent === curName) { sel.selectedIndex = si; break; }
        }
      }
      sel.addEventListener('change', function () {
        try { initial[f.name] = JSON.parse(sel.value); } catch (e) { initial[f.name] = ''; }
        sync();
      });
      fRow.appendChild(sel);
    } else if (idlIsScalarType(f.type)) {
      var inp = el('input', { class: 'param-input', type: 'text' });
      var fVal = initial[f.name];
      // 地址类字段：示例/默认为空时预填活跃账户地址
      if ((fVal === undefined || fVal === null || fVal === '') &&
          (f.type === 'Address' || f.type === 'PublicKey' || f.type === 'Signer' || f.type === 'AnySigner')) {
        var act = getCurrentAccount();
        if (act && act.address) { fVal = act.address; initial[f.name] = act.address; sync(); }
      }
      inp.value = (fVal === undefined || fVal === null) ? '' : String(fVal);
      inp.addEventListener('input', function () {
        initial[f.name] = idlCoerceScalar(f.type, inp.value);
        sync();
      });
      fRow.appendChild(inp);
    } else {
      // 嵌套复合字段：textarea JSON
      var ta = el('textarea', { class: 'body-editor idl-struct-nested', spellcheck: 'false' });
      ta.value = JSON.stringify(initial[f.name], null, 2);
      ta.addEventListener('input', function () {
        try { initial[f.name] = JSON.parse(ta.value || 'null'); } catch (e) {}
        sync();
      });
      fRow.appendChild(ta);
    }
    fieldsBox.appendChild(fRow);
  });
  container.appendChild(fieldsBox);
  return container;
}

// 根据类型名生成默认值（用于 struct 字段、enum 变体字段等）。
function idlDefaultForType(appName, typeStr) {
  var typeDef = idlGetTypeDef(appName, typeStr);
  if (typeDef && typeDef.kind === 'enum') {
    if (typeDef.variants && typeDef.variants.length) {
      var v0 = typeDef.variants[0];
      if (v0.kind === 'unit' || !v0.fields || v0.fields.length === 0) {
        return v0.name; // 字符串
      }
      var obj = {};
      v0.fields.forEach(function (f) { obj[f.name] = idlDefaultForType(appName, f.type); });
      return { variant: v0.name, value: obj };
    }
    return '';
  }
  if (typeDef && typeDef.kind === 'struct') {
    var o = {};
    typeDef.fields.forEach(function (f) { o[f.name] = idlDefaultForType(appName, f.type); });
    return o;
  }
  if (idlIsScalarType(typeStr)) return IDL_SCALAR_TYPES[typeStr];
  return idlDefaultComplexValue(typeStr);
}

// 将输入框文本按标量类型转换为合适的 JS 值。
function idlCoerceScalar(typeStr, raw) {
  if (typeStr === 'bool' || typeStr === 'boolean') {
    return raw === 'true' || raw === '1';
  }
  return raw;
}

// 判断字符串是否形如 secp256k1 压缩公钥（33 字节，02/03 开头，共 66 个 hex 字符）
function idlLooksLikeSecpPk(v) {
  if (typeof v !== 'string') return false;
  var s = v.replace(/^0x/i, '').replace(/\s/g, '');
  return /^(02|03)[0-9a-fA-F]{64}$/.test(s);
}

// 递归把复合参数示例里的 secp256k1 公钥字段替换为当前活跃账户的真实公钥，
// 避免示例里的假/占位公钥导致链端 "invalid base58 string" 报错。
function idlInjectAccountPublicKey(val, activeAcc) {
  if (!val || typeof val !== 'object') return val;
  if (Array.isArray(val)) {
    return val.map(function (v) { return idlInjectAccountPublicKey(v, activeAcc); });
  }
  var pk = (activeAcc && activeAcc.publicKey) ? activeAcc.publicKey : '';
  var out = {};
  Object.keys(val).forEach(function (k) {
    var v = val[k];
    // 公钥字段名（public_key / pubkey / *_pk），但排除 fndsa512/bls 等特殊曲线字段
    var isPkField = /(public_key|pubkey|pub_key|_pk)$/i.test(k) && !/(bls|consensus|fndsa|ed25519)/i.test(k);
    if (isPkField && idlLooksLikeSecpPk(v) && pk) {
      out[k] = pk;
    } else {
      out[k] = idlInjectAccountPublicKey(v, activeAcc);
    }
  });
  return out;
}

function idlDefaultComplexValue(typeStr) {
  if (typeStr.indexOf('vec<') === 0) return '[]';
  if (typeStr.indexOf('option<') === 0) return 'null';
  if (typeStr.indexOf('map<') === 0) return '{}';
  if (typeStr.indexOf('tuple<') === 0) return '[]';
  // 自定义 struct/enum：返回空对象
  return '{}';
}

// 从 signerLookups 推导支付/签名角色与对应参数。
// 返回 { role, argName }：role 为签名者角色名（如 owner/freezer），argName 为该角色对应的参数名（如 token）。
// 若当前方法没有 signerLookups，返回 null。
function idlDerivePayerRole(ix) {
  if (!ix || !ix.signerLookups) return null;
  var keys = Object.keys(ix.signerLookups);
  if (keys.length === 0) return null;
  // 取第一个 signer 角色（通常即为主要付款/签名者）
  var role = keys[0];
  var lookup = ix.signerLookups[role];
  return { role: role, argName: lookup ? lookup.arg : '' };
}

// 计算 entry 方法的付款/签名者角色提示（优先 signerLookups 推导，回退硬编码 payerRole）。
function idlPayerRoleLabel(ix, examplePay) {
  var derived = idlDerivePayerRole(ix);
  if (derived && derived.role) {
    return derived.role + '（signer）';
  }
  return (examplePay && examplePay.payerRole) ? examplePay.payerRole : '';
}

function buildIDLPaymentSection() {
  var sec = el('div', { class: 'param-section' });
  sec.appendChild(el('div', { class: 'param-section-title', text: '执行配置' }));
  var activeAcc = getCurrentAccount();
  if (activeAcc) {
    sec.appendChild(el('div', { class: 'idl-active-account-hint', text: '当前账户: ' + activeAcc.label + ' (' + activeAcc.address.slice(0, 10) + '...)' }));
  }

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
  // 根据实例支付配置自动选择支付模式
  var examplePay = state.currentIdlApp && state.currentIdlMethod
    ? IDL_EXAMPLE_PAYMENT[state.currentIdlApp + '.' + state.currentIdlMethod.name]
    : null;
  if (examplePay && examplePay.paymentMode) {
    pmSelect.value = examplePay.paymentMode;
  }
  if (state.currentIdlMethod && state.currentIdlMethod.sponsor) {
    pmSelect.value = 'sponsored';
  }
  pmRow.appendChild(pmSelect);
  sec.appendChild(pmRow);

  // signerHint 提示（多签名场景）
  if (examplePay && examplePay.signerHint) {
    sec.appendChild(el('div', { class: 'idl-signer-hint-box', text: examplePay.signerHint }));
  }

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
  var activeAcc = getCurrentAccount();
  var prefilledAddr = activeAcc ? activeAcc.address : '';
  var prefilledSk = activeAcc ? activeAcc.privateKey : '';
  var prefilledPk = (activeAcc && activeAcc.publicKey) ? activeAcc.publicKey : '';
  var sigModeTpl = prefilledPk
    ? '{\n  "type": "pubkey",\n  "publicKey": "' + prefilledPk + '"\n}'
    : '{\n  "type": "pubkey",\n  "publicKey": "base58公钥"\n}';
  // 若账户只有私钥而无公钥，则异步派生公钥并回填 signatureMode 字段
  if (activeAcc && !prefilledPk && prefilledSk) {
    fetch('/api/util/key/derive-public', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ privateKey: prefilledSk.replace(/\s/g, ''), keyType: 'secp256k1' })
    }).then(function (r) { return r.json(); }).then(function (resp) {
      if (resp && resp.success && resp.data && resp.data.publicKey) {
        var pk = resp.data.publicKey;
        activeAcc.publicKey = pk;
        var sigNode = document.querySelector('#idlPaymentFields [data-field="signatureMode"]');
        if (sigNode) sigNode.value = '{\n  "type": "pubkey",\n  "publicKey": "' + pk + '"\n}';
      }
    }).catch(function () {});
  }
  var signerEntryTpl = '{\n  "address": "' + (prefilledAddr || 'base58地址') + '",\n'
    + (isSubmit ? '  "privateKey": "' + (prefilledSk || 'hex或base58私钥') + '",\n' : '')
    + '  "signatureMode": ' + sigModeTpl.replace(/\n/g, '\n  ') + '\n}';

  // 通用：payer/owner 地址（优先用 signerLookups 推导签名角色，回退实例支付配置）
  var examplePay = state.currentIdlApp && state.currentIdlMethod
    ? IDL_EXAMPLE_PAYMENT[state.currentIdlApp + '.' + state.currentIdlMethod.name]
    : null;
  if (pm === 'multi_signer') {
    container.appendChild(buildIDLFieldRow('signers', 'signers (JSON 数组)', '[\n' + signerEntryTpl + '\n]', 'textarea', 'data-field', 'signers'));
    container.appendChild(buildIDLFieldRow('gasPayer', 'gasPayer (JSON，可选)', '', 'textarea', 'data-field', 'gasPayer'));
    return;
  }

  var payerRoleLabel = idlPayerRoleLabel(state.currentIdlMethod, examplePay);
  var payerLabel = payerRoleLabel
    ? payerRoleLabel + ' 地址 (base58)'
    : '付款/所有者地址 (base58)';
  container.appendChild(buildIDLFieldRow('payerAddress', payerLabel, prefilledAddr, 'text', 'data-field', 'payerAddress'));

  if (pm === 'split') {
    // split 模式用 owner 概念，复用 payerAddress 作为 owner 地址
    if (isSubmit) {
      container.appendChild(buildIDLFieldRow('ownerPrivateKey', 'owner 私钥 (hex/base58)', prefilledSk, 'password', 'data-field', 'ownerPrivateKey'));
    }
  } else {
    if (isSubmit) {
      container.appendChild(buildIDLFieldRow('payerPrivateKey', 'payer 私钥 (hex/base58)', prefilledSk, 'password', 'data-field', 'payerPrivateKey'));
    }
  }

  // dual_sign 需要 ix 字段
  if (modeDef && modeDef.needIx) {
    container.appendChild(buildIDLFieldRow('ixAddress', 'ix 签名者地址 (base58)', '', 'text', 'data-field', 'ixAddress'));
    if (isSubmit) {
      container.appendChild(buildIDLFieldRow('ixPrivateKey', 'ix 私钥 (hex/base58)', '', 'password', 'data-field', 'ixPrivateKey'));
    }
  }

  // signatureMode（JSON）— 若活跃账户有 publicKey 则自动预填
  container.appendChild(buildIDLFieldRow('signatureMode', 'signatureMode (JSON)', sigModeTpl, 'textarea', 'data-field', 'signatureMode'));
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
  // 占位：用于参数 JSON 解析失败时短路返回
  var args = {};

  // 收集所有参数（input + signer）
  var exampleArgs = IDL_EXAMPLE_ARGS[appName + '.' + ix.name];
  var examplePay = IDL_EXAMPLE_PAYMENT[appName + '.' + ix.name];
  var payerRole = (examplePay && examplePay.payerRole) ? examplePay.payerRole : '';
  var payerAddress = '';
  var payerAddressNode = document.querySelector('#idlPaymentFields [data-field="payerAddress"]');
  if (payerAddressNode) payerAddress = payerAddressNode.value.trim();

  // input 参数：从输入框获取
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
      catch (e) {
        showToast('参数 ' + name + ' 的 JSON 解析失败：' + e.message + '（请检查语法，特别是字符串是否带引号）', 'error', 6000);
        delete args.__invalidJson;
        args.__invalidJson = name;
      }
    }
  });
  if (args.__invalidJson) {
    return null; // 调用方会通过 if (!req) return; 终止
  }

  // signer / any_signer 参数：优先从 payerAddress 获取（signerLookups 推导的角色参数或仅有一个 signer 参数时），
  // 否则回退到示例值。输入框通常已预填活跃账户地址，此处为兜底逻辑。
  var derivedRole = idlDerivePayerRole(ix);
  if (ix.args) {
    var signerArgs2 = ix.args.filter(function (a) { return a.role === 'signer' || a.role === 'any_signer'; });
    signerArgs2.forEach(function (arg) {
      if (!args.hasOwnProperty(arg.name)) {
        var val = '';
        // 优先：payerRole（signerLookups 推导或硬编码）与参数名匹配时用 payerAddress
        if (payerRole === arg.name && payerAddress) {
          val = payerAddress;
        } else if (derivedRole && derivedRole.argName === arg.name && payerAddress) {
          // signerLookups 指明了该签名者对应的参数名，直接用 payerAddress
          val = payerAddress;
        } else if (signerArgs2.length === 1 && payerAddress) {
          // 兜底：只有一个 signer 参数时直接用 payerAddress（常见于 from/holder/owner 场景）
          val = payerAddress;
        } else if (exampleArgs && exampleArgs[arg.name]) {
          val = exampleArgs[arg.name];
        }
        if (val !== '') args[arg.name] = val;
      }
    });
  }

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

  function readKeyField(field) {
    var node = document.querySelector('#idlPaymentFields [' + 'data-field' + '="' + field + '"]');
    return node ? node.value.replace(/\s/g, '') : '';
  }

  if (pm !== 'multi_signer') {
    payload.payerAddress = readField('payerAddress');
  }

  if (pm === 'multi_signer') {
    // multi_signer uses signers/gasPayer instead of payerPrivateKey fields.
  } else if (pm === 'split') {
    if (isSubmit) {
      var ownerSk = readKeyField('ownerPrivateKey');
      if (ownerSk) payload.ownerPrivateKey = ownerSk;
    }
  } else {
    if (isSubmit) {
      var payerSk = readKeyField('payerPrivateKey');
      if (payerSk) payload.payerPrivateKey = payerSk;
    }
  }

  var modeDef = IDL_PAYMENT_MODES.find(function (m) { return m.value === pm; });
  if (pm === 'multi_signer') {
    var signersRaw = readField('signers');
    if (signersRaw) { try { payload.signers = JSON.parse(signersRaw); } catch (e) {} }
    var gasPayerRaw = readField('gasPayer');
    if (gasPayerRaw) { try { payload.gasPayer = JSON.parse(gasPayerRaw); } catch (e) {} }
  }
  if (modeDef && modeDef.needIx) {
    payload.ixAddress = readField('ixAddress');
    if (isSubmit) {
      var ixSk = readKeyField('ixPrivateKey');
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
  // 响应面板里 tx_hash 行的「⇄ base58」按钮（事件委托，覆盖控制台与 IDL 视图）
  document.addEventListener('click', function (e) {
    var btn = e.target.closest && e.target.closest('.txhash-convert-btn');
    if (!btn) return;
    var raw = (btn.getAttribute('data-hash') || '').replace(/^&quot;|&quot;$/g, '');
    var b58 = hexToBase58(raw);
    if (!b58) {
      showToast('无法转换该哈希', 'error');
      return;
    }
    copyToClipboard(b58, function () {
      showToast('base58: ' + b58 + ' （已复制）', 'success');
    }, function () {
      showToast('base58: ' + b58, 'success');
    });
  });
  loadHistory();
  loadAccounts();
  loadCurrentAccount();
  updateAccountLabel();
  $('accountBtn').addEventListener('click', openAccountModal);
  $('accountModalCloseBtn').addEventListener('click', closeAccountModal);
  $('accountModalOverlay').addEventListener('click', closeAccountModal);
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
