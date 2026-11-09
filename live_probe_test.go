//go:build liveprobe

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type probeBody struct {
	io.Reader
	io.Closer
}

type probeTransport struct {
	body        bytes.Buffer
	status      int
	contentType string
}

// RoundTrip 只记录图片响应体，不记录请求头或用户密钥，不改变流式读取行为。
func (transport *probeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(request)
	if err == nil && request.Method == http.MethodPost {
		transport.status = response.StatusCode
		transport.contentType = response.Header.Get("Content-Type")
		response.Body = probeBody{io.TeeReader(response.Body, &transport.body), response.Body}
	}
	return response, err
}

// inspectProbeImage 检查实际图片格式、尺寸和 GIF 帧，格式错误的响应也保留证据。
func inspectProbeImage(content []byte, directory string, index int) map[string]any {
	format := rasterFormat(content)
	if format == "" {
		if _, err := imageExtension(content, "svg"); err == nil {
			format = "svg"
		} else {
			format = "unknown"
		}
	}
	path := filepath.Join(directory, fmt.Sprintf("received-%02d.%s", index, format))
	result := map[string]any{"format": format, "bytes": len(content), "path": path}
	if err := os.WriteFile(path, content, 0600); err != nil {
		result["save_error"] = err.Error()
	}
	if config, _, err := image.DecodeConfig(bytes.NewReader(content)); err == nil {
		result["width"], result["height"] = config.Width, config.Height
	}
	if format == "gif" {
		animation, err := gif.DecodeAll(bytes.NewReader(content))
		if err != nil {
			result["decode_error"] = err.Error()
		} else {
			result["frames"], result["delay_centiseconds"], result["loop_count"] = len(animation.Image), animation.Delay, animation.LoopCount
			hashes := make([]string, 0, len(animation.Image))
			for _, frame := range animation.Image {
				hashes = append(hashes, fmt.Sprintf("%x", sha256.Sum256(frame.Pix)))
			}
			result["frame_pixel_hashes"] = hashes
		}
	}
	return result
}

// summarizeProbeValue 保留响应协议结构，以文件信息替代大段 Base64，不输出密钥。
func summarizeProbeValue(value any, directory, key string, images *[]any) any {
	switch item := value.(type) {
	case map[string]any:
		for name, child := range item {
			if text, ok := child.(string); ok && (name == "b64_json" || name == "partial_image_b64") {
				content, err := base64.StdEncoding.DecodeString(text)
				if err == nil {
					info := inspectProbeImage(content, directory, len(*images)+1)
					*images = append(*images, info)
					item[name] = info
					continue
				}
			}
			item[name] = summarizeProbeValue(child, directory, key, images)
		}
	case []any:
		for index, child := range item {
			item[index] = summarizeProbeValue(child, directory, key, images)
		}
	case string:
		item = redact(item, key)
		if len(item) > 2000 {
			item = item[:2000] + "...[truncated]"
		}
		return item
	}
	return value
}

// TestLiveProbe 需显式 build tag 和单个用例文件；每次只提交一次付费请求。
func TestLiveProbe(t *testing.T) {
	casePath := os.Getenv("JISHUYA_LIVE_CASE")
	if casePath == "" {
		t.Skip("需指定 JISHUYA_LIVE_CASE 用例文件")
	}
	content, err := os.ReadFile(casePath)
	if err != nil {
		t.Fatal(err)
	}
	var testCase struct {
		Case      string   `json:"case"`
		Arguments []string `json:"arguments"`
	}
	if err := json.Unmarshal(content, &testCase); err != nil {
		t.Fatal(err)
	}
	if len(testCase.Arguments) == 0 || (testCase.Arguments[0] != "generate" && testCase.Arguments[0] != "edit") {
		t.Fatal("用例必须指定生成或编辑命令")
	}
	settings, _, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateKey(settings.APIKey); err != nil {
		t.Fatal(err)
	}
	directory := strings.TrimSuffix(casePath, filepath.Ext(casePath))
	arguments := append(append([]string{}, testCase.Arguments[1:]...), "--output-dir", directory, "--timeout", "300")
	options, err := parseImageOptions(testCase.Arguments[0], arguments, settings.Model, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	transport := &probeTransport{}
	client := newAPIClient(settings.APIKey)
	client.http.Transport = transport
	started := time.Now()
	result, requestErr := client.generate(context.Background(), options)
	report := map[string]any{"case": testCase.Case, "arguments": testCase.Arguments, "request_fields": options.fields(), "elapsed_seconds": time.Since(started).Seconds(), "http_status": transport.status, "content_type": transport.contentType, "result": result, "success": requestErr == nil}
	if requestErr != nil {
		report["error"] = redact(requestErr.Error(), settings.APIKey)
	}
	var images []any
	var payload any
	if strings.HasPrefix(transport.contentType, "text/event-stream") {
		var events []any
		for _, line := range strings.Split(transport.body.String(), "\n") {
			line = strings.TrimSuffix(line, "\r")
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if json.Unmarshal([]byte(data), &payload) == nil {
					events = append(events, summarizeProbeValue(payload, directory, settings.APIKey, &images))
				} else {
					events = append(events, redact(data, settings.APIKey))
				}
			} else if strings.HasPrefix(line, "event:") {
				events = append(events, line)
			}
		}
		report["events"] = events
	} else if json.Unmarshal(transport.body.Bytes(), &payload) == nil {
		report["response"] = summarizeProbeValue(payload, directory, settings.APIKey, &images)
	} else {
		report["response_bytes"] = transport.body.Len()
	}
	report["received_images"] = images
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(directory+".result.json", []byte(redact(string(encoded), settings.APIKey)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("case=%s status=%d success=%t images=%d report=%s", testCase.Case, transport.status, requestErr == nil, len(images), directory+".result.json")
	if requestErr != nil {
		t.Error(redact(requestErr.Error(), settings.APIKey))
	}
}
