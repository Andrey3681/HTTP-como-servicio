# Aprovisionamiento de Instancias

Este documento describe cómo HTTPaaS crea una instancia Apache a partir de la plantilla en modo `multiattach` y publica su contenido web.

> **Flujo del proyecto:** docs → tests → implementación.

---

## 1. Flujo

1. **Validación:** nombre de host, inventario y seguridad del `.zip` (tamaño y *Zip Slip*).
2. **Máquina virtual:** `VBoxManage createvm` + un **disco diferencial** (`createmedium --diffparent`) cuyo padre es el disco base multiconexión. El disco base nunca se monta en escritura. Se conecta además una **semilla cloud-init propia** de la instancia: fija su nombre de host, autoriza la clave del laboratorio y sobrescribe el netplan estático heredado de la plantilla con DHCP, para que cada instancia obtenga una IP dinámica distinta.
3. **Arranque:** `startvm --type headless` con un único adaptador host-only.
4. **Dirección IP:** se consulta la **concesión DHCP por MAC** (`dhcpserver findlease --interface=<if> --mac-address=<mac>`); si el DHCP fue recreado y no reporta la concesión, se hace un barrido ARP del rango `.100–.199` y se busca la MAC en la tabla de vecinos del anfitrión. Se reintenta durante 120 s. La imagen cloud no incluye el servicio de usuario de Guest Additions, así que `guestproperty` solo se usa como último recurso.
5. **DNS:** se registra un `A` en `cloud.local.` mediante `nsupdate` contra la VM `ns`, enviando el guion RFC 2136 en un archivo temporal y fallando si `nsupdate` devuelve error.
6. **Despliegue:** antes de copiar se espera a que `sshd` acepte conexiones (hasta 90 s). El `.zip` se copia por `scp` con la clave del laboratorio y se descomprime como root en `/var/www/vhosts/<host>/public`; si el paquete trae una única carpeta raíz, su contenido sube un nivel. Al final se normalizan los permisos (`chmod -R a+rX`) para que Apache pueda leer el contenido.
7. **Virtual host:** se escribe `/etc/apache2/sites-available/<host>.conf` con `ServerName <host>.cloud.local`, se habilita con `a2ensite` y se recarga Apache.
8. **Persistencia:** la instancia queda en `instances.json` con su IP y estado.

Acceso esperado:

```bash
dig @192.168.57.10 paisajes.cloud.local +short
curl --resolve paisajes.cloud.local:80:192.168.57.103 http://paisajes.cloud.local/
```

---

## 2. Acceso SSH

La instancia arranca desde el disco diferencial de la plantilla, por lo que hereda el usuario creado por la semilla y la clave pública del laboratorio (`<app-dir>/ssh/id_ed25519.pub`). El despliegue usa esa misma clave y `sudo` sin contraseña.

```bash
ssh -i ~/.local/share/httpaas/ssh/id_ed25519 jorge@192.168.57.103
```

---

## 3. Compensación

Si falla el registro DNS, el despliegue o la persistencia, el orquestador revierte lo hecho: elimina el registro DNS (si se creó), apaga la VM y borra el disco diferencial. El directorio `/var/www/vhosts/<host>` se elimina junto con el disco, porque reside en la instancia.

---

## 4. Verificación

```bash
# IP asignada por DHCP a la instancia
VBoxManage dhcpserver findlease --interface=vboxnet1 --mac-address=<mac>

# Registro DNS
dig @192.168.57.10 <host>.cloud.local +short

# Virtual host y contenido dentro de la instancia
ssh -i ~/.local/share/httpaas/ssh/id_ed25519 <usuario>@<ip> \
  "ls /var/www/vhosts/<host>/public; apache2ctl -S | grep <host>"
```

---

## 5. DNS del anfitrión

El servidor DNS del laboratorio vive en `192.168.57.10`. Para abrir `http://<host>.cloud.local` en el navegador del anfitrión, el resolver del sistema debe consultarlo; sin privilegios no se puede cambiar, así que se configura una vez:

```bash
sudo ./scripts/configurar-dns-anfitrion.sh
```

El script lee la red derivada desde `/api/setup/estado` y ejecuta `resolvectl dns <interfaz> <ip_dns>` y `resolvectl domain <interfaz> '~cloud.local'`. Alternativa puntual: añadir `192.168.57.105 <host>.cloud.local` a `/etc/hosts`.

La consola consulta `GET /api/dns/anfitrion` y muestra un aviso cuando el anfitrión todavía no resuelve la zona; el registro en Bind9 sí existe y se puede comprobar con `dig @192.168.57.10 <host>.cloud.local`.

---

## 6. Evidencia

La prueba de extremo a extremo del 14 de noviembre de 2026 creó `paisajes` en 18 s:

```text
dig @192.168.57.10 paisajes.cloud.local +short   -> 192.168.57.105
curl --resolve paisajes.cloud.local:80:192.168.57.105      http://paisajes.cloud.local/                -> <h1>Paisajes HTTPaaS</h1>
```

Tras `DELETE /api/instancias/paisajes` desaparecieron el registro DNS, la máquina virtual y su entrada en el inventario.

---

## 7. Limitaciones conocidas

- Las instancias heredan las claves de host SSH y los paquetes de la plantilla; el nombre y la red se fijan en su propia semilla, y el FQDN se resuelve por DNS y el `ServerName` del virtual host.
- No se aplican cuotas por instancia; el disco diferencial crece hasta el tamaño del disco plantilla.
