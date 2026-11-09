package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
)

var version = "2.1.0"

const usage = `技术鸭生图 — 解压即用，无需安装语言运行环境

命令：
  init [--from-env] [--model 模型]           可选：保存本机 Key 并查询模型验证
  doctor [--offline]                        检查配置和连接
  models                                   查询当前 Key 可见的全部模型
  generate --prompt "图片需求" [选项]       生成图片
  edit --image 输入.png --prompt "修改需求"  编辑图片
  version                                  查看版本

执行 generate --help 或 edit --help 查看图片参数。
Key 从 JISHUYA_IMAGE_API_KEY 或用户配置读取；请勿通过命令行传入 Key。
`

// runCLI 分发命令；所有网络调用都沿用同一个取消上下文。
func runCLI(ctx context.Context, args []string, input *os.File, output io.Writer) (any, error) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" || args[0] == "-h" {
		fmt.Fprint(output, usage)
		return nil, nil
	}
	command := args[0]
	if command == "install" || command == "uninstall" {
		return nil, errors.New("当前版本统一免安装：解压后直接使用；放入技能目录即可发现，升级或卸载请按 README 操作文件夹")
	}
	if command == "version" {
		if len(args) != 1 {
			return nil, errors.New("version 不接受参数")
		}
		return map[string]string{"version": version, "platform": runtime.GOOS + "/" + runtime.GOARCH}, nil
	}
	if command == "init" || command == "doctor" {
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(output)
		var fromEnv, offline bool
		var model string
		switch command {
		case "init":
			flags.BoolVar(&fromEnv, "from-env", false, "从环境变量读取 Key")
			flags.StringVar(&model, "model", "", "默认图片模型")
		case "doctor":
			flags.BoolVar(&offline, "offline", false, "仅检查本机配置，不访问网络")
		}
		if err := flags.Parse(args[1:]); err != nil {
			return nil, err
		}
		if flags.NArg() > 0 {
			return nil, errors.New("存在无法识别的位置参数")
		}
		switch command {
		case "init":
			return initialize(ctx, fromEnv, model, input, output)
		case "doctor":
			return diagnose(ctx, offline)
		}
	}
	if command != "models" && command != "generate" && command != "edit" {
		return nil, fmt.Errorf("未知命令 %q；使用 --help 查看帮助", command)
	}
	settings, _, err := loadConfig()
	if err != nil {
		return nil, err
	}
	client := newAPIClient(settings.APIKey)
	if command == "models" {
		if len(args) != 1 {
			return nil, errors.New("models 不接受参数")
		}
		if err := validateKey(settings.APIKey); err != nil {
			return nil, err
		}
		models, err := client.models(ctx)
		return map[string]any{"models": models}, err
	}
	options, err := parseImageOptions(command, args[1:], settings.Model, output)
	if err != nil {
		return nil, err
	}
	if err := validateKey(settings.APIKey); err != nil {
		return nil, err
	}
	return client.generate(ctx, options)
}

// diagnose 输出可诊断状态，不显示密钥，也不提交生图请求。
func diagnose(ctx context.Context, offline bool) (map[string]any, error) {
	settings, source, err := loadConfig()
	if err != nil {
		return nil, err
	}
	path, _ := configPath()
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"version": version, "platform": runtime.GOOS + "/" + runtime.GOARCH, "mode": "portable", "executable": executable, "key_configured": settings.APIKey != "", "key_source": source, "config_path": path, "model": settings.Model, "network_checked": false}
	if err := validateKey(settings.APIKey); err != nil {
		return result, err
	}
	if offline {
		return result, nil
	}
	models, err := newAPIClient(settings.APIKey).models(ctx)
	result["network_checked"] = true
	if err != nil {
		return result, err
	}
	result["model_available"] = slices.Contains(models, settings.Model)
	if !slices.Contains(models, settings.Model) {
		return result, errors.New("默认模型不在当前 Key 的模型列表中；请用 init --model 重新选择")
	}
	return result, nil
}

// printResult 统一输出 JSON；失败时保留已经生成的图片路径。
func printResult(output io.Writer, result any, err error, key string) {
	var payload any = result
	if err != nil {
		payload = map[string]any{"error": err.Error(), "result": result}
	}
	if payload == nil {
		return
	}
	encoded, encodeErr := json.MarshalIndent(payload, "", "  ")
	if encodeErr != nil {
		encoded = []byte(`{"error":"无法编码结果"}`)
	}
	fmt.Fprintln(output, redact(string(encoded), key))
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	result, err := runCLI(ctx, os.Args[1:], os.Stdin, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	settings, _, _ := loadConfig()
	printResult(os.Stdout, result, err, settings.APIKey)
	if err != nil {
		os.Exit(1)
	}
}
