# Protocolo de Pruebas y Verificación - HTTPaaS

Este documento define la matriz de pruebas end-to-end y los comandos reproducibles para validar el funcionamiento del sistema HTTPaaS.

---

## 1. Verificación de Infraestructura Base

### 1.1 Verificación de Servidor DNS (VM `ns`)
```bash
# Consultar si el servidor Bind9 responde en la IP dedicada
dig @192.168.10.10 ns.cloud.local +short

# Verificar consulta SOA de la zona
dig @192.168.10.10 cloud.local SOA +noall +answer
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
Respuesta esperada: Código `201 Created` con JSON que incluya IP (ej. `192.168.10.30`) y FQDN `paisajes.cloud.local`.

### 2.3 Verificar Registro Dinámico en Bind9
```bash
dig @192.168.10.10 paisajes.cloud.local +short
# Debe responder: 192.168.10.30
```

### 2.4 Verificar Publicación del Servidor Apache
```bash
curl -I http://192.168.10.30/
curl http://192.168.10.30/
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
- `dig @192.168.10.10 paisajes.cloud.local +short` debe responder vacío (NXDOMAIN).

