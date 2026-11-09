# 技术鸭生图 Skill

通过技术鸭生成或编辑图片。发行包自带 Go 可执行程序，用户不需要安装 Python、Node.js 或 Go。源码构建才需要 Go。

## 安装与首次配置

当前发行版为 **v2.0.1**。请下载完整 ZIP，不要只复制 `SKILL.md`：

| 系统 | 下载 |
|---|---|
| Windows x64 | [jishuya-image-v2.0.1-windows-amd64.zip](https://github.com/zd-Ethan/jishuya-image/raw/refs/heads/main/dist/jishuya-image-v2.0.1-windows-amd64.zip) |
| macOS Intel | [jishuya-image-v2.0.1-darwin-amd64.zip](https://github.com/zd-Ethan/jishuya-image/raw/refs/heads/main/dist/jishuya-image-v2.0.1-darwin-amd64.zip) |
| macOS Apple Silicon | [jishuya-image-v2.0.1-darwin-arm64.zip](https://github.com/zd-Ethan/jishuya-image/raw/refs/heads/main/dist/jishuya-image-v2.0.1-darwin-arm64.zip) |
| Linux x64 | [jishuya-image-v2.0.1-linux-amd64.zip](https://github.com/zd-Ethan/jishuya-image/raw/refs/heads/main/dist/jishuya-image-v2.0.1-linux-amd64.zip) |
| Linux ARM64 | [jishuya-image-v2.0.1-linux-arm64.zip](https://github.com/zd-Ethan/jishuya-image/raw/refs/heads/main/dist/jishuya-image-v2.0.1-linux-arm64.zip) |

每个包只带对应系统和 CPU 架构的程序。Windows 的 `jishuya-image.exe` 不能直接在 macOS/Linux 运行；这两个系统请使用对应包内的 `bin/jishuya-image`。`darwin` 表示 macOS，`amd64` 表示 Intel/AMD x64，`arm64` 适用于 Apple Silicon 或 ARM64 Linux。macOS/Linux 可用 `uname -m` 查看架构：`x86_64` 选 amd64，`arm64`/`aarch64` 选 arm64。五个平台均不需要额外安装 Python、Node.js 或 Go；macOS/Linux 当前完成交叉编译和包检查，尚未实机验证。

下载对应系统和架构的压缩包并解压，在终端运行。Windows PowerShell：

```powershell
& '.\jishuya-image\bin\jishuya-image.exe' install --codex --prefer
& '.\jishuya-image\bin\jishuya-image.exe' init
```

macOS/Linux：

```sh
chmod +x ./jishuya-image/bin/jishuya-image
./jishuya-image/bin/jishuya-image install --codex --prefer
./jishuya-image/bin/jishuya-image init
```

`init` 在用户可见终端隐藏输入 Key，检查模型列表后保存配置；不会生成图片。默认模型为已完成基础生图实测的 `gpt-image-2`。可用 `init --model 模型名称` 保存其他默认模型，或在单次命令中用 `--model gpt-image-2.6-sunburst` / `--model gpt-image-2.6-flare` 显式选择 2.6。账号需具备对应图片模型权限。

已配置环境变量 `JISHUYA_IMAGE_API_KEY` 时可用 `init --from-env` 保存到本机。不要将 Key 放入命令参数、聊天、源码或分发包。

`install --codex --prefer` 启用生图默认偏好，向个人 Codex 规则追加可移除的管理块，保留原有规则并在修改前备份。无 `--prefer` 时仅安装；`install --codex --no-prefer` 取消管理块。安装和初始化是用户执行的入口，复制 skill 文件本身不会自动运行初始化程序。

安装程序优先升级现有同名技能；全新安装使用用户的 `.agents/skills/jishuya-image`。设置 `CODEX_HOME` 时使用其 `skills/jishuya-image`，已有 `.codex/skills/jishuya-image` 也原地升级。发现两份同名技能时报告冲突。安装后在 Codex 新对话检查技能是否已发现；未发现时重启 Codex。

## 配置与输出

读取顺序：`JISHUYA_IMAGE_API_KEY` → 本机配置。位置：

- Windows：`%AppData%\jishuya-image\config.json`。
- macOS：`~/Library/Application Support/jishuya-image/config.json`。
- Linux：`$XDG_CONFIG_HOME/jishuya-image/config.json`，未设置时为 `~/.config/jishuya-image/config.json`。

配置以本机权限保护，非加密存储。Windows 限制为当前用户与 SYSTEM；macOS/Linux 目录为 0700、文件为 0600。更新技能不会删除配置。程序只报告 Key 是否存在，不回显 Key。备份位于同一配置目录的 `backups/`。

受限沙箱可能以其他 Windows 用户身份运行，或禁用外网访问。宿主需要允许本次程序读取配置、访问技术鸭、写入输出目录；这些权限不能由 skill 自动授予。若已有运行环境的 Key 变量，程序优先使用，即使配置文件不可读也可运行。权限被拒绝时按宿主正常审批流程处理，禁止审批的环境可在本机终端执行；不建议关闭沙箱或放宽密钥文件权限。

图片默认写入调用工作目录的 `jishuya-output/`，可用 `--output-dir` 指定。stdout 返回 JSON，成功含 `model` 和绝对 `paths`，可选 `partial_paths` 和 `warnings`；平台报告尺寸或质量与请求不同、未提供中间预览图时会提示。尺寸/质量提示基于响应元数据，不能代替实际图片检查。失败退出码 1，返回 `error` 和 `result`，保留已保存路径。帮助和交互提示写入 stderr。

## 使用

安装后可说“生成一张咖啡店海报”“给这张产品图换背景”，也可显式使用 `$jishuya-image`。自动匹配仍取决于 Codex 模型、已有技能和更高优先级指令；默认偏好用于提高匹配机会，不承诺固定触发率。

以下 `PROGRAM` 表示程序完整路径：

```text
PROGRAM doctor
PROGRAM doctor --offline
PROGRAM models
PROGRAM generate --prompt "一只坐在窗边的小黄鸭"
PROGRAM edit --image input.png --prompt "换成浅蓝色背景"
PROGRAM edit --image first.png --image second.png --prompt "结合两张参考图"
PROGRAM edit --image input.png --mask mask.png --prompt "修改遮罩透明区域"
PROGRAM edit --image input.png --markup notes.png --prompt "按标注修改"
PROGRAM generate --prompt "透明背景的小黄鸭" --background transparent
PROGRAM generate --prompt "小黄鸭" --model gpt-image-2.6-flare --count 2
PROGRAM generate --model gpt-image-2.6-sunburst --prompt "生成真正的 GIF 动画：红球左右移动，2 帧循环，每帧 200 毫秒，输出 GIF 文件" --format gif --frame-count 2 --frame-duration-ms 200
PROGRAM generate --model gpt-image-2.6-flare --prompt "PNG 横向风景图，精确 1536×1024 像素，3:2 画幅" --size 1536x1024
PROGRAM generate --prompt "一只小黄鸭" --model gpt-image-2.6-flare --stream
PROGRAM generate --help
PROGRAM version
```

`models` 返回全部可见模型，保留服务商别名。`doctor` 检查配置、安装与模型访问，不生成图片；`--offline` 跳过联网。无 Key 或默认模型不可见时退出非零。Key 失效后重新执行 `init`。

参数范围见 `SKILL.md`。PNG/JPEG 输出检查签名和可解码头部，GIF 完整解码并要求至少两帧，WebP 检查签名，SVG 检查完整 XML 及根元素。输入图检查签名，尺寸和透明度由端点继续验证。

## 接口兼容性

请求固定发送到 `https://www.jishuya.net/v1`；生成使用 JSON，编辑使用 multipart，多参考图为 `image[]`，SVG 作者使用嵌套字段。支持 Base64 或 HTTPS 下载；仅同源下载携带鉴权，所有请求禁用重定向。失败、超时和流中断不自动重试。

客户端提供 GIF、SVG、标注图和 SSE 等参数入口；客户端能组装请求不等于服务端完整实现。2026-11-09 对最终包的代表性实测结果如下：

| 能力 | 实测模型 | 结果 |
|---|---|---|
| PNG 文生图 | gpt-image-2、2.6-sunburst、2.6-flare | 均返回可解码 PNG |
| 普通参考图编辑 | gpt-image-2、2.6-sunburst | 换背景成功 |
| 两张参考图合成 | 2.6-sunburst | 成功返回左右排列的两只鸭子 |
| 一次两张 | 2.6-flare | 收到两张 PNG |
| 透明 PNG | 2.6-flare | 检查到真实透明像素 |
| GIF 动画 | 2.6-sunburst、2.6-flare | 提示词明确要求 GIF 后，分别返回 2 帧/200 ms、4 帧/120 ms 的循环动画，帧间有运动 |
| 精确输出尺寸 | 2.6-flare | 明确提示词的 1536×1024 用例成功；默认 1024×1024 多次返回 1254×1254，仍须逐图检查 |
| 原生 SVG | 2.6-sunburst、2.6-flare | 明确要求 SVG XML 后仍返回 PNG，未通过 |
| JPEG / WebP | 2.6-sunburst、2.6-flare | 明确指定格式后仍返回 PNG，压缩率未获验证 |
| SSE 最终图片 | 2.6-flare | v2.0.1 修复结果事件解析，真实请求已成功保存最终图 |
| SSE 中间预览 | 2.6-flare | 请求 1 张中间图但未返回；进度文本不能算图片预览 |
| mask | 2.6-flare | 明确位置后仍修改了遮罩外像素，严格保护未通过 |
| markup | 2.6-flare | 提示词补充位置与坐标后，蓝星进入指定区域；不保证其他像素不变 |
| low / medium / high 质量 | 2.6-flare | 请求成功，响应质量标注一致；不等同于画质提升的量化评估 |
| xhigh / max 质量 | 2.6-flare | 请求成功但响应质量标注为 medium，未按指定档位通过 |
| complexity low / medium / high | 2.6-flare | 请求成功；响应未报告复杂度，不能确认参数的实际效果 |
| moderation low | 2.6-flare | 普通内容请求成功；未验证审核策略变化 |

表中 `2.6-*` 的完整模型名以 `gpt-image-2.6-` 开头。GIF 提示词应明确文件格式、运动内容、帧数和时长；仅传 `--format gif` 的旧用例曾返回 PNG。对精确尺寸和标注位置，同样在提示词说明要求并核对结果。不能据此宣称全功能或所有参数组合均已验证；默认采用 PNG、非流式调用。失败选项不自动重复提交、降级或换模型，也不通过本地格式转换或缩放冒充端点原生支持。完整请求记录与验证范围见源码目录的 `TEST-REPORT.md`。

单个 JSON/下载响应及单行 SSE 事件限制 256 MiB；超时默认 600 秒，可通过 `--timeout` 调整。SSE 兼容标准图片事件和技术鸭 `image.generation.result` 的 `data` 数组，忽略不含图片的进度事件；随图片事件保存文件，命令结束时输出路径，宿主没有实时预览。

## 开发与分发

```sh
go test ./...
go vet ./...
go build -trimpath -o bin/jishuya-image .
go run ./tools/release
```

Windows 构建输出改为 `bin/jishuya-image.exe`。正式版本使用 `CGO_ENABLED=0` 编译。仅源码构建需要 Go；终端隐藏输入和 Windows 权限依赖的 Go 模块编译进程序。

发布工具生成 Windows x64、macOS x64/ARM64、Linux x64/ARM64 压缩包及 SHA-256 清单。每包只含 `SKILL.md`、`README.md`、`agents/openai.yaml`、依赖许可说明和本平台程序。macOS 包未签名/公证；是否完成各平台运行测试见测试报告，不把交叉编译当作实机验证。

技术鸭网站：`https://www.jishuya.net`。教程：`https://docs.jishuya.net/`。
