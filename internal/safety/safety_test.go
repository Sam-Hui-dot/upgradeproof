package safety

import (
	"strings"
	"testing"
)

func compose(extraService, volumes string) []byte {
	return []byte("services:\n  app:\n    image: ${UPGRADEPROOF_IMAGE}\n" + extraService + "\nvolumes:\n" + volumes + "\n")
}

func TestSafeProjectScopedVolumeAndReadOnlyBind(t *testing.T) {
	data := compose("    volumes:\n      - data:/data\n      - ./config:/config:ro", "  data:\n")
	if err := CheckCompose(data, "app"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsUnsafeStorageAndContainerNames(t *testing.T) {
	tests := []struct {
		name, service, volumes, want string
	}{
		{"external", "    volumes: [data:/data]", "  data:\n    external: true", "external"},
		{"explicit name", "    volumes: [data:/data]", "  data:\n    name: production-data", "explicit name"},
		{"relative bind", "    volumes: [./data:/data]", "  data:", "writable bind"},
		{"absolute bind", "    volumes: [/srv/data:/data]", "  data:", "writable bind"},
		{"long bind", "    volumes:\n      - type: bind\n        source: ./data\n        target: /data", "  data:", "writable bind"},
		{"driver bind", "    volumes: [data:/data]", "  data:\n    driver: local\n    driver_opts:\n      type: none\n      o: bind\n      device: /srv/data", "custom volume driver"},
		{"remote driver", "    volumes: [data:/data]", "  data:\n    driver: nfs", "custom volume driver"},
		{"remote driver opts", "    volumes: [data:/data]", "  data:\n    driver_opts:\n      type: nfs\n      device: :/remote", "driver_opts"},
		{"container name", "    container_name: fixed\n    volumes: [data:/data]", "  data:", "fixed container_name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckCompose(compose(tc.service, tc.volumes), "app")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestResolvedComposeAuditAllowsUnnormalizedProjectVolume(t *testing.T) {
	data := []byte(`{"name":"upgradeproof-preflight","services":{"app":{"image":"app:v1","volumes":[{"type":"volume","source":"data","target":"/data","volume":{}}]}},"volumes":{"data":{}}}`)
	if err := CheckResolvedCompose(data, "app"); err != nil {
		t.Fatalf("normal project-scoped volume rejected: %v", err)
	}
}

func TestResolvedComposeAuditRejectsMergedUnsafeFields(t *testing.T) {
	tests := []struct {
		name, model, want string
	}{
		{"bind", `{"services":{"app":{"image":"app:v1","volumes":[{"type":"bind","source":"/host/data","target":"/data"}]}}}`, "writable bind"},
		{"container name", `{"services":{"app":{"image":"app:v1","container_name":"fixed"}}}`, "fixed container_name"},
		{"external volume", `{"services":{"app":{"image":"app:v1"}},"volumes":{"data":{"external":true,"name":"data"}}}`, "external"},
		{"explicit volume", `{"services":{"app":{"image":"app:v1"}},"volumes":{"data":{"name":"production-data"}}}`, "explicit name"},
		{"custom driver", `{"services":{"app":{"image":"app:v1"}},"volumes":{"data":{"driver":"custom"}}}`, "custom volume driver"},
		{"remote opts", `{"services":{"app":{"image":"app:v1"}},"volumes":{"data":{"driver_opts":{"type":"nfs","device":":/remote"}}}}`, "driver_opts"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckResolvedCompose([]byte(tc.model), "app")
			if err == nil || !IsViolation(err) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected resolved safety violation %q, got %v", tc.want, err)
			}
		})
	}
}

func TestCanonicalAuditAllowsOnlyComposeGeneratedVolumeName(t *testing.T) {
	safe := []byte(`{"name":"upgradeproof-preflight","services":{"app":{"image":"app:v1","volumes":[{"type":"volume","source":"data","target":"/data","volume":{}}]}},"volumes":{"data":{"name":"upgradeproof-preflight_data"}}}`)
	if err := CheckCanonicalCompose(safe, "app"); err != nil {
		t.Fatalf("Compose-generated volume name rejected: %v", err)
	}
	unsafe := []byte(`{"name":"upgradeproof-preflight","services":{"app":{"image":"app:v1"}},"volumes":{"data":{"name":"production-data"}}}`)
	if err := CheckCanonicalCompose(unsafe, "app"); err == nil || !IsViolation(err) {
		t.Fatalf("non-project-scoped canonical volume name accepted: %v", err)
	}
}

func TestRejectsMissingInterpolationContract(t *testing.T) {
	err := CheckCompose([]byte("services:\n  app:\n    image: app:latest\n"), "app")
	if err == nil || !strings.Contains(err.Error(), "interpolate") {
		t.Fatalf("expected interpolation error, got %v", err)
	}
}
