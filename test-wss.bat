@echo off

REM Gerar certificados se não existirem
if not exist certs\server.pem (
    echo Gerando certificados...
    call generate-certs.bat
)

REM Compilar e iniciar o servidor
echo Compilando e iniciando servidor...
go build -o bin\wsserver.exe cmd\wsserver\main.go
start /B bin\wsserver.exe

REM Aguardar servidor inicializar
timeout /t 2 /nobreak

REM Rodar cliente de teste
echo Executando cliente de teste...
go run cmd\wstest\main.go

REM Encerrar servidor (você precisará fechar manualmente no Windows)
echo Para encerrar o servidor, feche a janela do wsserver ou use o Gerenciador de Tarefas
