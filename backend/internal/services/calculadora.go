package services

// CalcularCreditos aplica las reglas del Reglamento N° 33/2020
func CalcularCreditos(tipoActividad, organizador string, horas float64) float64 {
	switch tipoActividad {
	case "curso_taller":
		creditos := horas / 16.0
		if creditos > 5.0 {
			return 5.0 // Cap máximo de 5 CAE
		}
		return creditos

	case "conferencia":
		var tasa float64
		switch organizador {
		case "FCYT":
			tasa = 16.0
		case "UC_OTRAS":
			tasa = 32.0
		default: // EXTERNO
			tasa = 48.0
		}
		creditos := horas / tasa
		if creditos > 4.0 {
			return 4.0 // Cap máximo de 4 CAE
		}
		return creditos

	case "investigacion", "extension":
		creditos := horas / 32.0
		if creditos > 4.0 {
			return 4.0 // Cap máximo de 4 CAE
		}
		return creditos

	default:
		return 0.0
	}
}
