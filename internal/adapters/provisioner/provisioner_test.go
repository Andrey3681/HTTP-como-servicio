// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package provisioner_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/provisioner"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

type fakeExecutor struct {
	calls     []string
	responses map[string]string
	failOn    string
	onCall    func(key string)
}

func (f *fakeExecutor) Execute(_ context.Context, command string, args ...string) ([]byte, []byte, error) {
	key := strings.Join(append([]string{command}, args...), " ")
	f.calls = append(f.calls, key)
	if f.onCall != nil {
		f.onCall(key)
	}
	if f.failOn != "" && strings.Contains(key, f.failOn) {
		return nil, []byte("boom"), os.ErrPermission
	}
	if f.responses != nil {
		if response, ok := f.responses[key]; ok {
			return []byte(response), nil, nil
		}
	}
	return []byte("OK"), nil, nil
}

func (f *fakeExecutor) contains(fragment string) bool {
	for _, call := range f.calls {
		if strings.Contains(call, fragment) {
			return true
		}
	}
	return false
}

func setupConfig() domain.SetupConfig {
	return domain.SetupConfig{
		DNSVMName:     "ns",
		BaseVMName:    "ApacheServer",
		MemoryMB:      512,
		DiskMB:        8192,
		GuestUser:     "http",
		GuestPassword: "httpaas",
	}
}

func TestParseHelpers(t *testing.T) {
	if names := provisioner.ParseVMNames("\"ns\" {abc}\n\"ApacheServer\" {def}\n"); len(names) != 2 || names[0] != "ns" {
		t.Errorf("unexpected VM names: %v", names)
	}

	ifaces := provisioner.ParseHostOnlyInterfaces("Name: vboxnet0\nIPAddress: 192.168.56.1\nNetworkMask: 255.255.255.0\n\nName: vboxnet1\nIPAddress: 192.168.57.1\nNetworkMask: 255.255.255.0\n")
	if len(ifaces) != 2 || ifaces[1].Name != "vboxnet1" || ifaces[1].IP != "192.168.57.1" || ifaces[1].Netmask != "255.255.255.0" {
		t.Errorf("unexpected interfaces: %+v", ifaces)
	}

	if got := provisioner.ParseCreatedInterface("Interface 'vboxnet2' was successfully created"); got != "vboxnet2" {
		t.Errorf("unexpected interface name: %q", got)
	}

	output := "name=\"paisajes\"\nCfgFile=\"/home/u/VMs/paisajes/paisajes.vbox\"\nSATA-0-0=\"/media/disk.vdi\"\n"
	if got := provisioner.ParseMachineReadableValue(output, "SATA-0-0"); got != "/media/disk.vdi" {
		t.Errorf("unexpected machine value: %q", got)
	}

	quoted := "name=\"paisajes\"\n\"SATA-0-0\"=\"/home/u/VMs/paisajes/paisajes.vdi\"\n\"SATA-ImageUUID-0-0\"=\"8ceac129\"\n"
	if got := provisioner.ParseMachineReadableValue(quoted, "SATA-0-0"); got != "/home/u/VMs/paisajes/paisajes.vdi" {
		t.Errorf("unexpected quoted machine value: %q", got)
	}

	if got := provisioner.ParseParentUUID("Parent UUID:    base\n"); got != "" {
		t.Errorf("expected no parent, got %q", got)
	}
	if got := provisioner.ParseParentUUID("Parent UUID:    ca3829ae-de90\n"); got != "ca3829ae-de90" {
		t.Errorf("unexpected parent: %q", got)
	}
	hdds := "UUID: ca3829ae\nLocation: /media/base.vdi\n\nUUID: child-1\nLocation: /media/child.vdi\n"
	if got := provisioner.ParseMediumPath(hdds, "child-1"); got != "/media/child.vdi" {
		t.Errorf("unexpected medium path: %q", got)
	}

	if !provisioner.IsMultiattach("Type: multiattach\n") {
		t.Error("expected multiattach detection")
	}
	if provisioner.IsMultiattach("Type: normal (base)\n") {
		t.Error("expected non-multiattach detection")
	}
}

func TestEnsureNetworkDerivesFromExistingInterface(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage list hostonlyifs": "Name: vboxnet0\nIPAddress: 192.168.56.1\nNetworkMask: 255.255.255.0\n\nName: vboxnet1\nIPAddress: 192.168.57.1\nNetworkMask: 255.255.255.0\n",
		"VBoxManage list dhcpservers": "NetworkName: HostInterfaceNetworking-vboxnet1\n",
	}}
	prov := provisioner.NewProvisioner(exec)

	network, err := prov.EnsureNetwork(context.Background(), setupConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if network.Interface != "vboxnet1" {
		t.Fatalf("expected vboxnet1, got %q", network.Interface)
	}
	if network.HostIP != "192.168.57.1" || network.DNSIP != "192.168.57.10" {
		t.Errorf("unexpected derived network: %+v", network)
	}
	if network.CIDR != "192.168.57.0/24" {
		t.Errorf("unexpected CIDR: %q", network.CIDR)
	}
	if exec.contains("hostonlyif create") {
		t.Error("should reuse the existing interface")
	}
	if exec.contains("hostonlyif ipconfig") {
		t.Error("must not reconfigure host interfaces (requires root)")
	}
	if !exec.contains("dhcpserver remove --interface=vboxnet1") {
		t.Errorf("expected stale DHCP reservations to be cleared, got %v", exec.calls)
	}
	if !exec.contains("dhcpserver add --interface=vboxnet1 --server-ip=192.168.57.1") {
		t.Errorf("expected DHCP configuration with the derived network, got %v", exec.calls)
	}
	if !exec.contains("--lower-ip=192.168.57.10 --upper-ip=192.168.57.199") {
		t.Errorf("expected derived DHCP range, got %v", exec.calls)
	}
	if !exec.contains("--suppress-opt=3") || !exec.contains("--suppress-opt=6") {
		t.Errorf("expected gateway/DNS suppression on the host-only DHCP, got %v", exec.calls)
	}
}

func TestNetworkFailsWithoutProjectInterface(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage list hostonlyifs": "Name: vboxnet0\nIPAddress: 192.168.56.1\nNetworkMask: 255.255.255.0\n",
	}}
	prov := provisioner.NewProvisioner(exec)

	if _, err := prov.Network(context.Background()); err == nil {
		t.Fatal("expected an error when only the default interface exists")
	}
}

func TestCreateVMCommands(t *testing.T) {
	folder := t.TempDir()
	base := filepath.Join(t.TempDir(), "base.vdi")
	if err := os.WriteFile(base, []byte("vdi"), 0o600); err != nil {
		t.Fatal(err)
	}
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage list vms":                        "",
		"VBoxManage showvminfo ns --machinereadable": "CfgFile=\"" + filepath.Join(folder, "ns.vbox") + "\"\n",
	}}
	prov := provisioner.NewProvisioner(exec)

	disk, err := prov.CreateVM(context.Background(), setupConfig(), "ns", "vboxnet1", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(folder, "ns.vdi")
	if disk != want {
		t.Errorf("unexpected disk path: %q", disk)
	}
	if !exec.contains("clonemedium disk " + base + " " + want + " --format VDI") {
		t.Errorf("expected the base VDI to be cloned, got %v", exec.calls)
	}
	for _, fragment := range []string{
		"createvm --name ns --ostype Debian13_64 --register",
		"--nic1 hostonly --hostonlyadapter1 vboxnet1 --nic2 nat",
		"--boot1 disk --boot2 dvd",
		"storagectl ns --name SATA",
		"modifymedium disk " + want + " --resize 8192",
		"storageattach ns --storagectl SATA",
	} {
		if !exec.contains(fragment) {
			t.Errorf("expected command containing %q, got %v", fragment, exec.calls)
		}
	}
}

func TestCreateSeedISOAndBoot(t *testing.T) {
	if _, err := exec.LookPath("genisoimage"); err != nil {
		if _, err := exec.LookPath("mkisofs"); err != nil {
			t.Skip("no hay genisoimage/mkisofs disponibles")
		}
	}
	execFake := &fakeExecutor{}
	prov := provisioner.NewProvisioner(execFake)
	cfg := setupConfig()
	cfg.SeedDir = t.TempDir()

	seed, err := prov.CreateSeedISO(context.Background(), cfg, "ns", "#cloud-config\nhostname: ns-server\n", "instance-id: iid-ns-01\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(seed, "ns-seed.iso") {
		t.Errorf("unexpected seed path: %q", seed)
	}
	for _, name := range []string{"user-data", "meta-data"} {
		if _, err := os.Stat(filepath.Join(cfg.SeedDir, "ns", name)); err != nil {
			t.Errorf("expected %s inside the staging directory: %v", name, err)
		}
	}
	for _, fragment := range []string{"-volid cidata", filepath.Join(cfg.SeedDir, "ns", "user-data"), filepath.Join(cfg.SeedDir, "ns", "meta-data")} {
		if !execFake.contains(fragment) {
			t.Errorf("expected command containing %q, got %v", fragment, execFake.calls)
		}
	}

	if err := prov.StartSeedBoot(context.Background(), cfg, "ns", seed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, fragment := range []string{
		"storageattach ns --storagectl SATA --port 1 --device 0 --type dvddrive --medium " + seed,
		"startvm ns --type headless",
	} {
		if !execFake.contains(fragment) {
			t.Errorf("expected command containing %q, got %v", fragment, execFake.calls)
		}
	}
}

func TestMarkBaseMultiattach(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage showvminfo ApacheServer --machinereadable":   "CfgFile=\"/home/user/VMs/ApacheServer/ApacheServer.vbox\"\nVMState=\"poweroff\"\nSATA-0-0=\"/media/ApacheServer.vdi\"\n",
		"VBoxManage showmediuminfo disk /media/ApacheServer.vdi": "Parent UUID:    base\nType: normal (base)\n",
	}}
	exec.onCall = func(key string) {
		if strings.Contains(key, "modifymedium disk /media/ApacheServer.vdi --type multiattach") {
			exec.responses["VBoxManage showmediuminfo disk /media/ApacheServer.vdi"] = "Parent UUID:    base\nType: multiattach\n"
		}
	}
	prov := provisioner.NewProvisioner(exec)

	disk, err := prov.MarkBaseMultiattach(context.Background(), setupConfig(), "ApacheServer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if disk != "/media/ApacheServer.vdi" {
		t.Errorf("unexpected disk: %q", disk)
	}
	for _, fragment := range []string{
		"storageattach ApacheServer --storagectl SATA --port 0 --device 0 --type hdd --medium none",
		"modifymedium disk /media/ApacheServer.vdi --type multiattach",
		"storageattach ApacheServer --storagectl SATA --port 0 --device 0 --type hdd --medium /media/ApacheServer.vdi",
	} {
		if !exec.contains(fragment) {
			t.Errorf("expected command containing %q, got %v", fragment, exec.calls)
		}
	}
}

func TestISOStoreLocateFindsRawImage(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"debian-13-generic-amd64.raw", "otra.raw"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("raw"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store := provisioner.NewISOStore()
	path, ok := store.Locate(context.Background(), dir)
	if !ok {
		t.Fatal("expected the raw image to be found")
	}
	if filepath.Base(path) != "debian-13-generic-amd64.raw" {
		t.Errorf("unexpected image: %q", path)
	}
}

func TestEnsureNetworkDoesNotRequirePrivileges(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage list hostonlyifs": "Name: vboxnet1\nIPAddress: 192.168.57.1\nNetworkMask: 255.255.255.0\n",
		"VBoxManage list dhcpservers": "NetworkName: HostInterfaceNetworking-vboxnet1\n",
	}}
	prov := provisioner.NewProvisioner(exec)

	if _, err := prov.EnsureNetwork(context.Background(), setupConfig()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, call := range exec.calls {
		if strings.Contains(call, "ipconfig") {
			t.Fatalf("el flujo no debe reconfigurar interfaces del host: %s", call)
		}
	}
}

func TestInspectNormalizesVersion(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage --version": "7.2.22_RPMFUSIONr175245\n",
		"VBoxManage list vms":  "",
	}}
	prov := provisioner.NewProvisioner(exec)

	status, err := prov.Inspect(context.Background(), setupConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.VBoxVersion != "7.2.22" {
		t.Errorf("expected a normalized version, got %q", status.VBoxVersion)
	}
}

func TestStartSeedBootRejectsRunningVM(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage showvminfo ns --machinereadable": "VMState=\"running\"\n",
	}}
	prov := provisioner.NewProvisioner(exec)

	err := prov.StartSeedBoot(context.Background(), setupConfig(), "ns", "/seeds/ns-seed.iso")
	if err == nil {
		t.Fatal("expected an error for a running VM")
	}
	if !strings.Contains(err.Error(), "Reiniciar") {
		t.Errorf("expected an actionable hint, got: %v", err)
	}
}

func TestCreateVMRepairsVMWithoutDisk(t *testing.T) {
	folder := t.TempDir()
	base := filepath.Join(t.TempDir(), "base.vdi")
	if err := os.WriteFile(base, []byte("vdi"), 0o600); err != nil {
		t.Fatal(err)
	}
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage list vms":                        `"ns" {uuid}`,
		"VBoxManage showvminfo ns --machinereadable": "CfgFile=\"" + filepath.Join(folder, "ns.vbox") + "\"\nVMState=\"poweroff\"\n",
	}}
	prov := provisioner.NewProvisioner(exec)

	disk, err := prov.CreateVM(context.Background(), setupConfig(), "ns", "vboxnet1", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if disk != filepath.Join(folder, "ns.vdi") {
		t.Errorf("unexpected disk path: %q", disk)
	}
	if exec.contains("createvm") {
		t.Error("should repair the existing VM instead of creating another")
	}
	if !exec.contains("storageattach ns --storagectl SATA") {
		t.Errorf("expected the disk to be attached, got %v", exec.calls)
	}
	if !exec.contains("clonemedium disk " + base + " " + disk + " --format VDI") {
		t.Errorf("expected the base VDI to be cloned, got %v", exec.calls)
	}
}

func TestCreateVMReplacesStaleDisk(t *testing.T) {
	folder := t.TempDir()
	base := filepath.Join(t.TempDir(), "base.vdi")
	if err := os.WriteFile(base, []byte("vdi"), 0o600); err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(folder, "ns.vdi")
	if err := os.WriteFile(disk, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	exec := &fakeExecutor{responses: map[string]string{
		"VBoxManage list vms":                        `"ns" {uuid}`,
		"VBoxManage showvminfo ns --machinereadable": "CfgFile=\"" + filepath.Join(folder, "ns.vbox") + "\"\nVMState=\"poweroff\"\n",
	}}
	prov := provisioner.NewProvisioner(exec)

	if _, err := prov.CreateVM(context.Background(), setupConfig(), "ns", "vboxnet1", base); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exec.contains("closemedium disk " + disk) {
		t.Errorf("expected the stale disk to be unregistered, got %v", exec.calls)
	}
	if !exec.contains("clonemedium disk " + base + " " + disk) {
		t.Errorf("expected a fresh clone, got %v", exec.calls)
	}
}
