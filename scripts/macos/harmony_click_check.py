#!/usr/bin/env python3
"""Collect device-reported click latency; never substitute host command duration."""
import argparse
import datetime
import json
from pathlib import Path
import re
import subprocess
import sys


def select_device(output, requested=None):
    devices = [line.strip() for line in output.splitlines() if line.strip()]
    if any(line.startswith('[') or 'failed' in line.lower() for line in devices) or not devices:
        raise ValueError('未连接可用设备。请连接鸿蒙真机、开启 USB 调试并在手机授权。')
    if requested:
        if requested not in devices:
            raise ValueError('指定设备未连接。')
        return requested
    if len(devices) != 1:
        raise ValueError('连接了多台设备，请用 --device 选择。')
    return devices[0]


def parse_result(output, threshold=250):
    # SmartPerf reports system touch-to-first-frame timing, not Python/hdc runtime.
    matches = re.findall(r'(?<![\w])ResponseTime\s*:\s*([0-9]+(?:\.[0-9]+)?)\s*ms\b', output)
    if len(matches) != 1 or float(matches[0]) <= 0:
        raise ValueError('未得到唯一有效 ResponseTime；本次不判定通过。检查设备 SmartPerf 支持、输出和实际点击。')
    response = float(matches[0])
    return {'responseMs': response, 'thresholdMs': threshold, 'passed': response <= threshold}


def main(argv=None):
    parser = argparse.ArgumentParser(description='鸿蒙真机点击响应采集；不安装、卸载或清除应用数据。')
    default_hdc = Path(__file__).resolve().parents[2] / '.local-tools/harmony/6.1.1/sdk/default/openharmony/toolchains/hdc'
    parser.add_argument('--hdc', type=Path, default=default_hdc)
    parser.add_argument('--device')
    parser.add_argument('--threshold-ms', type=float, default=250, help='本次审核目标250ms，可另设100观察通用体验目标')
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--probe', action='store_true', help='检查设备连接与设备自带性能工具')
    mode.add_argument('--measure', metavar='场景名', help='启动采集后，在手机手动点击一次目标控件')
    parser.add_argument('--out', type=Path, default=Path('.local-tools/harmony-performance'))
    args = parser.parse_args(argv)
    if not 0 < args.threshold_ms < float('inf'):
        parser.error('threshold-ms 必须是有限正数。')
    if not args.hdc.is_file():
        parser.error('找不到 hdc，请挂载官方工具链或通过 --hdc 指定。')
    try:
        listing = subprocess.run([str(args.hdc), 'list', 'targets'], capture_output=True, text=True, timeout=15, check=True)
        device = select_device(listing.stdout, args.device)
        command = [str(args.hdc), '-t', device, 'shell', 'SP_daemon']
        if args.probe:
            result = subprocess.run(command + ['--help'], capture_output=True, text=True, timeout=15, check=True)
            if 'SP_daemon' not in result.stdout or re.search(r'not found|inaccessible|permission denied', result.stdout, re.I):
                raise ValueError('设备未提供可调用的 SmartPerf；请使用 DevEco Profiler 或华为云测试。')
            print('设备已连接；SmartPerf 输出如下。具体 -editor timeDelay 支持仍以实际采集为准：')
            print(result.stdout)
            return 0
        args.out.mkdir(parents=True, exist_ok=True, mode=0o700)
        stamp = datetime.datetime.now().strftime('%Y%m%d-%H%M%S-%f')
        raw = args.out / (stamp + '.txt')
        print('请先停留在待测页面。采集开始后等待 1–2 秒，在手机点击一次目标控件；不要同时做其他操作。', flush=True)
        result = subprocess.run(command + ['-editor', 'timeDelay'], capture_output=True, text=True, timeout=45)
        output = result.stdout + result.stderr
        raw.write_text(output)
        raw.chmod(0o600)
        if result.returncode:
            raise ValueError('设备采集命令失败，原始输出保存在 ' + str(raw))
        try:
            summary = parse_result(output, args.threshold_ms)
        except ValueError as error:
            raise ValueError(str(error) + ' 原始输出：' + str(raw)) from error
        summary['scenario'] = args.measure
        report = raw.with_suffix('.json')
        report.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')
        report.chmod(0o600)
        print(json.dumps(summary, ensure_ascii=False))
        print('报告：' + str(report))
        return 0 if summary['passed'] else 1
    except subprocess.TimeoutExpired:
        print('设备命令超时，本次未取得有效测量。', file=sys.stderr)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(str(error), file=sys.stderr)
    return 2


if __name__ == '__main__':
    sys.exit(main())
