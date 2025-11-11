// internal/database/access.go
package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/alexbrainman/odbc"
)

type AccessDB struct {
	db *sql.DB
}

func NewAccessDB(dsn string) (*AccessDB, error) {
	// Abrir conexão usando a string DSN fornecida diretamente
	db, err := sql.Open("odbc", dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão com banco: %v", err)
	}

	// Testar conexão
	if err := db.Ping(); err != nil {
		db.Close() // Fechar conexão em caso de erro
		return nil, fmt.Errorf("erro ao testar conexão: %v", err)
	}

	return &AccessDB{db: db}, nil
}

// Close fecha a conexão com o banco
func (a *AccessDB) Close() error {
	return a.db.Close()
}

// GetPendingRecords retorna os agendamentos pendentes de confirmação
func (a *AccessDB) GetPendingRecords() ([]Record, error) {
	rows, err := a.db.Query(QueryPendingRecords)
	if err != nil {
		return nil, fmt.Errorf("erro ao executar query: %v", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var (
			identificacao int64
			dataAgenda    time.Time
			horario       string
			nomePaciente  string
			tipoConsulta  string
			nomeDentista  string
			confirmado    bool
			codigoAgenda  int64
		)

		err := rows.Scan(
			&identificacao,
			&dataAgenda,
			&horario,
			&nomePaciente,
			&tipoConsulta,
			&nomeDentista,
			&confirmado,
			&codigoAgenda,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao ler registro: %v", err)
		}

		// Criar um Agendamento com os dados
		agendamento := &Agendamento{
			Identificacao: identificacao,
			CodigoAgenda:  codigoAgenda,
			Horario:       horario,
			NomePaciente:  nomePaciente,
			TipoConsulta:  tipoConsulta,
			Confirmado:    confirmado,
		}

		// Converter para Record usando nossa função auxiliar
		record := AgendamentoToRecord(agendamento, dataAgenda, nomeDentista)
		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar registros: %v", err)
	}

	return records, nil
}

// ExecuteQuery executa uma query e retorna os resultados
func (a *AccessDB) ExecuteQuery(query string, args ...interface{}) (*sql.Rows, error) {
	return a.db.Query(query, args...)
}

// Função auxiliar para executar uma query de atualização
func (a *AccessDB) executeUpdate(query string, args ...interface{}) error {
	result, err := a.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("erro ao executar update: %v", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("erro ao obter linhas afetadas: %v", err)
	}

	if rows == 0 {
		return fmt.Errorf("nenhum registro foi atualizado")
	}

	return nil
}
