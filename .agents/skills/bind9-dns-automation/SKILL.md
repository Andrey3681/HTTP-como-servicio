---
name: bind9-dns-automation
description: Automatización de Bind9 para el registro y revocación dinámica de nombres de host en la zona DNS cloud.local.
---

# Habilidad: Automatización de DNS Dinámico con Bind9

## Propósito
Esta habilidad guía la administración automatizada y desatendida del servidor DNS Bind9 que opera en la máquina virtual dedicada `ns` (`192.168.10.10`), gestionando registros tipo A para las instancias aprovisionadas dentro de la zona `cloud.local.`.

## Arquitectura de la Zona DNS
- **Zona administrada**: `cloud.local.`
- **Servidor de nombres**: `ns.cloud.local.` (`192.168.10.10`)
- **Red de servicio**: `192.168.10.0/24`

## Mecanismos de Actualización Dinámica

### Opción A: Protocolo RFC 2136 (`nsupdate`)
Permite insertar y borrar registros directamente a través del protocolo DNS sin reiniciar el demonio `named`:
```text
server 192.168.10.10
zone cloud.local.
update add paisajes.cloud.local. 300 A 192.168.10.30
send
```
Para eliminación:
```text
server 192.168.10.10
zone cloud.local.
update delete paisajes.cloud.local. A
send
```

### Opción B: Automatización vía SSH / Script Remoto
Si no se utiliza TSIG, se puede invocar un script en la máquina virtual `ns`:
```bash
ssh -i /ruta/id_rsa root@192.168.10.10 "bash /opt/dns-scripts/add-record.sh paisajes 192.168.10.30"
```

## Verificación de Propagación
Antes de responder exitosamente al usuario en la creación de una instancia, se debe comprobar que la resolución DNS responde correctamente:
```bash
dig @192.168.10.10 paisajes.cloud.local +short
```
Si la respuesta coincide con la IP esperada (`192.168.10.30`), se confirma la disponibilidad del host.

