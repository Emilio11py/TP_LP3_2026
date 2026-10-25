export interface Usuario {
  id_usuario: number;
  nro_cedula: number;
  nombres: string;
  apellidos: string;
  codigo_matricula: string;
  email: string;
  telefono_celular: string;
  role_acceso: string;
  carrera: string;
  activo: number;
}

export interface DatosRegistro {
  nro_cedula: string;
  nombres: string;
  apellidos: string;
  codigo_matricula: string;
  email: string;
  telefono_celular: string;
  contrasenha: string;
  role_acceso: string;
  carrera: string;
}

export interface DatosUsuario extends DatosRegistro {
  activo: number;
}

export interface UsuarioPendiente {
  id: number;
  nro_cedula: number;
  nombres: string;
  apellidos: string;
  codigo_matricula: string;
  email: string;
  telefono_celular: string;
  role_acceso: string;
  carrera: string;
  estado: 'PENDIENTE' | 'APROBADO' | 'RECHAZADO';
  fecha_solicitud: string;
  fecha_revision: string;
  revisado_por: number;
  revisor_nombre: string;
}
