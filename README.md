# TP_LP3_2026

Trabajo práctico de Lenguaje de Programación 3. El backend está implementado en Go y usa SQLite.

## Ejecutar el backend

Desde PowerShell, en la raíz del repositorio:

```powershell
Set-Location .\backend
go run .\cmd\api
```

Por defecto, la API escucha en `http://localhost:8080` y abre `data/sistema_cae.db`, relativo a `backend`. Al iniciar valida las tablas y columnas requeridas; si el archivo o el esquema no coinciden, se detiene con un error en lugar de arrancar sobre una base vacía.
Se pueden cambiar los valores con estas variables de entorno:

| Variable | Predeterminado | Uso |
| --- | --- | --- |
| `PORT` | `8080` | Puerto HTTP |
| `DB_PATH` | `data/sistema_cae.db` | Ruta de SQLite, relativa al directorio de ejecución si no es absoluta |
| `FRONTEND_ORIGIN` | `http://localhost:4200` | Único origen de frontend habilitado por CORS |

La primera autenticación correcta transforma la contraseña de texto legado de ese usuario a PBKDF2-SHA256. Las sesiones usan un token Bearer aleatorio, duran 12 horas y se invalidan al reiniciar el servidor. En Angular se debe enviar `Authorization: Bearer <token>` en las rutas protegidas y `Content-Type: application/json` para los cuerpos JSON.

## Contrato HTTP

Todas las respuestas de error tienen el formato `{"error":"..."}`. Las rutas protegidas devuelven `401` sin sesión y `403` cuando el usuario no tiene permiso.

| Método | Ruta | Acceso | Descripción |
| --- | --- | --- | --- |
| `GET` | `/api/health` | Público | Verifica que API y base estén disponibles |
| `POST` | `/api/auth/login` | Público | Inicia sesión |
| `GET` | `/api/auth/me` | Autenticado | Devuelve el usuario de la sesión actual |
| `POST` | `/api/auth/logout` | Autenticado | Invalida el token actual |
| `GET` | `/api/actividades` | Autenticado | Lista actividades activas para el formulario |
| `GET` | `/api/solicitudes` | Autenticado | Estudiantes: solicitudes propias; `dir_academica` y `dir_carrera`: todas |
| `POST` | `/api/solicitudes` | Estudiante | Crea una solicitud y su detalle en una transacción |
| `GET` | `/api/solicitudes/{id}` | Dueño o revisor | Consulta una solicitud |
| `PATCH` | `/api/solicitudes/{id}/decision` | Revisor | Aprueba o rechaza una solicitud pendiente |
| `GET` | `/api/solicitudes/{id}/documentos` | Dueño o revisor | Lista metadatos de los documentos |
| `POST` | `/api/solicitudes/{id}/documentos` | Dueño o revisor | Adjunta un archivo de hasta 5 MB |
| `GET` | `/api/documentos/{id}` | Dueño o revisor | Descarga un documento |

### Ejemplos de cuerpos

Iniciar sesión:

```json
{"email":"estudiante@ejemplo.com","contrasenha":"contraseña"}
```

La respuesta incluye `token` y `usuario`; nunca incluye la contraseña.

Crear una solicitud:

```json
{
  "id_actividad": 2,
  "fecha_actividad": "2026-10-20",
  "nombre_actividad": "Taller de ejemplo",
  "carga_horaria_en_minutos": 120
}
```

`fecha_actividad` puede omitirse y se asigna la fecha actual. La identidad del solicitante se toma de la sesión, no del JSON. El endpoint devuelve `201` con `id_solicitud_cae`.

Resolver una solicitud:

```json
{"estado_solicitud":"RECHAZADO","observaciones":"Falta el certificado"}
```

El estado debe ser `APROBADO` o `RECHAZADO`; el rechazo requiere observación. Solo se puede resolver una solicitud una vez.

Subir un documento usa `multipart/form-data`: campos `tipo_documento` (máximo 32 caracteres), `observaciones` (opcional, máximo 180) y `documento` (archivo, máximo 5 MB).

## Ejecutar pruebas

```powershell
Set-Location .\backend
go test ./...
```

Las pruebas usan una base SQLite temporal; no modifican `data/sistema_cae.db`.

## Pendientes de negocio antes de cerrar el frontend

- Acordar cómo se calcula y persiste el crédito CAE. La calculadora existente requiere tipo de actividad y, para conferencias, organizador; el esquema actual no guarda el organizador ni el resultado calculado. El backend no inventa esos datos ni aplica una fórmula que pueda dar créditos incorrectos.
- Crear o confirmar cuentas con roles `dir_academica` o `dir_carrera` para probar el circuito de revisión en la base de desarrollo; actualmente las cuentas existentes son estudiantes.
- Definir si los documentos deben poder adjuntarse después de que una solicitud ya fue resuelta y qué formatos se aceptan. Actualmente se permite PDF, imágenes u otros archivos de hasta 5 MB.
