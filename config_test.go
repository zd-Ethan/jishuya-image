package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// isolatedHome 仅修改测试进程环境，防止测试触碰真实密钥或个人规则。
func isolatedHome(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	t.Setenv("USERPROFILE", directory)
	t.Setenv("HOME", directory)
	t.Setenv("APPDATA", filepath.Join(directory, "配置目录"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(directory, "配置目录"))
	t.Setenv("CODEX_HOME", filepath.Join(directory, ".codex"))
	t.Setenv(keyEnv, "")
	return directory
}

func TestConfigPersistenceAndEnvironmentPriority(t *testing.T) {
	isolatedHome(t)
	if err := writeConfig(config{APIKey: "local-fixture", Model: "my-model"}); err != nil {
		t.Fatal(err)
	}
	settings, source, err := loadConfig()
	if err != nil || settings.APIKey != "local-fixture" || settings.Model != "my-model" || source != "config" {
		t.Fatal(settings.Model, source, err)
	}
	if err := writeConfig(config{APIKey: "replacement-fixture", Model: "new-model"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(keyEnv, "environment-fixture")
	settings, source, err = loadConfig()
	if err != nil || settings.APIKey != "environment-fixture" || settings.Model != "new-model" || source != "environment" {
		t.Fatal(source, err)
	}
	path, _ := configPath()
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("配置文件权限错误", err)
		}
	}
}

func TestMissingKeyAndNonInteractiveInit(t *testing.T) {
	isolatedHome(t)
	result, err := runCLI(context.Background(), []string{"doctor", "--offline"}, os.Stdin, io.Discard)
	if err == nil || result.(map[string]any)["key_configured"] != false {
		t.Fatal("缺失 Key 检查失败")
	}
	file, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := initialize(context.Background(), false, "", file, io.Discard); err == nil {
		t.Fatal("后台 init 应提示打开终端")
	}
	if _, err := initialize(context.Background(), true, "", file, io.Discard); err == nil {
		t.Fatal("环境变量缺失时初始化应失败")
	}
}

func TestCLIHelpAndJSON(t *testing.T) {
	isolatedHome(t)
	for _, args := range [][]string{{"version"}, {"--help"}, {"generate", "--help"}} {
		_, err := runCLI(context.Background(), args, os.Stdin, io.Discard)
		if err != nil && !errors.Is(err, flag.ErrHelp) {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	printResult(&output, imageResult{Paths: []string{"C:/测试/图.png"}}, nil, "")
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "图.png") {
		t.Fatal("输出路径编码错误")
	}
}

func TestImageHelpWithBrokenConfig(t *testing.T) {
	isolatedHome(t)
	path, _ := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("invalid-json"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"generate", "edit"} {
		var output bytes.Buffer
		_, err := runCLI(context.Background(), []string{command, "--help"}, os.Stdin, &output)
		if !errors.Is(err, flag.ErrHelp) || !strings.Contains(output.String(), "-prompt") {
			t.Fatalf("%s 帮助被配置错误阻止：%v", command, err)
		}
	}
}

func TestEnvironmentWithoutUserDirectory(t *testing.T) {
	isolatedHome(t)
	t.Setenv("APPDATA", "")
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv(keyEnv, "environment-fixture")
	settings, source, err := loadConfig()
	if err != nil || source != "environment" || settings.APIKey != "environment-fixture" || settings.Model != defaultModel {
		t.Fatal("缺少用户目录时环境变量模式不可用", source, err)
	}
	status, err := diagnose(context.Background(), true)
	if err != nil || status["config_path"] != "" || status["key_source"] != "environment" {
		t.Fatal("免配置目录诊断失败", err)
	}
	t.Setenv(keyEnv, "")
	if _, _, err := loadConfig(); err == nil {
		t.Fatal("无配置目录且无环境变量时应报错")
	}
}

func TestEnvironmentWorksWithUnavailableConfig(t *testing.T) {
	isolatedHome(t)
	path, _ := configPath()
	// 目录占用配置文件位置可稳定模拟无法读取文件，不依赖管理员身份。
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadConfig(); err == nil {
		t.Fatal("配置不可读应报错")
	}
	t.Setenv(keyEnv, "environment-fixture")
	settings, source, err := loadConfig()
	if err != nil || settings.APIKey != "environment-fixture" || source != "environment" {
		t.Fatal("环境变量没有优先于不可读配置", source, err)
	}
}

type modelTransport func(*http.Request) (*http.Response, error)

// RoundTrip 以固定模型列表替代网络，验证初始化不会提交生图请求。
func (transport modelTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestInitOnlySavesConfiguration(t *testing.T) {
	home := isolatedHome(t)
	t.Setenv(keyEnv, "environment-fixture")
	rules := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(rules), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rules, []byte("用户自己的规则\n"), 0644); err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = modelTransport(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != "GET" || request.URL.String() != apiBase+"/models" {
			t.Fatalf("初始化发送了模型查询之外的请求：%s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"portable-model"}]}`))}, nil
	})
	var output bytes.Buffer
	result, err := runCLI(context.Background(), []string{"init", "--from-env", "--model", "portable-model"}, os.Stdin, &output)
	if err != nil || calls != 1 {
		t.Fatal("初始化失败或多次请求", calls, err)
	}
	printResult(&output, result, nil, "environment-fixture")
	if strings.Contains(output.String(), "environment-fixture") {
		t.Fatal("输出泄露 Key")
	}
	t.Setenv(keyEnv, "")
	settings, source, err := loadConfig()
	if err != nil || source != "config" || settings.APIKey != "environment-fixture" || settings.Model != "portable-model" {
		t.Fatal("配置未持久保存", source, err)
	}
	content, err := os.ReadFile(rules)
	if err != nil || string(content) != "用户自己的规则\n" {
		t.Fatal("初始化修改了个人规则", err)
	}
	for _, directory := range []string{filepath.Join(home, ".agents"), filepath.Join(home, ".codex", "skills")} {
		if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("初始化创建了技能目录", err)
		}
	}
}

func TestPortableCommandsDoNotInstall(t *testing.T) {
	home := isolatedHome(t)
	t.Setenv(keyEnv, "environment-fixture")
	files := map[string]string{
		".agents/skills/jishuya-image/SKILL.md": "现有技能",
		".codex/skills/jishuya-image/SKILL.md":  "旧技能",
		".codex/AGENTS.md":                      "个人规则\n<!-- jishuya-image:begin -->\n旧版偏好\n<!-- jishuya-image:end -->\n",
	}
	for name, content := range files {
		path := filepath.Join(home, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := runCLI(context.Background(), []string{"doctor", "--offline"}, os.Stdin, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	status := result.(map[string]any)
	if status["mode"] != "portable" || status["key_source"] != "environment" || status["network_checked"] != false || status["executable"] == "" {
		t.Fatal("免安装诊断状态错误", status)
	}
	for _, args := range [][]string{{"install", "--codex"}, {"uninstall"}, {"init", "--prefer"}} {
		if _, err := runCLI(context.Background(), args, os.Stdin, io.Discard); err == nil {
			t.Fatal("旧安装入口仍可执行", args)
		}
	}
	count := 0
	err = filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		count++
		name, _ := filepath.Rel(home, path)
		content, err := os.ReadFile(path)
		if err != nil || string(content) != files[filepath.ToSlash(name)] {
			t.Errorf("原有文件被修改或新增文件：%s", name)
		}
		return err
	})
	if err != nil || count != len(files) {
		t.Fatal("命令改变了用户目录", count, err)
	}
	path, _ := configPath()
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("环境变量模式不应创建配置目录", err)
	}
}
