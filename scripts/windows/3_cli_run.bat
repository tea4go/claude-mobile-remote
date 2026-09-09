@echo off

powershell.exe -ExecutionPolicy Bypass -File "%~dp03_cli_run.ps1" %*
pause
