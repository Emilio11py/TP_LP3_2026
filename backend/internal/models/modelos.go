package models

// Usuario representa a un usuario en la tabla USUARIOS
type Usuario struct {
	IDUsuario       int    `json:"id_usuario"`
	NroCedula       int    `json:"nro_cedula"`
	Nombres         string `json:"nombres"`
	Apellidos       string `json:"apellidos"`
	CodigoMatricula string `json:"codigo_matricula"`
	Email           string `json:"email"`
	TelefonoCelular string `json:"telefono_celular"`
	Contrasenha     string `json:"contrasenha,omitempty"`
	RoleAcceso      string `json:"role_acceso"` // 'ESTUDIANTE', 'DIR_ACADEMICA', 'DIR_CARRERA'
	Carrera         string `json:"carrera"`
	Activo          int    `json:"activo"`
}

// Actividad representa un tipo de actividad en la tabla ACTIVIDADES
type Actividad struct {
	IDActividad     int    `json:"id_actividad"`
	NombreActividad string `json:"nombre_actividad"`
	Activo          int    `json:"activo"`
}

// SolicitudCAE representa una solicitud en SOLICITUDES_CAES + DETALLE_SOLICITUDES_CAES
type SolicitudCAE struct {
	IDSolicitudCAE      int     `json:"id_solicitud_cae"`
	FechaSolicitud      string  `json:"fecha_solicitud"`
	HoraSolicitud       string  `json:"hora_solicitud"`
	IDActividad         int     `json:"id_actividad"`
	IDUsuarioSolicita   int     `json:"id_usuario_solicita"`
	EstadoSolicitud     string  `json:"estado_solicitud"` // 'PENDIENTE', 'APROBADO', 'RECHAZADO'
	Observaciones       string  `json:"observaciones"`
	ActividadNombre     string  `json:"actividad_nombre,omitempty"`
	CargaHorariaMinutos int     `json:"carga_horaria_en_minutos,omitempty"`
	CreditosCalculados  float64 `json:"creditos_calculados,omitempty"`
}
