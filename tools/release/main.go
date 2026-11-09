// 技术鸭发行工具：按平台构建无外部语言运行环境依赖的程序并打包。
package main

import (
	"archive/zip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// buildPackage 仅打包运行必需的文件；可执行权限写入 ZIP 元数据。
func buildPackage(root, output, goBinary, version, system, arch string) (string, error) {
	buildDir, err := os.MkdirTemp("", "jishuya-release-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(buildDir)
	name := "jishuya-image"
	if system == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(buildDir, name)
	command := exec.Command(goBinary, "build", "-trimpath", "-ldflags", "-s -w -X main.version="+version, "-o", binary, ".")
	command.Dir = root
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOOS=") && !strings.HasPrefix(entry, "GOARCH=") && !strings.HasPrefix(entry, "CGO_ENABLED=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "GOOS="+system, "GOARCH="+arch, "CGO_ENABLED=0")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return "", err
	}
	path := filepath.Join(output, fmt.Sprintf("jishuya-image-v%s-%s-%s.zip", version, system, arch))
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	writer := zip.NewWriter(file)
	for _, name := range []string{"SKILL.md", "README.md", "agents/openai.yaml", "THIRD-PARTY-NOTICES.txt", "bin/" + name} {
		source := filepath.Join(root, name)
		if strings.HasPrefix(name, "bin/") {
			source = binary
		}
		content, err := os.ReadFile(source)
		if err != nil {
			writer.Close()
			file.Close()
			return "", err
		}
		header := &zip.FileHeader{Name: "jishuya-image/" + name, Method: zip.Deflate}
		header.SetMode(0644)
		if strings.HasPrefix(name, "bin/") {
			header.SetMode(0755)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			writer.Close()
			file.Close()
			return "", err
		}
		if _, err := entry.Write(content); err != nil {
			writer.Close()
			file.Close()
			return "", err
		}
	}
	if err := writer.Close(); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func run() error {
	version := flag.String("version", "2.0.1", "发行版本")
	flag.Parse()
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("请在技术鸭源码目录运行：%w", err)
	}
	output := filepath.Join(root, "dist")
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBinary += ".exe"
	}
	var checksums strings.Builder
	for _, platform := range []struct{ system, arch string }{{"windows", "amd64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}} {
		fmt.Printf("构建 %s/%s\n", platform.system, platform.arch)
		path, err := buildPackage(root, output, goBinary, *version, platform.system, platform.arch)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		file.Close()
		if copyErr != nil {
			return copyErr
		}
		fmt.Fprintf(&checksums, "%x  %s\n", hash.Sum(nil), filepath.Base(path))
	}
	return os.WriteFile(filepath.Join(output, "SHA256SUMS.txt"), []byte(checksums.String()), 0644)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
