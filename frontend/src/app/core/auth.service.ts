import { HttpClient, HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { Injectable, inject, signal } from '@angular/core';
import { Observable, catchError, map, of, tap, throwError } from 'rxjs';
import { DatosRegistro, Usuario } from '../models/usuario';

interface LoginResponse {
  token: string;
  usuario: Usuario;
}

const API_URL = 'http://localhost:8080/api';
const TOKEN_KEY = 'cae_token';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);

  readonly usuarioActual = signal<Usuario | null>(null);

  get tokenActual(): string | null {
    return localStorage.getItem(TOKEN_KEY);
  }

  encabezadosAutorizacion(): HttpHeaders {
    const token = this.tokenActual;
    return token ? this.crearEncabezados(token) : new HttpHeaders();
  }

  restaurarSesion(): Observable<Usuario | null> {
    const token = localStorage.getItem(TOKEN_KEY);
    if (!token) {
      return of(null);
    }

    return this.http
      .get<Usuario>(`${API_URL}/auth/me`, { headers: this.crearEncabezados(token) })
      .pipe(
        tap((usuario) => this.usuarioActual.set(usuario)),
        catchError((error: HttpErrorResponse) => {
          if (error.status === 401) {
            this.limpiarSesion();
            return of(null);
          }
          return throwError(() => error);
        }),
      );
  }

  iniciarSesion(nroCedula: string, contrasenha: string): Observable<Usuario> {
    return this.http
      .post<LoginResponse>(`${API_URL}/auth/login`, {
        nro_cedula: nroCedula,
        contrasenha,
      })
      .pipe(
        tap((respuesta) => {
          localStorage.setItem(TOKEN_KEY, respuesta.token);
          this.usuarioActual.set(respuesta.usuario);
        }),
        map((respuesta) => respuesta.usuario),
      );
  }

  registrarse(datos: DatosRegistro): Observable<{ estado: string; mensaje: string }> {
    return this.http.post<{ estado: string; mensaje: string }>(`${API_URL}/auth/register`, datos);
  }

  cerrarSesion(): Observable<void> {
    const token = this.tokenActual;
    if (!token) {
      this.limpiarSesion();
      return of(void 0);
    }

    return this.http
      .post<void>(`${API_URL}/auth/logout`, {}, { headers: this.crearEncabezados(token) })
      .pipe(tap(() => this.limpiarSesion()));
  }

  private crearEncabezados(token: string): HttpHeaders {
    return new HttpHeaders({ Authorization: `Bearer ${token}` });
  }

  private limpiarSesion(): void {
    localStorage.removeItem(TOKEN_KEY);
    this.usuarioActual.set(null);
  }
}
