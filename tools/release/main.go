// 技术鸭发行工具：将五个平台程序放入同一个免安装包。
package main

import (
	"archive/zip"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var platforms = []struct{ system, arch string }{
	{"windows", "amd64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"},
}

// binaryPath 返回包内唯一的程序路径，供技能按系统和架构选择。
func binaryPath(system, arch string) string {
	name := "bin/jishuya-image-" + system + "-" + arch
	if system == "windows" {
		name += ".exe"
	}
	return name
}

// packageFiles 使用白名单，避免配置、日志和开发文件进入发行包。
func packageFiles() []string {
	files := []string{"SKILL.md", "README.md", "agents/openai.yaml", "THIRD-PARTY-NOTICES.txt"}
	for _, platform := range platforms {
		files = append(files, binaryPath(platform.system, platform.arch))
	}
	return files
}

// writeArchive 从源码读取文档、从临时构建目录读取程序，并保留 Unix 可执行权限。
func writeArchive(root, buildDir, path string) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	defer func() {
		err = errors.Join(err, writer.Close(), file.Close())
	}()
	for _, name := range packageFiles() {
		source := filepath.Join(root, name)
		isBinary := strings.HasPrefix(name, "bin/")
		if isBinary {
			source = filepath.Join(buildDir, name)
		}
		content, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		header := &zip.FileHeader{Name: "jishuya-image/" + name, Method: zip.Deflate}
		header.SetMode(0644)
		if isBinary {
			header.SetMode(0755)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := entry.Write(content); err != nil {
			return err
		}
	}
	return nil
}

// run 完成全部平台编译后才输出通用 ZIP 与校验清单。
func run() error {
	version := flag.String("version", "2.1.0", "发行版本")
	flag.Parse()
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("请在技术鸭源码目录运行：%w", err)
	}
	buildDir, err := os.MkdirTemp("", "jishuya-release-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(buildDir)
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBinary += ".exe"
	}
	for _, platform := range platforms {
		fmt.Printf("构建 %s/%s\n", platform.system, platform.arch)
		binary := filepath.Join(buildDir, binaryPath(platform.system, platform.arch))
		command := exec.Command(goBinary, "build", "-trimpath", "-ldflags", "-s -w -X main.version="+*version, "-o", binary, ".")
		command.Dir = root
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			switch strings.ToUpper(name) {
			case "GOOS", "GOARCH", "CGO_ENABLED":
				continue
			}
			command.Env = append(command.Env, entry)
		}
		command.Env = append(command.Env, "GOOS="+platform.system, "GOARCH="+platform.arch, "CGO_ENABLED=0")
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			return err
		}
	}
	output := filepath.Join(root, "dist")
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	path := filepath.Join(output, fmt.Sprintf("jishuya-image-v%s-universal.zip", *version))
	if err := writeArchive(root, buildDir, path); err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(content), filepath.Base(path))
	return os.WriteFile(filepath.Join(output, "SHA256SUMS.txt"), []byte(checksum), 0644)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
