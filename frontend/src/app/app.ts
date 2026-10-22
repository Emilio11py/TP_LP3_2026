import { Component, OnInit, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AuthService } from './auth.service';
import { Usuario } from './usuario';

type Seccion = 'usuario' | 'estudiante' | 'administrativo';
type PestanaAcceso = 'ingresar' | 'registrarme';

@Component({
  imports: [FormsModule],
  selector: 'app-root',
  templateUrl: './app.html',
})
export class App implements OnInit {
  private readonly authService = inject(AuthService);

  protected readonly auth = this.authService;
  protected pestana: PestanaAcceso = 'ingresar';
  protected seccion: Seccion = 'usuario';
  protected nroCedula = '';
  protected contrasenha = '';
  protected mostrarContrasenha = false;
  protected matricula = '';
  protected cargando = false;
  protected mensajeError = '';
  protected mensajeRegistro = 'El registro estará disponible próximamente.';

  ngOnInit(): void {
    this.cargando = true;
    this.authService.restaurarSesion().subscribe({
      next: () => {
        this.cargando = false;
      },
      error: () => {
        this.mensajeError = 'No se pudo conectar con el servidor. Intente nuevamente.';
        this.cargando = false;
      },
    });
  }

  protected seleccionarPestana(pestana: PestanaAcceso): void {
    this.pestana = pestana;
    this.mensajeError = '';
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
}
