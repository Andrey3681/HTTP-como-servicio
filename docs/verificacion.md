# Protocolo de Pruebas y Verificación - HTTPaaS

Este documento define la matriz de pruebas end-to-end y los comandos reproducibles para validar el funcionamiento del sistema HTTPaaS.

---

## 1. Verificación de Infraestructura Base

### 1.0 Configuración inicial del laboratorio
La primera vez se prepara todo desde la preparación del panel (`/api/setup/*`, ver [`setup-inicial.md`](setup-inicial.md)) o consultando su estado:

```bash
curl -s http://localhost:8080/api/setup/estado | jq '{requiere_configuracion, completado, pasos: [.pasos[] | {id, estado, progreso}]}'
```

Comprobaciones posteriores:

```bash
# La subred y la IP del DNS se derivan de la interfaz host-only real
curl -s http://localhost:8080/api/setup/estado | jq '.red'
DNS_IP=$(curl -s http://localhost:8080/api/setup/estado | jq -r '.red.ip_dns')

# Interfaz host-only del proyecto (la que no es 192.168.56.1)
VBoxManage list hostonlyifs

# Máquinas virtuales base
VBoxManage list vms | grep -E '"ns"|"ApacheServer"'

# El disco plantilla debe reportar Type: multiattach
VBoxManage showvminfo ApacheServer --machinereadable | grep SATA-0-0
VBoxManage showmediuminfo disk "<ruta-ApacheServer.vdi>" | grep -E '^Type'

# Guest Additions activas en el invitado (habilita la lectura de IP)
VBoxManage guestproperty get ns /VirtualBox/GuestAdd/Version
VBoxManage guestproperty get ApacheServer /VirtualBox/GuestAdd/Version

# El servidor DNS se resuelve a sí mismo
dig @"$DNS_IP" ns.cloud.local +short
```

### 1.1 Verificación de Servidor DNS (VM `ns`)
```bash
# Consultar si Bind9 responde en la IP derivada de la red
dig @"$DNS_IP" ns.cloud.local +short

# Verificar consulta SOA de la zona
dig @"$DNS_IP" cloud.local SOA +noall +answer
```

### 1.2 Verificación de Disco Base Multiconexión
```bash
# Comprobar el modo de disco del medio base
VBoxManage showmediuminfo "ApacheServer.vdi"
# Debe reportar: "Type: multiattach"
```

---

## 2. Verificación de la API REST del Orquestador

### 2.1 Comprobar Estado de Salud (`GET /api/salud`)
```bash
curl -s http://localhost:8080/api/salud | jq .
```
Respuesta esperada:
```json
{
  "estado": "ok",
  "vboxmanage": true,
  "bind9": true
}
```

### 2.2 Aprovisionar Instancia `paisajes` (`POST /api/instancias`)
```bash
# Preparar un archivo zip de prueba
mkdir -p /tmp/test-site
echo "<h1>Bienvenido a Paisajes Naturales</h1>" > /tmp/test-site/index.html
(cd /tmp/test-site && zip -r /tmp/archivos.zip .)

# Enviar solicitud multipart
curl -X POST http://localhost:8080/api/instancias \
  -F "nombre=paisajes" \
  -F "archivo=@/tmp/archivos.zip" | jq .
```
Respuesta esperada: Código `201 Created` con JSON que incluya la IP asignada por DHCP y el FQDN `paisajes.cloud.local`.

### 2.3 Verificar Registro Dinámico en Bind9
```bash
dig @"$DNS_IP" paisajes.cloud.local +short
# Debe responder la IP de la instancia (la del JSON de creación)
```

### 2.4 Verificar Publicación del Servidor Apache
```bash
IP_INSTANCIA=<ip-del-json-de-creacion>
curl -I http://"$IP_INSTANCIA"/
curl http://"$IP_INSTANCIA"/
# Debe retornar el HTML subido en el .zip
```

### 2.5 Consultar Lista de Instancias Activas (`GET /api/instancias`)
```bash
curl -s http://localhost:8080/api/instancias | jq .
```

### 2.6 Aprovisionar Segunda Instancia `negocios`
```bash
# Preparar segundo zip
mkdir -p /tmp/test-negocios
echo "<h1>Soluciones para tu empresa - Negocios</h1>" > /tmp/test-negocios/index.html
(cd /tmp/test-negocios && zip -r /tmp/pagina.zip .)

# Enviar solicitud multipart
curl -X POST http://localhost:8080/api/instancias \
  -F "nombre=negocios" \
  -F "archivo=@/tmp/pagina.zip" | jq .
```

### 2.7 Eliminar Instancia (`DELETE /api/instancias/{id}`)
```bash
curl -X DELETE http://localhost:8080/api/instancias/paisajes | jq .
```
Comprobación post-eliminación:
- `VBoxManage list vms` ya no debe listar `paisajes`.
- `dig @"$DNS_IP" paisajes.cloud.local +short` debe responder vacío (NXDOMAIN).

