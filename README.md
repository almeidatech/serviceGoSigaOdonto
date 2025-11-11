# Olmeda Service

Projeto Go para monitoramento e envio de eventos via WebSocket a partir de um banco Microsoft Access (ODBC). O serviço pode rodar em modo console (desenvolvimento) ou ser instalado/rodado como serviço Windows.

## Visão geral

- Entrada principal: `cmd/service/main.go` — executável serviço/console.
- Monitoramento e lógica principal: `internal/service/monitor.go`.
- Banco: usa ODBC para conectar a um arquivo Microsoft Access (.mdb/.accdb) através do driver `Microsoft Access Driver (*.mdb, *.accdb)`.
- Comunicação: cliente WebSocket implementado em `internal/websocket` (usa `github.com/gorilla/websocket`).

## Requisitos

- Windows (o código usa `golang.org/x/sys/windows/svc` para suportar execução como serviço).
- Go (versão indicada no `go.mod`: 1.23.4; use a versão instalada na sua máquina — use `go version`).
- Microsoft Access ODBC Driver instalado (parte do Microsoft Access Database Engine/Office): "Microsoft Access Driver (*.mdb, *.accdb)".
- Arquivo de banco Access acessível, por padrão o projeto usa `D:\teste\odontoDB.mdb` (veja `config.json`).
- Permissões de leitura/escrita no arquivo de banco e no diretório de logs configurado.

## Dependências

As dependências principais estão no `go.mod`:

- `github.com/alexbrainman/odbc` — driver ODBC para Go
- `github.com/gorilla/websocket` — WebSocket
- algumas libs indiretas para utilitários

Instale dependências com `go mod download` (no diretório do projeto):

```powershell
cd /d D:\serviceGo
go mod download
```

## Como compilar

Usando o Go toolchain (padrão):

```powershell
cd /d D:\serviceGo
go build -o bin\olmeda-service.exe ./cmd/service
```

Ou usar o `build.bat` que já existe no repositório (verifique o conteúdo e adapte se necessário):

```powershell
.\build.bat
```

## Arquivo de configuração (`config.json`)

Ao primeiro run, se não existir, o binário cria um `config.json` com valores padrão. Principais campos:

- `logLevel`: nível de log (ex.: `info`)
- `logPath`: caminho do arquivo de log (ex.: `D:/serviceGo/olmeda-service.log`)
- `development`: opções para desenvolvimento (auto restart, debug logs)
- `websocket`: configuração do cliente WebSocket
  - `enabled` (bool)
  - `serverUrl` (string) — ex.: `wss://...`
  - `authToken`, `clinicId`, `apiKey`, `instanceId`
  - `port`, `reconnectInterval`, `pingInterval`, `useSSL`
- `database`:
  - `dsn` (string) — DSN ODBC para o Access; por padrão: `Driver={Microsoft Access Driver (*.mdb, *.accdb)};DBQ=D:\teste\odontoDB.mdb;DefaultDir=D:\teste;`
  - `maxConns` (int)

Exemplo de `config.json` (gerado automaticamente quando ausente):

```json
{
    "logLevel": "info",
    "logPath": "<diretorio-do-executavel>/olmeda-service.log",
    "updateUrl": "http://localhost:8080/update",
    "development": {
        "enabled": true,
        "autoRestart": true,
        "debugLog": true
    },
    "websocket": {
        "enabled": true,
        "serverUrl": "wss://odonto-olmeda-production.up.railway.app/ws",
        "authToken": "",
        "port": 8081,
        "reconnectInterval": 30,
        "pingInterval": 30,
        "clinicId": "",
        "apiKey": "",
        "instanceId": "",
        "useSSL": true
    },
    "database": {
        "dsn": "Driver={Microsoft Access Driver (*.mdb, *.accdb)};DBQ=D:\\teste\\odontoDB.mdb;DefaultDir=D:\\teste;",
        "maxConns": 10
    }
}
```

Atenção: em ambiente de produção, garanta que `websocket.useSSL` seja `true` e que `authToken`, `clinicId` e `apiKey` estejam corretamente preenchidos.

## Como rodar (modo console)

Útil para desenvolvimento e debugging. Roda o serviço diretamente em foreground:

```powershell
cd /d D:\serviceGo
# Executar com go run (necessário ter dependências instaladas)
go run ./cmd/service

# ou executar o binário compilado
bin\olmeda-service.exe
```

Quando rodando em modo console, pressione Ctrl+C para encerrar. O serviço imprime logs no `logPath` configurado.

## Executar como Serviço do Windows

O binário foi escrito para suportar execução como serviço Windows (`svc`). Para instalar como serviço você pode usar `sc.exe` ou ferramentas como `nssm`:

Exemplo com `sc.exe` (execute em PowerShell como Administrador e ajuste o caminho do binário):

```powershell
sc create OlmedaService binPath= "C:\\caminho\\para\\olmeda-service.exe"
sc start OlmedaService
sc stop OlmedaService
sc delete OlmedaService
```

Observação: a criação/execução do serviço requer privilégios de administrador.

## Outros comandos/artefatos disponíveis

Na raiz do repositório existem scripts utilitários:

- `build.bat` — empacota/compila (verifique conteúdo)
- `start.bat` — script para iniciar o binário
- `set_env.bat` — define variáveis de ambiente usadas pelos scripts
- `generate-certs.bat` / `certs/` — scripts e certificados (se aplicável ao ambiente)
- `test-wss.bat` / `wstest/` — utilitários para testar WebSocket localmente

## Estrutura importante do projeto

- `cmd/service/` — código principal do serviço
- `wsserver/` — servidor WebSocket local (para testes)
- `wstest/` — utilitário de teste WebSocket
- `internal/` — pacotes internos: `config`, `database`, `models`, `service`, `websocket`, `updater`
- `build/`, `bin/`, `logs/`, `certs/` — artefatos e recursos

## Troubleshooting (problemas comuns)

- Erro ao conectar ao banco (ODBC):
  - Verifique se o driver ODBC do Access está instalado.
  - Verifique se o arquivo `.mdb` existe no caminho configurado (`database.dsn`) e as permissões de leitura/escrita.
  - Teste a conexão ODBC fora do app (ODBC Data Source Administrator) para garantir que o driver funcione.

- Erro ao abrir arquivo de log:
  - Verifique `logPath` e permissões do diretório. O serviço cria o diretório se necessário, mas o usuário que roda o serviço precisa ter permissão.

- WebSocket não conecta:
  - Verifique `websocket.serverUrl`, `authToken`, `clinicId`, `apiKey` e `useSSL`.
  - Tente rodar `wstest` local para reproduzir o comportamento.

- Execução como serviço falha:
  - Verifique logs gerados no `logPath` para mensagens de erro.
  - Confirme que o serviço foi criado com o caminho completo do executável e que o usuário do serviço tem permissões.

## Como contribuir / desenvolvimento

- Execute `go mod download` para preparar o ambiente.
- Rode em modo console (`go run ./cmd/service`) para desenvolvimento iterativo.
- Adicione testes em `internal/*` e utilize `go test ./...`.

## Contato

Se precisar de ajuda para configurar o Access DB ou a integração WebSocket, informe: ambiente (Windows versão), localização do arquivo `.mdb` e logs relevantes (`logs/` ou `logPath`).

---

README gerado automaticamente pelo assistente. Se quiser eu adapto com mais detalhes (comandos do `build.bat`, política de logs, exemplo de `sc` mais detalhado ou um `install-service.ps1`).
