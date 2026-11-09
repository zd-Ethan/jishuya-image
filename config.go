package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/term"
)

type config struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// configPath 使用操作系统的用户配置目录，升级 skill 不会丢失密钥。
func configPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "jishuya-image", "config.json"), nil
}

// loadConfig 环境变量优先；不向调用日志输出配置文件内容。
func loadConfig() (config, string, error) {
	settings := config{Model: defaultModel}
	environmentKey := strings.TrimSpace(os.Getenv(keyEnv))
	path, err := configPath()
	if err != nil {
		if environmentKey != "" {
			settings.APIKey = environmentKey
			return settings, "environment", nil
		}
		return settings, "", err
	}
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		if environmentKey != "" {
			settings.APIKey = environmentKey
			return settings, "environment", nil
		}
		return settings, "", fmt.Errorf("无法读取本机配置；请允许宿主读取该配置文件，或向其运行进程配置 %s：%w", keyEnv, err)
	}
	if len(content) != 0 && json.Unmarshal(content, &settings) != nil {
		if environmentKey != "" {
			return config{APIKey: environmentKey, Model: defaultModel}, "environment", nil
		}
		return settings, "", errors.New("本机配置格式无效；请重新运行 init")
	}
	if settings.Model == "" {
		settings.Model = defaultModel
	}
	source := "missing"
	if settings.APIKey != "" {
		source = "config"
	}
	if environmentKey != "" {
		settings.APIKey, source = environmentKey, "environment"
	}
	return settings, source, nil
}

// writeConfig 在写入密钥之前限制目录和临时文件权限。
func writeConfig(settings config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	if err := protectPath(directory, true); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := protectPath(file.Name(), false); err != nil {
		file.Close()
		return err
	}
	encodeErr := json.NewEncoder(file).Encode(settings)
	closeErr := file.Close()
	if err := errors.Join(encodeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// initialize 只查询模型检查连接；密钥通过终端隐藏输入或环境变量提供。
func initialize(ctx context.Context, fromEnvironment bool, model string, input *os.File, output io.Writer) (map[string]any, error) {
	settings, _, err := loadConfig()
	if err != nil {
		settings = config{Model: defaultModel}
	}
	key := strings.TrimSpace(os.Getenv(keyEnv))
	if !fromEnvironment {
		if !term.IsTerminal(int(input.Fd())) {
			return nil, errors.New("init 需要用户可见的交互终端；请在终端运行，或配置环境变量后使用 init --from-env")
		}
		fmt.Fprint(output, "请输入技术鸭 API Key（输入隐藏，回车可保留已配置的 Key）：")
		secret, err := term.ReadPassword(int(input.Fd()))
		fmt.Fprintln(output)
		if err != nil {
			return nil, err
		}
		key = strings.TrimSpace(string(secret))
		if key == "" {
			key = settings.APIKey
		}
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if model != "" {
		settings.Model = model
	}
	client := newAPIClient(key)
	models, err := client.models(ctx)
	if err != nil {
		return nil, errors.New(redact(err.Error(), key))
	}
	if !slices.Contains(models, settings.Model) {
		return nil, fmt.Errorf("当前 Key 无法访问模型 %s；请运行 models 查看列表，再用 init --model 指定模型", settings.Model)
	}
	settings.APIKey = key
	if err := writeConfig(settings); err != nil {
		return nil, errors.New(redact(err.Error(), key))
	}
	path, _ := configPath()
	return map[string]any{"configured": true, "model": settings.Model, "config_path": path, "note": "初始化仅保存本机配置并查询模型；模型可见不代表所有图片参数已验证"}, nil
}

// validateKey 只拒绝空密钥和非法头字符，不假设服务端的密钥长度。
func validateKey(key string) error {
	if key == "" {
		return errors.New("尚未配置技术鸭 Key；请在本机终端运行对应平台程序的 init 命令，或设置 JISHUYA_IMAGE_API_KEY")
	}
	if strings.ContainsAny(key, "\r\n\t ") {
		return errors.New("Key 不能包含空白字符")
	}
	return nil
}

// redact 对所有输出执行最后一道密钥脱敏。
func redact(text string, key string) string {
	if key != "" {
		encoded, _ := json.Marshal(key)
		text = strings.ReplaceAll(text, string(encoded[1:len(encoded)-1]), "[REDACTED]")
		text = strings.ReplaceAll(text, key, "[REDACTED]")
	}
	return text
}
