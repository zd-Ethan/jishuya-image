package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUniversalArchive(t *testing.T) {
	root, buildDir := t.TempDir(), t.TempDir()
	wanted := map[string]os.FileMode{
		"SKILL.md": 0644, "README.md": 0644, "agents/openai.yaml": 0644, "THIRD-PARTY-NOTICES.txt": 0644,
		"bin/jishuya-image-windows-amd64.exe": 0755,
		"bin/jishuya-image-darwin-amd64":      0755, "bin/jishuya-image-darwin-arm64": 0755,
		"bin/jishuya-image-linux-amd64": 0755, "bin/jishuya-image-linux-arm64": 0755,
	}
	for name := range wanted {
		directory := root
		if strings.HasPrefix(name, "bin/") {
			directory = buildDir
		}
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"config.json", "tests/private.log"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("private-fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "universal.zip")
	if err := writeArchive(root, buildDir, path); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != len(wanted) {
		t.Fatal("文件清单数量错误", len(reader.File))
	}
	for _, entry := range reader.File {
		name := strings.TrimPrefix(entry.Name, "jishuya-image/")
		mode, exists := wanted[name]
		if !exists || name == entry.Name || entry.Mode().Perm() != mode {
			t.Fatalf("包内路径或权限错误：%s %v", entry.Name, entry.Mode())
		}
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(file)
		closeErr := file.Close()
		if err != nil || closeErr != nil || string(content) != name {
			t.Fatal("包内文件内容错误", entry.Name, err, closeErr)
		}
		delete(wanted, name)
	}
	if len(wanted) != 0 {
		t.Fatal("缺少文件", wanted)
	}
}
