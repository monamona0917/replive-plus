package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfigOmitsEmptyLinuxMediaPath(t *testing.T) {
	data, err := yaml.Marshal(DefaultConfig())
	if err != nil {
		t.Fatalf("yaml.Marshal(DefaultConfig()) error = %v", err)
	}
	if strings.Contains(string(data), "media_path_linux:") {
		t.Fatalf("default config unexpectedly includes media_path_linux: %s", data)
	}
}
