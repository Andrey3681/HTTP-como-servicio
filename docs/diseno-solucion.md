# Diseño de la Solución HTTPaaS

## 1. Vista de Despliegue y Topología de Red

El laboratorio se ejecuta sobre el hipervisor **Oracle VirtualBox** utilizando una red host-only cuya subred asigna VirtualBox (por ejemplo `192.168.57.0/24`); no se impone un direccionamiento fijo.

```text
Host (Equipo Anfitrión)
 └── IP host-only: la asignada por VirtualBox (p. ej. 192.168.57.1)
 └── Orquestador Go: http://localhost:8080

VM DNS (ns)
 └── SO: Debian 13.0 CLI
 └── IP: subred de la interfaz + .10 (p. ej. 192.168.57.10)
 └── Servicio: Bind9 (named)
 └── Zona: cloud.local.
 └── Servidor de Nombres: ns.cloud.local.

VM Base (Plantilla / ApacheServer.vdi)
 └── Modo de disco: multiattach (multiconexión)
 └── SO: Debian 13.0 CLI con Apache HTTP Server y SSH habilitado

Instancias Dinámicas (ej. paisajes, negocios)
 └── Disco diferencial anclado a ApacheServer.vdi
 └── IPs dinámicas dentro de la subred (p. ej. 192.168.57.100, .101, ...)
 └── Apache sirviendo contenido en /var/www/vhosts/<host>/public/
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
   |                           |                         |<- 8. IP de la subred |                   |                    |
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
- `IP`: Dirección IPv4 asignada por DHCP dentro de la subred de la interfaz.
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
   - Prevención de **Zip Slip / Path Traversal**: Se verifica que ninguna ruta dentro del archivo `.zip` contenga secuencias `../` ni intente escribir fuera de `/var/www/vhosts/<host>/public/`.
   - Presencia de al menos un archivo `index.html` en la raíz del archivo comprimido o en el directorio principal.

