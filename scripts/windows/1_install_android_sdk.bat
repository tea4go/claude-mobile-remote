@echo off

powershell.exe -ExecutionPolicy Bypass -File "%~dp01_install_android_sdk.ps1" %*
pause
