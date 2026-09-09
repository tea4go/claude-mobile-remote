@echo off

powershell.exe -ExecutionPolicy Bypass -File "%~dp04_web_build.ps1" %*
pause
