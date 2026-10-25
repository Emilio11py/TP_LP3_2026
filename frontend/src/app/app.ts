import { ChangeDetectorRef, Component, OnInit, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TimeoutError, finalize, timeout } from 'rxjs';
import { AuthService } from './core/auth.service';
import { Usuario } from './models/usuario';
import { GestionUsuarios } from './features/user-management/gestion-usuarios';

type Seccion = 'usuario' | 'estudiante' | 'administrativo' | 'gestion-usuarios';
type PestanaAcceso = 'ingresar' | 'registrarme';

@Component({
  imports: [FormsModule, GestionUsuarios],
  selector: 'app-root',
  templateUrl: './app.html',
})
export class App implements OnInit {
  private readonly authService = inject(AuthService);
  private readonly changeDetector = inject(ChangeDetectorRef);

  protected readonly auth = this.authService;
  protected pestana: PestanaAcceso = 'ingresar';
  protected seccion: Seccion = 'usuario';
  protected nroCedula = '';
  protected contrasenha = '';
  protected mostrarContrasenha = false;
  protected mostrarContrasenhaRegistro = false;
  protected registro = {
    nro_cedula: '',
    nombres: '',
    apellidos: '',
    codigo_matricula: '',
    email: '',
    telefono_celular: '',
    contrasenha: '',
    role_acceso: 'estudiante',
    carrera: '',
  };
  protected cargando = false;
  protected mensajeError = '';
  protected mensajeRegistro = '';
  protected registroEnviado = false;

  ngOnInit(): void {
    this.cargando = true;
    this.authService.restaurarSesion().pipe(
      timeout({ first: 15000 }),
      finalize(() => {
        this.cargando = false;
        this.changeDetector.markForCheck();
      }),
    ).subscribe({
      error: () => {
        this.mensajeError = 'No se pudo verificar la sesión. Comprueba que el backend esté activo e intenta nuevamente.';
        this.changeDetector.markForCheck();
      },
    });
  }

  protected seleccionarPestana(pestana: PestanaAcceso): void {
    this.pestana = pestana;
    this.mostrarContrasenhaRegistro = false;
    this.mensajeError = '';
    this.mensajeRegistro = '';
    this.registroEnviado = false;
  }

  protected registrarse(): void {
    this.mensajeError = '';
    this.mensajeRegistro = '';
    this.cargando = true;
    this.authService.registrarse(this.registro).pipe(
      timeout({ first: 15000 }),
      finalize(() => {
        this.cargando = false;
        this.changeDetector.markForCheck();
      }),
    ).subscribe({
      next: (respuesta) => {
        this.mensajeRegistro = respuesta.mensaje;
        this.registroEnviado = true;
        this.registro = {
          nro_cedula: '',
          nombres: '',
          apellidos: '',
          codigo_matricula: '',
          email: '',
          telefono_celular: '',
          contrasenha: '',
          role_acceso: 'estudiante',
          carrera: '',
        };
        this.changeDetector.markForCheck();
      },
      error: (error: { error?: { error?: string } }) => {
        this.mensajeError = this.mensajeDeError(error, 'No se pudo enviar la solicitud. Intente nuevamente.');
        this.changeDetector.markForCheck();
      },
    });
  }

  protected ingresar(): void {
    this.mensajeError = '';
    this.cargando = true;
    this.authService.iniciarSesion(this.nroCedula.trim(), this.contrasenha).subscribe({
      next: () => {
        this.seccion = 'usuario';
        this.cargando = false;
      },
      error: (error: { error?: { error?: string } }) => {
        this.mensajeError = error.error?.error ?? 'No se pudo iniciar sesión. Intente nuevamente.';
        this.cargando = false;
      },
    });
  }

  protected cerrarSesion(): void {
    this.mensajeError = '';
    this.cargando = true;
    this.authService.cerrarSesion().subscribe({
      next: () => {
        this.nroCedula = '';
        this.contrasenha = '';
        this.cargando = false;
      },
      error: () => {
        this.mensajeError = 'No se pudo cerrar la sesión. Intente nuevamente.';
        this.cargando = false;
      },
    });
  }

  protected nombreCompleto(usuario: Usuario): string {
    return `${usuario.nombres} ${usuario.apellidos}`.trim();
  }

  private mensajeDeError(error: { error?: { error?: string } }, fallback: string): string {
    if (error instanceof TimeoutError) {
      return 'El servidor tardó demasiado en responder. Comprueba que el backend esté activo en localhost:8080.';
    }
    if (!error.error?.error || error.error.error === 'Ruta no encontrada') {
      return fallback;
    }
    return error.error.error;
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

  protected get esAdministrativo(): boolean {
    const role = this.auth.usuarioActual()?.role_acceso.toLowerCase().trim();
    return role === 'dir_academica' || role === 'dir_carrera';
  }
}
