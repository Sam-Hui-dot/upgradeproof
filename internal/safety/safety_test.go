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
		{"driver bind", "    volumes: [data:/data]", "  data:\n    driver: local\n    driver_opts:\n      type: none\n      o: bind\n      device: /srv/data", "local-driver bind"},
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

func TestRejectsMissingInterpolationContract(t *testing.T) {
	err := CheckCompose([]byte("services:\n  app:\n    image: app:latest\n"), "app")
	if err == nil || !strings.Contains(err.Error(), "interpolate") {
		t.Fatalf("expected interpolation error, got %v", err)
	}
}
