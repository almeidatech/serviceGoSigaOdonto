@echo off
cd %~dp0
echo Testando olmeda-service...
echo.
echo Verificando diretorios necessarios...
if not exist "logs" mkdir logs
echo.
echo Iniciando servico...
.\olmeda-service.exe
pause
