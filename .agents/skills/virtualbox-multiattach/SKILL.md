---
name: virtualbox-multiattach
description: Automatización de Oracle VM VirtualBox 7.x para aprovisionamiento mediante discos en modo multiconexión.
---

# Habilidad: VirtualBox y Discos Multiconexión

## Propósito
Esta habilidad orienta la interacción y automatización con la interfaz de línea de comandos `VBoxManage` de Oracle VM VirtualBox, enfocada en la creación rápida y no interactiva de máquinas virtuales a partir de discos en modo `multiattach`.

## Procedimientos Técnicos Clave

### 1. Configuración del Disco Base Multiconexión
El disco base de la VM plantilla (`ApacheServer.vdi`) debe marcarse como multiconexión para permitir que múltiples instancias generen discos diferenciales automáticos:
```bash
VBoxManage modifymedium disk /ruta/ApacheServer.vdi --type multiattach
```

### 2. Aprovisionamiento Desatendido de Nuevas Instancias
Para crear una nueva instancia web (ej. `paisajes`):
```bash
# 1. Crear y registrar la máquina virtual
VBoxManage createvm --name "paisajes" --ostype "Debian_64" --register

# 2. Configurar hardware y red interna
VBoxManage modifyvm "paisajes" --memory 1024 --cpus 1 --nic1 intnet --intnet1 "cloudnet"

# 3. Adjuntar controlador de almacenamiento y disco base
VBoxManage storagectl "paisajes" --name "SATA" --add sata --controller IntelAHCI
VBoxManage storageattach "paisajes" --storagectl "SATA" --port 0 --device 0 --type hdd --medium /ruta/ApacheServer.vdi

# 4. Iniciar máquina virtual en segundo plano
VBoxManage startvm "paisajes" --type headless
```

### 3. Concurrencia y Sincronización Obligatoria
`VBoxSVC` (el servicio de fondo de VirtualBox) no soporta invocaciones concurrentes que modifiquen el registro de máquinas. 
- En el adaptador de Go `internal/adapters/vbox`, se debe proteger cada llamada a `VBoxManage` con un cerrojo `sync.Mutex` a nivel de adaptador.

### 4. Detección de Dirección IP
Las instancias obtienen IP mediante DHCP o configuración de red. Para consultar la IP asignada:
```bash
VBoxManage guestproperty get "paisajes" "/VirtualBox/GuestInfo/Net/0/V4/IP"
```
Se debe implementar un bucle de reintentos con intervalo de 2 segundos (máximo 60 segundos) hasta que la propiedad retorne una dirección IPv4 válida.

### 5. Destrucción Limpia de Recursos
Para eliminar una instancia liberando memoria y almacenamiento diferencial:
```bash
# Apagar máquina virtual
VBoxManage controlvm "paisajes" poweroff

# Desregistrar y eliminar archivos asociados
VBoxManage unregistervm "paisajes" --delete
```

