#!/usr/bin/env python3
"""Isolated project preparation and local Previewer command host (stdlib only)."""
import argparse
import json
import os
from pathlib import Path
import secrets
import signal
import selectors
import shutil
import socket
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def prepare(repo, target=None, preview=True):
    source = repo / 'apps/harmony'
    target = target or repo / '.local-tools/harmony-preview/project'
    target.mkdir(parents=True, exist_ok=True)
    # Copy only project inputs. Dependencies are resolved by the paired ohpm.
    for name in ('AppScope', 'hvigor', 'hvigorfile.ts', 'oh-package.json5', 'oh-package-lock.json5', 'build-profile.json5'):
        src, dst = source / name, target / name
        if src.is_dir():
            shutil.copytree(src, dst, dirs_exist_ok=True, ignore=shutil.ignore_patterns('._*'))
        elif src.exists():
            shutil.copy2(src, dst)
    if preview:
        profile = target / "build-profile.json5"
        profile.write_text(profile.read_text().replace('"useNormalizedOHMUrl": true', '"useNormalizedOHMUrl": false'))
    entry = target / 'entry'
    entry.mkdir(exist_ok=True)
    # Synchronize sources so deleted production files cannot remain in the copy.
    if (entry / 'src').exists():
        shutil.rmtree(entry / 'src')
    shutil.copytree(source / 'entry/src', entry / 'src')
    for name in ('build-profile.json5', 'hvigorfile.ts', 'oh-package.json5', 'obfuscation-rules.txt'):
        shutil.copy2(source / 'entry' / name, entry / name)
    if preview:
        shutil.copy2(source / 'preview/Index.ets', entry / 'src/main/ets/pages/Index.ets')
    return target


def configure(project):
    entry = project / 'entry'
    original = entry / 'build/default/intermediates/res/default'
    preview = entry / '.preview/default/intermediates/res/default'
    shutil.copytree(original, preview, dirs_exist_ok=True)
    out = entry / '.preview/default/intermediates/assets/default/ets'
    config = {
        'buildPath': str(out), 'aceModuleBuild': str(out), 'abilityType': 'page',
        'aceModuleJsonPath': str(preview / 'module.json'),
        'aceProfilePath': str(preview / 'resources/base/profile'),
        'stageRouterConfig': {'paths': [str(preview / 'module.json'), str(preview / 'resources/base/profile/main_pages.json')],
                              'contents': [(original / 'module.json').read_text(), json.dumps({'src': ['pages/Index']})]},
    }
    dest = entry / '.preview/config/buildConfig.json'
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(json.dumps(config))


def compile_preview(project):
    args = [os.environ['HARMONY_HVIGORW'], '--mode', 'module', '-p', 'product=default', '-p',
            'module=entry@default', '-p', 'previewMode=true', 'PreviewBuild', '--no-daemon', '--no-incremental']
    process = subprocess.Popen(args, cwd=project, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, start_new_session=True)
    selector = selectors.DefaultSelector()
    selector.register(process.stdout, selectors.EVENT_READ)
    pending = b''
    success = False
    deadline = time.monotonic() + 180
    try:
        while time.monotonic() < deadline:
            for key, _ in selector.select(0.5):
                block = os.read(key.fileobj.fileno(), 65536)
                if not block:
                    if not success:
                        raise RuntimeError('预览编译提前退出')
                    return
                os.write(1, block)
                pending = (pending + block)[-65536:]
                if b'BUILD FAILED' in pending:
                    raise RuntimeError('预览编译失败')
                if b'BUILD SUCCESSFUL' in pending:
                    success = True
                    return
            if process.poll() is not None:
                raise RuntimeError('预览编译退出，未报告成功')
        raise RuntimeError('预览编译超过180秒')
    finally:
        selector.close()
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
        process.stdout.close()


def serve(project, tools):
    token = secrets.token_hex(16)
    base = 'cmr' + secrets.token_hex(8)
    pipe = Path('/tmp') / (base + '_commandPipe')
    listener = socket.socket(socket.AF_UNIX)
    listener.bind(str(pipe))
    os.chmod(pipe, 0o600)
    listener.listen(1)
    listener.settimeout(25)
    image_socket = socket.socket()
    image_socket.bind(('127.0.0.1', 0))
    image_port = image_socket.getsockname()[1]
    image_socket.close()
    connection = None
    lock = threading.Lock()

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_GET(self):
            if self.path != '/' + token:
                self.send_error(404)
                return
            body = HTML.replace('TOKEN', token).replace('IMAGE_PORT', str(image_port)).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'text/html; charset=utf-8')
            self.send_header('Cache-Control', 'no-store')
            self.end_headers()
            self.wfile.write(body)

        def do_POST(self):
            # Only our same-origin local page can issue commands.
            if self.path != '/' + token or self.headers.get('Origin') != origin:
                self.send_error(403)
                return
            try:
                size = int(self.headers.get('Content-Length', '0'))
                if not 0 < size < 2048:
                    raise ValueError('size')
                data = json.loads(self.rfile.read(size))
                command = data['command']
                if command not in ('MousePress', 'MouseRelease', 'MouseMove', 'BackClicked'):
                    raise ValueError('command')
                args = {} if command == 'BackClicked' else {'x': max(0, min(1079, int(data['x']))), 'y': max(0, min(2339, int(data['y'])))}
                message = json.dumps({'version': '1.0.1', 'type': 'action', 'command': command, 'args': args}).encode() + b'\0'
                with lock:
                    if connection is None:
                        raise ValueError('engine not ready')
                    connection.sendall(message)
                self.send_response(204)
                self.end_headers()
            except (ValueError, KeyError, TypeError, OSError):
                self.send_error(400)

    http = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    origin = 'http://127.0.0.1:' + str(http.server_port)
    binary_dir = tools / 'sdk/default/openharmony/previewer/common/bin'
    entry = project / 'entry'
    log = project.parent / 'previewer.log'
    args = [str(binary_dir / 'Previewer'), '-j', str(entry / '.preview/default/intermediates/assets/default/ets'),
            '-s', base, '-device', 'phone', '-shape', 'rect', '-sd', '480', '-or', '1080', '2340', '-cr', '540', '1170',
            '-refresh', 'full', '-ljPath', str(entry / 'build/default/intermediates/loader/default/loader.json'),
            '-n', 'entry', '-av', 'ACE_2_0', '-url', 'pages/Index', '-pages', 'main_pages',
            '-arp', str(entry / '.preview/default/intermediates/res/default'), '-pm', 'Stage',
            '-hsp', str(tools / 'sdk/default/hms/previewer'), '-l', 'zh_CN', '-cm', 'light', '-o', 'portrait',
            '-lws', str(image_port), '-sid', token]
    process = None
    serving = False
    try:
        with log.open('w') as output:
            process = subprocess.Popen(args, cwd=binary_dir, stdout=output, stderr=output)
            connection, _ = listener.accept()
            connection.settimeout(1)
            def drain():
                while process.poll() is None:
                    try:
                        if not connection.recv(65536):
                            break
                    except socket.timeout:
                        pass
                    except OSError:
                        break
            threading.Thread(target=drain, daemon=True).start()
            threading.Thread(target=http.serve_forever, daemon=True).start()
            serving = True
            url = origin + '/' + token
            print('预览地址：' + url, flush=True)
            print('Ctrl+C 停止。修改源码后重新运行脚本；日志：' + str(log), flush=True)
            subprocess.run(['open', url], check=False)
            while process.poll() is None:
                time.sleep(0.5)
            raise RuntimeError('Previewer 已退出，请检查 ' + str(log))
    finally:
        if serving:
            http.shutdown()
        http.server_close()
        if process is not None and process.poll() is None:
            process.terminate()
            try:
                process.wait(5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        if connection is not None:
            connection.close()
        listener.close()
        pipe.unlink(missing_ok=True)


HTML = '''<!doctype html><meta charset="utf-8"><title>Harmony ArkUI Preview</title>
<style>body{background:#e8e9ed;font:14px system-ui;text-align:center}canvas{width:360px;height:780px;background:white;touch-action:none;box-shadow:0 4px 24px #aaa}button{padding:8px;margin:8px}</style>
<p>真实 ArkUI 渲染 · 模拟数据 · 修改后重新运行脚本</p><button id="back">返回</button><span id="status">连接中…</span><br><canvas width="540" height="1170"></canvas>
<script>
const canvas=document.querySelector('canvas'),ctx=canvas.getContext('2d'),status=document.querySelector('#status');
const ws=new WebSocket('ws://127.0.0.1:IMAGE_PORT/TOKEN','ws');ws.binaryType='arraybuffer';
ws.onopen=()=>status.textContent='已连接，等待画面';ws.onclose=()=>status.textContent='预览已断开';ws.onerror=()=>status.textContent='预览连接失败，请检查终端日志';
let serial=Promise.resolve();ws.onmessage=e=>{serial=serial.then(async()=>{const bytes=new Uint8Array(e.data);let start=-1;for(let i=0;i<bytes.length-1;i++){if(bytes[i]===255&&bytes[i+1]===216){start=i;break}}if(start<0)return;const bitmap=await createImageBitmap(new Blob([bytes.slice(start)],{type:'image/jpeg'}));ctx.drawImage(bitmap,0,0,540,1170);bitmap.close();status.textContent='ArkUI 画面已就绪'}).catch(()=>status.textContent='图像解码失败')};
let queue=Promise.resolve();function command(command,e){const r=canvas.getBoundingClientRect();const data={command};if(e){data.x=Math.round((e.clientX-r.left)*1080/r.width);data.y=Math.round((e.clientY-r.top)*2340/r.height)}queue=queue.then(async()=>{const response=await fetch('/TOKEN',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});if(!response.ok)throw new Error('command failed')}).catch(()=>status.textContent='操作发送失败')}
canvas.onpointerdown=e=>{canvas.setPointerCapture(e.pointerId);command('MousePress',e)};canvas.onpointerup=e=>command('MouseRelease',e);canvas.onpointercancel=e=>command('MouseRelease',e);canvas.onpointermove=e=>{if(e.buttons)command('MouseMove',e)};document.querySelector('#back').onclick=()=>command('BackClicked');
</script>'''

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=('prepare', 'configure', 'compile', 'serve'))
    parser.add_argument('path', type=Path)
    parser.add_argument('--tools', type=Path)
    ns = parser.parse_args()
    signal.signal(signal.SIGTERM, lambda *_: (_ for _ in ()).throw(SystemExit(143)))
    try:
        if ns.action == 'prepare':
            print(prepare(ns.path.resolve()))
        elif ns.action == 'configure':
            configure(ns.path.resolve())
        elif ns.action == 'compile':
            compile_preview(ns.path.resolve())
        else:
            serve(ns.path.resolve(), ns.tools.resolve())
    except KeyboardInterrupt:
        pass
