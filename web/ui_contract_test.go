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
		`/js/router.js`,
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
		{"index.html", []string{`id="provisionForm"`, `id="hostnameInput"`, `id="zipInput"`, `id="checkHostBtn"`, `id="instancesList"`, `id="instancesCount"`, `id="refreshInstancesBtn"`, `id="delete-modal"`, `id="loadingOverlay"`, `id="statusMessage"`}},
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
	for _, bad := range []string{`journalctl`} {
		if strings.Contains(logs, bad) {
			t.Fatalf("js/logs-dock.js: no debe contener %q (dock limpio)", bad)
		}
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
