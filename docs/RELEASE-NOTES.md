# v1.0.0

## 中文

首个正式版本，提供 macOS / Windows 的命令行与桌面工具，直接通过静态链接
FFmpeg 库转换音频，无需安装或调用 ffmpeg/ffprobe 命令行程序。

### 功能

- DTS-HD MA 5.1/7.1 转 TrueHD，保留受支持的采样率、精度和声道标签，
  默认执行完整解码后的逐声道 PCM 一致性校验。
- 支持符合条件的 DTS-HD MA 转 E-AC-3；48 kHz 的 5.1(side)/7.1 使用
  布局对应的码率选项，默认分别为 960 / 1536 kb/s。
- 48 kHz 5.1(side) DTS-HD MA 转 AC-3，固定 640 kb/s。
- 普通 DTS 仅支持 48 kHz 5.1(side) 转 E-AC-3（固定 1024 kb/s）或
  AC-3（固定 640 kb/s）。有损输出严格检查解码、格式和采样长度，不宣称 PCM 相等。
- MKV 音轨清单与显式选轨，隔离非目标轨道的头部编码和探测；自动输出名包含
  全局轨道编号、语言和容器时间轴延迟。延迟仅写入名称，不调整音频。
- 中英文桌面界面、有界并行队列、取消与失败重试、紧凑音轨列表和按等级着色的纯文本日志。
- 同文件系统临时编码与验证后发布，默认保护已有输出；附完整源码、许可证及
  可替换 FFmpeg 静态库的 CLI/UI 重新链接材料。

### 系统与下载

| 平台 | 归档标识 | 最低支持系统 |
|---|---|---|
| macOS Apple Silicon | `darwin-arm64` | macOS 12.0 |
| macOS Intel | `darwin-amd64` | macOS 12.0 |
| Windows x64 | `windows-amd64` | Windows 10 22H2，build 19045 |
| Windows ARM64 | `windows-arm64` | Windows 11 24H2，build 26100 |

下载完整 `.tar.gz` / `.zip` 及对应 `.sha256`，解压前检查 SHA-256；
`SHA256SUMS` 汇总全部归档。Windows UI 需要匹配架构的 WebView2 Evergreen Runtime。
本版不提供发布者身份签名或 macOS 公证，首次启动可能显示系统安全提示。
安装与安全提示处理见
[下载与启动](https://github.com/ninghaofeng/hdma2truehd/blob/v1.0.0/docs/INSTALLATION.md)。

### 已知边界

- DTS:X/IMAX 仅转换兼容声道，不保留对象、对象元数据或高度渲染；不是 Atmos 编码器。
- 不自动升混、降混或重采样，不输出或重新封装 MKV。
- 原始 AC-3/E-AC-3 保留 256 个采样的编码延迟，以及对齐 1536 采样帧的尾部补齐。
- 声道标签、采样率或精度不受支持时拒绝转换，不静默改变音频几何。
- 无 Dolby/DTS 认证，不保证 Blu-ray 合规或所有硬件播放器兼容。
- 最低系统版本实机、Windows 桌面交互和硬件播放尚需验收；
  DTS:X 5.1 / IMAX 真实样本覆盖仍不完整。系统要求不是已完成实机验证的声明。
- 诊断日志在本机保存，可能含文件路径；提交问题前请检查并脱敏。

项目自身代码使用 MIT；FFmpeg、补丁和其他依赖保留各自许可证。
请分发完整归档，不要丢弃源码、许可证和重新链接材料。

## English

The first stable release provides native CLI and desktop applications for
macOS and Windows. Audio processing uses statically linked FFmpeg libraries;
no ffmpeg/ffprobe command-line installation or subprocess is required.

### Features

- DTS-HD MA 5.1/7.1 to TrueHD with supported rates, precision and speaker labels
  preserved; complete per-channel decoded PCM equality verification by default.
- Eligible DTS-HD MA to E-AC-3 at 48 kHz, with layout-specific 5.1(side)/7.1
  bitrate choices; defaults are 960 / 1536 kb/s respectively.
- 48 kHz 5.1(side) DTS-HD MA to AC-3 at a fixed 640 kb/s.
- Ordinary DTS supports only 48 kHz 5.1(side) to E-AC-3 at 1024 kb/s or AC-3
  at 640 kb/s. Lossy verification checks decoding, format and sample length,
  never PCM equality.
- MKV audio inventory and explicit selection, isolated non-target header/probe
  handling, and automatic names with global stream index, language and timeline
  delay. The recorded delay does not adjust audio samples.
- Chinese/English desktop UI, bounded parallel queues, cancellation and failed-job
  retry, compact track rows and severity-colored plain-text diagnostics.
- Same-filesystem staging and publication after verification, existing-output
  protection by default, and source/license/CLI+UI relinking materials.

### Systems and downloads

Targets and minimum systems are listed in the table above: macOS 12.0 for both
architectures, Windows 10 22H2/build 19045 for x64, and Windows 11 24H2/build
26100 for ARM64. Windows desktop UI requires matching WebView2 Evergreen Runtime.

Download the complete `.tar.gz`/`.zip` and `.sha256`; verify SHA-256 before
extraction. `SHA256SUMS` covers all archives. This release has no publisher
identity signing or macOS notarization, so OS prompts may appear.
See [download and launch instructions](https://github.com/ninghaofeng/hdma2truehd/blob/v1.0.0/docs/INSTALLATION.md).

### Known boundaries

- DTS:X/IMAX uses compatible channels only, without objects, object metadata
  preservation or height rendering. This is not an Atmos encoder.
- No automatic upmixing, downmixing, resampling, MKV output or remuxing.
- Raw AC-3/E-AC-3 retains 256 samples of encoder delay and padding to 1536-sample frames.
- Unsupported layouts, rates or precision are rejected, not silently changed.
- No Dolby/DTS certification, Blu-ray compliance or universal hardware playback guarantee.
- Minimum-OS devices, Windows desktop interactions and hardware playback still
  need acceptance testing; real DTS:X 5.1/IMAX fixture coverage is incomplete.
  Stated minimum requirements are not claims of completed device tests.
- Local diagnostic logs may contain paths; review and redact before sharing.

The project's own code is MIT licensed; FFmpeg, patches and other dependencies
retain their respective licenses. Distribute complete archives with source,
license and relinking materials.
