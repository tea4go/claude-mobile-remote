#!/usr/bin/env python3
"""Build/export/upload for external TestFlight; no stored credentials."""
import argparse
import json
import plistlib
import re
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MOBILE = ROOT / 'apps/mobile'
TEAM = '2WTXVQNV99'
BUNDLE = 'com.tea4go.claudemobileremote'


def run(args, **kwargs):
    subprocess.run([str(a) for a in args], check=True, **kwargs)


def options(path, team, destination):
    path.write_bytes(plistlib.dumps(dict(
        method='app-store-connect', destination=destination, teamID=team,
        signingStyle='automatic', manageAppVersionAndBuildNumber=False,
        testFlightInternalTestingOnly=False, uploadSymbols=True)))


def archive_info(archive):
    info = plistlib.loads((archive / 'Info.plist').read_bytes())
    props = info['ApplicationProperties']
    if props['CFBundleIdentifier'] != BUNDLE:
        raise ValueError('Archive Bundle ID mismatch')
    return props


def verify(ipa, dist):
    with zipfile.ZipFile(ipa) as package:
        names = package.namelist()
        infos = [n for n in names if re.fullmatch(r'Payload/[^/]+\.app/Info.plist', n)]
        if len(infos) != 1:
            raise ValueError('Expected one application')
        prefix = infos[0][:-len('Info.plist')]
        info = plistlib.loads(package.read(infos[0]))
        if info['CFBundleIdentifier'] != BUNDLE:
            raise ValueError('IPA Bundle ID mismatch')
        config = json.loads(package.read(prefix + 'capacitor.config.json'))
        if config.get('server', {}).get('url'):
            raise ValueError('Development server URL found')
        for source in dist.rglob('*'):
            if source.is_file() and package.read(prefix + 'public/' + source.relative_to(dist).as_posix()) != source.read_bytes():
                raise ValueError('Embedded web resource mismatch')
        # Decode the profile without logging device IDs or signing data.
        result = subprocess.run(['security', 'cms', '-D'], input=package.read(prefix + 'embedded.mobileprovision'), capture_output=True, check=True)
        signing = plistlib.loads(result.stdout)
        if signing.get('ProvisionedDevices') or signing.get('ProvisionsAllDevices') or signing['Entitlements'].get('get-task-allow'):
            raise ValueError('Expected App Store distribution profile')
        if signing['Entitlements'].get('application-identifier') != TEAM + '.' + BUNDLE:
            raise ValueError('Profile application identifier mismatch')
        app = ipa.parent / 'verification' / prefix
        if app.exists():
            raise ValueError('Verification directory already exists')
        package.extractall(ipa.parent / 'verification')
    run(['codesign', '--verify', '--deep', '--strict', app])
    print('Verified App Store IPA:', info['CFBundleShortVersionString'], info['CFBundleVersion'])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    build = sub.add_parser('build', help='Run web gates, archive and export; does not upload')
    build.add_argument('--build-number', required=True)
    build.add_argument('--output', type=Path, required=True)
    build.add_argument('--packages', type=Path, help='Existing cloned SourcePackages directory')
    upload = sub.add_parser('upload', help='Upload a previously verified output using Xcode account')
    upload.add_argument('--output', type=Path, required=True)
    qr = sub.add_parser('qr', help='Generate PNG from an actual TestFlight public invite URL')
    qr.add_argument('--url', required=True)
    qr.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.command == 'qr':
        if not re.fullmatch(r'https://testflight\.apple\.com/join/[A-Za-z0-9]+', args.url):
            parser.error('Expected an actual https://testflight.apple.com/join/... URL')
        output = args.output.expanduser().resolve()
        if output.exists():
            parser.error('Output already exists')
        output.parent.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='cmr-qr-modules-') as cache:
            run(['swift', '-module-cache-path', cache, ROOT / 'scripts/macos/testflight_qr.swift', args.url, output])
        return
    output = args.output.expanduser().resolve()
    archive = output / 'App.xcarchive'
    if args.command == 'build':
        if not re.fullmatch(r'[1-9][0-9]{0,3}', args.build_number):
            parser.error('Use a positive numeric build number (1–9999)')
        if output.exists():
            parser.error('Output must be a new directory')
        version = subprocess.check_output(['xcodebuild', '-version'], text=True)
        if int(re.search(r'Xcode (\d+)', version)[1]) < 26:
            parser.error('Xcode 26 or newer required')
        import os
        env = dict(os.environ)
        env.pop('CAP_SERVER_URL', None)
        run(['bash', ROOT / 'scripts/macos/4_web_build.sh'], env=env)
        run(['npm', 'exec', '--', 'cap', 'copy', 'ios'], cwd=MOBILE, env=env)
        output.mkdir(parents=True)
        command = ['xcodebuild', '-project', MOBILE / 'ios/App/App.xcodeproj', '-scheme', 'App', '-configuration', 'Release', '-destination', 'generic/platform=iOS', '-derivedDataPath', output / 'DerivedData', '-archivePath', archive, '-allowProvisioningUpdates']
        if args.packages:
            command += ['-clonedSourcePackagesDirPath', args.packages.expanduser().resolve(), '-disableAutomaticPackageResolution']
        run(command + ['archive', 'CURRENT_PROJECT_VERSION=' + args.build_number, 'DEVELOPMENT_TEAM=' + TEAM], env=env)
        archive_info(archive)
        plist = output / 'ExportOptions.plist'
        options(plist, TEAM, 'export')
        run(['xcodebuild', '-exportArchive', '-archivePath', archive, '-exportPath', output / 'export', '-exportOptionsPlist', plist, '-allowProvisioningUpdates'])
        ipas = list((output / 'export').glob('*.ipa'))
        if len(ipas) != 1:
            raise ValueError('Expected one exported IPA')
        verify(ipas[0], MOBILE / 'dist')
        (output / 'verified.json').write_text(json.dumps(dict(bundle=BUNDLE, build=args.build_number, ipa=ipas[0].name)))
        print('Ready for explicit upload:', output)
    else:
        marker = json.loads((output / 'verified.json').read_text())
        props = archive_info(archive)
        if marker['bundle'] != BUNDLE or props['CFBundleVersion'] != marker['build']:
            raise ValueError('Verified build does not match archive')
        plist = output / 'UploadOptions.plist'
        if (output / 'upload').exists():
            raise ValueError('Upload output exists; inspect Apple status before retrying')
        options(plist, TEAM, 'upload')
        run(['xcodebuild', '-exportArchive', '-archivePath', archive, '-exportPath', output / 'upload', '-exportOptionsPlist', plist, '-allowProvisioningUpdates'])
        print('Upload command completed; verify processing and external Beta review in App Store Connect.')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print('TestFlight operation failed:', type(error).__name__, file=sys.stderr)
        sys.exit(1)
