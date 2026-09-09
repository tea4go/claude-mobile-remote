@echo off

powershell.exe -ExecutionPolicy Bypass -File "%~dp02_check.ps1" %*
pause
