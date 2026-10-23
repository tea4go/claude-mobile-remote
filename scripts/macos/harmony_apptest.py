#!/usr/bin/env python3
"""Release APP build in a disposable private project; credentials never enter the source tree."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import secrets
import signal
import ssl
import subprocess
import tempfile
import zipfile
from harmony_preview import prepare


def run(args, cwd, secrets=(), env=None):
    process = subprocess.Popen(args, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, start_new_session=True)
    try:
        raw_output, _ = process.communicate()
    except BaseException:
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
        raise
    output = raw_output.decode(errors='replace')
    for secret in secrets:
        output = output.replace(secret, '<redacted>')
    print(output, end='', flush=True)
    if process.returncode:
        raise RuntimeError('工具执行失败，退出码 ' + str(process.returncode))
    return raw_output


def read_json5(path):
    plugin = Path(os.environ['HARMONY_TOOLS_HOME']) / 'hvigor/hvigor-ohos-plugin'
    result = subprocess.run(['node', '-e', 'process.stdout.write(JSON.stringify(require(process.argv[1]).parse(require("fs").readFileSync(process.argv[2],"utf8"))))',
                             str(plugin / 'node_modules/json5'), str(path)], check=True, capture_output=True)
    return json.loads(result.stdout)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('repo', type=Path)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--unsigned', action='store_true')
    group.add_argument('--config', type=Path)
    group.add_argument('--init-signing', type=Path)
    ns = parser.parse_args()
    repo = ns.repo.resolve()
    if ns.init_signing:
        target = ns.init_signing.expanduser().resolve()
        if repo == target or repo in target.parents:
            raise ValueError('签名目录必须位于仓库外')
        target.mkdir(mode=0o700, parents=True, exist_ok=False)
        password = target / 'password.txt'
        password.write_text(secrets.token_urlsafe(32) + '\n')
        os.chmod(password, 0o600)
        keytool = str(Path(os.environ['JAVA_HOME']) / 'bin/keytool')
        store = target / 'release.p12'
        args = [keytool, '-genkeypair', '-alias', 'cmr-release', '-keyalg', 'EC', '-groupname', 'secp256r1',
                '-sigalg', 'SHA256withECDSA', '-dname', 'CN=Claude Mobile Remote Release,O=Personal Development',
                '-validity', '9125', '-storetype', 'PKCS12', '-keystore', str(store), '-storepass:file', str(password)]
        run(args, target)
        csr = target / 'release.csr'
        run([keytool, '-certreq', '-alias', 'cmr-release', '-sigalg', 'SHA256withECDSA', '-keystore', str(store),
             '-storepass:file', str(password), '-file', str(csr)], target)
        run(['openssl', 'req', '-in', str(csr), '-noout', '-verify'], target)
        config = {'storeFile': 'release.p12', 'certpath': 'release.cer', 'profile': 'release.p7b', 'keyAlias': 'cmr-release',
                  'storePasswordFile': 'password.txt', 'keyPasswordFile': 'password.txt'}
        (target / 'signing.json').write_text(json.dumps(config, indent=2) + '\n')
        for path in target.iterdir():
            path.chmod(0o600)
        print('CSR：' + str(csr))
        print('请将CSR提交到华为后台申请发布证书；私钥和密码留在本机，不上传。')
        return
    materials = None
    passwords = []
    if ns.config:
        config = ns.config.resolve()
        if repo in config.parents:
            raise ValueError('签名配置必须位于仓库外')
        if config.stat().st_mode & 0o077:
            raise ValueError('签名配置权限应为0600')
        materials = json.loads(config.read_text())
        for key in ('storeFile', 'certpath', 'profile', 'storePasswordFile', 'keyPasswordFile'):
            path = Path(materials[key]).expanduser()
            if not path.is_absolute():
                path = config.parent / path
            path = path.resolve(strict=True)
            if repo in path.parents:
                raise ValueError('签名材料必须位于仓库外：' + key)
            if path.stat().st_mode & 0o077:
                raise ValueError('签名材料权限应为0600：' + key)
            materials[key] = str(path)
        for key in ('storePasswordFile', 'keyPasswordFile'):
            password = Path(materials[key]).read_text().rstrip('\r\n')
            if not password:
                raise ValueError('密码文件不能为空')
            passwords.append(password)
        if not materials.get('keyAlias'):
            raise ValueError('缺少keyAlias')
    work_root = repo / '.local-tools/harmony-apptest'
    work_root.mkdir(parents=True, exist_ok=True)
    os.chmod(work_root, 0o700)
    output_dir = work_root / ('unsigned' if ns.unsigned else 'signed')
    # Only report the artifact created by this invocation; old output is not a success fallback.
    with tempfile.TemporaryDirectory(prefix='cmr-harmony-apptest-') as temporary:
        project = prepare(repo, Path(temporary), preview=False)
        profile = read_json5(project / 'build-profile.json5')
        profile['app']['signingConfigs'] = []
        if materials:
            jar = Path(os.environ['HARMONY_TOOLS_HOME']) / 'sdk/default/openharmony/toolchains/lib/hap-sign-tool.jar'
            decoded = project / 'verified-profile.json'
            run([str(Path(os.environ['JAVA_HOME']) / 'bin/java'), '-jar', str(jar), 'verify-profile', '-inFile', materials['profile'], '-outFile', str(decoded)], project, passwords)
            # Extract CMS content without asserting independent root trust.
            content = subprocess.run(['openssl', 'cms', '-verify', '-inform', 'DER', '-in', materials['profile'], '-noverify'], check=True, capture_output=True).stdout
            provision = json.loads(content)
            if provision.get('type') != 'release':
                raise ValueError('AppTest发布构建要求release Profile，拒绝调试Profile')
            bundle = provision.get('bundle-info', {})
            if bundle.get('bundle-name') != 'com.tea4go.claudemobileremote':
                raise ValueError('Profile包名与工程不匹配')
            if provision.get('app-distribution-type') != 'app_gallery':
                raise ValueError('需应用市场发布Profile(app_gallery)，不接受指定设备或企业分发Profile')
            signing_dir = project / 'private-signing'
            signing_dir.mkdir(mode=0o700)
            temporary_store = signing_dir / 'release.p12'
            shutil.copyfile(materials['storeFile'], temporary_store)
            temporary_store.chmod(0o600)
            encrypted = subprocess.run(
                ['node', str(repo / 'scripts/macos/harmony_signing_material.js'), str(signing_dir)],
                input=json.dumps(passwords), text=True, check=True, capture_output=True)
            store_password, key_password = json.loads(encrypted.stdout)
            material = {key: materials[key] for key in ('certpath', 'profile', 'keyAlias')}
            material.update(storeFile=str(temporary_store), storePassword=store_password,
                            keyPassword=key_password, signAlg='SHA256withECDSA')
            profile['app']['signingConfigs'] = [{'name': 'apptest', 'type': 'HarmonyOS', 'material': material}]
        for product in profile['app']['products']:
            if materials:
                product['signingConfig'] = 'apptest'
            else:
                product.pop('signingConfig', None)
        config_path = project / 'build-profile.json5'
        config_path.write_text(json.dumps(profile))
        os.chmod(config_path, 0o600)
        run([os.environ['HARMONY_OHPM'], 'install', '--all'], project, passwords)
        build_env = os.environ.copy()
        if materials:
            # The installed JDK 21.0.0 rejects valid ZIP64 metadata during sign-app.
            # The completed APP and every embedded HAP are independently ZIP-checked below.
            version = subprocess.run([str(Path(os.environ['JAVA_HOME']) / 'bin/java'), '-version'],
                                     capture_output=True, text=True, check=True)
            if re.search(r'\bversion "21(?:\.0\.0)?"', version.stdout + version.stderr):
                build_env['JAVA_TOOL_OPTIONS'] = (build_env.get('JAVA_TOOL_OPTIONS', '') +
                    ' -Djdk.util.zip.disableZip64ExtraFieldValidation=true').strip()
        run([os.environ['HARMONY_HVIGORW'], '--mode', 'project', '-p', 'product=default', '-p', 'buildMode=release', 'assembleApp', '--no-daemon'], project, passwords, env=build_env)
        suffix = '-unsigned.app' if ns.unsigned else '-signed.app'
        artifacts = list((project / 'build/outputs/default').glob('*' + suffix))
        if len(artifacts) != 1:
            raise RuntimeError('未找到本轮唯一APP产物')
        artifact = artifacts[0]
        with zipfile.ZipFile(artifact) as app:
            if app.testzip() is not None:
                raise ValueError('APP压缩校验失败')
            haps = [name for name in app.namelist() if name.endswith('.hap')]
            if not haps or 'pack.info' not in app.namelist():
                raise ValueError('APP缺少HAP或pack.info')
            for index, name in enumerate(haps):
                data = app.read(name)
                with zipfile.ZipFile(io.BytesIO(data)) as hap:
                    if hap.testzip() is not None:
                        raise ValueError('内嵌HAP压缩校验失败')
                    module = json.loads(hap.read('module.json'))
                    info = module['app']
                    if info['bundleName'] != 'com.tea4go.claudemobileremote' or info.get('debug', False) or info.get('buildMode') != 'release':
                        raise ValueError('内嵌HAP包名或Release模式不匹配')
                    if info['minAPIVersion'] != 60101024:
                        raise ValueError('最低API不符合工程基准')
        if materials:
            # Normal APP packages carry unsigned HAPs; verify the signed APP itself.
            extracted_profile = project / 'verify-app.p7b'
            run([str(Path(os.environ['JAVA_HOME']) / 'bin/java'), '-jar', str(jar), 'verify-app', '-inFile', str(artifact),
                 '-outCertChain', str(project / 'verify-app.cer'), '-outProfile', str(extracted_profile)], project, passwords)
            if extracted_profile.read_bytes() != Path(materials['profile']).read_bytes():
                raise ValueError('APP内嵌Profile与指定材料不一致')
            def certificates(path):
                return {ssl.PEM_cert_to_DER_cert(block.decode()) for block in
                        re.findall(b'-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----',
                                   Path(path).read_bytes(), re.DOTALL)}
            if not certificates(project / 'verify-app.cer') & certificates(materials['certpath']):
                raise ValueError('APP签名证书与指定发布证书不一致')
        output_dir.mkdir(parents=True, exist_ok=True)
        destination = output_dir / ('ClaudeMobileRemote' + suffix)
        shutil.copy2(artifact, destination)
        print('APP：' + str(destination))
        print('SHA256：' + hashlib.sha256(destination.read_bytes()).hexdigest())
        print('仅验证编译，未签名，不能上传分发。' if ns.unsigned else '发布包本机校验完成；后台接收、AppTest分发及设备安装仍需验证。')


if __name__ == '__main__':
    os.umask(0o077)
    signal.signal(signal.SIGTERM, lambda *_: (_ for _ in ()).throw(SystemExit(143)))
    try:
        main()
    except (ValueError, OSError, KeyError, RuntimeError, subprocess.CalledProcessError) as error:
        # Avoid dumping configuration/password-bearing stack frames.
        raise SystemExit('AppTest构建失败：' + str(error))
