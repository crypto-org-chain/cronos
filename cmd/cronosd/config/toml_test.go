package config

import (
	"io"
	"testing"
	"text/template"
)

// TestDefaultCronosConfigTemplate_Renders guards against struct field renames
// on CronosConfig/RocksDBConfig drifting out of sync with the toml templates.
func TestDefaultCronosConfigTemplate_Renders(t *testing.T) {
	data := struct {
		Cronos  CronosConfig
		RocksDB RocksDBConfig
	}{
		Cronos:  DefaultCronosConfig(),
		RocksDB: DefaultRocksDBConfig(),
	}

	for name, tmpl := range map[string]string{
		"cronos":  DefaultCronosConfigTemplate,
		"rocksdb": DefaultRocksDBConfigTemplate,
	} {
		if _, err := template.New(name).Parse(tmpl); err != nil {
			t.Fatalf("parse %s template: %v", name, err)
		}
	}

	tpl, err := template.New("test").Parse(DefaultCronosConfigTemplate + DefaultRocksDBConfigTemplate)
	if err != nil {
		t.Fatalf("parse combined template: %v", err)
	}
	if err := tpl.Execute(io.Discard, data); err != nil {
		t.Fatalf("execute template against DefaultCronosConfig()/DefaultRocksDBConfig(): %v", err)
	}
}
