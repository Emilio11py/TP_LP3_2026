package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"tp-backend/internal/models"
)

type userInput struct {
	NroCedula       string `json:"nro_cedula"`
	Nombres         string `json:"nombres"`
	Apellidos       string `json:"apellidos"`
	CodigoMatricula string `json:"codigo_matricula"`
	Email           string `json:"email"`
	TelefonoCelular string `json:"telefono_celular"`
	Contrasenha     string `json:"contrasenha"`
	RoleAcceso      string `json:"role_acceso"`
	Carrera         string `json:"carrera"`
	Activo          int    `json:"activo"`
}

func (a *App) registrarUsuario(w http.ResponseWriter, r *http.Request) {
	input, user, err := leerDatosUsuario(w, r, true)
	if err != nil {
		return
	}
	if !esEstudiante(user.RoleAcceso) {
		escribirError(w, http.StatusBadRequest, "El registro público solo admite el rol de estudiante")
		return
	}
	if err := a.verificarConflictoUsuario(r, user, 0); err != nil {
		if errors.Is(err, errorUsuarioDuplicado) {
			escribirError(w, http.StatusConflict, "El documento, matrícula o correo ya corresponde a un usuario")
			return
		}
		escribirError(w, http.StatusInternalServerError, "No se pudieron validar los datos del registro")
		return
	}
	var duplicate int
	err = a.DB.QueryRowContext(r.Context(), `
		SELECT 1 FROM USUARIOS_PENDIENTES
		WHERE ESTADO = 'PENDIENTE'
		  AND (NRO_CEDULA = ? OR LOWER(CODIGO_MATRICULA) = LOWER(?) OR LOWER(EMAIL) = LOWER(?))
		LIMIT 1
	`, user.NroCedula, user.CodigoMatricula, user.Email).Scan(&duplicate)
	if err == nil {
		escribirError(w, http.StatusConflict, "Ya existe una solicitud pendiente con esos datos")
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		escribirError(w, http.StatusInternalServerError, "No se pudieron validar las solicitudes pendientes")
		return
	}

	_, err = a.DB.ExecContext(r.Context(), `
		INSERT INTO USUARIOS_PENDIENTES
			(NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA, EMAIL,
			 TELEFONO_CELULAR, CONTRASENHA, CARRERA, ESTADO, FECHA_SOLICITUD)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'PENDIENTE', ?)
	`, user.NroCedula, user.Nombres, user.Apellidos, user.CodigoMatricula,
		user.Email, user.TelefonoCelular, input.Contrasenha, user.Carrera,
		time.Now().Format(time.RFC3339))
	if err != nil {
		escribirError(w, http.StatusConflict, "No se pudo crear la solicitud; revise que los datos no estén registrados")
		return
	}
	escribirJSON(w, http.StatusCreated, map[string]string{
		"estado":  "PENDIENTE",
		"mensaje": "Solicitud enviada. El acceso estará disponible después de la aprobación.",
	})
}

func (a *App) listarUsuarios(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.exigirRevisor(w, r); !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT ID_USUARIO, NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA,
		       EMAIL, TELEFONO_CELULAR, ROLE_ACCESO, CARRERA, ACTIVO
		FROM USUARIOS ORDER BY APELLIDOS, NOMBRES
	`)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron consultar los usuarios")
		return
	}
	defer rows.Close()
	users := make([]models.Usuario, 0)
	for rows.Next() {
		var user models.Usuario
		if err := rows.Scan(&user.IDUsuario, &user.NroCedula, &user.Nombres,
			&user.Apellidos, &user.CodigoMatricula, &user.Email,
			&user.TelefonoCelular, &user.RoleAcceso, &user.Carrera, &user.Activo); err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudieron leer los usuarios")
			return
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron leer los usuarios")
		return
	}
	escribirJSON(w, http.StatusOK, users)
}

func (a *App) crearUsuario(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.exigirRevisor(w, r); !ok {
		return
	}
	input, user, err := leerDatosUsuario(w, r, true)
	if err != nil {
		return
	}
	if err := validarEstadoActivo(input.Activo); err != nil {
		escribirError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.verificarConflictoUsuario(r, user, 0); err != nil {
		if errors.Is(err, errorUsuarioDuplicado) {
			escribirError(w, http.StatusConflict, "El documento, matrícula o correo ya está registrado")
			return
		}
		escribirError(w, http.StatusInternalServerError, "No se pudieron validar los datos del usuario")
		return
	}
	result, err := a.DB.ExecContext(r.Context(), `
		INSERT INTO USUARIOS
			(NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA, EMAIL,
			 TELEFONO_CELULAR, CONTRASENHA, ROLE_ACCESO, CARRERA, FECHA_ALTA, ACTIVO)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.NroCedula, user.Nombres, user.Apellidos, user.CodigoMatricula,
		user.Email, user.TelefonoCelular, input.Contrasenha, user.RoleAcceso,
		user.Carrera, time.Now().Format("2006-01-02"), input.Activo)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo crear el usuario")
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo obtener el usuario creado")
		return
	}
	user.IDUsuario = int(id)
	user.Activo = input.Activo
	escribirJSON(w, http.StatusCreated, user)
}

func (a *App) modificarUsuario(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := a.exigirRevisor(w, r)
	if !ok {
		return
	}
	id, valid := obtenerIDDeRuta(r)
	if !valid {
		escribirError(w, http.StatusBadRequest, "ID de usuario inválido")
		return
	}
	input, user, err := leerDatosUsuario(w, r, false)
	if err != nil {
		return
	}
	if err := validarEstadoActivo(input.Activo); err != nil {
		escribirError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.verificarConflictoUsuario(r, user, id); err != nil {
		if errors.Is(err, errorUsuarioDuplicado) {
			escribirError(w, http.StatusConflict, "El documento, matrícula o correo ya está registrado por otro usuario")
			return
		}
		escribirError(w, http.StatusInternalServerError, "No se pudieron validar los datos del usuario")
		return
	}

	var oldRole string
	var oldActive int
	err = a.DB.QueryRowContext(r.Context(),
		"SELECT ROLE_ACCESO, ACTIVO FROM USUARIOS WHERE ID_USUARIO = ?", id,
	).Scan(&oldRole, &oldActive)
	if errors.Is(err, sql.ErrNoRows) {
		escribirError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	}
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo consultar el usuario")
		return
	}
	if id == reviewer.IDUsuario && (!esRevisor(user.RoleAcceso) || input.Activo != 1) {
		escribirError(w, http.StatusConflict, "No podés quitarte tu propio permiso administrativo ni desactivar tu cuenta")
		return
	}
	if oldActive == 1 && esRevisor(oldRole) &&
		(input.Activo != 1 || !esRevisor(user.RoleAcceso)) {
		count, err := a.contarRevisoresActivos(r)
		if err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudo validar el último usuario administrativo")
			return
		}
		if count <= 1 {
			escribirError(w, http.StatusConflict, "Debe quedar al menos un usuario administrativo activo")
			return
		}
	}

	query := `
		UPDATE USUARIOS
		SET NRO_CEDULA = ?, NOMBRES = ?, APELLIDOS = ?, CODIGO_MATRICULA = ?,
		    EMAIL = ?, TELEFONO_CELULAR = ?, ROLE_ACCESO = ?, CARRERA = ?, ACTIVO = ?
		WHERE ID_USUARIO = ?
	`
	args := []interface{}{
		user.NroCedula, user.Nombres, user.Apellidos, user.CodigoMatricula,
		user.Email, user.TelefonoCelular, user.RoleAcceso, user.Carrera, input.Activo, id,
	}
	if input.Contrasenha != "" {
		query = `
			UPDATE USUARIOS
			SET NRO_CEDULA = ?, NOMBRES = ?, APELLIDOS = ?, CODIGO_MATRICULA = ?,
			    EMAIL = ?, TELEFONO_CELULAR = ?, ROLE_ACCESO = ?, CARRERA = ?,
			    ACTIVO = ?, CONTRASENHA = ?
			WHERE ID_USUARIO = ?
		`
		args = []interface{}{
			user.NroCedula, user.Nombres, user.Apellidos, user.CodigoMatricula,
			user.Email, user.TelefonoCelular, user.RoleAcceso, user.Carrera,
			input.Activo, input.Contrasenha, id,
		}
	}
	result, err := a.DB.ExecContext(r.Context(), query, args...)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo modificar el usuario")
		return
	}
	changed, err := result.RowsAffected()
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo confirmar la modificación")
		return
	}
	if changed == 0 {
		escribirError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	}
	user.IDUsuario = id
	user.Activo = input.Activo
	escribirJSON(w, http.StatusOK, user)
}

func (a *App) eliminarUsuario(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := a.exigirRevisor(w, r)
	if !ok {
		return
	}
	id, valid := obtenerIDDeRuta(r)
	if !valid {
		escribirError(w, http.StatusBadRequest, "ID de usuario inválido")
		return
	}
	if id == reviewer.IDUsuario {
		escribirError(w, http.StatusConflict, "No podés eliminar tu propio usuario")
		return
	}
	var active int
	err := a.DB.QueryRowContext(r.Context(),
		"SELECT ACTIVO FROM USUARIOS WHERE ID_USUARIO = ?", id,
	).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		escribirError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	}
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo consultar el usuario")
		return
	}
	if active == 1 {
		escribirError(w, http.StatusConflict, "El usuario sigue activo en la UC y no se puede eliminar")
		return
	}
	if active != 0 {
		escribirError(w, http.StatusConflict, "El estado del usuario no es válido para eliminarlo")
		return
	}
	var references int
	err = a.DB.QueryRowContext(r.Context(), `
		SELECT
			(SELECT COUNT(*) FROM SOLICITUDES_CAES WHERE ID_USUARIO_SOLICITA = ?) +
			(SELECT COUNT(*) FROM SOLICITUDES_CAES WHERE ID_USUARIO_APRUEBA_O_RECHAZA = ?) +
			(SELECT COUNT(*) FROM DETALLE_SOLICITUDES_CAES WHERE ID_USUARIO_SUPERVISA = ?)
	`, id, id, id).Scan(&references)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo comprobar el historial del usuario")
		return
	}
	if references > 0 {
		escribirError(w, http.StatusConflict, "No se puede eliminar el usuario porque tiene actividad registrada en el sistema")
		return
	}
	if _, err := a.DB.ExecContext(r.Context(), "DELETE FROM USUARIOS WHERE ID_USUARIO = ?", id); err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudo eliminar el usuario")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) listarUsuariosPendientes(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.exigirRevisor(w, r); !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT p.ID_USUARIO_PENDIENTE, p.NRO_CEDULA, p.NOMBRES, p.APELLIDOS,
		       p.CODIGO_MATRICULA, p.EMAIL, p.TELEFONO_CELULAR, 'estudiante',
		       p.CARRERA, p.ESTADO, p.FECHA_SOLICITUD,
		       IFNULL(p.FECHA_REVISION, ''), IFNULL(p.ID_USUARIO_REVISOR, 0),
		       IFNULL(u.NOMBRES || ' ' || u.APELLIDOS, '')
		FROM USUARIOS_PENDIENTES p
		LEFT JOIN USUARIOS u ON u.ID_USUARIO = p.ID_USUARIO_REVISOR
		ORDER BY CASE p.ESTADO WHEN 'PENDIENTE' THEN 0 WHEN 'RECHAZADO' THEN 1 ELSE 2 END,
		         p.FECHA_SOLICITUD DESC
	`)
	if err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron consultar las solicitudes de usuarios")
		return
	}
	defer rows.Close()
	pending := make([]models.UsuarioPendiente, 0)
	for rows.Next() {
		var item models.UsuarioPendiente
		if err := rows.Scan(&item.ID, &item.NroCedula, &item.Nombres,
			&item.Apellidos, &item.CodigoMatricula, &item.Email,
			&item.TelefonoCelular, &item.RoleAcceso, &item.Carrera,
			&item.Estado, &item.FechaSolicitud, &item.FechaRevision,
			&item.RevisadoPor, &item.RevisorNombre); err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudieron leer las solicitudes de usuarios")
			return
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		escribirError(w, http.StatusInternalServerError, "No se pudieron leer las solicitudes de usuarios")
		return
	}
	escribirJSON(w, http.StatusOK, pending)
}

func (a *App) cambiarEstadoSolicitudPendiente(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := a.exigirRevisor(w, r)
	if !ok {
		return
	}
	id, valid := obtenerIDDeRuta(r)
	if !valid {
		escribirError(w, http.StatusBadRequest, "ID de solicitud inválido")
		return
	}
	var input struct {
		Estado string `json:"estado"`
	}
	if err := decodificarJSON(w, r, &input, 1<<20); err != nil {
		escribirError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	input.Estado = strings.ToUpper(strings.TrimSpace(input.Estado))
	switch input.Estado {
	case "APROBADO":
		if err := a.aprobarUsuarioPendiente(r, id, reviewer.IDUsuario); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				escribirError(w, http.StatusConflict, "La solicitud ya fue revisada o no existe")
				return
			}
			if errors.Is(err, errorUsuarioDuplicado) {
				escribirError(w, http.StatusConflict, "El documento, matrícula o correo ya pertenece a un usuario")
				return
			}
			escribirError(w, http.StatusInternalServerError, "No se pudo aprobar la solicitud")
			return
		}
	case "RECHAZADO":
		result, err := a.DB.ExecContext(r.Context(), `
			UPDATE USUARIOS_PENDIENTES
			SET ESTADO = 'RECHAZADO', FECHA_REVISION = ?, ID_USUARIO_REVISOR = ?
			WHERE ID_USUARIO_PENDIENTE = ? AND ESTADO = 'PENDIENTE'
		`, time.Now().Format(time.RFC3339), reviewer.IDUsuario, id)
		if err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudo rechazar la solicitud")
			return
		}
		changed, err := result.RowsAffected()
		if err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudo confirmar el rechazo")
			return
		}
		if changed == 0 {
			escribirError(w, http.StatusConflict, "La solicitud ya fue revisada o no existe")
			return
		}
	case "PENDIENTE":
		var user models.Usuario
		err := a.DB.QueryRowContext(r.Context(), `
			SELECT NRO_CEDULA, CODIGO_MATRICULA, EMAIL
			FROM USUARIOS_PENDIENTES WHERE ID_USUARIO_PENDIENTE = ? AND ESTADO = 'RECHAZADO'
		`, id).Scan(&user.NroCedula, &user.CodigoMatricula, &user.Email)
		if errors.Is(err, sql.ErrNoRows) {
			escribirError(w, http.StatusConflict, "Solo se pueden reabrir solicitudes rechazadas")
			return
		}
		if err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudo consultar la solicitud")
			return
		}
		err = a.verificarConflictoUsuario(r, user, 0)
		if errors.Is(err, errorUsuarioDuplicado) {
			escribirError(w, http.StatusConflict, "Los datos de esta solicitud ya pertenecen a un usuario")
			return
		}
		if err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudieron validar los datos de la solicitud")
			return
		}
		var duplicate int
		err = a.DB.QueryRowContext(r.Context(), `
			SELECT 1 FROM USUARIOS_PENDIENTES
			WHERE ID_USUARIO_PENDIENTE <> ? AND ESTADO = 'PENDIENTE'
			  AND (NRO_CEDULA = ? OR LOWER(CODIGO_MATRICULA) = LOWER(?) OR LOWER(EMAIL) = LOWER(?))
			LIMIT 1
		`, id, user.NroCedula, user.CodigoMatricula, user.Email).Scan(&duplicate)
		if err == nil {
			escribirError(w, http.StatusConflict, "Ya existe otra solicitud pendiente con esos datos")
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			escribirError(w, http.StatusInternalServerError, "No se pudieron validar las solicitudes pendientes")
			return
		}
		result, err := a.DB.ExecContext(r.Context(), `
			UPDATE USUARIOS_PENDIENTES
			SET ESTADO = 'PENDIENTE', FECHA_REVISION = NULL, ID_USUARIO_REVISOR = NULL
			WHERE ID_USUARIO_PENDIENTE = ? AND ESTADO = 'RECHAZADO'
		`, id)
		if err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudo reabrir la solicitud")
			return
		}
		changed, err := result.RowsAffected()
		if err != nil {
			escribirError(w, http.StatusInternalServerError, "No se pudo confirmar la reapertura")
			return
		}
		if changed == 0 {
			escribirError(w, http.StatusConflict, "Solo se pueden reabrir solicitudes rechazadas")
			return
		}
	default:
		escribirError(w, http.StatusBadRequest, "El estado debe ser APROBADO, RECHAZADO o PENDIENTE")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var errorUsuarioDuplicado = errors.New("el usuario ya existe")

func (a *App) aprobarUsuarioPendiente(r *http.Request, id, reviewerID int) error {
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var user models.Usuario
	var storedPassword string
	err = tx.QueryRowContext(r.Context(), `
		SELECT NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA, EMAIL,
		       TELEFONO_CELULAR, CONTRASENHA, CARRERA
		FROM USUARIOS_PENDIENTES
		WHERE ID_USUARIO_PENDIENTE = ? AND ESTADO = 'PENDIENTE'
	`, id).Scan(&user.NroCedula, &user.Nombres, &user.Apellidos,
		&user.CodigoMatricula, &user.Email, &user.TelefonoCelular,
		&storedPassword, &user.Carrera)
	if err != nil {
		return err
	}
	var existing int
	err = tx.QueryRowContext(r.Context(), `
		SELECT 1 FROM USUARIOS
		WHERE NRO_CEDULA = ? OR LOWER(CODIGO_MATRICULA) = LOWER(?) OR LOWER(EMAIL) = LOWER(?)
		LIMIT 1
	`, user.NroCedula, user.CodigoMatricula, user.Email).Scan(&existing)
	if err == nil {
		return errorUsuarioDuplicado
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO USUARIOS
			(NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA, EMAIL,
			 TELEFONO_CELULAR, CONTRASENHA, ROLE_ACCESO, CARRERA, FECHA_ALTA, ACTIVO)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'estudiante', ?, ?, 1)
	`, user.NroCedula, user.Nombres, user.Apellidos, user.CodigoMatricula,
		user.Email, user.TelefonoCelular, storedPassword, user.Carrera,
		time.Now().Format("2006-01-02"))
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(r.Context(), `
		UPDATE USUARIOS_PENDIENTES
		SET ESTADO = 'APROBADO', FECHA_REVISION = ?, ID_USUARIO_REVISOR = ?
		WHERE ID_USUARIO_PENDIENTE = ? AND ESTADO = 'PENDIENTE'
	`, time.Now().Format(time.RFC3339), reviewerID, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (a *App) exigirRevisor(w http.ResponseWriter, r *http.Request) (models.Usuario, bool) {
	user, ok := a.obtenerUsuarioAutenticado(w, r)
	if !ok {
		return models.Usuario{}, false
	}
	if !esRevisor(user.RoleAcceso) {
		escribirError(w, http.StatusForbidden, "No tenés permisos para gestionar usuarios")
		return models.Usuario{}, false
	}
	return user, true
}

func (a *App) contarRevisoresActivos(r *http.Request) (int, error) {
	var count int
	err := a.DB.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM USUARIOS WHERE ACTIVO = 1
		AND LOWER(ROLE_ACCESO) IN ('dir_academica', 'dir_carrera')
	`).Scan(&count)
	return count, err
}

func (a *App) verificarConflictoUsuario(r *http.Request, user models.Usuario, excludedID int) error {
	query := `
		SELECT 1 FROM USUARIOS
		WHERE (NRO_CEDULA = ? OR LOWER(CODIGO_MATRICULA) = LOWER(?) OR LOWER(EMAIL) = LOWER(?))
	`
	args := []interface{}{user.NroCedula, user.CodigoMatricula, user.Email}
	if excludedID > 0 {
		query += " AND ID_USUARIO <> ?"
		args = append(args, excludedID)
	}
	var duplicate int
	err := a.DB.QueryRowContext(r.Context(), query+" LIMIT 1", args...).Scan(&duplicate)
	if err == nil {
		return errorUsuarioDuplicado
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func leerDatosUsuario(w http.ResponseWriter, r *http.Request, requirePassword bool) (userInput, models.Usuario, error) {
	var input userInput
	if err := decodificarJSON(w, r, &input, 1<<20); err != nil {
		escribirError(w, http.StatusBadRequest, "Datos de usuario inválidos")
		return userInput{}, models.Usuario{}, err
	}
	input.NroCedula = strings.TrimSpace(input.NroCedula)
	input.Nombres = strings.TrimSpace(input.Nombres)
	input.Apellidos = strings.TrimSpace(input.Apellidos)
	input.CodigoMatricula = strings.TrimSpace(input.CodigoMatricula)
	input.Email = strings.TrimSpace(input.Email)
	input.TelefonoCelular = strings.TrimSpace(input.TelefonoCelular)
	input.RoleAcceso = strings.ToLower(strings.TrimSpace(input.RoleAcceso))
	input.Carrera = strings.TrimSpace(input.Carrera)
	nationalID, err := strconv.Atoi(input.NroCedula)
	email, emailErr := mail.ParseAddress(input.Email)
	validRole := input.RoleAcceso == "estudiante" || input.RoleAcceso == "dir_academica" || input.RoleAcceso == "dir_carrera"
	valid := err == nil && nationalID > 0 &&
		input.Nombres != "" && len(input.Nombres) <= 120 &&
		input.Apellidos != "" && len(input.Apellidos) <= 120 &&
		input.CodigoMatricula != "" && len(input.CodigoMatricula) <= 10 &&
		emailErr == nil && email.Address == input.Email && len(input.Email) <= 120 &&
		len(input.TelefonoCelular) > 0 && len(input.TelefonoCelular) <= 18 &&
		esCarreraPermitida(input.Carrera) && validRole
	if requirePassword {
		valid = valid && len(input.Contrasenha) >= 8 && len(input.Contrasenha) <= 128
	} else {
		valid = valid && (input.Contrasenha == "" || len(input.Contrasenha) >= 8 && len(input.Contrasenha) <= 128)
	}
	if !valid {
		message := "Complete los datos; documento numérico, correo válido y campos dentro de sus límites"
		if requirePassword {
			message += ", contraseña de 8 a 128 caracteres"
		}
		escribirError(w, http.StatusBadRequest, message)
		return userInput{}, models.Usuario{}, errorDatosUsuarioInvalidos
	}
	input.NroCedula = strconv.Itoa(nationalID)
	user := models.Usuario{
		NroCedula:       nationalID,
		Nombres:         input.Nombres,
		Apellidos:       input.Apellidos,
		CodigoMatricula: input.CodigoMatricula,
		Email:           input.Email,
		TelefonoCelular: input.TelefonoCelular,
		RoleAcceso:      input.RoleAcceso,
		Carrera:         input.Carrera,
	}
	return input, user, nil
}

func esCarreraPermitida(career string) bool {
	switch career {
	case "Ing. Informática", "Ing. Química", "Ing. Industrial":
		return true
	default:
		return false
	}
}

var errorDatosUsuarioInvalidos = errors.New("datos de usuario inválidos")

func validarEstadoActivo(active int) error {
	if active != 0 && active != 1 {
		return errors.New("El estado activo debe ser 0 o 1")
	}
	return nil
}
