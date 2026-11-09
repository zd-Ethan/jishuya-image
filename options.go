package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"slices"
	"strings"
)

const apiBase = "https://www.jishuya.net/v1"
const keyEnv = "JISHUYA_IMAGE_API_KEY"
const defaultModel = "gpt-image-2"

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ", ") }
func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type imageOptions struct {
	Command, Prompt, Model, Size, Quality, Complexity string
	Format, Background, Moderation, Mask, Markup      string
	SVGModel, SVGReasoning, OutputDir                 string
	Images                                            stringList
	Count, Compression, PartialImages                 int
	FrameCount, FrameDuration                         int
	Stream                                            bool
	Timeout                                           float64
}

// parseImageOptions 保留原有命令参数，校验失败时不发出付费请求。
func parseImageOptions(command string, args []string, model string, output io.Writer) (imageOptions, error) {
	options := imageOptions{Command: command}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&options.Prompt, "prompt", "", "图片需求，必填")
	flags.StringVar(&options.Model, "model", model, "技术鸭图片模型")
	flags.StringVar(&options.Size, "size", "1024x1024", "auto 或平台支持的尺寸")
	flags.StringVar(&options.Quality, "quality", "auto", "auto/low/medium/high/xhigh/max")
	flags.StringVar(&options.Complexity, "complexity", "", "low/medium/high")
	flags.StringVar(&options.Format, "format", "png", "png/jpeg/webp/gif/svg")
	flags.StringVar(&options.Background, "background", "", "auto/opaque/transparent")
	flags.StringVar(&options.Moderation, "moderation", "auto", "auto/low")
	flags.StringVar(&options.OutputDir, "output-dir", "", "图片保存目录，默认当前工作目录的 jishuya-output")
	flags.IntVar(&options.Count, "count", 1, "图片数量 1–10")
	flags.IntVar(&options.Compression, "compression", -1, "JPEG/WebP 压缩率 0–100")
	flags.IntVar(&options.PartialImages, "partial-images", 0, "中间图数量 0–3")
	flags.IntVar(&options.FrameCount, "frame-count", 0, "GIF 帧数 2–16")
	flags.IntVar(&options.FrameDuration, "frame-duration-ms", 0, "GIF 单帧时长 20–2000 毫秒")
	flags.StringVar(&options.SVGModel, "svg-model", "", "SVG 作者模型")
	flags.StringVar(&options.SVGReasoning, "svg-reasoning", "", "SVG 推理强度")
	flags.BoolVar(&options.Stream, "stream", false, "接收 SSE 响应")
	flags.Float64Var(&options.Timeout, "timeout", 600, "超时时间（秒）")
	if command == "edit" {
		flags.Var(&options.Images, "image", "参考图，可重复传入，最多 16 张")
		flags.StringVar(&options.Mask, "mask", "", "PNG 遮罩图")
		flags.StringVar(&options.Markup, "markup", "", "PNG 标注图")
	}
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, errors.New("不接受位置参数；请通过 --prompt 指定提示词")
	}
	// 区分未填写和显式传入非法哨兵值。
	var invalidOptional string
	flags.Visit(func(option *flag.Flag) {
		switch option.Name {
		case "compression":
			if options.Compression < 0 {
				invalidOptional = option.Name
			}
		case "frame-count":
			if options.FrameCount < 2 {
				invalidOptional = option.Name
			}
		case "frame-duration-ms":
			if options.FrameDuration < 20 {
				invalidOptional = option.Name
			}
		}
	})
	if invalidOptional != "" {
		return options, fmt.Errorf("--%s 超出有效范围", invalidOptional)
	}
	return options, options.validate()
}

// validate 只检查明确的输入约束，平台别名和自定义尺寸交由端点判断。
func (options imageOptions) validate() error {
	if strings.TrimSpace(options.Prompt) == "" || strings.TrimSpace(options.Model) == "" || strings.TrimSpace(options.Size) == "" {
		return errors.New("提示词、模型和尺寸不能为空")
	}
	choices := []struct {
		name, value string
		allowed     []string
	}{
		{"format", options.Format, []string{"png", "jpeg", "webp", "gif", "svg"}},
		{"quality", options.Quality, []string{"auto", "low", "medium", "high", "xhigh", "max"}},
		{"complexity", options.Complexity, []string{"", "low", "medium", "high"}},
		{"background", options.Background, []string{"", "auto", "opaque", "transparent"}},
		{"moderation", options.Moderation, []string{"auto", "low"}},
		{"svg-reasoning", options.SVGReasoning, []string{"", "none", "minimal", "low", "medium", "high", "xhigh", "max"}},
	}
	for _, choice := range choices {
		if !slices.Contains(choice.allowed, choice.value) {
			return fmt.Errorf("--%s 取值无效", choice.name)
		}
	}
	if options.Count < 1 || options.Count > 10 || options.PartialImages < 0 || options.PartialImages > 3 {
		return errors.New("--count 必须为 1–10，--partial-images 必须为 0–3")
	}
	if math.IsNaN(options.Timeout) || math.IsInf(options.Timeout, 0) || options.Timeout <= 0 || options.Timeout > 86400 {
		return errors.New("--timeout 必须为大于零且不超过 86400 的有限秒数")
	}
	if options.Command == "edit" && (len(options.Images) == 0 || len(options.Images) > 16) {
		return errors.New("编辑需要 1–16 张 --image 参考图")
	}
	if options.Mask != "" && options.Markup != "" {
		return errors.New("--mask 和 --markup 不能同时使用")
	}
	if options.Background == "transparent" && !slices.Contains([]string{"png", "webp", "svg"}, options.Format) {
		return errors.New("透明背景只适用于 PNG/WebP/SVG")
	}
	if options.Compression < -1 || options.Compression > 100 {
		return errors.New("--compression 必须为 0–100")
	}
	if options.Compression >= 0 && options.Format != "jpeg" && options.Format != "webp" {
		return errors.New("压缩率只支持 JPEG/WebP")
	}
	streaming := options.Stream || options.PartialImages > 0
	if options.Format == "gif" && (options.Mask != "" || streaming) {
		return errors.New("GIF 不支持遮罩或流式响应")
	}
	if options.Format == "svg" && (options.Count != 1 || streaming) {
		return errors.New("SVG 要求单张图片且不使用流式响应")
	}
	if (options.FrameCount != 0 || options.FrameDuration != 0) && options.Format != "gif" {
		return errors.New("GIF 帧参数只适用于 --format gif")
	}
	if options.FrameCount != 0 && (options.FrameCount < 2 || options.FrameCount > 16) {
		return errors.New("GIF 帧数必须为 2–16")
	}
	if options.FrameDuration != 0 && (options.FrameDuration < 20 || options.FrameDuration > 2000) {
		return errors.New("GIF 单帧时长必须为 20–2000 毫秒")
	}
	if (options.SVGModel != "" || options.SVGReasoning != "") && options.Format != "svg" {
		return errors.New("SVG 作者参数只适用于 --format svg")
	}
	if options.SVGReasoning != "" && options.SVGModel == "" {
		return errors.New("--svg-reasoning 需要 --svg-model")
	}
	return nil
}

// fields 将命令行参数转换为技术鸭请求字段。
func (options imageOptions) fields() map[string]any {
	fields := map[string]any{"model": options.Model, "prompt": options.Prompt, "n": options.Count, "size": options.Size, "quality": options.Quality, "output_format": options.Format, "moderation": options.Moderation}
	if options.Complexity != "" {
		fields["complexity"] = options.Complexity
	}
	if options.Background != "" {
		fields["background"] = options.Background
	}
	if options.Compression >= 0 {
		fields["output_compression"] = options.Compression
	}
	if options.Stream || options.PartialImages > 0 {
		fields["stream"] = true
	}
	if options.PartialImages > 0 {
		fields["partial_images"] = options.PartialImages
	}
	if options.FrameCount != 0 {
		fields["frame_count"] = options.FrameCount
	}
	if options.FrameDuration != 0 {
		fields["frame_duration_ms"] = options.FrameDuration
	}
	if options.SVGModel != "" {
		svg := map[string]any{"model": options.SVGModel}
		if options.SVGReasoning != "" {
			svg["reasoning"] = map[string]string{"effort": options.SVGReasoning}
		}
		fields["svg"] = svg
	}
	return fields
}

// outputDirectory 将输出目录固定为绝对路径，避免写进安装包。
func (options imageOptions) outputDirectory() (string, error) {
	path := options.OutputDir
	if path == "" {
		path = "jishuya-output"
	}
	return filepath.Abs(path)
}
