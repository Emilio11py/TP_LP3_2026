package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"tp-backend/internal/models"
	"tp-backend/internal/services"
)

// LoginUsuario verifica las credenciales en la tabla USUARIOS
func LoginUsuario(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Email       string `json:"email"`
		Contrasenha string `json:"contrasenha"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	var u models.Usuario
	query := `SELECT ID_USUARIO, NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA, EMAIL, TELEFONO_CELULAR, ROLE_ACCESO, CARRERA 
	          FROM USUARIOS WHERE EMAIL = ? AND CONTRASENHA = ? AND ACTIVO = 1`
	err := services.DB.QueryRow(query, creds.Email, creds.Contrasenha).Scan(
		&u.IDUsuario, &u.NroCedula, &u.Nombres, &u.Apellidos, &u.CodigoMatricula, &u.Email, &u.TelefonoCelular, &u.RoleAcceso, &u.Carrera,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Credenciales incorrectas", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Error en el servidor", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(u)
}

// ListarSolicitudes consulta SOLICITUDES_CAES unida a DETALLE_SOLICITUDES_CAES
func ListarSolicitudes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	usuarioSolicita := r.URL.Query().Get("id_usuario")
	query := `
		SELECT 
			s.ID_SOLICITUD_CAE, s.FECHA_SOLICITUD, s.HORA_SOLICITUD, s.ID_ACTIVIDAD, 
			s.ID_USUARIO_SOLICITA, s.ESTADO_SOLICITUD, IFNULL(s.OBSERVACIONES, ''),
			IFNULL(d.ACTIVIDAD, ''), IFNULL(d.CARGA_HORARIA_EN_MINUTOS, 0)
		FROM SOLICITUDES_CAES s
		LEFT JOIN DETALLE_SOLICITUDES_CAES d ON s.ID_SOLICITUD_CAE = d.ID_SOLICITUD_CAE
	`

	var rows *sql.Rows
	var err error
	if usuarioSolicita != "" {
		query += " WHERE s.ID_USUARIO_SOLICITA = ?"
		rows, err = services.DB.Query(query, usuarioSolicita)
	} else {
		rows, err = services.DB.Query(query)
	}

	if err != nil {
		http.Error(w, "Error al consultar las solicitudes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []models.SolicitudCAE
	for rows.Next() {
		var s models.SolicitudCAE
		if err := rows.Scan(&s.IDSolicitudCAE, &s.FechaSolicitud, &s.HoraSolicitud, &s.IDActividad, &s.IDUsuarioSolicita, &s.EstadoSolicitud, &s.Observaciones, &s.ActividadNombre, &s.CargaHorariaMinutos); err == nil {
			// Regla: 16 horas (960 min) = 1 crédito
			s.CreditosCalculados = float64(s.CargaHorariaMinutos) / 60.0 / 16.0
			lista = append(lista, s)
		}
	}

	if lista == nil {
		lista = []models.SolicitudCAE{}
	}

	json.NewEncoder(w).Encode(lista)
}

// CrearSolicitud registra en SOLICITUDES_CAES y DETALLE_SOLICITUDES_CAES
func CrearSolicitud(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		IDUsuarioSolicita   int    `json:"id_usuario_solicita"`
		IDActividad         int    `json:"id_actividad"`
		NombreActividad     string `json:"nombre_actividad"`
		CargaHorariaMinutos int    `json:"carga_horaria_en_minutos"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	ahora := time.Now()
	fecha := ahora.Format("2006-01-02")
	hora := ahora.Format("15:04:05")

	tx, err := services.DB.Begin()
	if err != nil {
		http.Error(w, "Error de transacción", http.StatusInternalServerError)
		return
	}

	res, err := tx.Exec(`INSERT INTO SOLICITUDES_CAES (FECHA_SOLICITUD, HORA_SOLICITUD, ID_ACTIVIDAD, ID_USUARIO_SOLICITA, ESTADO_SOLICITUD) VALUES (?, ?, ?, ?, 'PENDIENTE')`,
		fecha, hora, req.IDActividad, req.IDUsuarioSolicita)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Error insertando solicitud", http.StatusInternalServerError)
		return
	}

	idInsertado, _ := res.LastInsertId()

	_, err = tx.Exec(`INSERT INTO DETALLE_SOLICITUDES_CAES (ID_SOLICITUD_CAE, FECHA_ACTIVIDAD, ACTIVIDAD, CARGA_HORARIA_EN_MINUTOS) VALUES (?, ?, ?, ?)`,
		idInsertado, fecha, req.NombreActividad, req.CargaHorariaMinutos)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Error insertando detalle", http.StatusInternalServerError)
		return
	}

	tx.Commit()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":          "Solicitud creada con éxito",
		"id_solicitud_cae": idInsertado,
	})

}
