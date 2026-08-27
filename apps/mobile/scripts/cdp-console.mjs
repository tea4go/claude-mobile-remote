#!/usr/bin/env node
// 连 Android WebView 的 Chrome DevTools Protocol，实时打印 console 日志，并可抓取页面状态。
// 不依赖 chrome://inspect 弹窗（其前端资源可能被墙或弹窗被拦截），是命令行兜底调试入口。
//
// 用法（在 apps/mobile 目录下）：
//   node scripts/cdp-console.mjs               # 自动 adb forward + 持续监听 console
//   node scripts/cdp-console.mjs --state       # 额外 dump 一次页面文本/按钮
//   node scripts/cdp-console.mjs --once        # 只抓一次状态后退出
//   node scripts/cdp-console.mjs --pid <pid>   # 指定 App 进程 pid（默认自动从 adb 获取）
//
// 前置：模拟器已启动、App 已打开；adb 在 PATH 中（ANDROID_HOME/platform-tools）。

import { execSync } from 'node:child_process';

const APP_ID = 'com.tea4go.claudemobileremote';
const args = process.argv.slice(2);
const doState = args.includes('--state');
const once = args.includes('--once');
const pidIdx = args.indexOf('--pid');
const pidArg = pidIdx >= 0 ? args[pidIdx + 1] : null;

function sh(cmd) {
  try {
    return execSync(cmd, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim();
  } catch {
    return '';
  }
}

// 1. 自动 forward devtools 端口
let appPid = pidArg;
if (!appPid) {
  appPid = sh(`adb shell pidof ${APP_ID}`).split(/\s+/)[0];
  if (!appPid) {
    console.error(`找不到 App 进程（${APP_ID}）。请确认模拟器已启动、App 已打开，且 adb 可用。`);
    process.exit(1);
  }
}
sh(`adb forward tcp:9222 localabstract:webview_devtools_remote_${appPid}`);

// 2. 从 /json 拿 WebView 的 devtools ws 地址
const jsonUrl = 'http://localhost:9222/json';
let pages = [];
try {
  const res = await fetch(jsonUrl);
  pages = await res.json();
} catch {
  console.error('无法读取 http://localhost:9222/json，adbd forward 可能失败。');
  process.exit(1);
}
const page = pages.find((p) => p.url.includes('localhost')) || pages[0];
if (!page) {
  console.error('没有可调试的页面。确认 App 内的 WebView 已加载。');
  process.exit(1);
}

// 3. 连接 CDP
const ws = new WebSocket(page.webSocketDebuggerUrl);
let id = 0;
const pending = new Map();
function send(method, params = {}) {
  return new Promise((resolve, reject) => {
    const myId = ++id;
    pending.set(myId, { resolve, reject });
    ws.send(JSON.stringify({ id: myId, method, params }));
  });
}

const ts = () => new Date().toLocaleTimeString('zh-CN', { hour12: false });

ws.onmessage = (e) => {
  const msg = JSON.parse(e.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    msg.error ? reject(new Error(JSON.stringify(msg.error))) : resolve(msg.result);
    return;
  }
  if (msg.method === 'Runtime.consoleAPICalled') {
    const vals = (msg.params.args || []).map((a) => a.value ?? a.description ?? '').join(' ');
    console.log(`[${ts()}] console.${msg.params.type}: ${vals}`);
  } else if (msg.method === 'Log.entryAdded') {
    console.log(`[${ts()}] ${msg.params.entry.level}: ${msg.params.entry.text}`);
  } else if (msg.method === 'Runtime.exceptionThrown') {
    const d = msg.params.exceptionDetails;
    console.log(`[${ts()}] EXCEPTION: ${d.text} ${d.exception?.description || ''}`);
  }
};

ws.onopen = async () => {
  try {
    await send('Runtime.enable');
    await send('Log.enable');
    console.log('=== 已连上 WebView，监听 console 输出（Ctrl+C 退出）===');
    if (doState || once) await dumpState();
    if (once) {
      console.log('=== 单次抓取完成 ===');
      process.exit(0);
    }
  } catch (err) {
    console.error('连接失败:', err.message);
    process.exit(1);
  }
};

async function dumpState() {
  const r = await send('Runtime.evaluate', {
    expression: `JSON.stringify({
      url: location.href,
      text: document.body ? document.body.innerText.slice(0, 1200) : '(no body)',
      buttons: Array.from(document.querySelectorAll('button')).map(b => b.innerText)
    })`,
    returnByValue: true,
  });
  console.log('\n=== 当前页面状态 ===');
  console.log(r.result.value);
  console.log('====================\n');
}

ws.onerror = () => {
  console.error('WebSocket 连接出错。确认 adb forward 已建立。');
  process.exit(1);
};
process.on('SIGINT', () => {
  console.log('\n已退出');
  process.exit(0);
});
