package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed SKILL.md README.md agents/openai.yaml THIRD-PARTY-NOTICES.txt
var skillFiles embed.FS

const preferenceStart = "<!-- jishuya-image:begin -->"
const preferenceEnd = "<!-- jishuya-image:end -->"
const preferenceRule = "用户请求生成或编辑图片，且没有指定其他服务时，优先使用 jishuya-image 技能。图片需求无需包含技术鸭名称；连续修改应沿用前文图片。只分析图片、讨论设计或撰写提示词时不执行生图；用户明确选择其他服务时遵循其选择。遵守该技能的配置检查和失败处理规则。"

// codexHome 只读取 CODEX_HOME，不修改用户的全局环境变量。
func codexHome() (string, error) {
	if path := os.Getenv("CODEX_HOME"); path != "" {
		return filepath.Abs(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

// skillDirectory 升级已有同名目录；新安装使用 Codex 的用户技能目录。
func skillDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	codex, err := codexHome()
	if err != nil {
		return "", err
	}
	legacy := filepath.Join(codex, "skills", "jishuya-image")
	modern := filepath.Join(home, ".agents", "skills", "jishuya-image")
	_, legacyErr := os.Stat(filepath.Join(legacy, "SKILL.md"))
	_, modernErr := os.Stat(filepath.Join(modern, "SKILL.md"))
	if legacyErr != nil && !errors.Is(legacyErr, os.ErrNotExist) {
		return "", fmt.Errorf("无法检查现有技能 %s：%w", legacy, legacyErr)
	}
	if modernErr != nil && !errors.Is(modernErr, os.ErrNotExist) {
		return "", fmt.Errorf("无法检查现有技能 %s：%w", modern, modernErr)
	}
	if legacyErr == nil && modernErr == nil {
		return "", errors.New("检测到两份 jishuya-image；请先保留一份，避免重复发现")
	}
	if legacyErr == nil {
		return legacy, nil
	}
	if os.Getenv("CODEX_HOME") != "" {
		return legacy, nil
	}
	return modern, nil
}

// preferencePath 优先修改实际生效的个人 override 文件，保留原有规则。
func preferencePath() (string, error) {
	home, err := codexHome()
	if err != nil {
		return "", err
	}
	override := filepath.Join(home, "AGENTS.override.md")
	if _, err := os.Stat(override); err == nil {
		return override, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("无法检查个人规则 %s：%w", override, err)
	}
	return filepath.Join(home, "AGENTS.md"), nil
}

// preferenceContent 仅替换技术鸭管理块，支持重复安装和显式取消偏好。
func preferenceContent(content []byte, enabled bool) ([]byte, error) {
	text := string(content)
	start, end := strings.Index(text, preferenceStart), strings.Index(text, preferenceEnd)
	if (start < 0) != (end < 0) || (start >= 0 && end < start) {
		return nil, errors.New("个人规则中的技术鸭标记不完整，请先修复该段")
	}
	block := ""
	if enabled {
		block = preferenceStart + "\n" + preferenceRule + "\n" + preferenceEnd
	}
	if start >= 0 {
		if strings.Count(text, preferenceStart) != 1 || strings.Count(text, preferenceEnd) != 1 {
			return nil, errors.New("检测到重复的技术鸭规则块")
		}
		return []byte(text[:start] + block + text[end+len(preferenceEnd):]), nil
	}
	if !enabled {
		return content, nil
	}
	return append(content, []byte("\n\n"+block+"\n")...), nil
}

// setPreference 备份个人规则后写入管理块；取消时仅删除此块。
func setPreference(enabled bool) error {
	path, err := preferencePath()
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	updated, err := preferenceContent(content, enabled)
	if err != nil || bytes.Equal(content, updated) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if len(content) > 0 {
		if _, err := backupFiles(map[string][]byte{filepath.Base(path): content}); err != nil {
			return err
		}
	}
	return writeFileSafely(path, updated, 0644)
}

// preferenceEnabled 只返回状态，不输出个人规则内容。
func preferenceEnabled() bool {
	path, err := preferencePath()
	if err != nil {
		return false
	}
	content, _ := os.ReadFile(path)
	return bytes.Contains(content, []byte(preferenceStart)) && bytes.Contains(content, []byte(preferenceEnd))
}

// writeFileSafely 先写同目录临时文件，再替换目标文件。
func writeFileSafely(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".jishuya-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// backupFiles 将被替换的文件放到用户数据目录，避免备份技能被重复发现。
func backupFiles(files map[string][]byte) (string, error) {
	if len(files) == 0 {
		return "", nil
	}
	configFile, err := configPath()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(filepath.Dir(configFile), "backups")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	if err := protectPath(filepath.Dir(configFile), true); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, "jishuya-backup-*.zip")
	if err != nil {
		return "", err
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(filepath.ToSlash(name))
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
	zipErr := writer.Close()
	closeErr := file.Close()
	return file.Name(), errors.Join(zipErr, closeErr)
}

// installSkill 安装程序和内嵌说明；只更新本技能管理的文件。
func installSkill(prefer, noPrefer bool) (map[string]any, error) {
	if prefer && noPrefer {
		return nil, errors.New("--prefer 和 --no-prefer 不能同时使用")
	}
	directory, err := skillDirectory()
	if err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, err
	}
	name := "jishuya-image"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	files := make(map[string][]byte)
	for _, path := range []string{"SKILL.md", "README.md", "agents/openai.yaml", "THIRD-PARTY-NOTICES.txt"} {
		files[path], err = skillFiles.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	binaryPath := filepath.Join("bin", name)
	files[binaryPath], err = os.ReadFile(executable)
	if err != nil {
		return nil, err
	}
	previous := make(map[string][]byte)
	for name, content := range files {
		path := filepath.Join(directory, name)
		old, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if bytes.Equal(old, content) {
			delete(files, name)
		} else if err == nil {
			previous[name] = old
		}
	}
	backup, err := backupFiles(previous)
	if err != nil {
		return nil, err
	}
	// 二进制先落盘，文档随后更新；同路径运行时内容相同，不覆盖正在运行的程序。
	for _, name := range []string{binaryPath, "SKILL.md", "README.md", "agents/openai.yaml", "THIRD-PARTY-NOTICES.txt"} {
		content, exists := files[name]
		if !exists {
			continue
		}
		mode := os.FileMode(0644)
		if name == binaryPath {
			mode = 0755
		}
		if err := writeFileSafely(filepath.Join(directory, name), content, mode); err != nil {
			return nil, fmt.Errorf("安装失败，备份位置 %s：%w", backup, err)
		}
	}
	if prefer || noPrefer {
		if err := setPreference(prefer); err != nil {
			return nil, err
		}
	}
	return map[string]any{"installed": true, "directory": directory, "executable": filepath.Join(directory, binaryPath), "backup": backup, "preference_enabled": preferenceEnabled(), "next": "新用户请在本机终端运行 init；在 Codex 新对话检查技能是否已发现", "installed_at": time.Now().Format(time.RFC3339)}, nil
}
