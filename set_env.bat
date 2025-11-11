@echo off
SET CGO_ENABLED=1
SET GOARCH=amd64
SET GOOS=windows

:: Configurações do Projeto
SET OLMEDA_ENV=development
SET OLMEDA_DB_DSN=OlmedaDB
SET OLMEDA_WS_HOST=localhost
SET OLMEDA_WS_PORT=8080

:: Path do Go
SET PATH=%PATH%;C:\Go\bin
SET PATH=%PATH%;%USERPROFILE%\go\bin

echo Ambiente de desenvolvimento configurado!
