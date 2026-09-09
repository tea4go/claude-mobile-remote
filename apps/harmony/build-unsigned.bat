@echo off

powershell.exe -ExecutionPolicy Bypass -File "%~dp0build-unsigned.ps1"
pause
