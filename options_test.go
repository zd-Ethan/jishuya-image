package main

import (
	"io"
	"testing"
)

// testOptions 为行为测试提供有效默认请求。
func testOptions(t *testing.T, args ...string) imageOptions {
	t.Helper()
	options, err := parseImageOptions("generate", append([]string{"--prompt", "小黄鸭"}, args...), defaultModel, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	options.OutputDir = t.TempDir()
	return options
}

func TestInvalidOptions(t *testing.T) {
	cases := [][]string{
		{"--prompt", " "}, {"--count", "0"}, {"--count", "11"}, {"--partial-images", "4"},
		{"--timeout", "NaN"}, {"--timeout", "Inf"}, {"--timeout", "0"}, {"--timeout", "90000"},
		{"--format", "jpeg", "--background", "transparent"}, {"--compression", "-1"}, {"--compression", "101"},
		{"--compression", "80"}, {"--format", "svg", "--count", "2"}, {"--format", "svg", "--stream"},
		{"--format", "gif", "--stream"}, {"--frame-count", "8"}, {"--format", "gif", "--frame-count", "0"},
		{"--format", "gif", "--frame-count", "17"}, {"--format", "gif", "--frame-duration-ms", "19"},
		{"--format", "gif", "--frame-duration-ms", "2001"}, {"--format", "svg", "--svg-reasoning", "high"},
		{"--svg-model", "author"}, {"--unknown", "x"}, {"unexpected"}, {"--format", "pdf"},
	}
	for _, args := range cases {
		t.Run(joinArgs(args), func(t *testing.T) {
			_, err := parseImageOptions("generate", append([]string{"--prompt", "duck"}, args...), defaultModel, io.Discard)
			if err == nil {
				t.Fatal("无效参数被接受")
			}
		})
	}
}

func joinArgs(args []string) string {
	name := ""
	for _, arg := range args {
		name += arg + "_"
	}
	return name
}

func TestEditValidation(t *testing.T) {
	for _, args := range [][]string{
		{"--prompt", "duck"},
		{"--prompt", "duck", "--image", "a.png", "--mask", "m.png", "--markup", "n.png"},
		{"--prompt", "duck", "--image", "a.png", "--mask", "m.png", "--format", "gif"},
	} {
		if _, err := parseImageOptions("edit", args, defaultModel, io.Discard); err == nil {
			t.Fatal("无效编辑被接受", args)
		}
	}
	options := testOptions(t)
	options.Command, options.Images = "edit", make(stringList, 17)
	if options.validate() == nil {
		t.Fatal("超出参考图数量未被拒绝")
	}
}

func TestOptionalFieldsAndAlias(t *testing.T) {
	options := testOptions(t, "--model", "provider-custom-model", "--quality", "max", "--complexity", "high", "--format", "webp", "--compression", "0", "--background", "transparent", "--partial-images", "2")
	fields := options.fields()
	if fields["model"] != "provider-custom-model" || fields["output_compression"] != 0 || fields["stream"] != true || fields["partial_images"] != 2 || fields["quality"] != "max" {
		t.Fatal(fields)
	}
	defaults := testOptions(t).fields()
	for _, name := range []string{"stream", "partial_images", "output_compression", "complexity", "background", "svg", "frame_count"} {
		if _, exists := defaults[name]; exists {
			t.Fatal("默认请求多传参数", name)
		}
	}
}
