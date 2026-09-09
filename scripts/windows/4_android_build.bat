@echo off

powershell.exe -ExecutionPolicy Bypass -File "%~dp04_android_build.ps1" %*
pause
