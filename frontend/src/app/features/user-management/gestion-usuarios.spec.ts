import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DatosUsuario, UsuarioPendiente } from '../../models/usuario';
import { GestionUsuarios } from './gestion-usuarios';

describe('GestionUsuarios', () => {
  let httpTesting: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [GestionUsuarios],
      providers: [provideHttpClient(), provideHttpClientTesting()],
    }).compileComponents();
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    vi.useRealTimers();
    httpTesting.verify();
  });

  it('loads and displays the users table when the section opens', async () => {
    const fixture = TestBed.createComponent(GestionUsuarios);
    fixture.detectChanges(true);

    const usersRequest = httpTesting.expectOne('http://localhost:8080/api/usuarios');
    expect(usersRequest.request.method).toBe('GET');
    usersRequest.flush([existingUser]);
    httpTesting.expectOne('http://localhost:8080/api/usuarios/pendientes')
      .flush([usuarioPendienteDePrueba(1)]);
    fixture.detectChanges();
    expect(Reflect.get(fixture.componentInstance, 'usuarios')).toHaveLength(1);
    expect(Reflect.get(fixture.componentInstance, 'cargandoUsuarios')).toBe(false);
    expect(fixture.nativeElement.querySelector('.badge').textContent).toBe('1');
    await fixture.whenStable();
    fixture.detectChanges(true);

    expect(fixture.nativeElement.textContent).toContain('UsuarioExistente');
    expect(fixture.nativeElement.textContent).toContain('Usuarios existentes');
    expect(fixture.nativeElement.textContent).not.toContain('Solicitudes de registro');
    httpTesting.expectNone('http://localhost:8080/api/usuarios/pendientes');
  });

  it('creates a user and immediately updates the visible list', async () => {
    const fixture = TestBed.createComponent(GestionUsuarios);
    fixture.detectChanges();
    httpTesting.expectOne('http://localhost:8080/api/usuarios').flush([existingUser]);
    httpTesting.expectOne('http://localhost:8080/api/usuarios/pendientes').flush([]);
    fixture.detectChanges();

    const createButton = buscarBoton(fixture.nativeElement, 'Crear usuario');
    createButton.click();
    fixture.detectChanges();
    const passwordInput = fixture.nativeElement.querySelector('[name="nuevaContrasenha"]') as HTMLInputElement;
    expect(passwordInput.type).toBe('password');
    fixture.nativeElement.querySelector('[aria-label="Mostrar contraseña"]').click();
    fixture.detectChanges();
    expect(passwordInput.type).toBe('text');

    Object.assign(Reflect.get(fixture.componentInstance, 'nuevoUsuario') as DatosUsuario, {
      nro_cedula: '1002',
      nombres: 'Persona',
      apellidos: 'Creada',
      codigo_matricula: 'A002',
      email: 'persona@example.test',
      telefono_celular: '099000002',
      contrasenha: 'secure-password',
      carrera: 'Ing. Informática',
    });
    await fixture.whenStable();
    fixture.detectChanges(true);
    expect(Reflect.get(fixture.componentInstance, 'nuevoUsuario')).toMatchObject({
      nro_cedula: '1002',
      nombres: 'Persona',
      apellidos: 'Creada',
      codigo_matricula: 'A002',
      email: 'persona@example.test',
      telefono_celular: '099000002',
      contrasenha: 'secure-password',
    } satisfies Partial<DatosUsuario>);
    const form = fixture.nativeElement.querySelector('.user-form') as HTMLFormElement;
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    const createRequest = httpTesting.expectOne('http://localhost:8080/api/usuarios');
    expect(createRequest.request.method).toBe('POST');
    expect(createRequest.request.body).toMatchObject({
      nro_cedula: '1002',
      nombres: 'Persona',
      apellidos: 'Creada',
      codigo_matricula: 'A002',
      email: 'persona@example.test',
      telefono_celular: '099000002',
      contrasenha: 'secure-password',
      carrera: 'Ing. Informática',
    });
    createRequest.flush({ ...existingUser, id_usuario: 2, nombres: 'Persona', apellidos: 'Creada' }, {
      status: 201,
      statusText: 'Created',
    });
    const refreshedUsersRequest = httpTesting.expectOne('http://localhost:8080/api/usuarios');
    refreshedUsersRequest.flush([
      existingUser,
      { ...existingUser, id_usuario: 2, nombres: 'Persona', apellidos: 'Creada' },
    ]);
    await fixture.whenStable();
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent).toContain('El usuario se creó correctamente.');
    expect(fixture.nativeElement.textContent).toContain('PersonaCreada');
    expect(fixture.nativeElement.textContent).toContain('2 usuario(s) registrados');
  });

  it('loads pending registrations only when the pending button is pressed', () => {
    const fixture = TestBed.createComponent(GestionUsuarios);
    fixture.detectChanges();
    httpTesting.expectOne('http://localhost:8080/api/usuarios').flush([existingUser]);
    httpTesting.expectOne('http://localhost:8080/api/usuarios/pendientes')
      .flush([usuarioPendienteDePrueba(1), usuarioPendienteDePrueba(2)]);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.badge').textContent).toBe('2');

    buscarBoton(fixture.nativeElement, 'Solicitudes pendientes').click();
    fixture.detectChanges();
    const pendingRequest = httpTesting.expectOne('http://localhost:8080/api/usuarios/pendientes');
    expect(pendingRequest.request.method).toBe('GET');
    pendingRequest.flush([]);
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent).toContain('Solicitudes de registro');
    expect(fixture.nativeElement.textContent).toContain('Usuarios existentes');
  });

  it('refreshes the pending count automatically while user management is open', async () => {
    vi.useFakeTimers();
    const fixture = TestBed.createComponent(GestionUsuarios);
    fixture.detectChanges();
    httpTesting.expectOne('http://localhost:8080/api/usuarios').flush([existingUser]);
    httpTesting.expectOne('http://localhost:8080/api/usuarios/pendientes').flush([]);
    fixture.detectChanges();

    await vi.advanceTimersByTimeAsync(15000);
    const refresh = httpTesting.expectOne('http://localhost:8080/api/usuarios/pendientes');
    refresh.flush([usuarioPendienteDePrueba(1)]);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.badge').textContent).toBe('1');
    fixture.destroy();
  });
});

const existingUser = {
  id_usuario: 1,
  nro_cedula: 1001,
  nombres: 'Usuario',
  apellidos: 'Existente',
  codigo_matricula: 'A001',
  email: 'existente@example.test',
  telefono_celular: '099000001',
  role_acceso: 'estudiante',
  carrera: 'Ing. Informática',
  activo: 1,
};

function usuarioPendienteDePrueba(id: number): UsuarioPendiente {
  return {
    id,
    nro_cedula: 2000 + id,
    nombres: 'Pendiente',
    apellidos: `${id}`,
    codigo_matricula: `P00${id}`,
    email: `pendiente${id}@example.test`,
    telefono_celular: '099000010',
    role_acceso: 'estudiante',
    carrera: 'Ing. Informática',
    estado: 'PENDIENTE',
    fecha_solicitud: '2026-10-25T10:00:00Z',
    fecha_revision: '',
    revisado_por: 0,
    revisor_nombre: '',
  };
}

function buscarBoton(root: HTMLElement, label: string): HTMLButtonElement {
  const button = Array.from(root.querySelectorAll('button'))
    .find((item) => item.textContent?.includes(label));
  if (!button) {
    throw new Error(`No se encontró el botón ${label}`);
  }
  return button;
}
