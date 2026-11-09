# **Nota parcial #3 – HTTP como servicio**

### **Descripción general**

Este proyecto propone la implementación de un servicio administrado de Servidores HTTP (HTTPaaS) en un entorno local virtualizado, integrando las tecnologías Oracle VirtualBox, Apache Web Server y Bind9. El propósito es simular el funcionamiento de una plataforma de servicios en la nube, permitiendo que los usuarios soliciten, de forma automatizada, la creación y despliegue de instancias del servidor Apache para la publicación temporal de páginas y aplicaciones web dentro de un dominio local gestionado por un servidor DNS.

El entorno estará compuesto por tres elementos principales: (1) una plantilla base de servidor HTTP configurada en una máquina virtual Debian 13.0 con Apache HTTP Server, cuyo disco se utilizará en modo multiconexión para generar nuevas instancias a partir de la misma imagen; (2) un servidor DNS Bind9, encargado de administrar el dominio local y registrar automáticamente los nuevos hosts correspondientes a las instancias del servicio HTTP; y (3) una aplicación web de gestión, desarrollada en Golang, que funcionará como punto de acceso para los usuarios y como componente orquestador de la infraestructura.

A través de la aplicación web de gestión, los usuarios podrán enviar solicitudes de aprovisionamiento, cargar un archivo .zip con los contenidos de su página o aplicación web, y visualizar en un tablero de control (dashboard) el listado de las instancias activas con sus enlaces para acceso directo. La aplicación gestionará la creación de máquinas virtuales, la configuración de red, la actualización de registros DNS y el despliegue de contenido en el servidor Apache, utilizando comandos remotos y técnicas básicas de automatización.

El proyecto busca que los estudiantes consoliden sus habilidades en el aprovisionamiento automatizado, la administración de servicios de red y el despliegue de infraestructura virtualizada para el diseño de soluciones de infraestructura como servicio (IaaS) y plataforma como servicio (PaaS) en entornos de computación en la nube.

#### **Objetivo**

Diseñar e implementar un entorno local automatizado para el aprovisionamiento y despliegue de servicios HTTP con Apache Web Server, integrados en un dominio DNS gestionado con Bind9, empleando infraestructura virtualizada y técnicas básicas de automatización que simulen el comportamiento de una plataforma real de servicios en la nube.

## **Objetivos específicos**

- Comprender el funcionamiento de un servidor DNS y desplegar el servidor Bind9 para gestionar los nombres de host de la zona local, de acuerdo con los requisitos del proyecto, incluyendo el registro automático de los nuevos hosts en la zona DNS local.
- Desarrollar los mecanismos de aprovisionamiento y gestión del servicio, automatizando la creación, configuración y eliminación de instancias del servidor HTTP.
- Automatizar el despliegue de las páginas o aplicaciones web solicitadas por el usuario.

#### **Requisitos del Proyecto**

### *1. Infraestructura virtualizada*

Diseñar la arquitectura de la solución, identificando los componentes y la interacción entre ellos para poner en funcionamiento un servicio administrado de publicación temporal de páginas y aplicaciones web utilizando infraestructura virtualizada con Oracle VirtualBox, Apache Web Server y Bind9, incluyendo la red local, el dominio local y su correspondiente zona DNS.

#### *2. Servidor DNS (Bind9)*

- Desplegar y configurar Bind9 en una máquina virtual dedicada.
- Crear una zona DNS local (por ejemplo, cloud.local.).
- Automatizar el registro dinámico de los nuevos hosts (las máquinas virtuales que corren las instancias del Servidor Apache) en esta zona.
- Asegurar la resolución correcta de los nombres de host en el dominio hacia las direcciones IP de las máquinas virtuales donde corren las instancias del servidor Apache.

#### *3. Servidor HTTP (Apache)*

Utilizar una máquina virtual con la distribución GNU/Linux Debian 13 (modo CLI) en la que se debe:

- Instalar el Servidor HTTP Apache y realizar las configuraciones necesarias de acuerdo con los objetivos del proyecto, incluyendo el acceso remoto y la configuración de la red. Es decir, cada nueva instancia de Apache debe tener un hostname único dentro del dominio, su propio directorio de contenido web y su página web o aplicación desplegada a partir de un archivo .zip enviado por el usuario.
- Configurar el disco virtual en modo multiconexión, de manera que esta máquina funcione como plantilla base para la creación de nuevas instancias del servidor HTTP.

Cada vez que se solicite el despliegue de una nueva página o aplicación web, el disco multiconexión será utilizado en la creación de una nueva máquina virtual, cuya configuración deberá ser ajustada para su correcta utilización.

## *4. Aplicación web de gestión (Golang)*

Desarrollar una webapp en Go que permita:

- Recibir solicitudes de los usuarios para aprovisionar un nuevo servicio HTTP.
- Ejecutar los comandos remotos de creación y configuración de las máquinas virtuales (a partir del disco multiconexión).
- Actualizar automáticamente el servidor DNS para incluir el nuevo host.
- Desplegar el contenido web suministrado por el usuario en la nueva instancia Apache.
- Mostrar en un dashboard todas las instancias aprovisionadas con el nombre de host, la dirección IP asignada y el enlace correspondiente para acceder a la página desplegada, preferiblemente en una nueva pestaña del navegador.

#### *5. Automatización*

Diseñe y desarrolle los procedimientos automatizados crear y configurar las máquinas virtuales y aplicaciones de acuerdo con el enunciado del proyecto, evitando la interacción del usuario. Puede utilizar scripts o aplicaciones compiladas desarrolladas en algún lenguaje de programación.

#### **Entregables**

Los estudiantes deberán entregar:

- (1) Código fuente de la aplicación web.
- (2) Un documento detallado que incluya:
  - capturas de pantalla paso a paso describiendo el trabajo realizado y evidencia del despliegue del servicio solicitado
  - una sección con los problemas encontrados, propuestas de mejoras y oportunidades de trabajo futuro
  - una sección con los conocimientos relevantes aprendidos en este trabajo.

**Fecha de entrega:** Noviembre 11 de 2026 11:59 pm.

**Conformación de grupos:** Mínimo 2, máximo 3 estudiantes.

**Los equipos de trabajo serán registrados como respuesta a un mensaje en Classroom. Equipos de un (1) estudiante no serán autorizados. La conformación de los equipos de trabajo no es responsabilidad del profesor.**