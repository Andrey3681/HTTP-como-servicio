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
		"index.html", "dns.html", "salud.html", "detalle.html", "vhosts.html", "ayuda.html",
	}
	for _, p := range pages {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("debe existir la página de módulo %s: %v", p, err)
		}
	}
}

func TestAllPagesUseSharedShell(t *testing.T) {
	pages := []string{
		"index.html", "dns.html", "salud.html", "detalle.html", "vhosts.html", "ayuda.html",
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

func TestPagesDeclareDataPage(t *testing.T) {
	cases := map[string]string{
		"index.html":   `data-page="index"`,
		"dns.html":     `data-page="dns"`,
		"salud.html":   `data-page="salud"`,
		"detalle.html": `data-page="detalle"`,
		"vhosts.html":  `data-page="vhosts"`,
		"ayuda.html":   `data-page="ayuda"`,
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
		{"index.html", []string{`id="provisionForm"`, `id="hostnameInput"`, `id="zipInput"`, `id="zipDropZone"`, `id="checkHostBtn"`, `id="instancesList"`, `id="instancesCount"`, `id="refreshInstancesBtn"`, `id="delete-modal"`, `id="loadingOverlay"`, `id="statusMessage"`}},
		{"detalle.html", []string{`id="instanceSelect"`, `id="loadBtn"`}},
		{"dns.html", []string{`id="dnsTableBody"`, `id="refreshDnsBtn"`, `id="dnsZone"`}},
		{"salud.html", []string{`id="checkBtn"`, `id="healthRaw"`, `id="h-overall"`, `id="h-vbox"`, `id="h-bind9"`}},
		{"vhosts.html", []string{`id="vhostsTableBody"`, `id="refreshVhostsBtn"`}},
		{"ayuda.html", []string{`cloud.local`, `.zip`, `hostname`, `/index.html`}},
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
	for _, needle := range []string{`window.updateActiveNav`, `vhosts.html`, `ayuda.html`} {
		assertContains(t, components, needle, "js/components.js (API/navegación)")
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
		`nav-toggle`, `aria-expanded`, `site-nav`, `aria-controls`,
		`header-health-pill`, `data-health-state`, `/api/salud`,
	} {
		assertContains(t, components, needle, "js/components.js (nav accesible + verdad operativa)")
	}
	for _, bad := range []string{`Apache 2.4 + Bind9 Online`} {
		if strings.Contains(components, bad) {
			t.Fatalf("js/components.js: no debe afirmar estado estático engañoso %q", bad)
		}
	}

	logs := readFile(t, filepath.Join("js", "logs-dock.js"))
	for _, needle := range []string{`aria-expanded`, `aria-controls`, `system-logs-dock`} {
		assertContains(t, logs, needle, "js/logs-dock.js (a11y toggle/dock)")
	}

	index := readFile(t, "index.html")
	for _, needle := range []string{`role="dialog"`, `aria-modal="true"`, `aria-labelledby`, `role="status"`, `id="delete-modal-panel"`} {
		assertContains(t, index, needle, "index.html (modal/loading a11y)")
	}
	common := readFile(t, filepath.Join("js", "common.js"))
	for _, needle := range []string{`aria-hidden`, `aria-busy`, `loadingOverlay`} {
		assertContains(t, common, needle, "js/common.js (loading semántica accesible)")
	}

	page := readFile(t, filepath.Join("js", "pages", "index.js"))
	for _, needle := range []string{`focus`, `aria-busy`, `Escape`} {
		assertContains(t, page, needle, "js/pages/index.js (gestión foco/estados pendientes)")
	}

	css := readFile(t, filepath.Join("css", "stitch.css"))
	if !strings.Contains(css, `nav-toggle`) && !strings.Contains(css, `site-nav`) {
		t.Fatalf("css/stitch.css: debe definir estilos para navegación accesible <1024px (nav-toggle/site-nav)")
	}
}

func TestPageModulesExist(t *testing.T) {
	mods := []string{"index.js", "dns.js", "salud.js", "detalle.js", "vhosts.js", "ayuda.js"}
	for _, m := range mods {
		if _, err := os.Stat(filepath.Join("js", "pages", m)); err != nil {
			t.Fatalf("debe existir módulo de página js/pages/%s: %v", m, err)
		}
	}
	router := readFile(t, filepath.Join("js", "router.js"))
	for _, needle := range []string{`vhosts`, `ayuda`} {
		assertContains(t, router, needle, "js/router.js (enrutamiento)")
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
