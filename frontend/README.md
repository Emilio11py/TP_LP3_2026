# Frontend del Sistema CAE

Aplicación web de Angular para iniciar sesión y navegar por las secciones iniciales del sistema.

## Requisitos

- Node.js y npm
- API Go del proyecto ejecutándose en `http://localhost:8080`

## Ejecutar

Desde esta carpeta:

```powershell
npm install
npm start
```

Abre `http://localhost:4200`. El inicio de sesión usa el número de documento y la contraseña de una cuenta activa en la base de datos de la API.

La pestaña de registro y los paneles de estudiante y administración son pantallas iniciales; su integración funcional se realizará en las próximas etapas.

## Validar

```powershell
npm test
npm run build
```
