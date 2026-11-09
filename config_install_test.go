package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestPreferencePreservesOtherInstructions(t *testing.T) {
	isolatedHome(t)
	path, _ := preferencePath()
	original := []byte("\xef\xbb\xbf# 我的规则\r\n请使用中文。\r\n")
	if err := writeFileSafely(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := setPreference(true); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if !bytes.HasPrefix(first, original) || !preferenceEnabled() {
		t.Fatal("原有规则被改变")
	}
	if err := setPreference(true); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Fatal("重复启用不是幂等操作")
	}
	if err := setPreference(false); err != nil {
		t.Fatal(err)
	}
	third, _ := os.ReadFile(path)
	if !bytes.HasPrefix(third, original) || preferenceEnabled() {
		t.Fatal("取消偏好删除了其他规则")
	}
	if _, err := preferenceContent([]byte(preferenceStart), true); err == nil {
		t.Fatal("破损标记未报错")
	}
}

func TestPreferenceUsesOverride(t *testing.T) {
	isolatedHome(t)
	home, _ := codexHome()
	path := filepath.Join(home, "AGENTS.override.md")
	if err := writeFileSafely(path, []byte("个人覆盖规则\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := setPreference(true); err != nil {
		t.Fatal(err)
	}
	actual, _ := preferencePath()
	if path != actual {
		t.Fatal("未写入生效的 override 文件")
	}
}

func TestInstallAndUpgrade(t *testing.T) {
	isolatedHome(t)
	if err := writeConfig(config{APIKey: "persistent-fixture", Model: defaultModel}); err != nil {
		t.Fatal(err)
	}
	result, err := installSkill(true, false)
	if err != nil {
		t.Fatal(err)
	}
	directory := result["directory"].(string)
	for _, name := range []string{"SKILL.md", "README.md", "agents/openai.yaml"} {
		installed, err := os.ReadFile(filepath.Join(directory, name))
		original, _ := skillFiles.ReadFile(name)
		if err != nil || !bytes.Equal(installed, original) {
			t.Fatal("安装内容不一致", name, err)
		}
	}
	unrelated := filepath.Join(directory, "用户自己的文件.txt")
	if err := os.WriteFile(unrelated, []byte("保留"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte("旧版说明"), 0644); err != nil {
		t.Fatal(err)
	}
	upgraded, err := installSkill(true, false)
	if err != nil || upgraded["backup"] == "" {
		t.Fatal("升级未备份", err)
	}
	if content, err := os.ReadFile(unrelated); err != nil || string(content) != "保留" {
		t.Fatal("升级删除用户文件")
	}
	settings, _, err := loadConfig()
	if err != nil || settings.APIKey != "persistent-fixture" {
		t.Fatal("升级丢失配置")
	}
	if _, err := installSkill(false, true); err != nil {
		t.Fatal(err)
	}
	if preferenceEnabled() {
		t.Fatal("取消偏好失败")
	}
}

func TestFreshDiscoveryAndDuplicateDetection(t *testing.T) {
	home := isolatedHome(t)
	t.Setenv("CODEX_HOME", "")
	directory, err := skillDirectory()
	if err != nil || directory != filepath.Join(home, ".agents", "skills", "jishuya-image") {
		t.Fatal(directory, err)
	}
	for _, path := range []string{directory, filepath.Join(home, ".codex", "skills", "jishuya-image")} {
		if err := writeFileSafely(filepath.Join(path, "SKILL.md"), []byte("skill"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := skillDirectory(); err == nil {
		t.Fatal("重复技能未报告冲突")
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
	if _, err := initialize(context.Background(), false, false, "", file, io.Discard); err == nil {
		t.Fatal("后台 init 应提示打开终端")
	}
	if _, err := initialize(context.Background(), true, false, "", file, io.Discard); err == nil {
		t.Fatal("环境变量缺失时初始化应失败")
	}
}

func TestCLIHelpAndJSON(t *testing.T) {
	isolatedHome(t)
	for _, args := range [][]string{{"version"}, {"--help"}, {"generate", "--help"}} {
		_, err := runCLI(context.Background(), args, os.Stdin, io.Discard)
		if args[0] != "generate" && err != nil {
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
	if _, err := runCLI(context.Background(), []string{"install"}, os.Stdin, io.Discard); err == nil {
		t.Fatal("安装未限定目标")
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
