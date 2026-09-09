@echo off

powershell.exe -ExecutionPolicy Bypass -File "%~dp0build-signed.ps1"
pause
