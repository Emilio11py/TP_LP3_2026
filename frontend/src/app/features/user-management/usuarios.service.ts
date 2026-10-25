import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { map, Observable } from 'rxjs';
import { AuthService } from '../../core/auth.service';
import { DatosUsuario, Usuario, UsuarioPendiente } from '../../models/usuario';

const API_URL = 'http://localhost:8080/api/usuarios';

@Injectable({ providedIn: 'root' })
export class UsuariosService {
  private readonly http = inject(HttpClient);
  private readonly auth = inject(AuthService);

  listar(): Observable<Usuario[]> {
    return this.http.get<Usuario[]>(API_URL, { headers: this.auth.encabezadosAutorizacion() });
  }

  listarPendientes(): Observable<UsuarioPendiente[]> {
    return this.http.get<UsuarioPendiente[]>(`${API_URL}/pendientes`, {
      headers: this.auth.encabezadosAutorizacion(),
    });
  }

  contarPendientes(): Observable<number> {
    return this.listarPendientes().pipe(
      map((solicitudes) => solicitudes.filter((solicitud) => solicitud.estado === 'PENDIENTE').length),
    );
  }

  crear(datos: DatosUsuario): Observable<Usuario> {
    return this.http.post<Usuario>(API_URL, datos, {
      headers: this.auth.encabezadosAutorizacion(),
    });
  }

  modificar(id: number, datos: DatosUsuario): Observable<Usuario> {
    return this.http.put<Usuario>(`${API_URL}/${id}`, datos, {
      headers: this.auth.encabezadosAutorizacion(),
    });
  }

  eliminar(id: number): Observable<void> {
    return this.http.delete<void>(`${API_URL}/${id}`, {
      headers: this.auth.encabezadosAutorizacion(),
    });
  }

  cambiarEstadoSolicitud(id: number, estado: string): Observable<void> {
    return this.http.patch<void>(
      `${API_URL}/pendientes/${id}/estado`,
      { estado },
      { headers: this.auth.encabezadosAutorizacion() },
    );
  }
}
