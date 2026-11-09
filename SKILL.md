---
name: jishuya-image
description: 生成或编辑图片。适用于画图、生图、海报、封面、插画、头像、壁纸、产品图、参考图修改、换背景、改配色、局部重绘及连续修图；用户无需提及技术鸭或 jishuya。通过技术鸭图片 API 执行。仅分析图片、撰写提示词、搜索已有图片或修改网页/CSS/现有矢量代码时不触发；用户明确指定其他图片服务时遵循其选择。
---

# 技术鸭生图

使用本技能的 Go 程序完成图片生成和编辑。整个文件夹解压即用，用户端无需 Python、Node.js 或 Go，也无需执行安装命令。运行时使用程序绝对路径；不要假设已加入 PATH。

## 选择程序

通用包的 `bin/` 包含全部五个平台程序。首次调用时根据**实际执行命令的环境**选择系统与 CPU 架构；WSL 使用 Linux 程序。macOS/Linux 可用 `uname -s` 与 `uname -m` 判断；`x86_64` 对应 amd64，`arm64`/`aarch64` 对应 arm64。

| 执行环境 | 相对本技能目录的程序路径 |
|---|---|
| Windows Intel/AMD x64 | `bin/jishuya-image-windows-amd64.exe` |
| macOS Intel | `bin/jishuya-image-darwin-amd64` |
| macOS Apple Silicon | `bin/jishuya-image-darwin-arm64` |
| Linux x64 | `bin/jishuya-image-linux-amd64` |
| Linux ARM64 | `bin/jishuya-image-linux-arm64` |

Windows 可查询 `[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture`。未列出的系统/架构说明缺少对应程序，不猜测兼容性。PowerShell 使用 `& "程序绝对路径"`；macOS/Linux 若解压未保留执行权限，对所选程序运行 `chmod +x "程序绝对路径"`。macOS 包未经开发者签名/公证，系统若拦截，应告知用户按系统提示确认来源，不自动关闭系统保护。

## 执行流程

1. 首次使用或出现配置错误时运行 `doctor`；它只查询模型，不生图。Key 已配置且请求明确时直接执行，无需每张图再次确认。Key 缺失时给出用户在本机终端运行的完整 `init` 命令，也可由用户为宿主进程设置 `JISHUYA_IMAGE_API_KEY`；不要让用户在对话里粘贴密钥，也不要在不可交互的后台等待输入。已有配置或环境变量可直接使用，`init` 是可选的配置保存步骤。
2. 明确的新图片需求使用 `generate`；参考图修改或“把刚才那张……”使用 `edit`，传入用户提供或前文已生成的本地图片绝对路径。任务缺少实际参考图时先确定图片位置，不能捏造路径。
3. 按需求选择参数；没有明确数量时生成一张。未指定模型时使用初始化保存的模型，`models` 可查可见模型。不要擅自切换模型重试。
4. 保持当前请求运行直至完成；宿主返回运行会话时继续等待同一个进程。成功后以 `paths` 的实际文件为准展示图片并给出位置，同时告知 `warnings` 中未满足的要求；失败时检查返回的 `result.paths` / `result.partial_paths`，说明已有结果和错误。

Key 优先读取 `JISHUYA_IMAGE_API_KEY`，其次读取程序管理的用户配置文件。不要读取、显示或复制配置文件内的 Key。API 固定为 `https://www.jishuya.net/v1`。

宿主沙箱必须允许程序读取本机配置、写入指定输出目录并访问技术鸭。遇到权限拒绝时，按宿主提供的审批方式申请这次命令所需权限；禁止审批的环境应说明阻塞并给出本机终端命令。不要关闭沙箱、放宽 Key 文件 ACL 或自行复制密钥来绕过限制。用户已为宿主进程配置环境变量时，程序可直接使用它，即使本机配置文件不可读。

## 命令示例

下列 `PROGRAM` 替换为当前系统程序的绝对路径；PowerShell 调用带空格路径时使用 `& "完整路径"`。

```text
PROGRAM generate --prompt "一只坐在窗边的小黄鸭" --output-dir "项目目录/output/imagegen"
PROGRAM edit --image "原图绝对路径.png" --prompt "保持主体不变，将背景改为蓝色" --output-dir "项目目录/output/imagegen"
PROGRAM edit --image "参考图一.png" --image "参考图二.png" --prompt "参考第一张构图和第二张配色"
PROGRAM generate --prompt "透明背景的产品图" --background transparent
PROGRAM generate --model gpt-image-2.6-sunburst --prompt "生成真正的 GIF 动画：红球左右移动，2 帧循环，每帧 200 毫秒，输出 GIF 文件" --format gif --frame-count 2 --frame-duration-ms 200
PROGRAM generate --help
PROGRAM edit --help
```

普通图片默认使用 PNG、`1024x1024` 和 `auto` 质量；根据需求调整尺寸、质量和比例。用户要求具体尺寸时，同时在提示词中说明尺寸和宽高比，完成后核对实际图片。`--output-dir` 优先使用当前任务的输出目录；省略时写入当前工作目录的 `jishuya-output/`。默认超时 600 秒。

## 进阶参数与平台能力

2026-11-09 实测：`gpt-image-2`、`gpt-image-2.6-sunburst`、`gpt-image-2.6-flare` 均可返回 PNG；参考图编辑、多参考图、两张输出和透明 PNG 有成功案例。2.6 Sunburst/Flare 在提示词明确要求 GIF 动画后，分别通过 2 帧/200 毫秒和 4 帧/120 毫秒的循环动画验证。Flare 流式最终图片已通过，客户端兼容技术鸭的结果事件。以下限制来自端点实测，不代表模型本身的能力：

- 默认请求 `1024x1024` 多次返回 `1254x1254`；提示词与参数同时指定 `1536x1024` 有成功案例，仍须检查实际尺寸。
- JPEG、WebP、原生 SVG 尚未通过；明确提示词后仍有返回 PNG 的情况，不能只按扩展名判断成功。
- 流式最终图可用，但请求的中间预览图未返回；客户端会通过 `warnings` 提醒，不能宣称有实时图片预览。
- Flare 的 `low/medium/high` 请求成功且响应质量标注一致；`xhigh/max` 返回标注为 `medium`，不能宣称已按更高档生成。客户端会提示平台报告的尺寸、质量差异。
- mask 仍会修改遮罩外像素。markup 在提示词补充左上位置与坐标后有成功案例，但不能保证严格区域约束或其他像素不变。

用户需要上述未通过能力时先说明当前实测限制；不要将错误格式、错误区域或仅有成功退出码的结果宣称为满足需求。详细的模型与用例对应关系见 [README.md](README.md)。参数入口仍保留，供端点修复后验证。

- 通用参数：`--model`、`--size`、`--quality`、`--complexity`、`--count`、`--format`、`--background`、`--compression`、`--moderation`。质量可传 `auto/low/medium/high/xhigh/max`，输出可传 `png/jpeg/webp/gif/svg`；这只是客户端传参范围，必须以技术鸭端点实际支持为准。
- 编辑：重复 `--image` 最多 16 张；`--mask` 与 `--markup` 互斥且必须为 PNG。尺寸须匹配首张参考图；标注图应为独立透明层。程序检查输入文件签名，不代替端点检查尺寸和透明度。
- GIF：除 `--format gif` 外，提示词必须明确“生成 GIF 动画”、运动内容、帧数和单帧时长；仅传格式参数曾返回 PNG。`--frame-count 2–16`、`--frame-duration-ms 20–2000`；不能与 mask、流式或压缩率同用。程序拒绝损坏或单帧 GIF，动画效果及指定帧数/时长仍需核对。
- SVG：一次一张，可加 `--svg-model` 和 `--svg-reasoning`，后者需要作者模型；不能与流式或压缩率同用。SVG 编辑、透明 SVG、额外质量档等需要端点实测，不能承诺可用。
- SSE：`--stream` 或 `--partial-images 1–3`。程序随事件保存已收到的图片，结束后通过 JSON 返回路径；对话中不提供实时预览。流中断时保留已保存文件。

生成和编辑会消耗技术鸭额度。请求失败、超时或流中断后不要自动重复提交；先告知用户并核对平台记录。`models` 成功不等于生图功能验证成功。首次使用、配置、升级、停用和卸载见 [README.md](README.md)。程序不复制技能、不改写个人规则、不自动更新。
