// internal/database/queries.go
package database

import (
	"time"
)

// Estruturas que representam as tabelas
type Dentista struct {
	Codigo         int64  `json:"codigo"`
	Nome           string `json:"nome"`
	Especializacao string `json:"especializacao"`
}

type Agenda struct {
	Codigo         int64     `json:"codigo"`
	Data           time.Time `json:"data"`
	CodigoDentista int64     `json:"codigo_dentista"`
	Valendo        float64   `json:"valendo"`
}

type Paciente struct {
	Codigo   int64  `json:"codigo"`
	Nome     string `json:"nome"`
	Telefone string `json:"telefone"`
	Email    string `json:"email"`
}

type Agendamento struct {
	Identificacao int64   `json:"identificacao"`
	CodigoAgenda  int64   `json:"codigo_agenda"`
	Horario       string  `json:"horario"`
	AgendaFechada string  `json:"agenda_fechada"`
	PacienteNr    float64 `json:"paciente_nr"`
	NomePaciente  string  `json:"nome_paciente"`
	TipoConsulta  string  `json:"tipo_consulta"`
	Confirmado    bool    `json:"confirmado"`
	Faltou        bool    `json:"faltou"`
}

// Record é a estrutura que usamos para comunicação com o frontend/websocket
type Record struct {
	ID          int                    `json:"id"`
	DataCriacao time.Time              `json:"data_criacao"`
	Status      string                 `json:"status"`
	Dados       map[string]interface{} `json:"dados"`
}

// Queries constantes
const (
	// Query para buscar registros pendentes (não confirmados)
	QueryPendingRecords = `
        SELECT 
            a1.Identificação,
            a.DATA_AGENDA,
            a1.HORARIO_1,
            a1.NomePaciente_,
            a1.tipoconsulta1,
            d.NOMEDENTISTAAG,
            a1.confirma01,
            a1.ConfConsulta,
            a1.CODIGO_AGENDA
        FROM AGENDA1 a1
        INNER JOIN AGENDA a ON a1.CODIGO_AGENDA = a.CÓDIGOAGENDA
        INNER JOIN [DENTISTA AGENDA] d ON a.CODIGODENTISTA = d.CODIGODENTISTA
        WHERE a1.confirma01 = FALSE
        ORDER BY a.DATA_AGENDA, a1.HORARIO_1
    `

	// Query para atualizar status do agendamento
	QueryUpdateStatus = `
        UPDATE AGENDA1
        SET confirma01 = ?,
            ConfConsulta = ?,
            DATAAUDag = ?,
            HORARIOAUDag = ?,
            USUARIOAUDag = 'SISTEMA'
        WHERE Identificação = ?
    `
)

// Métodos do AccessDB
func (db *AccessDB) UpdateRecordStatus(id int64, confirma bool, status string) error {
	currentTime := time.Now()
	dataAud := currentTime.Format("2006-01-02")
	horaAud := currentTime.Format("15:04:05")

	_, err := db.db.Exec(QueryUpdateStatus, confirma, status, dataAud, horaAud, id)
	return err
}

// Função auxiliar para converter um Agendamento em Record
func AgendamentoToRecord(a *Agendamento, dataCriacao time.Time, nomeDentista string) Record {
	return Record{
		ID:          int(a.Identificacao),
		DataCriacao: dataCriacao,
		Status:      map[bool]string{true: "CONFIRMADO", false: "PENDENTE"}[a.Confirmado],
		Dados: map[string]interface{}{
			"codigo_agenda": a.CodigoAgenda,
			"horario":       a.Horario,
			"nome_paciente": a.NomePaciente,
			"tipo_consulta": a.TipoConsulta,
			"dentista":      nomeDentista,
			"confirmado":    a.Confirmado,
		},
	}
}
