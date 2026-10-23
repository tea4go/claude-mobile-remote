#!/usr/bin/env python3
"""Validate local official archives and SDK metadata without executing them."""
import hashlib
from collections import deque
import json
import pathlib
import re
import stat
import sys
import zipfile


def validate_archive(archive, expected):
    if not re.fullmatch(r"[0-9a-fA-F]{64}", expected):
        raise ValueError("必须提供官方下载页的完整 SHA-256")
    digest = hashlib.sha256()
    with open(archive, "rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    if digest.hexdigest() != expected.lower():
        raise ValueError("SHA-256 不匹配，拒绝解压或运行")
    with zipfile.ZipFile(archive) as bundle:
        total = 0
        for entry in bundle.infolist():
            path = pathlib.PurePosixPath(entry.filename)
            if path.is_absolute() or ".." in path.parts or "\\" in entry.filename:
                raise ValueError("ZIP 含不安全路径")
            if stat.S_ISLNK(entry.external_attr >> 16):
                target = bundle.read(entry).decode("utf-8")
                # Resolve relative symlinks lexically; JDK archives may contain them.
                parts = list(path.parent.parts)
                if target.startswith("/") or "\\" in target:
                    raise ValueError("ZIP 含外部符号链接")
                for part in pathlib.PurePosixPath(target).parts:
                    if part == "..":
                        if not parts:
                            raise ValueError("ZIP 符号链接越界")
                        parts.pop()
                    elif part != ".":
                        parts.append(part)
            total += entry.file_size
        # An archive must never extract through a symlink directory.
        symlinks = {e.filename.rstrip("/") for e in bundle.infolist()
                    if stat.S_ISLNK(e.external_attr >> 16)}
        for link in symlinks:
            pending = deque(pathlib.PurePosixPath(link).parts)
            resolved = []
            hops = 0
            while pending:
                part = pending.popleft()
                if part == "..":
                    if not resolved:
                        raise ValueError("ZIP 符号链接链越界")
                    resolved.pop()
                    continue
                resolved.append(part)
                name = "/".join(resolved)
                if name in symlinks:
                    hops += 1
                    if hops > 40:
                        raise ValueError("ZIP 符号链接循环")
                    resolved.pop()
                    target = bundle.read(name).decode("utf-8")
                    pending.extendleft(reversed(pathlib.PurePosixPath(target).parts))
        for entry in bundle.infolist():
            if any(str(p) in symlinks for p in pathlib.PurePosixPath(entry.filename).parents):
                raise ValueError("ZIP 含通过符号链接写入的路径")
    print(f"SHA-256 校验通过；解压内容约 {total / 1024**3:.2f} GiB")
    return total


def validate_sdk(root):
    matches = []
    # HarmonyOS SDK metadata is shipped as sdk-pkg.json or oh-uni-package.json.
    for pattern in ("**/sdk-pkg.json", "**/oh-uni-package.json"):
        for path in pathlib.Path(root).glob(pattern):
            if len(path.relative_to(root).parts) > 6:
                continue
            data = json.loads(path.read_text())
            data = data.get("data", data)
            if str(data.get("apiVersion")) == "24" and str(data.get("version", "")).startswith("6.1.1."):
                matches.append(path)
    if not matches:
        raise ValueError("未找到 API 24 / 6.1.1 的 SDK 元数据；请下载匹配版本")
    print("SDK 核验：6.1.1 / API 24")


def validate_hap(path):
    with zipfile.ZipFile(path) as bundle:
        data = json.loads(bundle.read("module.json"))
    if data.get("app", {}).get("minAPIVersion") != 60101024:
        raise ValueError("HAP 最低 API 不匹配 6.1.1(24)")
    print("HAP 最低 API 核验：60101024")


if __name__ == "__main__":
    try:
        if sys.argv[1] == "archive":
            validate_archive(sys.argv[2], sys.argv[3])
        elif sys.argv[1] == "sdk":
            validate_sdk(sys.argv[2])
        elif sys.argv[1] == "hap":
            validate_hap(sys.argv[2])
        else:
            raise ValueError("未知操作")
    except (ValueError, OSError, zipfile.BadZipFile, IndexError, KeyError) as error:
        sys.exit(str(error))
