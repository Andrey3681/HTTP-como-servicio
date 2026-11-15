package web_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests de contrato de la interfaz web (TDD). Validan los contratos definidos en
// docs/modulos-web.md §8: módulos, componentes reutilizables e IDs requeridos.

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", name, err)
	}
	return string(b)
}

func assertContains(t *testing.T, haystack, needle, ctx string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%s: falta %q", ctx, needle)
	}
}

func TestModulePagesExist(t *testing.T) {
	pages := []string{
		"index.html", "dns.html", "salud.html", "vhosts.html",
	}
	for _, p := range pages {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("debe existir la página de módulo %s: %v", p, err)
		}
	}
	for _, removed := range []string{"detalle.html", "ayuda.html"} {
		if _, err := os.Stat(removed); err == nil {
			t.Fatalf("%s no debe existir", removed)
		}
	}
}

func TestAllPagesUseSharedShell(t *testing.T) {
	pages := []string{
		"index.html", "dns.html", "salud.html", "vhosts.html",
	}
	required := []string{
		`lang="es"`,
		`id="site-header-mount"`,
		`id="site-footer-mount"`,
		`/js/common.js`,
		`/js/components.js`,
		`/js/logs-dock.js`,
		`/js/setup-wizard.js`,
		`/js/router.js`,
		`setup-checking`,
	}
	for _, p := range pages {
		content := readFile(t, p)
		for _, needle := range required {
			assertContains(t, content, needle, p+" (shell compartido)")
		}
	}
}

func TestNoTailwindRuntimeAndLocalUtilities(t *testing.T) {
	pages := []string{"index.html", "dns.html", "salud.html", "vhosts.html"}
	for _, p := range pages {
		content := readFile(t, p)
		if strings.Contains(content, "cdn.tailwindcss.com") {
			t.Fatalf("%s: no debe cargar el runtime CDN de Tailwind", p)
		}
		if strings.Contains(content, "tailwind.config") {
			t.Fatalf("%s: no debe incluir configuración inline de Tailwind", p)
		}
		assertContains(t, content, `/css/utilities.css`, p+" (utilidades locales)")
	}
	if _, err := os.Stat(filepath.Join("css", "utilities.css")); err != nil {
		t.Fatalf("debe existir css/utilities.css: %v", err)
	}
}

func TestIndexHelpIsTooltip(t *testing.T) {
	content := readFile(t, "index.html")
	for _, needle := range []string{`data-tooltip=`, `aria-describedby="provision-help"`, `id="provision-help"`, `info-tooltip-btn`} {
		assertContains(t, content, needle, "index.html (ayuda contextual)")
	}
	for _, oldBlock := range []string{`role="note"`, `Cómo funciona:</strong>`} {
		if strings.Contains(content, oldBlock) {
			t.Fatalf("index.html: la ayuda ya no debe ser un bloque permanente (%q)", oldBlock)
		}
	}
}

func TestInstanceActionsAndDnsRenameUI(t *testing.T) {
	panel := readFile(t, filepath.Join("js", "instances-panel.js"))
	for _, needle := range []string{`d-restart-btn`, `d-ssh-btn`, `d-redeploy-btn`, `d-delete-btn`, `d-redeploy-drop`, `zip-dropzone compact`, `data-detail`} {
		assertContains(t, panel, needle, "js/instances-panel.js (acciones del drawer)")
	}
	detalle := readFile(t, filepath.Join("js", "pages", "detalle.js"))
	for _, needle := range []string{`/servicios/reiniciar`, `/acceso`, `/despliegue`, `closeInstanceDrawer`, `acceptRedeployFile`} {
		assertContains(t, detalle, needle, "js/pages/detalle.js (acciones de instancia)")
	}

	vhosts := readFile(t, filepath.Join("js", "pages", "vhosts.js"))
	for _, needle := range []string{`data-power`, `/activar`, `/desactivar`, `vhost-switch`} {
		assertContains(t, vhosts, needle, "js/pages/vhosts.js (activar/desactivar)")
	}

	dns := readFile(t, "dns.html")
	for _, needle := range []string{`dns-rename-modal`, `dns-rename-input`, `dns-rename-confirm`} {
		assertContains(t, dns, needle, "dns.html (renombrar registro)")
	}
	dnsJS := readFile(t, filepath.Join("js", "pages", "dns.js"))
	for _, needle := range []string{`/api/dns/registros`, `PATCH`, `nuevo_nombre`, `data-rename`} {
		assertContains(t, dnsJS, needle, "js/pages/dns.js (renombrar registro)")
	}
}

func TestHeaderUsesWordmark(t *testing.T) {
	components := readFile(t, filepath.Join("js", "components.js"))
	for _, needle := range []string{`brand-wordmark`, `brand-dot`, `HTTPaaS`} {
		assertContains(t, components, needle, "js/components.js (logotipo)")
	}
	if strings.Contains(components, "header-brand-outfit") {
		t.Fatalf("js/components.js: no debe usar el estilo de marca anterior")
	}
	for _, p := range []string{"index.html", "dns.html", "salud.html", "vhosts.html"} {
		assertContains(t, readFile(t, p), `brand-wordmark`, p+" (logotipo del header)")
	}
}

func TestPageTitlesAreDescriptive(t *testing.T) {
	cases := map[string]string{
		"index.html":  "<title>Agregar nueva instancia · HTTPaaS</title>",
		"dns.html":    "<title>DNS de la zona local · HTTPaaS</title>",
		"salud.html":  "<title>Salud del entorno · HTTPaaS</title>",
		"vhosts.html": "<title>VHosts de Apache · HTTPaaS</title>",
	}
	for p, want := range cases {
		assertContains(t, readFile(t, p), want, p+" (title descriptivo)")
	}
}

func TestPagesHaveFaviconAndThemeColor(t *testing.T) {
	pages := []string{"index.html", "dns.html", "salud.html", "vhosts.html"}
	for _, p := range pages {
		content := readFile(t, p)
		assertContains(t, content, `rel="icon"`, p+" (favicon)")
		assertContains(t, content, `theme-color`, p+" (theme-color)")
	}
	for _, asset := range []string{"favicon.svg", "favicon.ico", "apple-touch-icon.png"} {
		if _, err := os.Stat(asset); err != nil {
			t.Fatalf("debe existir el recurso de icono %s: %v", asset, err)
		}
	}
}

func TestPagesDeclareDataPage(t *testing.T) {
	cases := map[string]string{
		"index.html":  `data-page="index"`,
		"dns.html":    `data-page="dns"`,
		"salud.html":  `data-page="salud"`,
		"vhosts.html": `data-page="vhosts"`,
	}
	for p, needle := range cases {
		assertContains(t, readFile(t, p), needle, p+" (data-page)")
	}
}

func TestModuleContracts(t *testing.T) {
	cases := []struct {
		page    string
		needles []string
	}{
		{"index.html", []string{`id="provisionForm"`, `id="hostnameInput"`, `id="zipInput"`, `id="zipDropZone"`, `id="checkHostBtn"`, `id="statusMessage"`, `id="instances-panel-mount"`}},
		{"dns.html", []string{`id="dnsTableBody"`, `id="refreshDnsBtn"`, `id="dnsZone"`}},
		{"salud.html", []string{`id="checkBtn"`, `id="healthRaw"`, `id="h-overall"`, `id="h-vbox"`, `id="h-bind9"`, `id="health-readiness"`, `id="health-raw-details"`}},
		{"vhosts.html", []string{`id="vhostsTableBody"`, `id="refreshVhostsBtn"`}},
	}
	for _, tc := range cases {
		content := readFile(t, tc.page)
		for _, needle := range tc.needles {
			assertContains(t, content, needle, tc.page+" (contrato de módulo)")
		}
	}
}

func TestReusableComponentAPIs(t *testing.T) {
	components := readFile(t, filepath.Join("js", "components.js"))
	for _, needle := range []string{`window.updateActiveNav`, `side-dock`, `vhosts.html`, `salud.html`, `footer-latest-log`, `/api/logs/stream`} {
		assertContains(t, components, needle, "js/components.js (API/navegación y estado en vivo)")
	}
	for _, bad := range []string{`detalle.html`, `>Detalle<`, `ayuda.html`, `>Ayuda<`, `site-nav`, `nav-toggle`} {
		if strings.Contains(components, bad) {
			t.Fatalf("js/components.js: no debe exponer módulo/página o navegación removida (%q)", bad)
		}
	}

	logs := readFile(t, filepath.Join("js", "logs-dock.js"))
	for _, needle := range []string{`window.toggleLogsDock`, `window.openLogsDock`, `window.closeLogsDock`, `logs-closed`, `logs-open`} {
		assertContains(t, logs, needle, "js/logs-dock.js (API/comportamiento)")
	}
	for _, needle := range []string{`/api/logs`, `/api/logs/stream`, `EventSource`} {
		assertContains(t, logs, needle, "js/logs-dock.js (conexión API de logs)")
	}
	for _, bad := range []string{`journalctl`} {
		if strings.Contains(logs, bad) {
			t.Fatalf("js/logs-dock.js: no debe contener %q (dock limpio)", bad)
		}
	}
}

func TestA11yAndOperationalTruthContracts(t *testing.T) {
	components := readFile(t, filepath.Join("js", "components.js"))
	for _, needle := range []string{
		`side-dock`, `aria-label="Módulos"`, `data-path="index.html"`, `data-path="dns.html"`, `data-path="vhosts.html"`, `data-path="salud.html"`,
	} {
		assertContains(t, components, needle, "js/components.js (dock de navegación)")
	}
	for _, bad := range []string{`Apache 2.4 + Bind9 Online`, `Salud: OK`} {
		if strings.Contains(components, bad) {
			t.Fatalf("js/components.js: no debe incluir texto/estado estático engañoso %q", bad)
		}
	}

	logs := readFile(t, filepath.Join("js", "logs-dock.js"))
	for _, needle := range []string{`aria-expanded`, `aria-controls`, `system-logs-dock`} {
		assertContains(t, logs, needle, "js/logs-dock.js (a11y toggle/dock)")
	}

	panel := readFile(t, filepath.Join("js", "instances-panel.js"))
	for _, needle := range []string{`id="delete-modal"`, `role="dialog"`, `aria-modal="true"`, `aria-labelledby`, `id="delete-modal-panel"`, `id="loadingOverlay"`, `role="status"`, `id="instancesList"`, `id="instancesCount"`, `id="refreshInstancesBtn"`, `id="instance-detail-drawer"`, `id="instance-detail-overlay"`, `id="instance-detail-close"`} {
		assertContains(t, panel, needle, "js/instances-panel.js (panel compartido/a11y)")
	}
	common := readFile(t, filepath.Join("js", "common.js"))
	for _, needle := range []string{`aria-hidden`, `aria-busy`, `loadingOverlay`} {
		assertContains(t, common, needle, "js/common.js (loading semántica accesible)")
	}

	page := readFile(t, filepath.Join("js", "pages", "index.js"))
	assertContains(t, page, `aria-busy`, "js/pages/index.js (estados pendientes)")
	for _, needle := range []string{`focus`, `Escape`} {
		assertContains(t, panel, needle, "js/instances-panel.js (gestión foco/modal)")
	}

	css := readFile(t, filepath.Join("css", "stitch.css"))
	if !strings.Contains(css, `side-dock`) {
		t.Fatalf("css/stitch.css: debe definir estilos para el dock de navegación (side-dock)")
	}
}

func TestPageModulesExist(t *testing.T) {
	mods := []string{"index.js", "dns.js", "salud.js", "detalle.js", "vhosts.js"}
	for _, m := range mods {
		if _, err := os.Stat(filepath.Join("js", "pages", m)); err != nil {
			t.Fatalf("debe existir módulo de página js/pages/%s: %v", m, err)
		}
	}
	if _, err := os.Stat(filepath.Join("js", "instances-panel.js")); err != nil {
		t.Fatalf("debe existir js/instances-panel.js: %v", err)
	}
	if _, err := os.Stat(filepath.Join("js", "pages", "ayuda.js")); err == nil {
		t.Fatalf("js/pages/ayuda.js no debe existir")
	}
	router := readFile(t, filepath.Join("js", "router.js"))
	for _, needle := range []string{`vhosts`, `salud`} {
		assertContains(t, router, needle, "js/router.js (enrutamiento)")
	}
	if strings.Contains(router, "ayuda") {
		t.Fatalf("js/router.js: no debe referenciar ayuda")
	}
}

func TestSetupWizardContract(t *testing.T) {
	js := readFile(t, filepath.Join("js", "setup-wizard.js"))
	for _, needle := range []string{
		"/api/setup/estado",
		"/api/setup/iniciar",
		"/api/setup/stream",
		"EventSource",
		"setup-wizard",
		"data-screen",
		"/api/setup/configuracion",
		"setup-user",
		"setup-pass",
		"setup-steps",
		"setup-log",
		"setup-copy-log",
		"setup-reset-btn",
		"/api/logs/stream",
		"data-setup-step",
		`data-screen`,
		`aria-live`,
		"setup-progress-track",
		"setup-start-btn",
		"openSetupWizard",
		"setup-gate",
		"prefers-reduced-motion",
	} {
		assertContains(t, js, needle, "js/setup-wizard.js")
	}

	components := readFile(t, filepath.Join("js", "components.js"))
	assertContains(t, components, "setup-open-btn", "js/components.js (acceso a la configuración inicial)")
}
