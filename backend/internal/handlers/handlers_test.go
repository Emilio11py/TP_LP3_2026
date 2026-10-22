package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"tp-backend/internal/services"
)

func TestRequestLifecycle(t *testing.T) {
	db := testDB(t)
	handler := New(db, "http://localhost:4200")

	loginResponse := call(handler, http.MethodPost, "/api/auth/login", "", `{"email":"student@example.test","contrasenha":"student-password"}`, "")
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	if strings.Contains(loginResponse.Body.String(), "contrasenha") {
		t.Fatal("login response exposed a password field")
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &login); err != nil || login.Token == "" {
		t.Fatalf("invalid login response: %v", err)
	}

	var storedPassword string
	if err := db.QueryRow("SELECT CONTRASENHA FROM USUARIOS WHERE ID_USUARIO = 1").Scan(&storedPassword); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(storedPassword, "pbkdf2-sha256$") {
		t.Fatal("legacy password was not upgraded after login")
	}

	activities := call(handler, http.MethodGet, "/api/actividades", login.Token, "", "http://localhost:4200")
	if activities.Code != http.StatusOK || !strings.Contains(activities.Body.String(), "Cursos o Talleres") {
		t.Fatalf("activity response = %d, %s", activities.Code, activities.Body.String())
	}
	if activities.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4200" {
		t.Fatal("expected frontend CORS origin to be allowed")
	}

	created := call(handler, http.MethodPost, "/api/solicitudes", login.Token,
		`{"id_actividad":2,"fecha_actividad":"2026-10-20","nombre_actividad":"Taller de prueba","carga_horaria_en_minutos":120}`, "")
	if created.Code != http.StatusCreated {
		t.Fatalf("create request status = %d, body = %s", created.Code, created.Body.String())
	}
	var result struct {
		ID int64 `json:"id_solicitud_cae"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil || result.ID <= 0 {
		t.Fatalf("invalid create response: %v", err)
	}

	var ownerID int
	if err := db.QueryRow("SELECT ID_USUARIO_SOLICITA FROM SOLICITUDES_CAES WHERE ID_SOLICITUD_CAE = ?", result.ID).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	if ownerID != 1 {
		t.Fatalf("request owner = %d, want authenticated user 1", ownerID)
	}

	otherLogin := call(handler, http.MethodPost, "/api/auth/login", "", `{"email":"other@example.test","contrasenha":"other-password"}`, "")
	var other struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(otherLogin.Body.Bytes(), &other); err != nil || other.Token == "" {
		t.Fatalf("could not log in second student: %v", err)
	}
	otherRequests := call(handler, http.MethodGet, "/api/solicitudes", other.Token, "", "")
	if otherRequests.Code != http.StatusOK || strings.TrimSpace(otherRequests.Body.String()) != "[]" {
		t.Fatalf("student saw another user's requests: %d, %s", otherRequests.Code, otherRequests.Body.String())
	}
	hiddenRequest := call(handler, http.MethodGet, "/api/solicitudes/"+jsonNumber(result.ID), other.Token, "", "")
	if hiddenRequest.Code != http.StatusNotFound {
		t.Fatalf("other student's request detail status = %d, want 404", hiddenRequest.Code)
	}

	var upload bytes.Buffer
	form := multipart.NewWriter(&upload)
	if err := form.WriteField("tipo_documento", "CERTIFICADO"); err != nil {
		t.Fatal(err)
	}
	file, err := form.CreateFormFile("documento", "certificado.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("documento de prueba")); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/solicitudes/"+jsonNumber(result.ID)+"/documentos", &upload)
	request.Header.Set("Authorization", "Bearer "+login.Token)
	request.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", response.Code, response.Body.String())
	}
	var uploaded struct {
		ID int64 `json:"id_documento"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &uploaded); err != nil || uploaded.ID <= 0 {
		t.Fatalf("invalid upload response: %v", err)
	}
	download := call(handler, http.MethodGet, "/api/documentos/"+jsonNumber(uploaded.ID), login.Token, "", "")
	if download.Code != http.StatusOK || download.Body.String() != "documento de prueba" {
		t.Fatalf("download response = %d, %q", download.Code, download.Body.String())
	}

	reviewerLogin := call(handler, http.MethodPost, "/api/auth/login", "", `{"email":"reviewer@example.test","contrasenha":"reviewer-password"}`, "")
	var reviewer struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(reviewerLogin.Body.Bytes(), &reviewer); err != nil || reviewer.Token == "" {
		t.Fatalf("could not log in reviewer: %v", err)
	}
	decision := call(handler, http.MethodPatch, "/api/solicitudes/"+jsonNumber(result.ID)+"/decision",
		reviewer.Token, `{"estado_solicitud":"APROBADO","observaciones":"Documentación revisada"}`, "")
	if decision.Code != http.StatusNoContent {
		t.Fatalf("decision status = %d, body = %s", decision.Code, decision.Body.String())
	}
	repeatedDecision := call(handler, http.MethodPatch, "/api/solicitudes/"+jsonNumber(result.ID)+"/decision",
		reviewer.Token, `{"estado_solicitud":"RECHAZADO","observaciones":"Ya resuelta"}`, "")
	if repeatedDecision.Code != http.StatusConflict {
		t.Fatalf("second decision status = %d, want 409", repeatedDecision.Code)
	}
	detail := call(handler, http.MethodGet, "/api/solicitudes/"+jsonNumber(result.ID), login.Token, "", "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"estado_solicitud":"APROBADO"`) {
		t.Fatalf("student cannot see request decision: %d, %s", detail.Code, detail.Body.String())
	}
}

func TestAuthenticationAndCORS(t *testing.T) {
	handler := New(testDB(t), "http://localhost:4200")

	unauthorized := call(handler, http.MethodGet, "/api/solicitudes", "", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request status = %d, want 401", unauthorized.Code)
	}
	wrongPassword := call(handler, http.MethodPost, "/api/auth/login", "", `{"email":"student@example.test","contrasenha":"wrong"}`, "")
	if wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want 401", wrongPassword.Code)
	}
	forbiddenOrigin := call(handler, http.MethodGet, "/api/health", "", "", "https://not-allowed.example")
	if forbiddenOrigin.Code != http.StatusForbidden {
		t.Fatalf("unapproved CORS origin status = %d, want 403", forbiddenOrigin.Code)
	}
	preflight := call(handler, http.MethodOptions, "/api/solicitudes", "", "", "http://localhost:4200")
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", preflight.Code)
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := services.OpenDB(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	schema := []string{
		`CREATE TABLE USUARIOS (
			ID_USUARIO INTEGER PRIMARY KEY, NRO_CEDULA INTEGER NOT NULL, NOMBRES TEXT NOT NULL,
			APELLIDOS TEXT NOT NULL, CODIGO_MATRICULA TEXT NOT NULL, EMAIL TEXT NOT NULL,
			TELEFONO_CELULAR TEXT NOT NULL, CONTRASENHA VARCHAR(60) NOT NULL,
			ROLE_ACCESO TEXT NOT NULL, CARRERA TEXT NOT NULL, FECHA_ALTA DATE NOT NULL, ACTIVO INTEGER NOT NULL
		)`,
		`CREATE TABLE ACTIVIDADES (
			ID_ACTIVIDAD INTEGER PRIMARY KEY, NOMBRE_ACTIVIDAD TEXT NOT NULL, ACTIVO INTEGER NOT NULL
		)`,
		`CREATE TABLE SOLICITUDES_CAES (
			ID_SOLICITUD_CAE INTEGER PRIMARY KEY, FECHA_SOLICITUD DATE NOT NULL, HORA_SOLICITUD TIME NOT NULL,
			ID_ACTIVIDAD INTEGER NOT NULL, ID_USUARIO_SOLICITA INTEGER NOT NULL, ESTADO_SOLICITUD TEXT NOT NULL,
			OBSERVACIONES TEXT, FECHA_APROBACION_O_RECHAZO DATE, ID_USUARIO_APRUEBA_O_RECHAZA INTEGER,
			FOREIGN KEY(ID_ACTIVIDAD) REFERENCES ACTIVIDADES(ID_ACTIVIDAD),
			FOREIGN KEY(ID_USUARIO_SOLICITA) REFERENCES USUARIOS(ID_USUARIO),
			FOREIGN KEY(ID_USUARIO_APRUEBA_O_RECHAZA) REFERENCES USUARIOS(ID_USUARIO)
		)`,
		`CREATE TABLE DETALLE_SOLICITUDES_CAES (
			ID_DETALLE_SOLICITUD_CAE INTEGER PRIMARY KEY, ID_SOLICITUD_CAE INTEGER NOT NULL,
			FECHA_ACTIVIDAD DATE NOT NULL, ACTIVIDAD TEXT NOT NULL, CARGA_HORARIA_EN_MINUTOS INTEGER NOT NULL,
			SUPERVISADO TEXT, ID_USUARIO_SUPERVISA INTEGER,
			FOREIGN KEY(ID_SOLICITUD_CAE) REFERENCES SOLICITUDES_CAES(ID_SOLICITUD_CAE)
		)`,
		`CREATE TABLE DETALLE_DOCUMENT_SOLICITUD_CAES (
			ID_DETALLE_DOC_SOLICITUD_CAE INTEGER PRIMARY KEY, ID_SOLICITUD_CAE INTEGER NOT NULL,
			DOCUMENTO BLOB NOT NULL, TIPO_DOCUMENTO TEXT NOT NULL, OBSERVACIONES TEXT,
			FOREIGN KEY(ID_SOLICITUD_CAE) REFERENCES SOLICITUDES_CAES(ID_SOLICITUD_CAE)
		)`,
	}
	for _, statement := range schema {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	users := []struct {
		id, nationalID                                                 int
		name, surname, studentID, email, phone, password, role, career string
	}{
		{1, 1001, "Estudiante", "Uno", "A001", "student@example.test", "099000001", "student-password", "estudiante", "Informática"},
		{2, 1002, "Estudiante", "Dos", "A002", "other@example.test", "099000002", "other-password", "estudiante", "Informática"},
		{3, 1003, "Revisor", "Uno", "D001", "reviewer@example.test", "099000003", "reviewer-password", "dir_academica", "Informática"},
	}
	for _, user := range users {
		if _, err := db.Exec(`INSERT INTO USUARIOS
			(ID_USUARIO, NRO_CEDULA, NOMBRES, APELLIDOS, CODIGO_MATRICULA, EMAIL,
			 TELEFONO_CELULAR, CONTRASENHA, ROLE_ACCESO, CARRERA, FECHA_ALTA, ACTIVO)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '2026-01-01', 1)`,
			user.id, user.nationalID, user.name, user.surname, user.studentID, user.email,
			user.phone, user.password, user.role, user.career,
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO ACTIVIDADES VALUES (2, 'Cursos o Talleres', 1)"); err != nil {
		t.Fatal(err)
	}
	if err := services.ValidateSchema(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func call(handler http.Handler, method, path, token, body, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func jsonNumber(value int64) string {
	return strconv.FormatInt(value, 10)
}
