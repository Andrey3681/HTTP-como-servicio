# Diseño de la Solución HTTPaaS

## 1. Vista de Despliegue y Topología de Red

El laboratorio se ejecuta sobre el hipervisor **Oracle VirtualBox** utilizando una red interna denominada `cloudnet` con direccionamiento privado `192.168.10.0/24`.

```text
Host (Equipo Anfitrión)
 └── IP Host-Only: 192.168.10.1
 └── Orquestador Go: http://localhost:8080 (o http://192.168.10.1:8080)

VM DNS (ns)
 └── SO: Debian 13.0 CLI
 └── IP fija: 192.168.10.10/24
 └── Servicio: Bind9 (named)
 └── Zona: cloud.local.
 └── Servidor de Nombres: ns.cloud.local.

VM Base (Plantilla / ApacheServer.vdi)
 └── Modo de disco: multiattach (multiconexión)
 └── SO: Debian 13.0 CLI con Apache HTTP Server y Guest Additions / SSH habilitado

Instancias Dinámicas (ej. paisajes, negocios)
 └── Disco diferencial anclado a ApacheServer.vdi
 └── IPs dinámicas o asignadas: 192.168.10.30, 192.168.10.31, ...
 └── Apache sirviendo contenido en /var/www/html/
 └── Hostname y FQDN: <host>.cloud.local.
```

---

## 2. Vista de Procesos: Flujo de Aprovisionamiento

```text
[Usuario]                 [Web / API]              [Orquestador]             [VBox]              [Bind9]             [Deployer]
   |                           |                         |                      |                   |                    |
   |-- 1. POST /api/instancias |                         |                      |                   |                    |
   |   (nombre, archivo.zip) ->|                         |                      |                   |                    |
   |                           |-- 2. Validar entrada -> |                      |                   |                    |
   |                           |-- 3. Aprovisionar() --> |                      |                   |                    |
   |                           |                         |-- 4. Crear VM -----> |                   |                    |
   |                           |                         |      (multiattach)   |                   |                    |
   |                           |                         |<- 5. VM creada ------|                   |                    |
   |                           |                         |-- 6. Iniciar VM ---> |                   |                    |
   |                           |                         |-- 7. Esperar IP ---> |                   |                    |
   |                           |                         |<- 8. IP 192.168.10.x |                   |                    |
   |                           |                         |-- 9. Registrar DNS --------------------> |                    |
   |                           |                         |<- 10. DNS OK ---------------------------|                    |
   |                           |                         |-- 11. Desplegar .zip ---------------------------------------> |
   |                           |                         |<- 12. Despliegue OK <---------------------------------------- |
   |                           |                         |-- 13. Guardar en BD  |                   |                    |
   |                           |<- 14. Instancia lista --|                      |                   |                    |
   |<- 15. 201 Created --------|                         |                      |                   |                    |
```

### Manejo de Compensaciones (Rollback)
- Si falla el paso 9 (DNS): se destruye la VM creada en VBox.
- Si falla el paso 11 (Deploy): se elimina el registro en Bind9 y se destruye la VM creada en VBox.

---

## 3. Vista de Datos y Modelo del Dominio

### Entidad `Instance`
- `ID`: Identificador único (UUID o slug normalizado).
- `Hostname`: Nombre único dentro del dominio (ej. `paisajes`).
- `FQDN`: Nombre de dominio completamente cualificado (ej. `paisajes.cloud.local`).
- `IP`: Dirección IPv4 asignada (`192.168.10.x`).
- `URL`: Enlace directo de acceso (`http://paisajes.cloud.local`).
- `Status`: Estado de la instancia (`CREATING`, `RUNNING`, `ERROR`, `TERMINATED`).
- `CreatedAt`: Marca temporal de creación.

### Persistencia
La información de las instancias activas se persiste en un archivo JSON atómico o base SQLite liviana en la carpeta `data/instances.json` para garantizar que el panel web conserve el estado tras reinicios del orquestador.

---

## 4. Validación y Seguridad

1. **Reglas de Hostname**:
   - Longitud entre 3 y 32 caracteres.
   - Caracteres permitidos: letras minúsculas `[a-z]`, números `[0-9]` y guiones `-`.
   - No puede iniciar ni terminar con guion.
   - No colisionar con nombres reservados del sistema (`ns`, `router`, `gateway`, `localhost`).
2. **Validación de Paquete Web (`.zip`)**:
   - Tamaño máximo admitido: 50 MB.
   - Prevención de **Zip Slip / Path Traversal**: Se verifica que ninguna ruta dentro del archivo `.zip` contenga secuencias `../` ni intente escribir fuera de `/var/www/html/`.
   - Presencia de al menos un archivo `index.html` en la raíz del archivo comprimido o en el directorio principal.

