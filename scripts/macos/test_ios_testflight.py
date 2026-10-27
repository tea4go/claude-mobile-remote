import contextlib
import importlib.util
import io
import json
import plistlib
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('ios_testflight', Path(__file__).with_name('ios_testflight.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseSafetyTests(unittest.TestCase):
    def test_external_export_and_upload_options(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'options.plist'
            for destination in ['export', 'upload']:
                release.options(path, release.TEAM, destination)
                data = plistlib.loads(path.read_bytes())
                self.assertEqual(data['method'], 'app-store-connect')
                self.assertEqual(data['destination'], destination)
                self.assertFalse(data['testFlightInternalTestingOnly'])
                self.assertFalse(data['manageAppVersionAndBuildNumber'])

    def test_rejects_non_public_or_credential_urls(self):
        for url in ['http://testflight.apple.com/join/abc', 'https://example.com/join/abc', 'https://testflight.apple.com/join/abc?token=secret']:
            with patch.object(sys, 'argv', ['release', 'qr', '--url', url, '--output', '/tmp/unused-qr.png']), patch.object(release, 'run') as run, contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit):
                    release.main()
                run.assert_not_called()

    def test_upload_requires_verified_build_before_any_command(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(sys, 'argv', ['release', 'upload', '--output', directory]), patch.object(release, 'run') as run:
            with self.assertRaises(FileNotFoundError):
                release.main()
            run.assert_not_called()

    def test_build_rejects_existing_output_without_running_tools(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(sys, 'argv', ['release', 'build', '--build-number', '2', '--output', directory]), patch.object(release, 'run') as run, contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                release.main()
            run.assert_not_called()

    def test_verify_rejects_development_server(self):
        with tempfile.TemporaryDirectory() as directory:
            ipa = Path(directory) / 'App.ipa'
            with zipfile.ZipFile(ipa, 'w') as package:
                package.writestr('Payload/App.app/Info.plist', plistlib.dumps({'CFBundleIdentifier': release.BUNDLE}))
                package.writestr('Payload/App.app/capacitor.config.json', json.dumps({'server': {'url': 'http://localhost:5173'}}))
            with self.assertRaisesRegex(ValueError, 'Development server'):
                release.verify(ipa, Path(directory))


if __name__ == '__main__':
    unittest.main()
