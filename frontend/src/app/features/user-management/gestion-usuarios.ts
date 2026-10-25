import { Component, OnInit, inject } from '@angular/core';
import { DatePipe } from '@angular/common';
import { ChangeDetectorRef, DestroyRef } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { EMPTY, TimeoutError, catchError, exhaustMap, finalize, interval, startWith, timeout } from 'rxjs';
import { DatosUsuario, Usuario, UsuarioPendiente } from '../../models/usuario';
import { UsuariosService } from './usuarios.service';

@Component({
  imports: [DatePipe, FormsModule],
  selector: 'app-gestion-usuarios',
  styleUrl: './gestion-usuarios.css',
  templateUrl: './gestion-usuarios.html',
})
export class GestionUsuarios implements OnInit {
  private readonly usuariosService = inject(UsuariosService);
  private readonly changeDetector = inject(ChangeDetectorRef);
  private readonly destroyRef = inject(DestroyRef);

  protected usuarios: Usuario[] = [];
  protected solicitudes: UsuarioPendiente[] = [];
  protected cantidadPendiente = 0;
  protected nuevoUsuario = this.usuarioVacio();
  protected usuarioEditado: DatosUsuario | null = null;
  protected editandoId = 0;
  protected crearAbierto = false;
  protected pendientesAbiertos = false;
  protected mostrarContrasenhaNueva = false;
  protected mostrarContrasenhaEdicion = false;
  protected cargando = false;
  protected cargandoUsuarios = false;
  protected cargandoSolicitudes = false;
  protected mensaje = '';
  protected tipoMensaje: 'error' | 'success' = 'success';
  private originalEdicion = '';

  ngOnInit(): void {
    this.cargarUsuarios();
    interval(15000).pipe(
      startWith(0),
      exhaustMap(() => this.usuariosService.contarPendientes().pipe(
        timeout({ first: 10000 }),
        catchError((error: { error?: { error?: string } }) => {
          this.mostrarError(this.mensajeDeError(error, 'No se pudo actualizar la cantidad de solicitudes pendientes.'));
          this.changeDetector.markForCheck();
          return EMPTY;
        }),
      )),
      takeUntilDestroyed(this.destroyRef),
    ).subscribe((cantidad) => {
      this.cantidadPendiente = cantidad;
      this.changeDetector.markForCheck();
    });
  }

  protected cargarUsuarios(): void {
    this.cargandoUsuarios = true;
    this.usuariosService.listar().pipe(
      timeout({ first: 15000 }),
      finalize(() => {
        this.cargandoUsuarios = false;
        this.changeDetector.markForCheck();
      }),
    ).subscribe({
      next: (usuarios) => {
        this.usuarios = usuarios;
        this.changeDetector.markForCheck();
      },
      error: (error: { error?: { error?: string } }) => {
        this.mostrarError(this.mensajeDeError(error, 'No se pudieron cargar los usuarios.'));
        this.changeDetector.markForCheck();
      },
    });
  }

  protected abrirPendientes(): void {
    this.pendientesAbiertos = true;
    this.cargandoSolicitudes = true;
    this.usuariosService.listarPendientes().pipe(
      timeout({ first: 15000 }),
      finalize(() => {
        this.cargandoSolicitudes = false;
        this.changeDetector.markForCheck();
      }),
    ).subscribe({
      next: (solicitudes) => {
        this.solicitudes = solicitudes;
        this.cantidadPendiente = solicitudes.filter((solicitud) => solicitud.estado === 'PENDIENTE').length;
        this.changeDetector.markForCheck();
      },
      error: (error: { error?: { error?: string } }) => {
        this.mostrarError(this.mensajeDeError(error, 'No se pudieron cargar las solicitudes.'));
        this.changeDetector.markForCheck();
      },
    });
  }

  protected abrirCrear(): void {
    this.crearAbierto = true;
    this.mostrarContrasenhaNueva = false;
    this.editandoId = 0;
    this.usuarioEditado = null;
    this.nuevoUsuario = this.usuarioVacio();
    this.mensaje = '';
  }

  protected guardarNuevo(): void {
    this.cargando = true;
    this.usuariosService.crear(this.nuevoUsuario).pipe(
      timeout({ first: 15000 }),
      finalize(() => {
        this.cargando = false;
        this.changeDetector.markForCheck();
      }),
    ).subscribe({
      next: (usuario) => {
        this.usuarios = [...this.usuarios, usuario].sort((a, b) =>
          a.apellidos.localeCompare(b.apellidos) || a.nombres.localeCompare(b.nombres),
        );
        this.crearAbierto = false;
        this.nuevoUsuario = this.usuarioVacio();
        this.mostrarExito('El usuario se creó correctamente.');
        this.cargarUsuarios();
        this.changeDetector.markForCheck();
      },
      error: (error: { error?: { error?: string } }) => {
        this.mostrarError(this.mensajeDeError(error, 'No se pudo crear el usuario.'));
        this.changeDetector.markForCheck();
      },
    });
  }

  protected cancelarCrear(): void {
    if (this.datosIngresados(this.nuevoUsuario)) {
      this.nuevoUsuario = this.usuarioVacio();
      this.mostrarExito('Se borraron los datos. Presiona Cancelar otra vez para cerrar.');
      return;
    }
    this.crearAbierto = false;
    this.mensaje = '';
  }

  protected editar(usuario: Usuario): void {
    this.crearAbierto = false;
    this.mostrarContrasenhaEdicion = false;
    this.editandoId = usuario.id_usuario;
    this.usuarioEditado = {
      nro_cedula: String(usuario.nro_cedula),
      nombres: usuario.nombres,
      apellidos: usuario.apellidos,
      codigo_matricula: usuario.codigo_matricula,
      email: usuario.email,
      telefono_celular: usuario.telefono_celular,
      contrasenha: '',
      role_acceso: usuario.role_acceso,
      carrera: this.nombreCarrera(usuario.carrera),
      activo: usuario.activo,
    };
    this.originalEdicion = JSON.stringify(this.usuarioEditado);
    this.mensaje = '';
  }

  protected guardarEdicion(): void {
    if (!this.usuarioEditado || !this.editandoId) {
      return;
    }
    this.cargando = true;
    this.usuariosService.modificar(this.editandoId, this.usuarioEditado).pipe(
      timeout({ first: 15000 }),
      finalize(() => {
        this.cargando = false;
        this.changeDetector.markForCheck();
      }),
    ).subscribe({
      next: (usuario) => {
        this.usuarios = this.usuarios.map((item) =>
          item.id_usuario === usuario.id_usuario ? usuario : item,
        );
        this.usuarioEditado = null;
        this.editandoId = 0;
        this.mostrarExito('Los datos del usuario se guardaron correctamente.');
        this.cargarUsuarios();
        this.changeDetector.markForCheck();
      },
      error: (error: { error?: { error?: string } }) => {
        this.mostrarError(this.mensajeDeError(error, 'No se pudo modificar el usuario.'));
        this.changeDetector.markForCheck();
      },
    });
  }

  protected cancelarEdicion(): void {
    if (!this.usuarioEditado) {
      return;
    }
    if (JSON.stringify(this.usuarioEditado) !== this.originalEdicion) {
      this.usuarioEditado = JSON.parse(this.originalEdicion) as DatosUsuario;
      this.mostrarExito('Se descartaron los cambios. Presiona Cancelar otra vez para cerrar.');
      return;
    }
    this.usuarioEditado = null;
    this.editandoId = 0;
    this.mensaje = '';
  }

  protected eliminar(usuario: Usuario): void {
    if (!window.confirm(`¿Estás seguro de que deseas eliminar a ${usuario.nombres} ${usuario.apellidos}?`)) {
      return;
    }
    this.cargando = true;
    this.usuariosService.eliminar(usuario.id_usuario).pipe(
      timeout({ first: 15000 }),
      finalize(() => {
        this.cargando = false;
        this.changeDetector.markForCheck();
      }),
    ).subscribe({
      next: () => {
        this.usuarios = this.usuarios.filter((item) => item.id_usuario !== usuario.id_usuario);
        this.usuarioEditado = null;
        this.editandoId = 0;
        this.mostrarExito('El usuario se eliminó correctamente.');
        this.cargarUsuarios();
        this.changeDetector.markForCheck();
      },
      error: (error: { error?: { error?: string } }) => {
        this.mostrarError(this.mensajeDeError(error, 'No se pudo eliminar el usuario.'));
        this.changeDetector.markForCheck();
      },
    });
  }

  protected eliminarEditado(): void {
    const usuario = this.usuarios.find((item) => item.id_usuario === this.editandoId);
    if (usuario) {
      this.eliminar(usuario);
    }
  }

  protected cambiarEstado(solicitud: UsuarioPendiente, estado: 'APROBADO' | 'RECHAZADO' | 'PENDIENTE'): void {
    const accion = estado === 'APROBADO' ? 'aceptar' : estado === 'RECHAZADO' ? 'rechazar' : 'volver a pendiente';
    if (!window.confirm(`¿Estás seguro de que deseas ${accion} esta solicitud?`)) {
      return;
    }
    this.cargando = true;
    this.usuariosService.cambiarEstadoSolicitud(solicitud.id, estado).pipe(
      timeout({ first: 15000 }),
      finalize(() => {
        this.cargando = false;
        this.changeDetector.markForCheck();
      }),
    ).subscribe({
      next: () => {
        solicitud.estado = estado;
        if (estado === 'PENDIENTE') {
          solicitud.fecha_revision = '';
          solicitud.revisor_nombre = '';
        }
        this.mostrarExito(
          estado === 'APROBADO'
            ? 'Solicitud aceptada. El usuario ya puede iniciar sesión.'
            : estado === 'RECHAZADO'
              ? 'Solicitud rechazada.'
              : 'La solicitud volvió al estado pendiente.',
        );
        this.abrirPendientes();
        if (estado === 'APROBADO') {
          this.cargarUsuarios();
        }
        this.changeDetector.markForCheck();
      },
      error: (error: { error?: { error?: string } }) => {
        this.mostrarError(this.mensajeDeError(error, 'No se pudo actualizar la solicitud.'));
        this.changeDetector.markForCheck();
      },
    });
  }

  protected nombreCompleto(usuario: Usuario | UsuarioPendiente): string {
    return `${usuario.nombres} ${usuario.apellidos}`.trim();
  }

  protected nombreRol(rol: string): string {
    switch (rol.toLowerCase().trim()) {
      case 'estudiante':
        return 'Estudiante';
      case 'dir_academica':
        return 'Dirección Académica';
      case 'dir_carrera':
        return 'Dirección de Carrera';
      default:
        return rol;
    }
  }

  protected nombreCarrera(carrera: string): string {
    switch (carrera.toLowerCase().trim()) {
      case 'inf. informatica':
      case 'informática':
      case 'informatica':
      case 'ing. informatica':
      case 'ing. informática':
        return 'Ing. Informática';
      case 'ing. quimica':
      case 'ing. química':
        return 'Ing. Química';
      case 'ing. industrial':
        return 'Ing. Industrial';
      default:
        return carrera;
    }
  }

  protected abrirSolicitudes(): void {
    this.abrirPendientes();
  }

  protected cerrarSolicitudes(): void {
    this.pendientesAbiertos = false;
  }

  private usuarioVacio(): DatosUsuario {
    return {
      nro_cedula: '',
      nombres: '',
      apellidos: '',
      codigo_matricula: '',
      email: '',
      telefono_celular: '',
      contrasenha: '',
      role_acceso: 'estudiante',
      carrera: '',
      activo: 1,
    };
  }

  private datosIngresados(usuario: DatosUsuario): boolean {
    return Object.entries(usuario).some(([campo, valor]) =>
      campo === 'role_acceso' ? valor !== 'estudiante' : campo === 'activo' ? valor !== 1 : valor !== '',
    );
  }

  private mostrarExito(texto: string): void {
    this.mensaje = texto;
    this.tipoMensaje = 'success';
  }

  private mostrarError(texto: string): void {
    this.mensaje = texto;
    this.tipoMensaje = 'error';
  }

  private mensajeDeError(error: { error?: { error?: string } }, fallback: string): string {
    if (error instanceof TimeoutError) {
      return 'El servidor tardó demasiado en responder. Verifica que el backend siga activo en localhost:8080.';
    }
    if (!error.error?.error || error.error.error === 'Ruta no encontrada') {
      return fallback;
    }
    return error.error.error;
  }
}
