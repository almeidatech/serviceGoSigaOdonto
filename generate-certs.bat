@echo off
echo Gerando certificados...

REM Criar diretório para os certificados se não existir
if not exist certs mkdir certs

REM Precisamos do OpenSSL instalado. Vamos verificar:
where openssl >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo OpenSSL não encontrado! Por favor, instale o OpenSSL e adicione ao PATH
    exit /b 1
)

REM Gerar chave privada para a CA
openssl genrsa -out certs/ca.key 2048

REM Gerar certificado CA
openssl req -x509 -new -nodes -key certs/ca.key -sha256 -days 1024 -out certs/ca.pem -subj "/C=BR/ST=DF/L=Brasilia/O=OdontoTest/CN=OdontoCA"

REM Gerar chave privada para o servidor
openssl genrsa -out certs/server.key 2048

REM Gerar CSR
openssl req -new -key certs/server.key -out certs/server.csr -subj "/C=BR/ST=DF/L=Brasilia/O=OdontoTest/CN=localhost"

REM Criar arquivo de configuração para o certificado
echo authorityKeyIdentifier=keyid,issuer > certs/server.ext
echo basicConstraints=CA:FALSE >> certs/server.ext
echo keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment >> certs/server.ext
echo subjectAltName = @alt_names >> certs/server.ext
echo. >> certs/server.ext
echo [alt_names] >> certs/server.ext
echo DNS.1 = localhost >> certs/server.ext
echo IP.1 = 127.0.0.1 >> certs/server.ext

REM Gerar certificado do servidor
openssl x509 -req -in certs/server.csr -CA certs/ca.pem -CAkey certs/ca.key -CAcreateserial -out certs/server.pem -days 365 -sha256 -extfile certs/server.ext

echo Certificados gerados com sucesso!
