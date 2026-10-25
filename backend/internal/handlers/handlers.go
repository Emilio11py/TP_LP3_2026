package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"tp-backend/internal/models"
	"tp-backend/internal/services"
)

const maxDocumentSize = 5 << 20

type App struct {
	DB       *sql.DB
	Origin   string
	mu       sync.Mutex
	sessions map[string]session
}

type session struct {
	userID  int
	expires time.Time
}

func CrearEnrutador(db *sql.DB, allowedOrigin string) http.Handler {
	app := &App{
		DB:       db,
		Origin:   allowedOrigin,
		sessions: make(map[string]session),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", app.verificarEstado)
	mux.HandleFunc("POST /api/auth/login", app.iniciarSesion)
	mux.HandleFunc("POST /api/auth/register", app.registrarUsuario)
	mux.HandleFunc("GET /api/auth/me", app.obtenerUsuarioActual)
	mux.HandleFunc("POST /api/auth/logout", app.cerrarSesion)
	mux.HandleFunc("GET /api/usuarios", app.listarUsuarios)
	mux.HandleFunc("POST /api/usuarios", app.crearUsuario)
	mux.HandleFunc("PUT /api/usuarios/{id}", app.modificarUsuario)
	mux.HandleFunc("DELETE /api/usuarios/{id}", app.eliminarUsuario)
	mux.HandleFunc("GET /api/usuarios/pendientes", app.listarUsuariosPendientes)
	mux.HandleFunc("PATCH /api/usuarios/pendientes/{id}/estado", app.cambiarEstadoSolicitudPendiente)
	mux.HandleFunc("GET /api/actividades", app.listarActividades)
	mux.HandleFunc("GET /api/solicitudes", app.listarSolicitudes)
	mux.HandleFunc("POST /api/solicitudes", app.crearSolicitud)
	mux.HandleFunc("GET /api/solicitudes/{id}", app.obtenerSolicitud)
	mux.HandleFunc("PATCH /api/solicitudes/{id}/decision", app.resolverSolicitud)
	mux.HandleFunc("GET /api/solicitudes/{id}/documentos", app.listarDocumentos)
	mux.HandleFunc("POST /api/solicitudes/{id}/documentos", app.subirDocumento)
	mux.HandleFunc("GET /api/documentos/{id}", app.descargarDocumento)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		escribirError(w, http.StatusNotFound, "Ruta no encontrada")
	})
	return app.configurarCORS(mux)
}

func (a *App) configurarCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if origin != a.Origin {
				escribirError(w, http.StatusForbidden, "Origen no permitido")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) verificarEstado(w http.ResponseWriter, r *http.Request) {
	if err := a.DB.PingContext(r.Context()); err != nil {
		escribirError(w, http.StatusServiceUnavailable, "Base de datos no disponible")
		return
	}
	escribirJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
}

func (a *App) iniciarSesion(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NroCedula   string `json:"nro_cedula"`
		Email       string `json:"email"`
		Contrasenha string `json:"contrasenha"`
	}
	if err := decodificarJSON(w, r, &input, 1<<20); err != nil {
		escribirError(w, http.StatusBadRequest, "Solicitud de inicio de sesión inválida")
		return
	}
	nroCedula := strings.TrimSpace(input.NroCedula)
	email := strings.TrimSpace(input.Email)
	if (nroCedula == "") == (email == "") ||
		len(nroCedula) > 120 || len(email) > 120 ||
		input.Contrasenha == "" || len(input.Contrasenha) > 256 {
		escribirError(w, http.StatusBadRequest, "Ingrese un número de documento o email y una contraseña")
		return
	}

	var user models.Usuario
	var storedPassword string
	query := `
		SELECT ID_USUARIO, NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA,
		       EMAIL, TELEFONO_CELULAR, CONTRASENHA, ROLE_ACCESO, CARRERA, ACTIVO
		FROM USUARIOS
		WHERE LOWER(EMAIL) = LOWER(?) AND ACTIVO = 1
	`
	identifier := email
	if nroCedula != "" {
		query = `
			SELECT ID_USUARIO, NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA,
			       EMAIL, TELEFONO_CELULAR, CONTRASENHA, ROLE_ACCESO, CARRERA, ACTIVO
			FROM USUARIOS
			WHERE CAST(NRO_CEDULA AS TEXT) = ? AND ACTIVO = 1
		`
		identifier = nroCedula
	}
	err := a.DB.QueryRowContext(r.Context(), query, identifier).Scan(
		&user.IDUsuario, &user.NroCedula, &user.Nombres, &user.Apellidos,
		&user.CodigoMatricula, &user.Email, &user.TelefonoCelular,
		&storedPassword, &user.RoleAcceso, &user.Carrera, &user.Activo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		escribirError(w, http.StatusUnauthorized, "Credenciales incorrectas")
		return
	}
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo validar el usuario")
		return
	}

	if !services.VerificarContrasena(input.Contrasenha, storedPassword) {
		escribirError(w, http.StatusUnauthorized, "Credenciales incorrectas")
		return
	}
	if services.EsHashAnterior(storedPassword) {
		if _, err := a.DB.ExecContext(r.Context(),
			"UPDATE USUARIOS SET CONTRASENHA = ? WHERE ID_USUARIO = ? AND CONTRASENHA = ?",
			input.Contrasenha, user.IDUsuario, storedPassword,
		); err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudo actualizar la contraseña")
			return
		}
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo iniciar la sesión")
		return
	}
	token := hex.EncodeToString(tokenBytes)
	a.mu.Lock()
	for existing, current := range a.sessions {
		if time.Now().After(current.expires) {
			delete(a.sessions, existing)
		}
	}
	a.sessions[token] = session{userID: user.IDUsuario, expires: time.Now().Add(12 * time.Hour)}
	a.mu.Unlock()

	escribirJSON(w, http.StatusOK, map[string]interface{}{
		"token":   token,
		"usuario": user,
	})
}

func (a *App) obtenerUsuarioActual(w http.ResponseWriter, r *http.Request) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if ok {
		escribirJSON(w, http.StatusOK, user)
	}
}

func (a *App) cerrarSesion(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.obtenerUsuarioAutenticado(w, r); !ok {
		return
	}
	token := obtenerTokenAutorizacion(r)
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) listarActividades(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.obtenerUsuarioAutenticado(w, r); !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT ID_ACTIVIDAD, NOMBRE_ACTIVIDAD, ACTIVO
		FROM ACTIVIDADES
		WHERE ACTIVO = 1
		ORDER BY NOMBRE_ACTIVIDAD
	`)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron consultar las actividades")
		return
	}
	defer rows.Close()

	activities := make([]models.Actividad, 0)
	for rows.Next() {
		var activity models.Actividad
		if err := rows.Scan(&activity.IDActividad, &activity.NombreActividad, &activity.Activo); err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudieron leer las actividades")
			return
		}
		activities = append(activities, activity)
	}
	if err := rows.Err(); err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron leer las actividades")
		return
	}
	escribirJSON(w, http.StatusOK, activities)
}

func (a *App) listarSolicitudes(w http.ResponseWriter, r *http.Request) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if !ok {
		return
	}
	query := requestSelect + " ORDER BY s.ID_SOLICITUD_CAE DESC"
	args := []interface{}{}
	if !esRevisor(user.RoleAcceso) {
		query = requestSelect + " WHERE s.ID_USUARIO_SOLICITA = ? ORDER BY s.ID_SOLICITUD_CAE DESC"
		args = append(args, user.IDUsuario)
	}
	requests, err := a.consultarSolicitudes(r, query, args...)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron consultar las solicitudes")
		return
	}
	escribirJSON(w, http.StatusOK, requests)
}

func (a *App) obtenerSolicitud(w http.ResponseWriter, r *http.Request) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if !ok {
		return
	}
	id, valid := obtenerIDDeRuta(r)
	if !valid {
		escribirError(w, http.StatusBadRequest, "ID de solicitud inválido")
		return
	}
	query := requestSelect + " WHERE s.ID_SOLICITUD_CAE = ?"
	args := []interface{}{id}
	if !esRevisor(user.RoleAcceso) {
		query += " AND s.ID_USUARIO_SOLICITA = ?"
		args = append(args, user.IDUsuario)
	}
	requests, err := a.consultarSolicitudes(r, query, args...)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo consultar la solicitud")
		return
	}
	if len(requests) == 0 {
		escribirError(w, http.StatusNotFound, "Solicitud no encontrada")
		return
	}
	escribirJSON(w, http.StatusOK, requests[0])
}

func (a *App) crearSolicitud(w http.ResponseWriter, r *http.Request) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if !ok {
		return
	}
	if !esEstudiante(user.RoleAcceso) {
		escribirError(w, http.StatusForbidden, "Solo los estudiantes pueden crear solicitudes")
		return
	}
	var input struct {
		IDActividad         int    `json:"id_actividad"`
		FechaActividad      string `json:"fecha_actividad"`
		NombreActividad     string `json:"nombre_actividad"`
		CargaHorariaMinutos int    `json:"carga_horaria_en_minutos"`
	}
	if err := decodificarJSON(w, r, &input, 1<<20); err != nil {
		escribirError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	input.NombreActividad = strings.TrimSpace(input.NombreActividad)
	if input.IDActividad <= 0 || input.NombreActividad == "" || len(input.NombreActividad) > 256 || input.CargaHorariaMinutos <= 0 {
		escribirError(w, http.StatusBadRequest, "Actividad, nombre y carga horaria válida son obligatorios")
		return
	}
	if input.FechaActividad == "" {
		input.FechaActividad = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", input.FechaActividad); err != nil {
		escribirError(w, http.StatusBadRequest, "La fecha de actividad debe tener formato AAAA-MM-DD")
		return
	}

	var active int
	err := a.DB.QueryRowContext(r.Context(),
		"SELECT ACTIVO FROM ACTIVIDADES WHERE ID_ACTIVIDAD = ?", input.IDActividad,
	).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && active != 1) {
		escribirError(w, http.StatusBadRequest, "La actividad seleccionada no existe o está inactiva")
		return
	}
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo validar la actividad")
		return
	}

	now := time.Now()
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo iniciar la transacción")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO SOLICITUDES_CAES
			(FECHA_SOLICITUD, HORA_SOLICITUD, ID_ACTIVIDAD, ID_USUARIO_SOLICITA, ESTADO_SOLICITUD)
		VALUES (?, ?, ?, ?, 'PENDIENTE')
	`, now.Format("2006-01-02"), now.Format("15:04:05"), input.IDActividad, user.IDUsuario)
	if err != nil {
		_ = tx.Rollback()
		escribirError(w, http.StatusInternalServerError, "No se pudo crear la solicitud")
		return
	}
	requestID, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		escribirError(w, http.StatusInternalServerError, "No se pudo obtener el ID de la solicitud")
		return
	}
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO DETALLE_SOLICITUDES_CAES
			(ID_SOLICITUD_CAE, FECHA_ACTIVIDAD, ACTIVIDAD, CARGA_HORARIA_EN_MINUTOS)
		VALUES (?, ?, ?, ?)
	`, requestID, input.FechaActividad, input.NombreActividad, input.CargaHorariaMinutos)
	if err != nil {
		_ = tx.Rollback()
		escribirError(w, http.StatusInternalServerError, "No se pudo guardar el detalle de la solicitud")
		return
	}
	if err := tx.Commit(); err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo confirmar la solicitud")
		return
	}
	escribirJSON(w, http.StatusCreated, map[string]int64{"id_solicitud_cae": requestID})
}

func (a *App) resolverSolicitud(w http.ResponseWriter, r *http.Request) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if !ok {
		return
	}
	if !esRevisor(user.RoleAcceso) {
		escribirError(w, http.StatusForbidden, "No tenés permisos para resolver solicitudes")
		return
	}
	id, valid := obtenerIDDeRuta(r)
	if !valid {
		escribirError(w, http.StatusBadRequest, "ID de solicitud inválido")
		return
	}
	var input struct {
		Estado        string `json:"estado_solicitud"`
		Observaciones string `json:"observaciones"`
	}
	if err := decodificarJSON(w, r, &input, 1<<20); err != nil {
		escribirError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	input.Estado = strings.ToUpper(strings.TrimSpace(input.Estado))
	input.Observaciones = strings.TrimSpace(input.Observaciones)
	if input.Estado != "APROBADO" && input.Estado != "RECHAZADO" {
		escribirError(w, http.StatusBadRequest, "El estado debe ser APROBADO o RECHAZADO")
		return
	}
	if len(input.Observaciones) > 256 || (input.Estado == "RECHAZADO" && input.Observaciones == "") {
		escribirError(w, http.StatusBadRequest, "La observación es obligatoria al rechazar y no puede superar 256 caracteres")
		return
	}
	result, err := a.DB.ExecContext(r.Context(), `
		UPDATE SOLICITUDES_CAES
		SET ESTADO_SOLICITUD = ?, OBSERVACIONES = ?, FECHA_APROBACION_O_RECHAZO = ?,
		    ID_USUARIO_APRUEBA_O_RECHAZA = ?
		WHERE ID_SOLICITUD_CAE = ? AND ESTADO_SOLICITUD = 'PENDIENTE'
	`, input.Estado, input.Observaciones, time.Now().Format("2006-01-02"), user.IDUsuario, id)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo resolver la solicitud")
		return
	}
	changed, err := result.RowsAffected()
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo confirmar el resultado")
		return
	}
	if changed == 0 {
		escribirError(w, http.StatusConflict, "La solicitud no existe o ya fue resuelta")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) listarDocumentos(w http.ResponseWriter, r *http.Request) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if !ok {
		return
	}
	requestID, valid := obtenerIDDeRuta(r)
	if !valid {
		escribirError(w, http.StatusBadRequest, "ID de solicitud inválido")
		return
	}
	allowed, err := a.puedeAccederSolicitud(r, user, requestID)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo validar el acceso a la solicitud")
		return
	}
	if !allowed {
		escribirError(w, http.StatusNotFound, "Solicitud no encontrada")
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT ID_DETALLE_DOC_SOLICITUD_CAE, TIPO_DOCUMENTO, IFNULL(OBSERVACIONES, '')
		FROM DETALLE_DOCUMENT_SOLICITUD_CAES
		WHERE ID_SOLICITUD_CAE = ?
		ORDER BY ID_DETALLE_DOC_SOLICITUD_CAE
	`, requestID)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron consultar los documentos")
		return
	}
	defer rows.Close()
	type document struct {
		ID            int    `json:"id_documento"`
		Tipo          string `json:"tipo_documento"`
		Observaciones string `json:"observaciones"`
	}
	documents := make([]document, 0)
	for rows.Next() {
		var item document
		if err := rows.Scan(&item.ID, &item.Tipo, &item.Observaciones); err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudieron leer los documentos")
			return
		}
		documents = append(documents, item)
	}
	if err := rows.Err(); err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron leer los documentos")
		return
	}
	escribirJSON(w, http.StatusOK, documents)
}

func (a *App) subirDocumento(w http.ResponseWriter, r *http.Request) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if !ok {
		return
	}
	requestID, valid := obtenerIDDeRuta(r)
	if !valid {
		escribirError(w, http.StatusBadRequest, "ID de solicitud inválido")
		return
	}
	allowed, err := a.puedeAccederSolicitud(r, user, requestID)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo validar el acceso a la solicitud")
		return
	}
	if !allowed {
		escribirError(w, http.StatusNotFound, "Solicitud no encontrada")
		return
	}
	if r.ContentLength > maxDocumentSize+(1<<20) {
		escribirError(w, http.StatusRequestEntityTooLarge, "El documento supera el límite de 5 MB")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxDocumentSize+(1<<20))
	if err := r.ParseMultipartForm(maxDocumentSize); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			escribirError(w, http.StatusRequestEntityTooLarge, "El documento supera el límite de 5 MB")
			return
		}
		escribirError(w, http.StatusBadRequest, "Formulario de documento inválido o demasiado grande")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("documento")
	if err != nil {
		escribirError(w, http.StatusBadRequest, "Falta el archivo en el campo documento")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxDocumentSize+1))
	if err != nil {
		escribirError(w, http.StatusBadRequest, "No se pudo leer el documento")
		return
	}
	if len(data) == 0 || len(data) > maxDocumentSize {
		escribirError(w, http.StatusRequestEntityTooLarge, "El documento debe pesar entre 1 byte y 5 MB")
		return
	}
	documentType := strings.TrimSpace(r.FormValue("tipo_documento"))
	observations := strings.TrimSpace(r.FormValue("observaciones"))
	if documentType == "" || len(documentType) > 32 || len(observations) > 180 {
		escribirError(w, http.StatusBadRequest, "Tipo de documento obligatorio (máximo 32 caracteres) y observaciones de hasta 180 caracteres")
		return
	}
	result, err := a.DB.ExecContext(r.Context(), `
		INSERT INTO DETALLE_DOCUMENT_SOLICITUD_CAES
			(ID_SOLICITUD_CAE, DOCUMENTO, TIPO_DOCUMENTO, OBSERVACIONES)
		VALUES (?, ?, ?, ?)
	`, requestID, data, documentType, observations)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo guardar el documento")
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo obtener el ID del documento")
		return
	}
	escribirJSON(w, http.StatusCreated, map[string]int64{"id_documento": id})
}

func (a *App) descargarDocumento(w http.ResponseWriter, r *http.Request) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if !ok {
		return
	}
	id, valid := obtenerIDDeRuta(r)
	if !valid {
		escribirError(w, http.StatusBadRequest, "ID de documento inválido")
		return
	}
	var requestID int
	var data []byte
	err := a.DB.QueryRowContext(r.Context(), `
		SELECT d.ID_SOLICITUD_CAE, d.DOCUMENTO
		FROM DETALLE_DOCUMENT_SOLICITUD_CAES d
		WHERE d.ID_DETALLE_DOC_SOLICITUD_CAE = ?
	`, id).Scan(&requestID, &data)
	if errors.Is(err, sql.ErrNoRows) {
		escribirError(w, http.StatusNotFound, "Documento no encontrado")
		return
	}
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo consultar el documento")
		return
	}
	allowed, err := a.puedeAccederSolicitud(r, user, requestID)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo validar el acceso al documento")
		return
	}
	if !allowed {
		escribirError(w, http.StatusNotFound, "Documento no encontrado")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="documento-%d"`, id))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *App) obtenerUsuarioAutenticado(w http.ResponseWriter, r *http.Request) (models.Usuario, bool) {
	token := obtenerTokenAutorizacion(r)
	if token == "" {
		escribirError(w, http.StatusUnauthorized, "Se requiere autenticación")
		return models.Usuario{}, false
	}
	a.mu.Lock()
	current, exists := a.sessions[token]
	if exists && time.Now().After(current.expires) {
		delete(a.sessions, token)
		exists = false
	}
	a.mu.Unlock()
	if !exists {
		escribirError(w, http.StatusUnauthorized, "La sesión expiró o no es válida")
		return models.Usuario{}, false
	}
	var user models.Usuario
	err := a.DB.QueryRowContext(r.Context(), `
		SELECT ID_USUARIO, NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA,
		       EMAIL, TELEFONO_CELULAR, ROLE_ACCESO, CARRERA, ACTIVO
		FROM USUARIOS WHERE ID_USUARIO = ? AND ACTIVO = 1
	`, current.userID).Scan(
		&user.IDUsuario, &user.NroCedula, &user.Nombres, &user.Apellidos,
		&user.CodigoMatricula, &user.Email, &user.TelefonoCelular,
		&user.RoleAcceso, &user.Carrera, &user.Activo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		a.mu.Lock()
		delete(a.sessions, token)
		a.mu.Unlock()
		escribirError(w, http.StatusUnauthorized, "El usuario ya no está activo")
		return models.Usuario{}, false
	}
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo validar la sesión")
		return models.Usuario{}, false
	}
	return user, true
}

func (a *App) puedeAccederSolicitud(r *http.Request, user models.Usuario, requestID int) (bool, error) {
	query := "SELECT 1 FROM SOLICITUDES_CAES WHERE ID_SOLICITUD_CAE = ?"
	args := []interface{}{requestID}
	if !esRevisor(user.RoleAcceso) {
		query += " AND ID_USUARIO_SOLICITA = ?"
		args = append(args, user.IDUsuario)
	}
	var exists int
	err := a.DB.QueryRowContext(r.Context(), query, args...).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

const requestSelect = `
	SELECT s.ID_SOLICITUD_CAE, s.FECHA_SOLICITUD, s.HORA_SOLICITUD,
	       s.ID_ACTIVIDAD, s.ID_USUARIO_SOLICITA, s.ESTADO_SOLICITUD,
	       IFNULL(s.OBSERVACIONES, ''), IFNULL(d.ACTIVIDAD, ''),
	       IFNULL(d.CARGA_HORARIA_EN_MINUTOS, 0), IFNULL(d.FECHA_ACTIVIDAD, ''),
	       u.NOMBRES || ' ' || u.APELLIDOS, u.EMAIL
	FROM SOLICITUDES_CAES s
	LEFT JOIN DETALLE_SOLICITUDES_CAES d ON d.ID_SOLICITUD_CAE = s.ID_SOLICITUD_CAE
	JOIN USUARIOS u ON u.ID_USUARIO = s.ID_USUARIO_SOLICITA
`

func (a *App) consultarSolicitudes(r *http.Request, query string, args ...interface{}) ([]models.SolicitudCAE, error) {
	rows, err := a.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := make([]models.SolicitudCAE, 0)
	for rows.Next() {
		var item models.SolicitudCAE
		if err := rows.Scan(
			&item.IDSolicitudCAE, &item.FechaSolicitud, &item.HoraSolicitud,
			&item.IDActividad, &item.IDUsuarioSolicita, &item.EstadoSolicitud,
			&item.Observaciones, &item.ActividadNombre, &item.CargaHorariaMinutos,
			&item.FechaActividad, &item.SolicitanteNombre, &item.SolicitanteEmail,
		); err != nil {
			return nil, err
		}
		requests = append(requests, item)
	}
	return requests, rows.Err()
}

func obtenerTokenAutorizacion(r *http.Request) string {
	header := strings.Fields(r.Header.Get("Authorization"))
	if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
		return ""
	}
	return header[1]
}

func esRevisor(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "dir_academica", "dir_carrera":
		return true
	default:
		return false
	}
}

func esEstudiante(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "estudiante")
}

func obtenerIDDeRuta(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	return id, err == nil && id > 0
}

func decodificarJSON(w http.ResponseWriter, r *http.Request, destination interface{}, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("el cuerpo debe contener un único objeto JSON")
	}
	return nil
}

func escribirJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func escribirError(w http.ResponseWriter, status int, message string) {
	escribirJSON(w, status, map[string]string{"error": message})
}
