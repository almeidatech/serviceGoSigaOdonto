@echo off
SETLOCAL EnableDelayedExpansion

:: Configurações do build
SET PROJ_NAME=olmeda-service
SET BUILD_DIR=build
SET MAIN_PATH=cmd\service

echo ===================================================
echo         INICIANDO BUILD DO %PROJ_NAME%
echo ===================================================

:: Verifica ambiente Go
echo Verificando ambiente Go...
go version
IF %ERRORLEVEL% NEQ 0 (
    echo ERRO: Go nao esta instalado ou nao esta no PATH
    exit /b 1
)

:: Configuração do ambiente
SET GOOS=windows
SET GOARCH=amd64
SET CGO_ENABLED=1

:: Limpa e recria diretório de build
echo.
echo Preparando diretorios...
IF EXIST "%BUILD_DIR%" rd /s /q "%BUILD_DIR%"
mkdir "%BUILD_DIR%"
mkdir "%BUILD_DIR%\certs"
mkdir "%BUILD_DIR%\logs"

:: Download de dependências
echo.
echo Baixando dependencias...
go mod download
IF %ERRORLEVEL% NEQ 0 (
    echo ERRO: Falha ao baixar dependencias
    exit /b 1
)

:: Compilação
echo.
echo Compilando %PROJ_NAME%...
go build -o "%BUILD_DIR%\%PROJ_NAME%.exe" .\%MAIN_PATH%
IF %ERRORLEVEL% NEQ 0 (
    echo ERRO: Falha na compilacao
    exit /b 1
)

:: Copia arquivos de configuração
echo.
echo Copiando arquivos de configuracao...
IF NOT EXIST "%MAIN_PATH%\config.json" (
    echo ERRO: config.json nao encontrado em %MAIN_PATH%
    exit /b 1
)
copy "%MAIN_PATH%\config.json" "%BUILD_DIR%\" /Y

:: Copia certificados
echo.
echo Copiando certificados...
IF EXIST "certs" (
    copy "certs\*.pem" "%BUILD_DIR%\certs\" /Y
    copy "certs\*.key" "%BUILD_DIR%\certs\" /Y
) ELSE (
    echo AVISO: Diretorio de certificados nao encontrado
)

:: Cria arquivo de teste da instalação
echo.
echo Criando arquivo de teste...
(
echo @echo off
echo cd %%~dp0
echo echo Testando %PROJ_NAME%...
echo echo.
echo echo Verificando diretorios necessarios...
echo if not exist "logs" mkdir logs
echo echo.
echo echo Iniciando servico...
echo .\%PROJ_NAME%.exe
echo pause
) > "%BUILD_DIR%\start.bat"

:: Cria arquivo README com instruções
echo.
echo Criando arquivo README...
(
echo === INSTRUCOES DE INSTALACAO ===
echo.
echo 1. Copie toda esta pasta para o computador destino
echo 2. Execute teste.bat para verificar se o servico esta funcionando
echo 3. Os logs serao gerados na pasta 'logs'
echo 4. Em caso de problemas, verifique:
echo    - Se o arquivo config.json esta configurado corretamente
echo    - Se os certificados estao presentes na pasta certs
echo    - Se o banco de dados esta acessivel no caminho configurado
echo    - Se ha permissao de escrita na pasta logs
) > "%BUILD_DIR%\README.txt"

:: Cria um arquivo .gitkeep na pasta logs para manter a estrutura
echo. > "%BUILD_DIR%\logs\.gitkeep"

echo.
echo ===================================================
echo Build concluido com sucesso!
echo Executavel: %BUILD_DIR%\%PROJ_NAME%.exe
echo ===================================================
echo Para testar, execute teste.bat na pasta %BUILD_DIR%
echo ===================================================

pause
