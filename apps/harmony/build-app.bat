@echo off

powershell.exe -ExecutionPolicy Bypass -File "%~dp0build-app.ps1" %*
pause
