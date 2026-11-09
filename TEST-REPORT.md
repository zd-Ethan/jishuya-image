# 技术鸭生图 v2.1.0 验证报告

日期：2026-11-09（Asia/Shanghai）。本次统一为通用免安装包，图片 API 请求与解析代码未改变，没有新增真实生图调用。下面先列出 v2.1.0 本地验证，再保留 v2.0.x 的接口历史记录；旧版本的请求次数、覆盖率和 SHA-256 不代表当前包。

## v2.1.0 免安装改动与验证

- 删除安装、偏好写入及其嵌入资源逻辑。`init` 只保存用户配置并查询模型；环境变量可直接使用，不必初始化。旧 `install` / `uninstall` 命令返回迁移提示，`init --prefer` 明确拒绝。
- `doctor` 报告实际程序路径、平台、`mode: portable` 和 Key 来源，不检测安装状态或修改个人规则。原有用户配置目录与读取格式保持兼容。
- 一个 `jishuya-image-v2.1.0-universal.zip`，根目录为 `jishuya-image/`，白名单恰好 9 个文件，包含五个平台的程序和四个文档/元数据文件。发布工具使用 `CGO_ENABLED=0` 编译。
- `go test ./... -count=1 -json`：**71 个测试事件通过**（含子测试），两个包通过，0 失败。新增初始化仅保存配置、免安装命令无副作用、通用包白名单及文件权限测试；已移除旧安装器专属测试。
- `go vet ./...` 和技能结构校验通过。结构校验使用开发机工具，发行包不包含 Python，也不需要用户安装 Python。
- 发行包检查：SHA-256 一致；包内文档与源码相同；没有 Key 模式或无关品牌；程序为 Windows PE/x64、macOS Mach-O/x64 与 ARM64、Linux ELF/x64 与 ARM64；文档权限 0644、程序权限 0755。
- Windows 从 ZIP 解压到含中文与空格的路径后，版本查询及离线诊断通过；子进程 PATH 仅包含 Windows 系统目录。环境变量模式不创建用户目录或配置，缺 Key 时明确报错。
- 隔离目录中以旧版包模拟整体升级，Key 配置与用户文件保留；移出技能目录、移回、卸载移出后重新解压，均保留并可复用配置。此检查验证文件操作与程序行为，未模拟宿主刷新技能列表。
- 当前用户技能副本已整体替换为通用版，9 个文件与 ZIP 一致；`doctor --offline` 成功，读取现有配置，Key 配置和个人规则哈希未变。旧副本、旧源码程序和旧 ZIP 均移到发现目录之外归档。

当前 ZIP SHA-256：`02d9a6161ddcf65af07a88a30301a55d14119740eb404cdedb04c9e656c3d8d7`。

交付位置：`D:\project\jishuya-newapi\jishuya-image-validation\dist`；可直接使用的完整文件夹为同目录上一级的 `jishuya-image/`。`D:\XT\zm\jishuya-image.zip` 同步为通用包。检查证据：验证目录中的 `evidence/portable-go-tests.jsonl`、`evidence/portable-package-checks.json`、`evidence/portable-local-upgrade.json`；可重复执行的包检查脚本为 `evidence/check-portable.ps1`。

本轮未覆盖：macOS/Linux 实机运行、macOS 系统来源确认、新对话自动触发率、宿主实际停用刷新、真实删除用户配置。API 能力边界沿用下方实测结果，不宣称全功能均通过。v2.0.x 文件已归档到 `archive/pre-portable-20261109`，没有再次付费生图。

---

# 历史记录：v2.0.1 接口验证

日期：2026-11-09（Asia/Shanghai）。结论：GIF 动画和流式最终图片已验证；仍不能宣称全功能支持。失败项按实际文件与像素检查判定，不以 HTTP 200 或命令退出码代替功能验收。

## 本轮改动

- GIF 提示词明确文件格式、运动内容、帧数和时长；两个 2.6 模型均返回真正的多帧循环 GIF。程序新增完整解码和至少两帧检查，拒绝单帧或损坏的 GIF。
- 修复技术鸭 SSE 事件兼容：从 `object=image.generation.result` 的 `data` 数组保存最终图片；进度文本不作为图片。标准图片事件保持兼容。
- 流式错误即使没有 `type` 也会报错，并保留此前已保存图片。
- 新增 `warnings`：平台报告的尺寸/质量与请求不同、未返回中间预览图时，向调用方说明。元数据提示不代替实际像素检查。
- 运行端保持 Go 单文件，不新增 Python、Node 或 Go 运行时依赖。

## 本轮真实接口测试

使用原有本机配置，固定访问技术鸭端点。诊断阶段共 22 次生成/编辑请求，每个用例一次；修复流式解析后有一次明确的回归请求，没有自动重试。15 次客户端成功、7 次报错；另从响应中提取了 22 份文件证据，其中 2 份为 GIF、20 份为 PNG。成功退出的用例仍需按下表判断语义和参数是否满足。

| 能力 | 模型 / 用例 | 验证结果 |
|---|---|---|
| GIF 动画 | 2.6 Sunburst，明确 GIF 提示词 | 2 帧，每帧 200 ms，循环播放；接触表确认红球从左移到右 |
| GIF 动画 | 2.6 Flare，明确 GIF 提示词 | 4 帧，每帧 120 ms，循环播放，帧内容不同 |
| SSE 最终图 | 2.6 Flare | 首次捕获实际事件发现解析缺口；修复后真实请求保存最终 PNG |
| SSE 中间预览 | 2.6 Flare，partial_images=1 | 仅进度文本与最终图，没有中间图，未通过 |
| JPEG / WebP | 两款 2.6 模型各一例 | 明确提示词和格式参数后仍返回 PNG；未通过，压缩率无法验收 |
| 原生 SVG | 两款 2.6 模型各一例 | 明确要求完整 SVG XML 和纯矢量后仍返回 PNG；未通过 |
| 横图尺寸 | 2.6 Flare，1536×1024 | 提示词与参数共同指定，实际文件尺寸精确符合 |
| 默认方图尺寸 | 多个 1024×1024 请求 | 大多返回 1254×1254；GIF 也返回不同尺寸，不能保证尺寸遵循 |
| mask | 2.6 Flare，明确左上坐标 | 局部变绿，但 1,510,016 个受保护像素中有 1,107,706 个改变；严格保护未通过 |
| markup | 2.6 Flare，明确左上坐标 | 蓝星包围盒 (112,127)–(252,261)，落在指定 (30,30)–(280,280) 范围；只确认本例位置，不保证其余像素不变 |
| quality low / medium / high | 2.6 Flare，各一例 | 请求成功，响应质量标注分别一致；未做主观画质或量化效果比较 |
| quality xhigh / max | 2.6 Flare，各一例 | 请求成功但响应均标注 medium，指定高档位未通过 |
| complexity low / medium / high | 2.6 Flare，各一例 | 请求均成功，但响应不报告复杂度，不能确定实际生效 |
| moderation low | 2.6 Flare，普通鸭子图 | 请求成功；没有验证审核策略变化 |

上述结论只覆盖对应模型与用例。基础 PNG、常规编辑、两张参考图合成、两张输出、透明 PNG 沿用首轮成功记录；本轮没有重复这些已通过的付费测试。

## 本地回归

- `go test ./... -count=1`：72 个测试事件通过（含子测试），0 失败；全部包合计语句覆盖率 69.5%，发行工具未覆盖。
- `go vet ./...` 通过。
- 技能结构检查通过。开发校验器需以 UTF-8 运行，不随发行包提供，不构成用户端 Python 依赖。
- 新增回归覆盖实际 SSE 数组、多图保存、无 type 错误、已有结果保留、中间图缺失提醒、JSON/SSE 元数据差异提醒、单帧与损坏 GIF 拒绝。
- 真实测试入口为 `live_probe_test.go`，需显式 `-tags liveprobe` 与 `JISHUYA_LIVE_CASE`，普通测试不会调用付费接口。只保存脱敏响应结构与图片，不记录请求鉴权头。

## 证据与未覆盖范围

诊断证据现整理到 `D:\project\jishuya-newapi\jishuya-image-validation\evidence\capabilities-20261109`。每例 `.json` 为参数，`.result.json` 为脱敏响应及结果；对应目录保留实际文件。`gif-frames.png` 和 `mask-pixel-inspection.json` 分别记录动画帧和编辑范围检查。历史记录中的绝对路径是执行时路径；目录迁移不改写原始响应，现位置见验证目录的 `README.md`。

未覆盖：GIF 2–16 帧及时间范围的全部边界、其他尺寸和数量上限、16 张参考图上限、所有模型的所有参数组合、SVG 作者模型与推理参数、macOS/Linux 实机运行、新 Codex 对话完整触发率。SVG 主路径仍未通过，没有为不可验收的作者参数盲目增加付费调用。

首轮 v2.0.0 共 15 次真实请求的记录保留在验证目录的 `evidence/initial-validation`。该轮 GIF、SSE 和标注的失败结论已由本轮有条件成功的结果更新，不再作为当前版本的一概判断。

## 发行包与最终验证

- 五个平台构建完成；每包 5 个文件，校验和与 `dist/SHA256SUMS.txt` 一致，文本扫描没有密钥模式或无关品牌。macOS/Linux 仅交叉编译与包检查，未原生运行。
- Windows ZIP 隔离安装和当前用户升级均通过；安装副本的 5 个文件与 ZIP 相同。源码 `bin/jishuya-image.exe` 和 `D:\XT\zm\jishuya-image.zip` 已同步。
- 配置文件与个人规则哈希未变；仅保留 Windows 系统 PATH 时，程序版本和离线诊断仍成功，确认用户运行不依赖语言环境。
- 最终 ZIP 额外执行 1 次真实流式请求（Flare、xhigh、partial_images=1），27.36 秒成功保存最终图，同时提示实际尺寸 1254×1254、实际质量 medium、缺少中间图。此次请求使用当前交付二进制；本轮总计 **23 次**真实生成/编辑请求。
- 上述真实流式测试所用 Windows ZIP SHA-256：`2b11736ade123ba3aeaf6e81a6fd364d7366438a3099cb8992f26824a52050f7`。
- `package-checks.json`、`archives.json` 和 `final-package-stream.json` 保存最终验证证据。Flare 的 4 帧接触表确认圆球横向移动。

## 目录整理后的最终包

同日按用户要求整理验证目录并补充跨平台选包说明；没有修改 API 请求代码，也没有新增付费调用。最终包位于 `D:\project\jishuya-newapi\jishuya-image-validation\dist`，源码仍在 `D:\XT\zm\jishuya-image`。

- 重新生成五平台 v2.0.1 包，核对 SHA-256、文件清单、文档一致性和程序架构：Windows PE/x64、macOS Mach-O/x64 与 ARM64、Linux ELF/x64 与 ARM64。Unix 可执行权限元数据为 0755。
- Windows 升级安装、仅保留系统 PATH 的离线诊断通过；安装副本与源码分发文件一致，Key 配置和个人规则哈希未变。macOS/Linux 仍未实机运行。
- 当前 Windows ZIP SHA-256：`e51828b0c3b7d9d1a99057d0ed2a921e44ec602603814bb3bf98b4a8de61cd09`。完整清单见 `dist/SHA256SUMS.txt`。
- 目录内 `evidence/final-package-checks.json` 和 `evidence/windows-install-check.json` 保存本次检查结果；`archive/index.json` 记录旧文件位置。递归删除被执行策略拒绝，旧 Python、重复解压包和临时安装资料已移动至 `archive/obsolete`，没有实际删除。
