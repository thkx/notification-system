@echo off

cd "%~dp0.."

echo Starting notification system...
go run cmd/main.go
pause
