# 技术鸭生图 Skill

通过技术鸭生成或编辑图片。统一采用免安装方式：解压一个文件夹即可使用，包内包含五个平台程序，AI 按执行环境选择。用户不需要安装 Python、Node.js 或 Go；源码构建才需要 Go。

## 下载与使用位置

当前发行版为 **v2.1.1**。所有受支持系统下载同一个 [通用免安装包](https://github.com/zd-Ethan/jishuya-image/raw/refs/heads/main/dist/jishuya-image-v2.1.1-universal.zip)，[SHA-256 校验清单](https://github.com/zd-Ethan/jishuya-image/raw/refs/heads/main/dist/SHA256SUMS.txt) 与包同目录。请解压并保留整个 `jishuya-image` 文件夹：

```text
jishuya-image/
  SKILL.md
  README.md
  THIRD-PARTY-NOTICES.txt
  agents/openai.yaml
  bin/
    jishuya-image-windows-amd64.exe
    jishuya-image-darwin-amd64
    jishuya-image-darwin-arm64
    jishuya-image-linux-amd64
    jishuya-image-linux-arm64
```

GitHub 的 **Code → Download ZIP** 和 `git clone` 获取的是源码仓库；仓库根目录的 `bin/` 不受版本控制。普通用户应下载上面的发行 ZIP。若通过技能管理器从仓库复制了 `SKILL.md`，仍需要放入完整发行包，才能获得可执行程序。

使用位置有两种，程序均直接运行：

1. **日常自动匹配**：把整个文件夹放到 Codex 用户技能目录 `~/.agents/skills/jishuya-image`，或当前项目的 `.agents/skills/jishuya-image`。Windows 用户路径一般为 `%USERPROFILE%\.agents\skills\jishuya-image`。打开新对话检查是否发现，必要时重启宿主；同名技能只保留一份。
2. **任意目录临时使用**：解压到任意位置，在对话中说明“读取 `完整路径/jishuya-image/SKILL.md`，使用其中程序生成图片”。该位置不会仅因解压而在所有新对话自动生效，也可以直接在终端运行程序。

已有版本位于 `.codex/skills/jishuya-image` 或自定义 `CODEX_HOME` 下时，可替换已被当前宿主发现的同一目录，不要再放一份重复技能。其他 Agent 将整个文件夹放入其支持的技能目录，或明确提供 `SKILL.md` 路径；发现机制以对应宿主为准。

包内每个程序只适用于对应系统和架构；`SKILL.md` 内含 AI 选择表。`darwin` 表示 macOS，`amd64` 表示 Intel/AMD x64，`arm64` 表示 ARM64。macOS/Linux 用 `uname -s` 和 `uname -m` 判断：`x86_64` 选 amd64，`arm64`/`aarch64` 选 arm64。Windows 可查询 `[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture`。WSL 应选 Linux；本包不含 Windows ARM64、32 位或其他平台原生程序。

Windows 已本机运行验证；macOS/Linux 完成交叉编译及程序格式检查，尚未实机运行。macOS 包未经开发者签名/公证，可能需要用户在系统安全设置中确认来源。Unix 解压工具未保留可执行权限时，对所选程序执行 `chmod +x`。

## 首次配置 Key

已有本机配置会继续使用，无需重新输入。新用户可在可见终端运行对应程序的 `init`。在解压目录的上一级打开 Windows PowerShell：

```powershell
& '.\jishuya-image\bin\jishuya-image-windows-amd64.exe' init
```

macOS Apple Silicon 示例；Intel Mac 或 Linux 按上述规则替换程序名：

```sh
chmod +x ./jishuya-image/bin/jishuya-image-darwin-arm64
./jishuya-image/bin/jishuya-image-darwin-arm64 init
```

`init` 在用户可见终端隐藏输入 Key，检查模型列表后保存配置；不会生成图片。默认模型为已完成基础生图实测的 `gpt-image-2`。可用 `init --model 模型名称` 保存其他默认模型，或在单次命令中用 `--model gpt-image-2.6-sunburst` / `--model gpt-image-2.6-flare` 显式选择 2.6。账号需具备对应图片模型权限。

也可以为实际运行程序的宿主进程设置环境变量 `JISHUYA_IMAGE_API_KEY`，直接使用，无需 `init`，不会将 Key 保存到配置文件。需要持久保存时再运行 `init --from-env`。不要将 Key 放入命令参数、聊天、源码或分发包。

`init` 只负责保存 Key 与默认模型，不复制技能或修改个人规则。v2.1.0 已取消 `install`、`uninstall` 和 `init --prefer`，旧命令会返回迁移提示或参数错误。

## 升级、停用与卸载

| 操作 | 步骤 | Key 与图片 |
|---|---|---|
| 升级 | 等待正在运行的任务结束；将旧文件夹移到技能发现目录之外备份，再把新版完整文件夹放回原位置，打开新对话 | 用户配置在技能目录外，自动复用；保留旧目录内自己的图片或其他文件 |
| 停用 | 将整个技能文件夹移出所有技能发现目录，再开启新对话；恢复时移回 | 配置和图片保留 |
| 卸载 | 停止使用后移除技能文件夹，开启新对话 | 默认保留用户配置和图片，日后重新解压可继续使用 |
| 彻底清理 | 卸载后按下文位置删除 `jishuya-image` 用户配置目录（含旧版 `backups/`），清除自行设置的 Key 环境变量及旧偏好块 | 图片由用户自行决定是否删除；删除配置后需要重新配置 Key |

备份目录应放在 `.agents/skills`、`.codex/skills` 等技能发现目录之外，仅在原目录改名可能仍会被扫描。升级时不要只覆盖 `SKILL.md`，应整体更换文档和五个平台程序；本项目不自动下载更新。

**从 v2.0.x 迁移**：按升级步骤整体替换，不再运行旧版安装命令。原有 `config.json` 不变。若旧版曾启用默认偏好，程序不会自行修改它；停用或卸载时，在当时使用的个人 `AGENTS.md` 或 `AGENTS.override.md` 中仅移除 `<!-- jishuya-image:begin -->` 到 `<!-- jishuya-image:end -->` 的整段管理块，保留其他规则。继续使用时也可保留该偏好。新版本首次使用不会写入这个管理块。

## 配置与输出

读取顺序：`JISHUYA_IMAGE_API_KEY` → 本机配置。位置：

- Windows：`%AppData%\jishuya-image\config.json`。
- macOS：`~/Library/Application Support/jishuya-image/config.json`。
- Linux：`$XDG_CONFIG_HOME/jishuya-image/config.json`，未设置时为 `~/.config/jishuya-image/config.json`。

环境变量只覆盖 Key；配置文件可读且有效时，仍沿用其中的默认模型。配置不可读、格式损坏或系统没有用户配置目录时，有环境变量 Key 即可直接运行，模型回退为 `gpt-image-2`，可用 `--model` 指定。此时 `doctor --offline` 的 `config_path` 可能为空；保存配置的 `init` 仍需要有效的用户配置目录。

配置以本机权限保护，非加密存储。Windows 限制为当前用户与 SYSTEM；macOS/Linux 目录为 0700、文件为 0600。更新技能不会删除配置。程序只报告 Key 是否存在，不回显 Key。旧版备份可能位于同一配置目录的 `backups/`；新版本不会再创建安装或规则备份。移动文件夹到另一台电脑不会带走本机配置，需要在目标电脑配置 Key。

受限沙箱可能以其他 Windows 用户身份运行，或禁用外网访问。宿主需要允许本次程序读取配置、访问技术鸭、写入输出目录；这些权限不能由 skill 自动授予。若已有运行环境的 Key 变量，程序优先使用，即使配置文件不可读也可运行。权限被拒绝时按宿主正常审批流程处理，禁止审批的环境可在本机终端执行；不建议关闭沙箱或放宽密钥文件权限。

图片默认写入调用工作目录的 `jishuya-output/`，可用 `--output-dir` 指定。stdout 返回 JSON，成功含 `model` 和绝对 `paths`，可选 `partial_paths` 和 `warnings`；平台报告尺寸或质量与请求不同、未提供中间预览图时会提示。尺寸/质量提示基于响应元数据，不能代替实际图片检查。失败退出码 1，返回 `error` 和 `result`，保留已保存路径。帮助和交互提示写入 stderr。

## 使用

放入技能目录并被宿主发现后，可说“生成一张咖啡店海报”“给这张产品图换背景”，也可显式使用 `$jishuya-image`。技能保留常见图片需求描述与 `allow_implicit_invocation: true`，允许自动匹配；实际选择仍取决于模型、已有技能和更高优先级指令，不承诺每次必然调用。

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

`models` 返回全部可见模型，保留服务商别名。`doctor` 报告程序绝对路径、平台、`mode: portable`、配置来源并检查模型访问，不生成图片；`--offline` 跳过联网，不创建配置或复制文件。无 Key 或默认模型不可见时退出非零。Key 失效后重新执行 `init` 或更新宿主环境变量。

参数范围见 `SKILL.md`。PNG/JPEG 输出检查签名和可解码头部，GIF 完整解码并要求至少两帧，WebP 检查签名，SVG 检查完整 XML 及根元素。输入图检查签名，尺寸和透明度由端点继续验证。

## 接口兼容性

请求固定发送到 `https://www.jishuya.net/v1`；生成使用 JSON，编辑使用 multipart，多参考图为 `image[]`，SVG 作者使用嵌套字段。支持 Base64 或 HTTPS 下载；仅同源下载携带鉴权，所有请求禁用重定向。失败、超时和流中断不自动重试。

客户端提供 GIF、SVG、标注图和 SSE 等参数入口；客户端能组装请求不等于服务端完整实现。以下为 2026-11-09 v2.0.x 的真实接口实测结果；v2.1.0 调整免安装流程，v2.1.1 修复配置、错误处理与打包边界，本轮未重复付费调用：

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

单个 JSON/下载响应及单行 SSE 事件限制 256 MiB；超时默认 600 秒，可通过 `--timeout` 调整。SSE 兼容标准图片事件和技术鸭 `image.generation.result` 的 `data` 数组，忽略不含图片的进度事件；随图片事件保存文件，命令结束时输出路径，宿主没有实时预览。普通 JSON 同时包含图片与 `error` 时，保留成功保存的图片并返回失败；按 `result.paths` 获取已有结果，不自动重试。

## 常见问题

| 现象 | 检查与处理 |
|---|---|
| AI 没有自动调用 | 确认完整文件夹在宿主的技能发现目录，只保留一份同名技能，打开新对话；也可显式使用 `$jishuya-image` 或提供 `SKILL.md` 路径 |
| 找不到程序 | 确认解压的是发行 ZIP，且 `bin/` 包含五个程序；按执行环境选择，不使用旧 `jishuya-image.exe` 路径 |
| macOS/Linux 报执行格式错误 | 用 `uname -s`、`uname -m` 核对系统和架构；WSL 使用 Linux 程序 |
| macOS/Linux 无执行权限 | 对选定的程序执行 `chmod +x`；系统来源拦截按系统提示由用户确认 |
| 尚未配置 Key | 运行 `doctor --offline` 检查来源；在可见终端运行 `init`，或向实际宿主进程设置环境变量 |
| 配置文件损坏或无法读取 | 仍可使用 `generate --help` / `edit --help`；用 `init` 重建配置，或使用环境变量 Key；权限限制按宿主流程处理 |
| 模型不可见、HTTP 401/403 | 核对 Key 是否有效及模型权限，用 `models` 查询，选择账号实际可用的模型 |
| 超时、HTTP 429/5xx、流中断 | 先核对控制台任务与额度、返回的已有路径；确认结果后再决定是否重试 |
| 请求 GIF/JPEG/WebP/SVG 却报格式不符 | 检查能力表；GIF 提示词明确要求动画，其他未通过格式不能按扩展名伪装成功 |

`doctor --offline` 只检查本地配置；`doctor` / `models` 查询模型并有 30 秒超时。只有 `generate` / `edit` 提交图片请求；命令成功不代表图片语义、尺寸或所有选项都符合要求，仍需检查实际文件与 `warnings`。

## 开发与分发

本机唯一维护目录为 `D:\project\jishuya-newapi\jishuya-image-validation`，根目录包含 `.git` 和完整源码。`dist/` 存放最终通用包；本机的 `jishuya-image/` 是已解压的使用副本，`evidence/` 保存验证资料，`archive/` 保存可恢复的旧文件。这三个本机资料目录不提交 Git，也不进入发行包。历史报告位于源码的 `docs/TEST-HISTORY.md`，其中旧绝对路径是执行时记录。

源码需要 Go 1.26 或更新版本。主要文件职责：

| 路径 | 职责 |
|---|---|
| `SKILL.md`、`agents/openai.yaml` | 图片需求触发描述、平台选择与执行指令 |
| `main.go`、`config.go` | 命令入口、诊断和用户配置 |
| `options.go` | 图片参数校验与请求字段映射 |
| `api.go` | HTTP、编辑上传、流式解析及图片保存 |
| `permissions_*.go` | Windows ACL 与 Unix 文件权限 |
| `*_test.go` | 默认运行的本地测试；网络测试使用本地模拟服务 |
| `live_probe_test.go` | 需显式开启的真实接口验证，会使用技术鸭额度 |
| `tests/trigger-cases.json` | 人工触发评估样例，不是自动触发率测试结果 |
| `tools/release/` | 五平台编译、通用包白名单与归档测试 |
| `TEST-REPORT.md` | 当前复核结论和历史接口验证范围 |

在仓库根目录运行：

```sh
go test ./...
go vet ./...
go run ./tools/release
```

发行工具使用 `CGO_ENABLED=0` 编译。仅源码构建需要 Go；终端隐藏输入和 Windows 权限依赖的 Go 模块编译进程序。

发布工具生成 `dist/jishuya-image-v2.1.1-universal.zip` 和 `dist/SHA256SUMS.txt`，完成临时 ZIP 后才替换目标，文件缺失等打包失败不会覆盖已有包。ZIP 白名单共 9 个文件：4 个文档/元数据文件和 5 个程序，不包含 Key、用户配置、源码或测试日志。五个平台的选择路径与 `SKILL.md` 一致。发布时应一并更新程序版本、发行工具默认版本和本文下载链接，重新生成 ZIP 与校验清单；`dist/` 当前版本之外的旧包另行归档。

v2.1.1 修复：损坏配置不再阻止图片命令帮助；环境变量 Key 不再依赖用户配置目录；普通 JSON 的部分成功结果保留图片并报告错误；打包失败保留旧 ZIP。完整测试与未覆盖范围见 `TEST-REPORT.md`，不把本地模拟测试当作真实端点验收。

Codex 技能发现与隐式调用说明：[官方文档](https://developers.openai.com/codex/skills/)。

技术鸭网站：`https://www.jishuya.net`。教程：`https://docs.jishuya.net/`。
