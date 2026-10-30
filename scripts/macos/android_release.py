#!/usr/bin/env python3
"""Create an explicit first-release identity, or sign an existing release APK."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import pty
import secrets
import shutil
import ssl
import subprocess
import tempfile
import termios
import zipfile

REPO = Path(__file__).resolve().parents[2]
APP_ID = 'com.tea4go.claudemobileremote.android'
DEFAULT_CONFIG = Path.home() / '.config/claude-mobile-remote/android-signing/signing.json'


def run(args, **kwargs):
    subprocess.run([str(arg) for arg in args], check=True, **kwargs)


def java_tool(name):
    home = Path(os.environ.get('JAVA_HOME', str(Path.home() / 'jdk')))
    path = home / 'bin' / name
    if not path.is_file():
        raise SystemExit('Set JAVA_HOME to an installed JDK before signing.')
    return path


def outside_repo(path):
    path = path.expanduser().resolve()
    if path == REPO or REPO in path.parents:
        raise SystemExit('Signing material must be stored outside the repository.')
    return path


def initialize(config_path):
    folder = outside_repo(config_path).parent
    # Never overwrite or silently rotate a release identity.
    folder.mkdir(mode=0o700, parents=True, exist_ok=False)
    password = secrets.token_urlsafe(36)
    store = folder / 'whale-android-release.jks'
    env = dict(os.environ, CMR_KEYSTORE_PASSWORD=password)
    run([java_tool('keytool'), '-genkeypair', '-keystore', store, '-storetype', 'JKS',
         '-alias', 'whale-android-release', '-keyalg', 'RSA', '-keysize', '3072',
         '-sigalg', 'SHA256withRSA', '-validity', '10000',
         '-dname', 'CN=Whale Remote Control',
         '-storepass:env', 'CMR_KEYSTORE_PASSWORD', '-keypass:env', 'CMR_KEYSTORE_PASSWORD'], env=env)
    store.chmod(0o600)
    config = dict(applicationId=APP_ID, keystore=str(store), alias='whale-android-release',
                  storePassword=password, keyPassword=password)
    with config_path.open('x') as output:
        json.dump(config, output, indent=2)
        output.write('\n')
    config_path.chmod(0o600)
    run([java_tool('keytool'), '-exportcert', '-rfc', '-keystore', store,
         '-alias', config['alias'], '-storepass:env', 'CMR_KEYSTORE_PASSWORD',
         '-file', folder / 'release-certificate.pem'], env=env)
    backup = folder / 'local-backup'
    backup.mkdir(mode=0o700)
    for source in (store, config_path):
        shutil.copy2(source, backup / source.name)
    print('Created private Android signing identity and local backup:', folder)
    print('Keep an additional backup on a separate trusted device.')


def sign(config_path, apk_path):
    config_path = outside_repo(config_path)
    if config_path.stat().st_mode & 0o077:
        raise SystemExit('Signing config must have mode 0600.')
    config = json.loads(config_path.read_text())
    if config.get('applicationId') != APP_ID:
        raise SystemExit('Signing config belongs to another Android application.')
    store = outside_repo(Path(config['keystore']))
    if store.stat().st_mode & 0o077:
        raise SystemExit('Keystore must have mode 0600.')
    sdk = Path(os.environ.get('ANDROID_HOME', os.environ.get('ANDROID_SDK_ROOT', '')))
    tools = sdk / 'build-tools' / '36.0.0'
    if not (tools / 'apksigner').is_file():
        raise SystemExit('Android Build Tools 36.0.0 are required.')
    with zipfile.ZipFile(apk_path) as apk:
        native = json.loads(apk.read('assets/capacitor.config.json'))
        if native.get('appId') != APP_ID or native.get('appName') != 'Whale Remote Control':
            raise SystemExit('Unexpected Capacitor Android identity.')
        if native.get('server', {}).get('url') or native.get('android', {}).get('webContentsDebuggingEnabled') is not False:
            raise SystemExit('APK contains a development server or WebView debugging.')
    badging = subprocess.check_output([str(tools / 'aapt'), 'dump', 'badging', str(apk_path)], text=True)
    expected = f"package: name='{APP_ID}' versionCode='1' versionName='1.0.0'"
    if expected not in badging or 'application-debuggable' in badging or 'POST_NOTIFICATIONS' in badging:
        raise SystemExit('APK identity/version/debug/notification validation failed.')
    output_dir = REPO / '.local-tools/android-release'
    output_dir.mkdir(parents=True, exist_ok=True)
    output = output_dir / 'WhaleRemoteControl-1.0.0-android-signed.apk'
    env = dict(os.environ, CMR_STORE_PASSWORD=config['storePassword'], CMR_KEY_PASSWORD=config['keyPassword'],
               JAVA_HOME=str(java_tool('java').parent.parent))
    with tempfile.TemporaryDirectory(prefix='cmr-android-sign-') as temporary:
        aligned = Path(temporary) / 'aligned.apk'
        signed = Path(temporary) / 'signed.apk'
        run([tools / 'zipalign', '-P', '16', '-f', '4', apk_path, aligned])
        run([tools / 'apksigner', 'sign', '--ks', store, '--ks-key-alias', config['alias'],
             '--ks-pass', 'env:CMR_STORE_PASSWORD', '--key-pass', 'env:CMR_KEY_PASSWORD',
             '--v4-signing-enabled', 'false', '--out', signed, aligned], env=env)
        run([tools / 'apksigner', 'verify', '--verbose', '--print-certs', signed], env=env)
        run([tools / 'zipalign', '-c', '-P', '16', '4', signed])
        shutil.copy2(signed, output)
    print('Signed APK:', output)
    print('SHA256:', hashlib.sha256(output.read_bytes()).hexdigest())


def export_agc(config_path, pepk, encryption_key):
    config_path = outside_repo(config_path)
    if config_path.stat().st_mode & 0o077:
        raise SystemExit('Signing config must have mode 0600.')
    config = json.loads(config_path.read_text())
    if config.get('applicationId') != APP_ID:
        raise SystemExit('Signing config belongs to another Android application.')
    store = outside_repo(Path(config['keystore']))
    if store.stat().st_mode & 0o077:
        raise SystemExit('Keystore must have mode 0600.')
    output = config_path.parent / 'agc-encrypted-signing-key.zip'
    if output.exists():
        raise SystemExit('Encrypted export already exists; refusing to overwrite it.')
    # PEPK requires java.io.Console. Supply a private PTY with echo disabled,
    # never putting passwords in command arguments, shell expansion or logs.
    master, slave = pty.openpty()
    attributes = termios.tcgetattr(slave)
    attributes[3] &= ~termios.ECHO
    termios.tcsetattr(slave, termios.TCSANOW, attributes)
    process = subprocess.Popen([str(java_tool('java')), '-jar', str(pepk.resolve()),
                                '--keystore', str(store), '--alias', config['alias'],
                                '--output=' + str(output), '--encryptionkey=' + encryption_key,
                                '--include-cert'], stdin=slave, stdout=slave, stderr=slave)
    os.close(slave)
    os.write(master, (config['storePassword'] + '\n' + config['keyPassword'] + '\n').encode())
    captured = bytearray()
    try:
        while True:
            try:
                chunk = os.read(master, 4096)
            except OSError:
                break
            if not chunk:
                break
            captured.extend(chunk)
    finally:
        os.close(master)
    if process.wait():
        message = captured.decode(errors='replace')
        for secret in (config['storePassword'], config['keyPassword']):
            message = message.replace(secret, '[REDACTED]')
        raise SystemExit('PEPK export failed: ' + message)
    output.chmod(0o600)
    with zipfile.ZipFile(output) as archive:
        if archive.testzip() is not None or 'encryptedPrivateKey' not in archive.namelist():
            raise SystemExit('Invalid PEPK encrypted archive.')
        certificate = archive.read('certificate.pem').decode('ascii')
        expected = (config_path.parent / 'release-certificate.pem').read_text()
        if ssl.PEM_cert_to_DER_cert(certificate) != ssl.PEM_cert_to_DER_cert(expected):
            raise SystemExit('PEPK certificate does not match release certificate.')
    print('Encrypted AGC export:', output)
    print('Public certificate SHA256:', hashlib.sha256(ssl.PEM_cert_to_DER_cert(certificate)).hexdigest())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, default=DEFAULT_CONFIG)
    parser.add_argument('--init-signing', action='store_true')
    parser.add_argument('--apk', type=Path)
    parser.add_argument('--pepk', type=Path)
    parser.add_argument('--encryption-key')
    args = parser.parse_args()
    if args.init_signing:
        initialize(outside_repo(args.config))
    elif args.apk:
        sign(args.config, args.apk.resolve())
    elif args.pepk and args.encryption_key:
        export_agc(args.config, args.pepk, args.encryption_key)
    else:
        parser.error('Use --init-signing explicitly, or provide --apk.')


if __name__ == '__main__':
    main()
