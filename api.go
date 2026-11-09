package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const maxResponseBytes = 256 << 20

type apiClient struct {
	base, key string
	http      *http.Client
}

type imageItem struct {
	Base64 string `json:"b64_json"`
	URL    string `json:"url"`
}

type imageResult struct {
	Model        string   `json:"model"`
	Paths        []string `json:"paths"`
	PartialPaths []string `json:"partial_paths,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

// newAPIClient 禁用重定向和重试，避免泄漏密钥或重复计费。
func newAPIClient(key string) *apiClient {
	return &apiClient{base: apiBase, key: key, http: &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// request 仅向固定 API 地址发送鉴权；端点未完成同步请求时明确报错。
func (client *apiClient) request(ctx context.Context, method, endpoint, contentType string, body io.Reader, stream bool) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, client.base+endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+client.key)
	request.Header.Set("User-Agent", "JishuyaImage/"+version)
	request.Header.Set("Accept", "application/json")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if stream {
		request.Header.Set("Accept", "text/event-stream, application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求未完成，勿自动重提；请检查控制台记录：%w", err)
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("HTTP %d，勿自动重提：%s", response.StatusCode, errorDetail(body))
	}
	return response, nil
}

// errorDetail 提取接口错误，避免将整个响应体作为常规日志输出。
func errorDetail(body []byte) string {
	var payload struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
		Detail  string          `json:"detail"`
	}
	if json.Unmarshal(body, &payload) == nil {
		var detail struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(payload.Error, &detail) == nil && detail.Message != "" {
			return detail.Message
		}
		var text string
		if json.Unmarshal(payload.Error, &text) == nil && text != "" {
			return text
		}
		if payload.Message != "" {
			return payload.Message
		}
		if payload.Detail != "" {
			return payload.Detail
		}
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		text = text[:500]
	}
	return text
}

// models 查询授权模型列表；模型可见不代表其所有图片参数可用。
func (client *apiClient) models(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := client.request(ctx, http.MethodGet, "/models", "", nil, false)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("模型列表不是有效 JSON：%w", err)
	}
	if payload.Error != nil && string(payload.Error) != "null" {
		return nil, fmt.Errorf("模型查询失败：%s", errorDetail(payload.Error))
	}
	if payload.Data == nil {
		return nil, errors.New("模型响应缺少 data 数组")
	}
	models := make([]string, 0, len(payload.Data))
	for _, model := range payload.Data {
		if model.ID != "" {
			models = append(models, model.ID)
		}
	}
	sort.Strings(models)
	return models, nil
}

// multipartBody 使用明确的嵌套字段名，并隐藏用户本地文件名。
func multipartBody(options imageOptions) ([]byte, string, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	fields := options.fields()
	delete(fields, "svg")
	if options.SVGModel != "" {
		fields["svg[model]"] = options.SVGModel
	}
	if options.SVGReasoning != "" {
		fields["svg[reasoning][effort]"] = options.SVGReasoning
	}
	for name, value := range fields {
		if err := writer.WriteField(name, fmt.Sprint(value)); err != nil {
			return nil, "", err
		}
	}
	type upload struct{ field, path string }
	files := make([]upload, 0, len(options.Images)+2)
	field := "image"
	if len(options.Images) > 1 {
		field = "image[]"
	}
	for _, path := range options.Images {
		files = append(files, upload{field, path})
	}
	if options.Mask != "" {
		files = append(files, upload{"mask", options.Mask})
	}
	if options.Markup != "" {
		files = append(files, upload{"markup", options.Markup})
	}
	for index, file := range files {
		content, err := os.ReadFile(file.path)
		if err != nil {
			return nil, "", fmt.Errorf("读取参考图失败：%w", err)
		}
		format := rasterFormat(content)
		if format != "png" && format != "jpeg" && format != "webp" {
			return nil, "", errors.New("参考图仅支持 PNG/JPEG/WebP")
		}
		if (file.field == "mask" || file.field == "markup") && format != "png" {
			return nil, "", errors.New("遮罩和标注图必须为 PNG")
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="reference_%d.%s"`, file.field, index, format))
		header.Set("Content-Type", "image/"+format)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(content); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

// generate 保存已收到的结果；后续失败时仍返回这些路径。
func (client *apiClient) generate(ctx context.Context, options imageOptions) (imageResult, error) {
	result := imageResult{Model: options.Model, Paths: []string{}}
	if err := options.validate(); err != nil {
		return result, err
	}
	directory, err := options.outputDirectory()
	if err != nil {
		return result, err
	}
	body, err := json.Marshal(options.fields())
	contentType, endpoint := "application/json", "/images/generations"
	if options.Command == "edit" {
		body, contentType, err = multipartBody(options)
		endpoint = "/images/edits"
	}
	if err != nil {
		return result, err
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return result, err
	}
	// 请求之前检查目录可写，避免生成成功后才发现无法保存。
	probe, err := os.CreateTemp(directory, ".jishuya-write-check-*")
	if err != nil {
		return result, err
	}
	probe.Close()
	os.Remove(probe.Name())
	ctx, cancel := context.WithTimeout(ctx, time.Duration(options.Timeout*float64(time.Second)))
	defer cancel()
	response, err := client.request(ctx, http.MethodPost, endpoint, contentType, bytes.NewReader(body), options.Stream || options.PartialImages > 0)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	observe := func(size, quality string) {
		for _, field := range []struct{ name, requested, actual string }{{"尺寸", options.Size, size}, {"质量", options.Quality, quality}} {
			if field.actual != "" && field.requested != "auto" && field.actual != field.requested {
				message := fmt.Sprintf("平台报告%s为 %s，与请求的 %s 不一致", field.name, field.actual, field.requested)
				if !slices.Contains(result.Warnings, message) {
					result.Warnings = append(result.Warnings, message)
				}
			}
		}
	}
	save := func(item imageItem, partial bool) error {
		path, saveErr := client.saveImage(ctx, item, directory, options.Format, partial)
		if saveErr != nil {
			return saveErr
		}
		if partial {
			result.PartialPaths = append(result.PartialPaths, path)
		} else {
			result.Paths = append(result.Paths, path)
		}
		return nil
	}
	if strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		err = readSSE(response.Body, save, observe)
	} else {
		var payload struct {
			Data    []imageItem     `json:"data"`
			Error   json.RawMessage `json:"error"`
			Size    string          `json:"size"`
			Quality string          `json:"quality"`
		}
		data, readErr := readLimited(response.Body)
		if readErr != nil {
			return result, readErr
		}
		if err = json.Unmarshal(data, &payload); err != nil {
			return result, fmt.Errorf("图片响应不是有效 JSON：%w", err)
		}
		if len(payload.Data) == 0 {
			return result, fmt.Errorf("平台没有返回图片：%s", errorDetail(data))
		}
		observe(payload.Size, payload.Quality)
		for _, item := range payload.Data {
			if err = save(item, false); err != nil {
				break
			}
		}
	}
	if err == nil && len(result.Paths) != options.Count {
		err = fmt.Errorf("请求 %d 张，收到 %d 张最终图片；请核对控制台记录，勿自动重提", options.Count, len(result.Paths))
	}
	if options.PartialImages > 0 && len(result.PartialPaths) == 0 && len(result.Paths) > 0 {
		result.Warnings = append(result.Warnings, "平台未返回中间预览图；已保存最终图片")
	}
	return result, err
}

// readSSE 逐条保存流式图片；支持事件名、JSON type、多行 data 和 DONE。
func readSSE(reader io.Reader, save func(imageItem, bool) error, observe func(string, string)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), maxResponseBytes)
	var eventName string
	var lines []string
	done := false
	consume := func() error {
		if len(lines) == 0 {
			return nil
		}
		data := strings.Join(lines, "\n")
		if strings.TrimSpace(data) == "[DONE]" {
			done = true
			return nil
		}
		var event struct {
			Type    string          `json:"type"`
			Object  string          `json:"object"`
			Data    []imageItem     `json:"data"`
			Size    string          `json:"size"`
			Quality string          `json:"quality"`
			Base64  string          `json:"b64_json"`
			URL     string          `json:"url"`
			Error   json.RawMessage `json:"error"`
			Message string          `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("流式事件格式无效：%w", err)
		}
		kind := event.Type
		if kind == "" {
			kind = eventName
		}
		if (len(event.Error) > 0 && string(event.Error) != "null") || kind == "error" || strings.HasSuffix(kind, ".failed") || strings.HasSuffix(kind, ".error") {
			return fmt.Errorf("图片流失败：%s", errorDetail([]byte(data)))
		}
		// 技术鸭结果事件将最终图片放在 data 数组，进度 chunk 不代表图片。
		if event.Object == "image.generation.result" {
			observe(event.Size, event.Quality)
			for _, item := range event.Data {
				if err := save(item, false); err != nil {
					return err
				}
			}
			return nil
		}
		if strings.HasSuffix(kind, ".partial_image") {
			return save(imageItem{event.Base64, event.URL}, true)
		}
		if strings.HasSuffix(kind, ".completed") {
			observe(event.Size, event.Quality)
			return save(imageItem{event.Base64, event.URL}, false)
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := consume(); err != nil {
				return err
			}
			if done {
				return nil
			}
			eventName, lines = "", nil
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			lines = append(lines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("图片流中断：%w", err)
	}
	return consume()
}

// readLimited 防止异常响应无限占用内存。
func readLimited(reader io.Reader) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(reader, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxResponseBytes {
		return nil, errors.New("单个响应超过 256 MiB 上限")
	}
	return content, nil
}

// saveImage 只给同源 HTTPS 下载附加密钥，所有下载均禁止重定向。
func (client *apiClient) saveImage(ctx context.Context, item imageItem, directory, format string, partial bool) (string, error) {
	var content []byte
	var err error
	if item.Base64 != "" {
		content, err = base64.StdEncoding.Strict().DecodeString(item.Base64)
	} else if item.URL != "" {
		base, _ := url.Parse(client.base + "/")
		reference, parseErr := url.Parse(item.URL)
		if parseErr != nil {
			return "", errors.New("图片下载地址无效")
		}
		target := base.ResolveReference(reference)
		if target.Scheme != "https" || target.Host == "" || target.User != nil {
			return "", errors.New("图片下载地址必须为无用户信息的 HTTPS 地址")
		}
		downloadCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		request, requestErr := http.NewRequestWithContext(downloadCtx, http.MethodGet, target.String(), nil)
		if requestErr != nil {
			return "", requestErr
		}
		request.Header.Set("User-Agent", "JishuyaImage/"+version)
		if strings.EqualFold(target.Host, base.Host) && target.Scheme == base.Scheme {
			request.Header.Set("Authorization", "Bearer "+client.key)
		}
		response, requestErr := client.http.Do(request)
		if requestErr != nil {
			return "", fmt.Errorf("图片下载失败：%w", requestErr)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return "", fmt.Errorf("图片下载 HTTP %d", response.StatusCode)
		}
		content, err = readLimited(response.Body)
	} else {
		return "", errors.New("图片条目缺少 b64_json 或 url")
	}
	if err != nil {
		return "", fmt.Errorf("图片读取失败：%w", err)
	}
	extension, err := imageExtension(content, format)
	if err != nil {
		return "", err
	}
	var identifier [16]byte
	if _, err := rand.Read(identifier[:]); err != nil {
		return "", err
	}
	prefix := "jishuya_"
	if partial {
		prefix = "jishuya_partial_"
	}
	path := filepath.Join(directory, fmt.Sprintf("%s%x%s", prefix, identifier, extension))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// rasterFormat 根据文件签名判断图片格式。
func rasterFormat(content []byte) string {
	switch {
	case bytes.HasPrefix(content, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case bytes.HasPrefix(content, []byte("\xff\xd8\xff")):
		return "jpeg"
	case len(content) >= 12 && string(content[:4]) == "RIFF" && string(content[8:12]) == "WEBP":
		return "webp"
	case bytes.HasPrefix(content, []byte("GIF87a")) || bytes.HasPrefix(content, []byte("GIF89a")):
		return "gif"
	default:
		return ""
	}
}

// imageExtension 验证实际格式；SVG 要求完整 XML，常见位图至少可解码尺寸。
func imageExtension(content []byte, expected string) (string, error) {
	if expected == "svg" {
		decoder := xml.NewDecoder(bytes.NewReader(content))
		depth, roots := 0, 0
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", errors.New("平台返回的 SVG 不是有效 XML")
			}
			switch value := token.(type) {
			case xml.StartElement:
				if depth == 0 {
					roots++
					if value.Name.Local != "svg" {
						return "", errors.New("XML 根元素不是 SVG")
					}
				}
				depth++
			case xml.EndElement:
				depth--
			case xml.CharData:
				if depth == 0 && strings.TrimSpace(string(value)) != "" {
					return "", errors.New("SVG 根元素之外存在文本")
				}
			}
		}
		if roots != 1 || depth != 0 {
			return "", errors.New("SVG 根元素不完整")
		}
		return ".svg", nil
	}
	actual := rasterFormat(content)
	if actual == "" {
		return "", errors.New("平台返回了无法识别的图片")
	}
	if actual != expected {
		return "", fmt.Errorf("请求格式为 %s，实际返回 %s", expected, actual)
	}
	if actual == "gif" {
		animation, err := gif.DecodeAll(bytes.NewReader(content))
		if err != nil {
			return "", errors.New("平台返回的 GIF 无法完整解码")
		}
		if len(animation.Image) < 2 {
			return "", errors.New("平台返回的是单帧 GIF，未生成动画")
		}
	}
	if actual != "webp" {
		if _, _, err := image.DecodeConfig(bytes.NewReader(content)); err != nil {
			return "", errors.New("图片头部不完整或尺寸无效")
		}
	}
	if actual == "jpeg" {
		return ".jpg", nil
	}
	return "." + actual, nil
}
