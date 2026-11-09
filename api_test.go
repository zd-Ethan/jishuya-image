package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testImage 生成可解码的小图片，避免测试只验证伪造签名。
func testImage(t *testing.T, format string) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.RGBA{255, 200, 0, 255})
	var buffer bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&buffer, picture)
	case "jpeg":
		err = jpeg.Encode(&buffer, picture, nil)
	case "gif":
		first := image.NewPaletted(picture.Bounds(), []color.Color{color.White, color.Black})
		second := image.NewPaletted(picture.Bounds(), first.Palette)
		second.SetColorIndex(0, 0, 1)
		err = gif.EncodeAll(&buffer, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{20, 20}})
	case "svg":
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="2" height="2"><rect width="2" height="2"/></svg>`)
	case "webp":
		content, decodeErr := base64.StdEncoding.DecodeString("UklGRh4AAABXRUJQVlA4TBEAAAAvAAAAAAfQ//73v/+BiOh/AAA=")
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		return content
	}
	if err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// mockClient 在 TLS 本地端点执行真实 HTTP 编解码。
func mockClient(t *testing.T, handler http.HandlerFunc) (*apiClient, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	client := newAPIClient("fixture-key")
	client.base, client.http = server.URL+"/v1", server.Client()
	client.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return client, server
}

func TestGenerateFormats(t *testing.T) {
	for _, format := range []string{"png", "jpeg", "webp", "gif", "svg"} {
		t.Run(format, func(t *testing.T) {
			picture := testImage(t, format)
			options := testOptions(t, "--format", format)
			if format == "gif" {
				options.FrameCount, options.FrameDuration = 8, 120
			}
			if format == "svg" {
				options.SVGModel, options.SVGReasoning, options.Background = "author", "high", "transparent"
			}
			client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/v1/images/generations" || request.Method != "POST" || request.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Error("请求路径或鉴权错误")
				}
				var fields map[string]any
				if err := json.NewDecoder(request.Body).Decode(&fields); err != nil {
					t.Error(err)
				}
				if fields["prompt"] != "小黄鸭" || fields["output_format"] != format {
					t.Error(fields)
				}
				if format == "gif" && (fields["frame_count"] != float64(8) || fields["frame_duration_ms"] != float64(120)) {
					t.Error(fields)
				}
				if format == "svg" && fields["svg"].(map[string]any)["model"] != "author" {
					t.Error(fields)
				}
				json.NewEncoder(writer).Encode(map[string]any{"data": []imageItem{{Base64: base64.StdEncoding.EncodeToString(picture)}}})
			})
			result, err := client.generate(context.Background(), options)
			if err != nil || len(result.Paths) != 1 {
				t.Fatal(result, err)
			}
			content, err := os.ReadFile(result.Paths[0])
			if err != nil || !bytes.Equal(content, picture) || !filepath.IsAbs(result.Paths[0]) {
				t.Fatal("保存结果不正确", err)
			}
		})
	}
}

func TestMultipartEdit(t *testing.T) {
	for _, overlay := range []string{"mask", "markup", "svg"} {
		t.Run(overlay, func(t *testing.T) {
			options := testOptions(t)
			options.Command = "edit"
			picture := testImage(t, "png")
			path := filepath.Join(t.TempDir(), "用户的原始文件名.png")
			if err := os.WriteFile(path, picture, 0600); err != nil {
				t.Fatal(err)
			}
			options.Images = []string{path, path}
			if overlay == "mask" {
				options.Mask = path
			}
			if overlay == "markup" {
				options.Markup = path
			}
			if overlay == "svg" {
				options.Format, options.SVGModel, options.SVGReasoning = "svg", "author", "high"
			}
			client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/v1/images/edits" {
					t.Error(request.URL.Path)
				}
				if err := request.ParseMultipartForm(1 << 20); err != nil {
					t.Error(err)
					return
				}
				defer request.MultipartForm.RemoveAll()
				if len(request.MultipartForm.File["image[]"]) != 2 {
					t.Error("多图字段不正确")
				}
				for _, file := range request.MultipartForm.File["image[]"] {
					if strings.Contains(file.Filename, "原始") || file.Header.Get("Content-Type") != "image/png" {
						t.Error(file)
					}
				}
				if overlay != "svg" && len(request.MultipartForm.File[overlay]) != 1 {
					t.Error("缺少编辑图层", overlay)
				}
				if overlay == "svg" && (request.FormValue("svg[model]") != "author" || request.FormValue("svg[reasoning][effort]") != "high") {
					t.Error("SVG 嵌套字段错误")
				}
				json.NewEncoder(writer).Encode(map[string]any{"data": []imageItem{{Base64: base64.StdEncoding.EncodeToString(testImage(t, options.Format))}}})
			})
			if _, err := client.generate(context.Background(), options); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSEAndPreservedResults(t *testing.T) {
	for _, ending := range []string{"done", "eof", "error", "missing_final", "malformed"} {
		t.Run(ending, func(t *testing.T) {
			encoded := base64.StdEncoding.EncodeToString(testImage(t, "png"))
			client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(writer, ": keepalive\r\nevent: image_generation.partial_image\r\ndata: {\"b64_json\":\"%s\"}\r\n\r\n", encoded)
				if ending != "missing_final" {
					fmt.Fprintf(writer, "data: {\"type\":\"image_generation.completed\",\n")
					fmt.Fprintf(writer, "data: \"b64_json\":\"%s\"}\n\n", encoded)
				}
				switch ending {
				case "done":
					fmt.Fprint(writer, "data: [DONE]\n\n")
				case "error":
					fmt.Fprint(writer, "event: error\ndata: {\"message\":\"upstream failed\"}\n\n")
				case "malformed":
					fmt.Fprint(writer, "data: invalid\n\n")
				}
			})
			result, err := client.generate(context.Background(), testOptions(t, "--partial-images", "1"))
			if (ending == "done" || ending == "eof") != (err == nil) {
				t.Fatal(ending, err)
			}
			if len(result.PartialPaths) != 1 {
				t.Fatal("中间图没有保留", result)
			}
			if ending != "missing_final" && len(result.Paths) != 1 {
				t.Fatal("最终图没有保留", result)
			}
		})
	}
}

func TestNoRetryOrRedirect(t *testing.T) {
	for _, status := range []int{202, 301, 307, 401, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var count atomic.Int32
			client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
				count.Add(1)
				writer.Header().Set("Location", "/retry")
				writer.WriteHeader(status)
				fmt.Fprint(writer, `{"error":{"message":"rejected fixture-key"}}`)
			})
			result, err := client.generate(context.Background(), testOptions(t))
			if err == nil || count.Load() != 1 {
				t.Fatal(err, count.Load())
			}
			var output bytes.Buffer
			printResult(&output, result, err, client.key)
			if strings.Contains(output.String(), client.key) {
				t.Fatal("错误输出泄漏密钥")
			}
		})
	}
}

// TestJishuyaSSEProtocol 复现实际端点的 data 数组和无 type 错误，避免流式图片被静默丢弃。
func TestJishuyaSSEProtocol(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(testImage(t, "png"))
	for _, ending := range []string{"done", "error"} {
		t.Run(ending, func(t *testing.T) {
			client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(writer, "data: {\"object\":\"image.generation.chunk\",\"data\":[],\"progress_text\":\"生成中\"}\n\n")
				fmt.Fprintf(writer, "data: {\"object\":\"image.generation.result\",\"data\":[{\"b64_json\":\"%s\"},{\"b64_json\":\"%s\"}]}\n\n", encoded, encoded)
				if ending == "error" {
					fmt.Fprint(writer, "data: {\"error\":{\"message\":\"upstream failed\"}}\n\n")
				}
				fmt.Fprint(writer, "data: [DONE]\n\n")
			})
			result, err := client.generate(context.Background(), testOptions(t, "--stream", "--partial-images", "1", "--count", "2"))
			if (err == nil) != (ending == "done") || len(result.Paths) != 2 || len(result.PartialPaths) != 0 {
				t.Fatal("平台结果事件应保存所有图片，后续错误应保留结果", result, err)
			}
			if len(result.Warnings) != 1 {
				t.Fatal("缺少中间预览图时应告知用户", result)
			}
		})
	}
}

func TestDownloadsRestrictCredentials(t *testing.T) {
	picture := testImage(t, "png")
	var auth string
	client, server := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
		auth = request.Header.Get("Authorization")
		writer.Write(picture)
	})
	if _, err := client.saveImage(context.Background(), imageItem{URL: server.URL + "/image.png"}, t.TempDir(), "png", false); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer fixture-key" {
		t.Fatal("同源鉴权缺失")
	}
	other := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		auth = request.Header.Get("Authorization")
		writer.Write(picture)
	}))
	defer other.Close()
	if _, err := client.saveImage(context.Background(), imageItem{URL: other.URL + "/image.png"}, t.TempDir(), "png", false); err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		t.Fatal("跨源发送了密钥")
	}
	for _, target := range []string{"http://example.com/a.png", "file:///x", "https://user:password@example.com/x"} {
		if _, err := client.saveImage(context.Background(), imageItem{URL: target}, t.TempDir(), "png", false); err == nil {
			t.Fatal("危险下载地址被接受", target)
		}
	}
}

func TestImageValidation(t *testing.T) {
	for _, content := range [][]byte{[]byte("<html/>"), []byte("<svg>"), []byte("<svg/><svg/>"), []byte("<svg/>garbage"), []byte(" ")} {
		if _, err := imageExtension(content, "svg"); err == nil {
			t.Fatal("无效 SVG 被接受")
		}
	}
	if _, err := imageExtension(testImage(t, "png"), "gif"); err == nil {
		t.Fatal("格式不符被接受")
	}
	if _, err := imageExtension([]byte("\x89PNG\r\n\x1a\n"), "png"); err == nil {
		t.Fatal("损坏的 PNG 被接受")
	}
}

// TestGIFRequiresAnimation 防止把静态或损坏的 GIF 当作动画成功交付。
func TestGIFRequiresAnimation(t *testing.T) {
	var still bytes.Buffer
	if err := gif.Encode(&still, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	animation := testImage(t, "gif")
	for _, content := range [][]byte{still.Bytes(), animation[:len(animation)/2]} {
		if _, err := imageExtension(content, "gif"); err == nil {
			t.Fatal("静态或损坏 GIF 被当成动画")
		}
	}
	if _, err := imageExtension(animation, "gif"); err != nil {
		t.Fatal(err)
	}
}

// TestResponseMetadataWarnings 提示平台报告的尺寸或质量降档，保留已生成图片。
func TestResponseMetadataWarnings(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, mismatch := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/mismatch=%t", stream, mismatch), func(t *testing.T) {
				client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
					payload := map[string]any{"size": "2x2", "quality": "high", "data": []imageItem{{Base64: base64.StdEncoding.EncodeToString(testImage(t, "png"))}}}
					if stream {
						writer.Header().Set("Content-Type", "text/event-stream")
						payload["object"] = "image.generation.result"
						encoded, _ := json.Marshal(payload)
						fmt.Fprintf(writer, "data: %s\n\ndata: [DONE]\n\n", encoded)
					} else {
						json.NewEncoder(writer).Encode(payload)
					}
				})
				options := testOptions(t, "--size", "2x2", "--quality", "high")
				options.Stream = stream
				if mismatch {
					options.Size, options.Quality = "3x3", "xhigh"
				}
				result, err := client.generate(context.Background(), options)
				wantWarnings := 0
				if mismatch {
					wantWarnings = 2
				}
				if err != nil || len(result.Paths) != 1 || len(result.Warnings) != wantWarnings {
					t.Fatal(result, err)
				}
			})
		}
	}
}

func TestJSONErrorPreservesImages(t *testing.T) {
	var calls atomic.Int32
	client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		json.NewEncoder(writer).Encode(map[string]any{
			"data":  []imageItem{{Base64: base64.StdEncoding.EncodeToString(testImage(t, "png"))}},
			"error": map[string]string{"message": "upstream failed"},
		})
	})
	result, err := client.generate(context.Background(), testOptions(t))
	if err == nil || !strings.Contains(err.Error(), "upstream failed") || len(result.Paths) != 1 || calls.Load() != 1 {
		t.Fatal("返回图片时仍必须报告平台错误且不重试", result, err, calls.Load())
	}
	if _, err := os.Stat(result.Paths[0]); err != nil {
		t.Fatal("已收到的图片没有保留", err)
	}
}

func TestRequestCancellation(t *testing.T) {
	var count atomic.Int32
	client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
		count.Add(1)
		select {
		case <-request.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	})
	options := testOptions(t)
	options.Timeout = 0.05
	if _, err := client.generate(context.Background(), options); err == nil {
		t.Fatal("超时未报错")
	}
	if count.Load() > 1 {
		t.Fatal("超时后重复提交")
	}
}

func TestModelAliasesAndBadResponse(t *testing.T) {
	client, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, `{"data":[{"id":"custom-alias"},{"id":"gpt-image-2.6-sunburst"}]}`)
	})
	models, err := client.models(context.Background())
	if err != nil || len(models) != 2 || models[0] != "custom-alias" {
		t.Fatal(models, err)
	}
	bad, _ := mockClient(t, func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, `{"error":{"message":"no models"}}`)
	})
	if _, err := bad.models(context.Background()); err == nil {
		t.Fatal("错误模型响应未报错")
	}
}
