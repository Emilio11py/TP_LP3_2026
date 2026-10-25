import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { App } from './app';
import { AuthService } from './core/auth.service';
import { DatosRegistro, Usuario } from './models/usuario';

describe('App', () => {
  let httpTesting: HttpTestingController;

  beforeEach(async () => {
    localStorage.clear();
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [provideHttpClient(), provideHttpClientTesting()],
    }).compileComponents();
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpTesting.verify());

  it('shows login and registration tabs', async () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    await fixture.whenStable();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Iniciar sesión');
    expect(compiled.textContent).toContain('Registrarme');
    expect(compiled.querySelector('#nro-cedula')).toBeTruthy();
    expect(compiled.querySelector('#contrasenha')).toBeTruthy();
  });

  it('hides administration screens from students', () => {
    const fixture = TestBed.createComponent(App);
    TestBed.inject(AuthService).usuarioActual.set(usuarioDePrueba('estudiante'));
    fixture.detectChanges();
    const navigation = fixture.nativeElement.querySelector('nav') as HTMLElement;
    expect(navigation.textContent).toContain('Panel del Estudiante');
    expect(navigation.textContent).not.toContain('Panel Administrativo');
    expect(navigation.textContent).not.toContain('Gestión de Usuarios');
  });

  it('shows user management to administrative roles', () => {
    const fixture = TestBed.createComponent(App);
    TestBed.inject(AuthService).usuarioActual.set(usuarioDePrueba('dir_carrera'));
    fixture.detectChanges();
    const navigation = fixture.nativeElement.querySelector('nav') as HTMLElement;
    expect(navigation.textContent).toContain('Panel Administrativo');
    expect(navigation.textContent).toContain('Gestión de Usuarios');
    navigation.querySelector('button:last-child')?.dispatchEvent(new Event('click'));
    fixture.detectChanges();
    httpTesting.expectOne('http://localhost:8080/api/usuarios').flush([]);
    httpTesting.expectOne('http://localhost:8080/api/usuarios/pendientes').flush([]);
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Usuarios existentes');

    navigation.querySelector('button:nth-child(3)')?.dispatchEvent(new Event('click'));
    fixture.detectChanges();
    navigation.querySelector('button:last-child')?.dispatchEvent(new Event('click'));
    fixture.detectChanges();
    httpTesting.expectOne('http://localhost:8080/api/usuarios').flush([]);
    httpTesting.expectOne('http://localhost:8080/api/usuarios/pendientes').flush([]);
  });

  it('submits a registration request for manual approval', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    fixture.nativeElement.querySelector('#register-tab').click();
    fixture.detectChanges();
    const registrationPassword = fixture.nativeElement.querySelector('#registro-contrasenha') as HTMLInputElement;
    expect(registrationPassword.type).toBe('password');
    fixture.nativeElement.querySelector('[aria-label="Mostrar contraseña"]').click();
    fixture.detectChanges();
    expect(registrationPassword.type).toBe('text');
    fixture.nativeElement.querySelector('[aria-label="Ocultar contraseña"]').click();
    fixture.detectChanges();
    expect(registrationPassword.type).toBe('password');
    Object.assign(Reflect.get(fixture.componentInstance, 'registro') as DatosRegistro, {
      nro_cedula: '2020',
      nombres: 'Persona',
      apellidos: 'Nueva',
      codigo_matricula: 'P020',
      email: 'nueva@example.test',
      telefono_celular: '099000020',
      contrasenha: 'secure-password',
      carrera: 'Ing. Informática',
    });
    fixture.detectChanges();
    (fixture.nativeElement.querySelector('.registration-form') as HTMLFormElement)
      .dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    const request = httpTesting.expectOne('http://localhost:8080/api/auth/register');
    expect(request.request.body.role_acceso).toBe('estudiante');
    expect(request.request.body.nro_cedula).toBe('2020');
    expect(request.request.body.carrera).toBe('Ing. Informática');
    request.flush({
      estado: 'PENDIENTE',
      mensaje: 'Solicitud enviada. El acceso estará disponible después de la aprobación.',
    }, { status: 201, statusText: 'Created' });
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Solicitud enviada');
    expect(fixture.nativeElement.querySelector('button[type="submit"]').disabled).toBe(true);
  });
});

function usuarioDePrueba(role: string): Usuario {
  return {
    id_usuario: 1,
    nro_cedula: 1001,
    nombres: 'Usuario',
    apellidos: 'Prueba',
    codigo_matricula: 'A001',
    email: 'usuario@example.test',
    telefono_celular: '099000001',
    role_acceso: role,
    carrera: 'Informática',
    activo: 1,
  };
}
