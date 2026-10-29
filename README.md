# hdma2truehd

[简体中文](#简体中文) | [English](#english)

## 简体中文

将 **DTS-HD MA 音频（含 DTS:X 兼容声道）转换为 Dolby TrueHD `.thd` 或 E-AC-3 `.eac3`** 的实验性 Go 命令行及桌面工具，
面向 macOS 和 Windows。通过 **cgo 静态链接 FFmpeg 库**，探测、解码、编码及
PCM 校验均在进程内完成；运行时不启动 `ffmpeg` / `ffprobe`，也不需要安装它们。

默认仍只输出 TrueHD；可选择 E-AC-3 或同时输出两种格式。
TrueHD 支持 5.1、7.1，默认完整比较逐声道 PCM SHA-256 和精确采样数；
不自动降混、重采样或降低有效精度。
E-AC-3 是有损编码，目前支持 48 kHz 的 5.1(side) 和真实 7.1；
校验完整解码、声道布局和精确的含延迟/补零长度，不声称 PCM 无损一致。

DTS:X / DTS:X IMAX 仅解码兼容的 5.1 或 7.1 声道，保持原布局；
不执行上混、对象渲染，也不保留对象元数据或生成 Atmos。
PCM 一致性校验只覆盖解码后的兼容声道，不表示完整保留 DTS:X 对象场景。
部分 DTS:X 文件可能仅被探测为 `DTS-HD MA`，该标签不保证不存在对象元数据。

> **验证边界：** macOS 已通过 288 组原生库回环测试、取消/错误传播测试、
> 无 PATH 的单文件自检以及 LGPL 对象文件重新链接测试。另已通过两份约 30 秒的
> 真实 DTS-HD MA 样本（48 kHz，5.1(side)/16 bit、7.1/24 bit）端到端转换、
> 完整 PCM 校验及外部 FFmpeg 解码比较。另已用真实 DTS:X 7.1 文件的约 30 秒片段
> （48 kHz / 24 bit）通过同样的兼容声道转换与校验；
> DTS:X 5.1、IMAX 真实样本、Windows 原生运行和硬件播放兼容性尚未在本地验证。
> 这不是 Dolby 认证的编码器，也不保证 Blu-ray 合规。

### v1.0.0、系统要求与发布

CLI 的 `hdma2truehd version`、帮助信息和 UI 页脚统一显示 **v1.0.0** 与
`Copyright (c) 2026 ninghaofeng`。版本、版权和系统要求集中在
[`internal/buildinfo/info.go`](internal/buildinfo/info.go)；
macOS 应用元数据和 Windows CLI/UI 文件版本由构建工具生成。
系统要求数字版本的字段使用 `1.0.0`。

| 发布平台 | 归档中的架构名 | 最低支持系统 |
|---|---|---|
| macOS Apple Silicon | `darwin-arm64` | macOS 12.0 |
| macOS Intel | `darwin-amd64` | macOS 12.0 |
| Windows x64 | `windows-amd64` | Windows 10 22H2，build 19045 |
| Windows ARM64 | `windows-arm64` | Windows 11 24H2，build 26100 |

发布 CI 使用 **Go 1.26.x**，四个平台均原生构建，不通过纯 Go 交叉编译代替 cgo。
macOS 的 FFmpeg、cgo 和重新链接结果都以 12.0 为部署目标，并检查实际 Mach-O
最低系统版本；旧库若提高该版本，必须重建后端。
[Go 1.26 是最后支持 macOS 12 的版本](https://go.dev/doc/go1.26#darwin)，
不能用 Go 1.27 加部署参数来恢复其已移除的系统支持。
这些是发布支持基线，不代表已经完成最低版本系统上的实机验收；
Windows build 号是支持政策，并非程序内的启动拦截。
Windows UI 还需要匹配架构的 Microsoft Edge WebView2 Evergreen Runtime。

GitHub Actions 在普通提交和 PR 上构建、测试并上传四份完整发布归档。
推送与源码版本一致的 `v1.0.0` 标签后，四个平台全部成功才创建 **草稿 Release**，
由维护者验收后手动公开；标签与源码版本不一致会失败。
macOS 使用 `.tar.gz` 保留执行权限，Windows 使用 `.zip`，
命名为 `hdma2truehd-v1.0.0-<os>-<arch>`，附每份归档的 `.sha256` 和汇总 `SHA256SUMS`。
归档包含 CLI、UI、FFmpeg 源码、许可证和两套重新链接材料；
当前流水线不执行代码签名或 macOS 公证。

### 桌面界面（macOS / Windows）

- 批量选择或拖入 `.dts`、`.dtshd`、`.mkv`，直接读取本地文件，不上传或复制整部 MKV。
- MKV 展示音轨索引、语言、标题、格式、声道和时长；手动勾选一条或多条音轨。
  非 DTS-HD MA 音轨也会显示，但不可转换；有损 DTS core / HRA 仍不在支持范围。
  不兼容原因按界面语言显示，并随所选输出格式更新；仅 E-AC-3 不兼容时，
  可取消 E-AC-3、保留 TrueHD。存在不兼容的已选音轨时不能开始转换。
- TrueHD 与 E-AC-3 可同时勾选；桌面端默认勾选两种，CLI 默认仍仅 TrueHD。
  E-AC-3 的 5.1 / 7.1 码率可分别选择自动或预设值，混合队列按音轨布局使用对应设置；
  自动为 5.1 **960 kb/s**、7.1 **1536 kb/s**。
- 可选输出目录，留空时与源文件同目录；MKV 输出使用 `电影名.track-索引.thd/.eac3`
  避免同一电影的不同音轨互相覆盖。覆盖已有文件需要显式勾选。
- 并行队列默认同时处理 **2** 项任务，可选 **1–8**，选择 1 即为串行。
  同一文件的多个音轨、TrueHD / E-AC-3 输出也可并行；每项输出独立校验与发布。
  较高并行数会增加 CPU、内存和磁盘负载。提供每项结果、错误提示、取消，以及可记住的中英文切换。
  总进度条按已结束任务计数；每个文件分别显示转码、源 PCM 校验和输出校验阶段，
  以及已处理时长、阶段耗时和估计百分比。估计依据实际处理采样数与媒体时长；
  时长未知时显示不定进度，阶段真正成功才显示完成。CLI 同样输出阶段进度到 stderr。
  百分比不代替完整采样校验。关闭窗口会取消并等待清理，已发布的输出保留；
  其他技术诊断仍保留后端原始消息。
- 仅生成独立音频文件，不封装回 MKV、不解码视频或字幕，也不解析 DTS:X 对象。

下文的平台构建入口默认同时生成 CLI 和 UI，并打包到同一发布目录。
如果已经构建好本机 **v3** FFmpeg 后端，也可直接打包：

```bash
# macOS 终端 / Windows MSYS2 UCRT64 或 CLANGARM64 终端
# 使用兼容的发布工具链
export GOTOOLCHAIN=go1.26.10
bash scripts/build-release.sh
# 可指定尚不存在的发布目录
bash scripts/build-release.sh /absolute/path/to/new-release
# 仅构建 UI；原 build-ui.sh 入口也继续可用
bash scripts/build-release.sh --target ui
```

无需 Node.js、npm 或全局 Wails CLI；HTML/CSS/JavaScript 随 Go 程序嵌入，
模块最低 Go 版本为 1.24，发布使用 Go 1.26.x；macOS 构建只接受 Go 1.25.x/1.26.x。
仍需 cgo 和原生 C 工具链。桌面使用 Wails v2，macOS 使用系统 WebKit，
Windows 需要现代 Microsoft Edge WebView2 Evergreen Runtime（拖放需要较新版本）。
这不是新增外部 FFmpeg 依赖。
macOS 默认发布目录为 `dist/hdma2truehd-darwin-<arch>/`，双击其中的
`HDMA Converter.app`；Windows 为 `dist/hdma2truehd-windows-<arch>/hdma2truehd-ui.exe`。
同一目录也包含 CLI `hdma2truehd`（Windows 为 `.exe`）。
单独构建 UI 时仍使用原来的 `dist/hdma2truehd-ui-<os>-<arch>/` 默认目录。
发布目录包含 FFmpeg 重新链接材料及实际链接的 Go 模块许可证；发布前自行签名/公证并审查许可。
Windows 原生 UI 与拖放尚需实机验证。

应用使用原创“声波＋转换箭头”图标，统一用于应用图标、窗口界面和网页图标。
矢量源为 `cmd/hdma2truehd-ui/frontend/icon.svg`；PNG、ICO 和 ICNS 已提交，
正常构建无需图像工具。修改矢量图后，可在 macOS 执行
`swift scripts/generate-icons.swift` 重新生成资源，再重建 UI。
Windows 打包用 UCRT64 binutils 或 CLANGARM64 LLVM 的 `windres` 嵌入 EXE 图标和版本；
macOS 打包自动复制 ICNS。

CLI 也可显式选择 MKV 音轨（索引使用容器全局索引，不是音频列表中的序号）：

```bash
hdma2truehd probe --list-tracks movie.mkv
hdma2truehd convert --stream 2 --format both movie.mkv
```

`--stream` 仅用于单个输入；CLI 默认命名仍为 `movie.thd/.eac3`，
转换同一 MKV 的不同音轨时请使用不同输出目录或单格式 `-o`。
未明确选择时，多轨容器会被拒绝，不会自动选取默认音轨。

### 架构

- Go：CLI、桌面队列、路径安全、逐声道哈希和最终输出发布。
- 桌面：Wails 与嵌入式 HTML/CSS/JavaScript；`internal/gui/` 独立于 webview，可无界面测试。
- cgo/C 桥接：使用 `libavformat` 探测（含 MKV 音轨）与封装、
  `libavcodec` 解码与 TrueHD / E-AC-3 编码。
- `libswresample`：只转换整数样本的平面/交错和容器表示；输入输出采样率、
  声道标签完全相同，关闭抖动，不执行重采样或降混，并检查所有有效位。
- 进程内处理检查 context 取消，FFmpeg I/O 配置中断回调；
  出错时释放 decoder、encoder、packet、frame、FIFO 和格式上下文。

普通 FFmpeg 的原生 TrueHD 编码器未提供 7.1 编码支持。本项目固定使用
FFmpeg 8.0.1 及 [TrueHD 补丁](patches/ffmpeg/README.md)，处理八声道
restart header、声道顺序和极短输入 flush。库版本在运行时检查。
普通 FFmpeg 的 E-AC-3 编码器最多编码 5.1；另一个补丁增加可选四声道依赖子流，
与独立 5.1 核心组成完整 7.1，不以最终输出降混代替八声道编码。

#### 代码阅读路径

建议按以下顺序阅读；源码注释重点解释数据约定、资源所有权和失败边界：

1. [`cmd/hdma2truehd/main.go`](cmd/hdma2truehd/main.go)：参数解析、整批路径冲突
   预检查、逐文件执行及退出码。单个文件失败后继续后续任务，取消则停止。
2. [`internal/audio/convert.go`](internal/audio/convert.go)：转换事务主线，
   即“探测并验证输入 → 同文件系统临时编码 → 输出格式检查 → 完整 PCM 比较
   → 发布”。未显式允许覆盖时，用硬链接创建最终名称，避免检查后的竞争覆盖。
3. [`internal/audio/probe.go`](internal/audio/probe.go) 与
   [`internal/audio/pcm.go`](internal/audio/pcm.go)：格式白名单和精度约束；
   将交错、低位补零的 32 位整数 PCM 按声道分别哈希，并检查精确采样数。
4. [`internal/av/native.go`](internal/av/native.go) 与
   [`internal/av/native.c`](internal/av/native.c)：同步 cgo 句柄、取消和错误回传，
   以及原生解码、整数表示转换、FIFO 分帧、编码与尾部 flush。
   DTS:X 在此仅解码兼容声道，不接入对象或高度渲染器。
5. [`scripts/build-common.sh`](scripts/build-common.sh)、
   [`scripts/build-release.sh`](scripts/build-release.sh) 与
   [`tools/link-wrapper/main.go`](tools/link-wrapper/main.go)：跨平台构建、安全清理、
   静态依赖检查及重新链接输入捕获；FFmpeg 编码补丁见
   [`patches/ffmpeg/README.md`](patches/ffmpeg/README.md)。
6. [`tests/integration/`](tests/integration/)：原生 TrueHD 合成信号回环及可选真实
   DTS 样本测试。合成回环验证编码器，不代替真实 DTS 解码或硬件播放验收。
7. [`cmd/hdma2truehd-ui/main.go`](cmd/hdma2truehd-ui/main.go)、
   [`internal/gui/service.go`](internal/gui/service.go) 与
   [`frontend/app.js`](cmd/hdma2truehd-ui/frontend/app.js)：桌面生命周期、批量预检和有界并行队列；
   注释说明快照所有权、修订号排序、取消/关闭等待及按声道布局保存码率的约束。
   [`tests/build/`](tests/build/) 使用隔离目录和模拟编译器验证构建/清理流程，
   不将这些测试视作原生编译或音频保真的证明。

E-AC-3 分支见 [`internal/audio/eac3.go`](internal/audio/eac3.go) 和
[`internal/av/eac3.c`](internal/av/eac3.c)：独立/依赖子流同步编码、声道替换映射、
有损完整性校验及编码延迟处理，与 TrueHD 的无损验证保持区分。

参考项目 [HG3112/md71](https://github.com/HG3112/md71) 生成的是**有损 E-AC-3 7.1**，
不是 TrueHD。本项目的 E-AC-3 方案参考其依赖子流设计（HellGauss，LGPL 2.1），
但通过本项目原生库接口在进程内编排，不调用其可执行文件或脚本。

### 构建

发布使用 **Go 1.26.x、cgo 和原生 C 工具链**，以及 make、curl、tar、xz、patch 和
SHA-256 校验工具。FFmpeg 构建脚本只生成静态库，不编译命令行程序。
构建环境需要这些工具，最终用户运行单个程序时不需要。

库路径固定为 `.backend/ffmpeg-8.0.1/`。平台入口默认依次构建静态库、运行测试和
静态检查、生成 LGPL 完整发布目录，并验证重新链接和 `doctor` 自检。
已有源码目录时默认拒绝覆盖，防止破坏本地改动；可以显式复用现有库，
或先将旧目录重命名备份再重建。已有发布目录默认拒绝覆盖；
可显式使用 `--clean`（Windows 为 `-Clean`）清理本次发布目录后重新构建。
清理仅允许项目 `dist/` 的直接子目录，拒绝符号链接，不删除 `dist/` 本身、
`.backend/`、`bin/`、`samples/` 或其他发布目录。
依赖和后端检查通过后才清理；后续构建若失败，被清理的旧发布结果不会恢复。
`--clean` 不重建静态库，已有静态库时请搭配 `--reuse-backend`；
日常 `--binary-only` 已覆盖主程序，无需且不能搭配 `--clean`。

**清理并重建静态库：** 使用 `--clean-backend`（Windows 为 `-CleanBackend`），
明确删除且仅删除 `.backend/ffmpeg-8.0.1/` 整个源码和编译目录，再重新解压、打补丁和编译。
该目录中的本地修改会丢失；不跟随 `.backend/` 或源码目录的符号链接。
下载归档和其他后端备份保留；已有归档重新校验 SHA-256，缺失时才下载。
参数不能与复用库或日常模式组合；要同时清理旧发布目录，再加 `--clean` / `-Clean`。
清理发生在工具、平台和输出路径检查之后；后续重建失败不会恢复已删除的后端。
替换静态库后应使用 `go build -a` / `go test -a` 强制重新链接，避免 Go 缓存
继续使用外部库的旧版本。

**旧后端升级：** MKV 支持要求版本标记 `8.0.1-hdma2truehd-71-v3`。
旧的 `v1` / `v2` 静态库不能复用来构建此版本；先将 `.backend/ffmpeg-8.0.1/`
重命名为一个尚未占用的备份目录，再执行不带 `--reuse-backend` / `-ReuseBackend`
的完整平台构建。不要仅执行 `--binary-only`；程序会拒绝旧版本后端。

两平台共用构建流程，始终通过 cgo 链接四个 FFmpeg `.a` 静态库，
不依赖系统安装的 FFmpeg 动态库或命令行程序：

| 模式 | macOS 参数 | Windows 参数 | 行为 |
|---|---|---|---|
| 完整发布（默认） | 无 | 无 | 同时构建 CLI/UI，共享源码和许可证材料，分别重新链接；CLI 执行自检 |
| 复用库完整发布 | `--reuse-backend` | `-ReuseBackend` | 复用现有库，其余发布步骤不变 |
| 重建静态库完整发布 | `--clean-backend` | `-CleanBackend` | 删除当前 FFmpeg 源码/编译目录后重建，再构建 CLI/UI；可加发布目录清理参数 |
| 日常仅编译应用 | `--binary-only` | `-BinaryOnly` | 必须已有本机静态库；同时编译 CLI/UI 并检查动态依赖 |
| 单独构建目标 | `--target cli` / `--target ui` | `-Target cli` / `-Target ui` | 只构建所选工具，可搭配完整发布或日常模式；默认 `both` |

日常模式不下载、不重建 FFmpeg，不执行测试、`doctor` 或发布打包；
无需 FFmpeg 原始源码归档及 make、curl、tar、xz、patch，但仍需 Go、原生 C 工具链，
以及 macOS 的 `otool` 或 Windows 对应 MSYS2 环境的 `objdump` 和 `windres`。
该模式也使用 `go build -a`，避免替换静态库后误用缓存；已有应用会被重新编译覆盖。
macOS 同时生成带图标的 `.app`；单目标日常构建不会删除另一个已有工具。
它只用于本地开发，**不能代替附带 LGPL 材料的完整发布目录**。

#### macOS

安装 Xcode Command Line Tools 并准备其他构建工具后：

```sh
export GOTOOLCHAIN=go1.26.10
bash scripts/build-macos.sh

# 已有本机静态库时复用，并选择新的发布目录
bash scripts/build-macos.sh --reuse-backend --output-dir dist/macos-new

# 清理默认发布目录后重建，不重新编译 FFmpeg 静态库
bash scripts/build-macos.sh --reuse-backend --clean

# 连同 FFmpeg 静态库一起从头重建（会丢失后端源码目录中的本地修改）
bash scripts/build-macos.sh --clean-backend --clean

# 日常开发：复用静态库，编译 CLI、UI 和 macOS .app 到 bin/
bash scripts/build-macos.sh --binary-only

# 仅编译 CLI 到指定目录
bash scripts/build-macos.sh --binary-only --target cli --output-dir bin/dev

# 仅编译 UI（含 .app）
bash scripts/build-macos.sh --binary-only --target ui

# 两个工具编译到指定目录
bash scripts/build-macos.sh --binary-only --output-dir bin/dev
```

#### Windows

安装 Go 和 [MSYS2](https://www.msys2.org/)。首次可由构建入口安装并校验 MSYS2 依赖：

```powershell
.\scripts\build-windows.ps1 -InstallDependencies
```

在 x64 主机安装原生 amd64 Go 并使用 UCRT64 GCC；在 ARM64 主机安装原生 arm64 Go，
使用 CLANGARM64 Clang。脚本根据 Go 架构选择环境，并拒绝主机/编译器架构不匹配。

`-InstallDependencies` 在所选 MSYS2 的 UCRT64 或 CLANGARM64 环境中执行
`pacman -S --needed`，安装缺失的构建包；由 pacman 提示确认，失败则立即停止，
成功后重新检查工具并继续构建。它不安装 Go/MSYS2、不执行系统升级，
也不会自动修改执行策略。后续构建无需再次指定该参数。
搭配 `-BinaryOnly` 时仅安装对应架构的编译器、链接器和资源工具，不安装完整发布所需的其他工具。

也可以在 **UCRT64** 终端手动安装同样的完整依赖：

```sh
pacman -S --needed make diffutils patch curl tar xz mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-binutils
```

ARM64 则在 **CLANGARM64** 终端安装：

```sh
pacman -S --needed make diffutils patch curl tar xz mingw-w64-clang-aarch64-clang mingw-w64-clang-aarch64-llvm-tools mingw-w64-clang-aarch64-lld mingw-w64-clang-aarch64-compiler-rt
```

然后在项目目录下的 **PowerShell** 中运行：

```powershell
.\scripts\build-windows.ps1

# MSYS2 安装在非默认位置
.\scripts\build-windows.ps1 -Msys2Root 'D:\msys64'

# 自定义 MSYS2 路径，并安装所需依赖后构建
.\scripts\build-windows.ps1 -Msys2Root 'D:\msys64' -InstallDependencies

# 复用已经编译的 Windows 静态库
.\scripts\build-windows.ps1 -ReuseBackend -OutputDir 'dist\windows-new'

# 清理默认发布目录后重建，不重新编译 FFmpeg 静态库
.\scripts\build-windows.ps1 -ReuseBackend -Clean

# 连同 FFmpeg 静态库一起从头重建（会丢失后端源码目录中的本地修改）
.\scripts\build-windows.ps1 -CleanBackend -Clean

# 日常开发：复用静态库，同时编译两个 EXE 到 bin\
.\scripts\build-windows.ps1 -BinaryOnly

# 仅编译 CLI 到指定目录
.\scripts\build-windows.ps1 -BinaryOnly -Target cli -OutputDir 'bin\dev'

# 仅编译 UI
.\scripts\build-windows.ps1 -BinaryOnly -Target ui

# 两个工具编译到指定目录
.\scripts\build-windows.ps1 -BinaryOnly -OutputDir 'bin\dev'
```

默认 MSYS2 路径为 `C:\msys64`，也可以通过 `MSYS2_ROOT` 环境变量指定。
PowerShell 入口会自动启动匹配架构的 MSYS2 UCRT64 / CLANGARM64 登录 shell，并继承系统 PATH。
默认只检查依赖，仅显式指定 `-InstallDependencies` 时安装 MSYS2 构建包；
不会改变执行策略或永久修改环境变量。
对应架构的原生 Go 必须在 Windows PATH 中。
FFmpeg 与 Go/cgo 必须使用匹配的目标架构和工具链；
不能用 `CGO_ENABLED=0` 或只设置 `GOOS=windows` 的纯 Go 交叉编译代替原生构建。
Windows 链接配置使用静态编译器运行库；仍需通过 Windows 原生 CI/运行验证
确认实际系统 DLL 依赖及兼容性。

两个平台完整发布默认输出 `dist/hdma2truehd-<os>-<arch>/`，
包含 CLI、UI，以及 macOS 的 `HDMA Converter.app`。
CLI 的重新链接材料在 `relink/`，UI 的在 `relink-ui/`；FFmpeg 源码和许可证只打包一次。
单目标发布使用 `relink/`；单独 UI 发布的默认目录为 `dist/hdma2truehd-ui-<os>-<arch>/`。
日常模式默认将两种程序输出到 `bin/`，macOS 还生成 `bin/HDMA Converter.app`。
macOS 构建会检查仅依赖系统动态库；Windows 会检查没有 FFmpeg/GCC 运行库 DLL 依赖。
相对输出目录以项目根目录为基准。底层 `build-ffmpeg.sh` 和 `build-release.sh`
继续作为公共步骤保留，避免复制 FFmpeg 配置和许可证打包逻辑。
Go 链接辅助工具位于 `tools/link-wrapper/`，参与普通 `go test ./...`，
不再与 shell/PowerShell 脚本混放；重新链接文档位于 `docs/RELINKING.md`。

### 使用

**选项放在输入路径之前。** 路径包含空格时加引号：

```sh
hdma2truehd doctor
hdma2truehd probe "input.dts"
hdma2truehd convert -o "output.thd" "input.dts"
hdma2truehd convert "input.dtshd"
hdma2truehd convert --output-dir "converted" "film-5.1.dts" "film-7.1.dtshd"
hdma2truehd convert --jobs 4 --output-dir "converted" "film-5.1.dts" "film-7.1.dtshd"
hdma2truehd convert --overwrite -o "output.thd" "input.dts"
hdma2truehd convert --skip-verify -o "output.thd" "input.dts"
hdma2truehd convert --format eac3 -o "output.eac3" "input.dts"
hdma2truehd convert --format both "input.dts"
hdma2truehd convert --format both --eac3-bitrate 1536 "input-7.1.dts"
```

`--format` 支持 `truehd`（默认）、`eac3`、`both`。两种格式默认与输入同目录、同名，
分别使用 `.thd` 和 `.eac3`；双格式不能使用 `-o`，可以用 `--output-dir`。
两种输出独立验证与发布，不保证成对原子提交；一种失败不会删除另一种已成功输出。

`--ffmpeg` 和 `--ffprobe` 参数已移除；无需指定外部程序。
`doctor` 检查链接版本、编解码能力，并实际执行三种布局的 24 位 PCM 回环；
它不能代替真实 MA 文件验收或硬件认证。
另会执行 5.1(side)/7.1 的 E-AC-3 编码、完整解码及格式/长度自检。

TrueHD 默认校验会另外完整解码源文件和输出文件，因此需要额外处理时间，但内存有界。
比较声道标签、整数 PCM 的所有有效位、每个声道的 SHA-256 和精确采样数。
不使用时长近似、浮点容差、自动对齐或裁剪来掩盖差异。
跳过完整比较时明确标记 `UNVERIFIED`，不宣称已证明无损。

#### E-AC-3 的有损编码边界

- 仅接受 48 kHz、`5.1(side)` 或 `7.1`，不自动重采样或改变声道布局。
  高采样率、后环绕 `5.1` 等输入仍可输出 TrueHD，但 E-AC-3 分支会明确拒绝。
- 默认总码率：5.1 为 **960 kb/s**，7.1 为 **1536 kb/s**。
  `--eac3-bitrate` 单位为 kb/s：5.1 允许 384–1024，7.1 允许 768–1536，
  都必须为 32 的倍数；`0` 表示按布局选默认值。
- 7.1 使用独立 5.1 核心和四声道依赖子流，总码率按 **5:3** 分配。
  核心包含兼容性环绕降混；完整解码时，依赖子流替换核心环绕并增加后环绕，
  最终保留全部八个原始声道的独立编码信号，不是 5.1 上混。
- 原始 `.eac3` 没有无缝裁剪元数据，保留 **256 样本（约 5.33 ms）的编码延迟**
  及最后一帧补零。源长为 `N` 时，解码长度应为
  `ceil((N + 256) / 1536) * 1536`；重新封装时需正确处理同步，不应直接假设与
  TrueHD 样本起止完全相同。
- 默认完整解码输出，验证 codec、48 kHz、原声道布局和上述精确长度；
  显示 `VALIDATED (LOSSY)`，而不是无损 `VERIFIED`。`--skip-verify` 会跳过
  完整解码检查，并显示 `UNVERIFIED`，但仍检查输出格式。
- 实验性 7.1 需要支持依赖子流的播放器；仅识别核心的设备可能只播放 5.1。
  软件测试不等于 Dolby、Blu-ray 或硬件认证，也不保证主观音质。

每个任务在目标文件系统创建唯一临时目录，编码、元数据检查及默认 PCM 校验
成功后才发布。默认不覆盖；无需覆盖时使用原子硬链接，因此目标文件系统必须
支持硬链接（如 APFS、NTFS）。不支持时明确报错，不采用不安全回退。
`--overwrite` 也只在新结果成功后替换原输出。

批量任务默认同时处理 **2** 项，用 `--jobs 1` 恢复串行，`--jobs N` 可设置 **1–8**。
并行单位为单个音轨的一种输出格式，同一文件的多个输出也可并行。
单项失败不阻止其他任务，最后统一汇总；同名输出冲突在开始前拒绝。
进度信息带任务编号与输出路径，避免混淆。Ctrl+C 停止调度新任务、取消所有运行中的
原生操作，并等待临时文件清理后退出；已发布的结果保留。提高并行数会增加资源负载。
退出码：`0` 成功，`1` 编解码/校验失败，`2` 参数错误，`130` 取消。

### 实验性支持矩阵

| 项目 | 范围 |
|---|---|
| 输入 | 本地单音轨音频文件或明确选择的 MKV 音轨，库探测确认为 `dts` / `DTS-HD MA`、`DTS-HD MA + DTS:X` 或 `DTS-HD MA + DTS:X IMAX` |
| 布局 | `5.1(side)`、`5.1`（后环绕）、`7.1`（保留侧环绕和后环绕） |
| 有效精度 | 16 或 24 位；未知、20 位、32 位输入明确拒绝 |
| 采样率 | 44.1、48、88.2、96、176.4、192 kHz；不重采样 |
| 输出 | 原始 TrueHD `.thd`，无 AC-3 伴随流 |
| 可选有损输出 | E-AC-3 `.eac3`，仅 48 kHz / 5.1(side)、7.1；不生成 Atmos |
| 编解码回环 | 三种布局 × 六种采样率 × 两种位深 × 八种长度，共 288 组 |
| 真实 MA 验收 | macOS 本地两份约 30 秒样本通过：48 kHz，5.1(side)/16 bit、7.1/24 bit；完整 PCM 校验及外部 FFmpeg 解码比较一致 |
| 真实 DTS:X 验收 | macOS 本地约 30 秒的 48 kHz / 24 bit / 7.1 片段通过；逐声道 PCM 校验及外部 FFmpeg 解码比较一致；尚无 DTS:X 5.1 / IMAX 真实样本验收 |

矩阵描述的是实验性软件编解码能力，不是播放器、功放或 Blu-ray 支持声明。
特别是 7.1 的 176.4/192 kHz 不应被当作 Blu-ray 兼容输出；Dolby 公布的
Blu-ray 上限是八声道 96 kHz、六声道 192 kHz。
7.1 输出不生成额外的 5.1 降混 presentation；如用于特定设备或光盘制作，
需另行验证码率、各 presentation 及播放兼容性。

不支持将 DTS 有损核心、DTS-HD HRA 冒充 MA，不自动猜测未知布局。
不提供 MKV 输出/重新封装、DTS:X 对象元数据保留、Atmos 生成或批量并行。

### 测试

#### 用主程序批量转换本地样本

先按上文完成发布构建，将有权测试的 `.dts` / `.dtshd` 文件放入 `samples/`，然后运行：

```sh
# macOS：自动查找 dist 中的发布程序
bash scripts/test-samples.sh

# 显式覆盖已有 .thd，重新转换并完整校验
bash scripts/test-samples.sh --overwrite

# 也可以测试指定的发布程序
bash scripts/test-samples.sh --binary ./dist/hdma2truehd-darwin-arm64/hdma2truehd

# 同时测试两种格式；允许替换此前的测试输出
bash scripts/test-samples.sh --format both --overwrite
```

Windows 在 PowerShell 中运行，无需通过 MSYS2 Bash 启动测试：

```powershell
# 自动查找 dist 中的发布程序
.\scripts\test-samples.ps1

# 显式覆盖已有 .thd，重新转换并完整校验
.\scripts\test-samples.ps1 -Overwrite

# 测试指定的发布程序
.\scripts\test-samples.ps1 -Binary '.\dist\hdma2truehd-windows-amd64\hdma2truehd.exe'

# 同时测试两种格式
.\scripts\test-samples.ps1 -Format both -Overwrite
```

测试脚本默认在仓库 `dist/` 及其一级发布子目录中查找普通版主程序：
Bash 为 `hdma2truehd`（Windows 下为 `.exe`），PowerShell 为 `hdma2truehd.exe`。
只找到一个时打印所选路径并运行；找不到则提示先构建发布版本，
找到多个则列出候选并要求通过 `--binary` / `-Binary` 显式选择，不自动选择最新版本。
不会选择 `hdma2truehd-relinked`，不再默认查找 `bin/`，也不自动构建。
如需测试仅编译主程序的结果，可显式指定 `--binary ./bin/hdma2truehd`
或 `-Binary '.\bin\hdma2truehd.exe'`。
扫描仓库 `samples/` 顶层的输入文件，扩展名不区分大小写，
将同名 `.thd` / `.eac3` 写回该目录，不把已有输出当输入，也不修改源文件。
默认仅输出 TrueHD；`--format` / `-Format` 可选择格式，
`--eac3-bitrate` / `-Eac3Bitrate` 可指定总码率。
始终保留相应格式的完整校验，不覆盖已有输出；任何转换失败均返回非零退出码。
可从任意工作目录运行脚本；`--binary` / `-Binary` 的相对路径以当前工作目录为准。
PowerShell 脚本直接传递主程序退出码；测试本身不需要 MSYS2、Go 或外部 FFmpeg。
Windows 也可在对应架构的 MSYS2 UCRT64 / CLANGARM64 Bash 中运行 Bash 测试脚本。

#### 自动化测试

先构建静态库，再运行；原生集成测试不再依赖 FFmpeg 命令或后端路径变量：

```sh
go test -race ./...
go vet ./...

# 可选：用另一套 FFmpeg 命令额外解码，交叉检查原生编码结果
HDMA_TEST_DECODER=/absolute/path/to/ffmpeg \
go test ./tests/integration -run TestTrueHDRoundTrip -count=1
```

外部解码器仅是**可选测试工具**，产品没有对应子进程路径。
另一套 FFmpeg 仍属于同一解码器家族，不等同于独立实现交叉认证。

真实样本测试不会自动选择或下载输入。通过环境变量指定自己有权测试的本地文件：

```sh
HDMA_TEST_DTS_51="/absolute/path/to/5.1.dts" \
HDMA_TEST_DTS_71="/absolute/path/to/7.1.dtshd" \
HDMA_TEST_DTSX_51="/absolute/path/to/dtsx-5.1.dts" \
HDMA_TEST_DTSX_71="/absolute/path/to/dtsx-7.1.dts" \
go test ./tests/integration -run TestDTSHDMAConversion -count=1 -v
```

四个样本变量分别可选。DTS:X 测试要求探测到上述两个 DTS:X profile 之一，
并执行完整兼容声道 PCM 校验；没有对应样本时不会以普通 MA 代替验收。
符合 48 kHz / 5.1(side) 或 7.1 的样本还会测试 E-AC-3 转换。
原生 E-AC-3 测试覆盖短流、帧边界、码率和延迟/补零，并以各声道不同频率的
测试信号检查映射和可测的信噪比阈值；这些阈值不是一般音质保证。

未提供真实样本时对应测试明确 `SKIP`。CI 配置在 macOS/Windows 原生构建静态库、
测试、生成完整发布材料并实际重新链接；配置存在不代表已经远端运行成功。

### 单文件发布与 LGPL 重新链接

运行时只需单个可执行文件；macOS 仍依赖操作系统提供的框架和系统库，
“单文件”不表示静态链接整个操作系统。

**静态链接 FFmpeg 后，不能只分发可执行文件而省略 LGPL 所需材料。**
使用发布脚本生成应一同打包的目录：

```sh
bash scripts/build-release.sh
# 默认输出 dist/hdma2truehd-<os>-<arch>/，已有目录时拒绝覆盖

bash dist/hdma2truehd-darwin-arm64/relink/relink.sh
dist/hdma2truehd-darwin-arm64/hdma2truehd-relinked doctor
bash dist/hdma2truehd-darwin-arm64/relink-ui/relink.sh
```

发布目录包含：

- CLI、UI 单文件程序，以及 macOS `.app`；可用 `--target cli` / `--target ui` 只打包一种。
- Go/cgo 可重新链接对象、FFmpeg 静态库、链接脚本、Windows 导出定义文件（如有），
  以及捕获的原始外部链接命令；重新链接不依赖 Go 临时构建目录。
- FFmpeg 原始源码归档、补丁、配置后的完整源码、构建脚本和工具链信息。
- LGPL、Go 许可证及 [重新链接说明](docs/RELINKING.md)。

用户可以在相同平台/架构上替换兼容的 FFmpeg 静态库，再运行 `relink.sh`。
不同 SDK 路径可能需要调整；ABI/头文件变化则需重新编译应用。
请将整个发布目录打包分发，并根据实际分发方式核实许可证要求。

编码器补丁沿用上游文件的 LGPL 2.1-or-later 条款；版本、校验值和注意事项见
[后端说明](patches/ffmpeg/README.md)。开源许可证不等同于 Dolby/DTS 商标、
专利授权或产品认证。

---

## English

An experimental Go command-line and desktop tool that **converts DTS-HD MA audio, including
DTS:X compatible channels, to Dolby TrueHD `.thd` or E-AC-3 `.eac3`** on macOS and Windows.
It **statically links FFmpeg libraries through cgo**: probing, decoding,
encoding and PCM verification all run in-process. No `ffmpeg` / `ffprobe`
executable is launched or required at runtime.

TrueHD remains the default; E-AC-3 or both formats can be selected.
TrueHD supports 5.1 and 7.1, with complete per-channel PCM SHA-256 and exact sample-count
comparison enabled by default. No automatic downmixing, resampling or reduction
in effective precision is performed.
E-AC-3 is lossy and currently supports 48 kHz 5.1(side) and genuine 7.1.
Validation checks complete decoding, layout and exact length including delay
and padding; it does not claim lossless PCM equality.

DTS:X / DTS:X IMAX decoding uses only the compatible 5.1 or 7.1 channels and
preserves their layout. It does not upmix, render objects, retain object metadata
or generate Atmos. PCM verification covers only the decoded compatible channels,
not the complete DTS:X object scene. Some DTS:X files may be identified simply
as `DTS-HD MA`; that label does not prove that object metadata is absent.

> **Validation boundaries:** On macOS, 288 native-library round trips,
> cancellation/error propagation, a single-executable self-test without PATH,
> and relinking from LGPL object materials have passed. Two real, approximately
> 30-second DTS-HD MA samples (48 kHz, 5.1(side)/16 bit and 7.1/24 bit) have also
> passed end-to-end conversion, full PCM verification and comparison after
> decoding with an external FFmpeg. An approximately 30-second real DTS:X 7.1
> clip (48 kHz / 24 bit) has passed the same compatible-channel checks.
> Real DTS:X 5.1 / IMAX samples, native Windows execution and hardware playback
> compatibility have not been verified locally. This is not a Dolby-certified
> encoder and does not guarantee Blu-ray compliance.

### v1.0.0, system requirements and releases

CLI `hdma2truehd version`, help output and the UI footer display **v1.0.0** and
`Copyright (c) 2026 ninghaofeng`. Version, copyright and OS requirements are
shared through [`internal/buildinfo/info.go`](internal/buildinfo/info.go).
Build tooling generates macOS bundle metadata and Windows CLI/UI file versions;
OS fields requiring numeric versions use `1.0.0`.

| Release platform | Architecture suffix | Minimum supported system |
|---|---|---|
| macOS Apple Silicon | `darwin-arm64` | macOS 12.0 |
| macOS Intel | `darwin-amd64` | macOS 12.0 |
| Windows x64 | `windows-amd64` | Windows 10 22H2, build 19045 |
| Windows ARM64 | `windows-arm64` | Windows 11 24H2, build 26100 |

Release CI uses **Go 1.26.x** and native builds on all four targets, not pure-Go
cross-compilation in place of cgo. macOS FFmpeg, cgo and relinked executables
target 12.0; the actual Mach-O minimum is checked, rejecting old libraries
that raise it. [Go 1.26 is the last release supporting macOS 12](https://go.dev/doc/go1.26#darwin);
deployment flags cannot restore OS support removed in Go 1.27.
These are release support baselines, not a claim of completed minimum-OS
on-device acceptance. Windows build numbers are support policy, not startup
guards. Windows UI also needs the matching-architecture Microsoft Edge WebView2
Evergreen Runtime.

GitHub Actions builds, tests and uploads four complete archives on regular
pushes and PRs. Pushing a `v1.0.0` tag matching the source version creates a
**draft Release** only after all four targets succeed; a maintainer makes it
public after acceptance. Mismatching tags fail. macOS uses `.tar.gz` to retain
executable permissions; Windows uses `.zip`. Names follow
`hdma2truehd-v1.0.0-<os>-<arch>`, with individual `.sha256` sidecars and aggregate
`SHA256SUMS`. Archives include CLI, UI, FFmpeg sources, notices and both relinking
kits. The workflow currently does not sign code or notarize macOS applications.

### Desktop UI (macOS / Windows)

- Batch-select or drop `.dts`, `.dtshd` and `.mkv` files. Files are read locally;
  no upload or full-MKV copy is needed.
- MKV inputs list container stream indices, languages, titles, formats, layouts
  and durations. Explicitly select one or more audio tracks. Unsupported tracks
  remain visible but disabled; lossy DTS core / HRA remain unsupported.
  Reasons use the interface language and update with selected output formats.
  If only E-AC-3 is incompatible, deselect it to retain TrueHD. Incompatible
  selected tracks block conversion until the selection or formats are changed.
- Select TrueHD, E-AC-3 or both. The desktop defaults to both; the CLI still
  defaults to TrueHD only. E-AC-3 has separate automatic/preset bitrate choices
  for 5.1 and 7.1; mixed queues use each track's layout-specific setting.
  Automatic is **960 kb/s for 5.1**, **1536 kb/s for 7.1**.
- Choose an output directory or leave it empty to use each source directory.
  MKV outputs use `movie.track-INDEX.thd/.eac3` to distinguish selected tracks.
  Existing outputs require explicit overwrite permission.
- The queue defaults to **2** concurrent jobs, adjustable from **1–8**; choose
  1 for serial operation. Tracks and TrueHD / E-AC-3 outputs from the same
  file may run concurrently, each independently verified and published.
  Higher concurrency increases CPU, memory and disk load.
  Per-job results, errors and cancellation remain available. The queue
  bar counts finished jobs; per-file progress shows encoding, source PCM
  verification and output validation stages, processed duration, stage elapsed
  time and estimated percentages based on actual samples and media duration.
  Unknown duration uses indeterminate progress; stages complete only on success.
  The CLI also reports stage progress on stderr. Estimates never replace full
  sample verification. Closing the window cancels and waits for cleanup,
  preserving published outputs. Chinese/English selection is remembered;
  other technical diagnostics retain the original backend messages.
- Outputs are separate audio files, not a remuxed MKV. No video/subtitle
  decoding or DTS:X object rendering is added.

The platform build entry points below now build CLI and UI into one release by
default. With an existing native **v3** FFmpeg backend, package them directly:

```bash
# macOS terminal / Windows MSYS2 UCRT64 or CLANGARM64 terminal
# Select a compatible release toolchain
export GOTOOLCHAIN=go1.26.10
bash scripts/build-release.sh
# Optionally choose a new, nonexistent release directory
bash scripts/build-release.sh /absolute/path/to/new-release
# UI only; the original build-ui.sh entry point remains available too
bash scripts/build-release.sh --target ui
```

No Node.js, npm or global Wails CLI is needed: HTML/CSS/JavaScript are embedded.
The module minimum is Go 1.24; releases use Go 1.26.x, and macOS builds accept
only Go 1.25.x/1.26.x. cgo and a native C toolchain are required. Wails v2 uses system
WebKit on macOS and requires a modern Microsoft Edge WebView2 Evergreen Runtime
on Windows (recent versions are needed for file drop). No external FFmpeg
dependency is introduced.
Open `HDMA Converter.app` inside `dist/hdma2truehd-darwin-<arch>/` on macOS;
run `dist/hdma2truehd-windows-<arch>/hdma2truehd-ui.exe` on Windows.
The same directory contains the CLI `hdma2truehd` (`.exe` on Windows).
UI-only releases retain the default `dist/hdma2truehd-ui-<os>-<arch>/` directory.
Releases include FFmpeg relinking materials and licenses for linked Go modules.
Review distribution obligations and sign/notarize as needed. Native Windows UI
and drag-and-drop still require on-device validation.

An original waveform-and-conversion-arrows design is shared by the application,
interface and favicon. Its vector source is `cmd/hdma2truehd-ui/frontend/icon.svg`;
PNG, ICO and ICNS assets are committed, so normal builds need no image tools.
After editing the SVG, run `swift scripts/generate-icons.swift` on macOS and
rebuild the UI. Windows packaging embeds EXE icons and versions using UCRT64
binutils' or CLANGARM64 LLVM's `windres`; macOS copies the ICNS into the app bundle.

The CLI can also select MKV audio explicitly, using global container stream
indices rather than audio-list positions:

```bash
hdma2truehd probe --list-tracks movie.mkv
hdma2truehd convert --stream 2 --format both movie.mkv
```

`--stream` accepts one input only. CLI output naming remains `movie.thd/.eac3`;
use separate output directories or single-format `-o` when converting multiple
tracks from the same MKV. Multi-track containers without explicit selection
are rejected, never silently mapped to a default track.

### Architecture

- Go: CLI, desktop queue, path safety, per-channel hashing and final output
  publication.
- Desktop: Wails with embedded HTML/CSS/JavaScript; `internal/gui/` is
  independent of the webview and can be tested headlessly.
- cgo/C bridge: `libavformat` for probing (including MKV tracks) and muxing;
  `libavcodec` for decoding and TrueHD / E-AC-3 encoding.
- `libswresample`: converts only integer sample representations, including
  planar/interleaved storage and container width. Input/output sample rates and
  speaker labels remain identical; dithering is disabled. No resampling or
  downmixing is performed, and all effective bits are checked.
- In-process operations check context cancellation; FFmpeg I/O uses an interrupt
  callback. Decoder, encoder, packet, frame, FIFO and format resources are
  released on failure.

The stock FFmpeg native TrueHD encoder does not provide 7.1 encoding.
This project pins FFmpeg 8.0.1 with a [TrueHD patch](patches/ffmpeg/README.md)
for eight-channel restart headers, channel ordering and flushing very short
inputs. The linked library version is checked at runtime.
Stock FFmpeg E-AC-3 encoding stops at 5.1. A second patch adds an optional
four-channel dependent substream, paired with an independent 5.1 core for
complete 7.1 rather than downmixing the final output.

#### Code reading guide

Read in the following order. Source comments focus on data contracts, resource
ownership and failure boundaries:

1. [`cmd/hdma2truehd/main.go`](cmd/hdma2truehd/main.go): argument parsing,
   batch-wide path-conflict checks, per-file execution and exit codes.
   A file failure allows later jobs to continue; cancellation stops the batch.
2. [`internal/audio/convert.go`](internal/audio/convert.go): the conversion
   transaction: probe/validate input, encode to a temporary file on the same
   filesystem, check output metadata, compare complete PCM, then publish.
   Without explicit overwrite permission, a hard link creates the final name
   without a check-then-overwrite race.
3. [`internal/audio/probe.go`](internal/audio/probe.go) and
   [`internal/audio/pcm.go`](internal/audio/pcm.go): supported formats and
   precision constraints; per-channel hashing of interleaved, left-aligned
   32-bit integer PCM with zero low padding bits, plus exact sample counts.
4. [`internal/av/native.go`](internal/av/native.go) and
   [`internal/av/native.c`](internal/av/native.c): synchronous cgo handles,
   cancellation/error propagation, native decoding, integer representation
   conversion, FIFO reblocking, encoding and final flushing.
   DTS:X uses compatible channels only, without an object or height renderer.
5. [`scripts/build-common.sh`](scripts/build-common.sh),
   [`scripts/build-release.sh`](scripts/build-release.sh) and
   [`tools/link-wrapper/main.go`](tools/link-wrapper/main.go): cross-platform
   builds, safe cleanup, static dependency checks and relink-input capture.
   See [`patches/ffmpeg/README.md`](patches/ffmpeg/README.md) for the encoder patch.
6. [`tests/integration/`](tests/integration/): native TrueHD synthetic-signal
   round trips and optional real DTS fixture tests. Synthetic round trips test
   the encoder; they do not replace real DTS decoding or hardware playback tests.
7. [`cmd/hdma2truehd-ui/main.go`](cmd/hdma2truehd-ui/main.go),
   [`internal/gui/service.go`](internal/gui/service.go) and
   [`frontend/app.js`](cmd/hdma2truehd-ui/frontend/app.js): desktop lifecycle,
   batch preflight and bounded parallel queue. Comments explain snapshot ownership,
   revision ordering, cancellation/shutdown waits and per-layout bitrate capture.
   [`tests/build/`](tests/build/) uses isolated directories and stub compilers to
   test build/cleanup orchestration, not native compilation or audio fidelity.

The E-AC-3 branch in [`internal/audio/eac3.go`](internal/audio/eac3.go) and
[`internal/av/eac3.c`](internal/av/eac3.c) handles synchronized independent/dependent
encoding, channel replacement, lossy validation and encoder delay separately
from TrueHD's lossless verification.

The reference project [HG3112/md71](https://github.com/HG3112/md71) produces
**lossy E-AC-3 7.1**, not TrueHD. This project's E-AC-3 approach references its
dependent-substream design (HellGauss, LGPL 2.1), but orchestrates the native
libraries in-process without invoking its executables or scripts.

### Building

Releases use **Go 1.26.x, cgo and a native C toolchain**, plus make, curl, tar, xz,
patch and a SHA-256 utility. The FFmpeg build script produces static libraries,
not command-line programs. These tools are build prerequisites, not requirements
for end users running the single executable.

The library path is fixed at `.backend/ffmpeg-8.0.1/`. By default, the platform
entry point builds the libraries, runs tests and static analysis, creates a
complete release with LGPL materials, verifies relinking and runs `doctor`.
Existing source directories are not overwritten, protecting local changes.
Explicitly reuse the libraries, or rename/back up the old source directory before
rebuilding. Existing release directories are also protected by default.

Use `--clean` (`-Clean` on Windows) to remove the selected release directory
before rebuilding. Cleanup permits only a direct child of the project's `dist/`
directory, rejects symlinks, and never removes `dist/` itself, `.backend/`, `bin/`,
`samples/` or other releases. Dependencies and the backend are checked before
cleanup; a later build failure does not restore the removed release.
`--clean` does not rebuild static libraries: combine it with `--reuse-backend`
when libraries already exist. `--binary-only` already replaces the executable
and cannot be combined with `--clean`.

**Clean and rebuild static libraries:** Pass `--clean-backend` (`-CleanBackend`
on Windows) to explicitly remove only the entire `.backend/ffmpeg-8.0.1/`
source/build directory, then extract, patch and compile it again.
Local changes inside that directory are lost. Symlinked backend parents or
source directories are rejected. The download archive and other backend backups
are preserved; cached archives are SHA-256 checked, downloading only if missing.
This option cannot be combined with library reuse or executable-only mode.
Add `--clean` / `-Clean` to also replace an existing release directory.
Tool, platform and output-path checks precede deletion; a later rebuild failure
does not restore the removed backend.

After replacing static libraries, use `go build -a` / `go test -a` to force
relinking rather than reusing a cached result linked against old external
archives.

**Upgrading an old backend:** This feature requires the marker
`8.0.1-hdma2truehd-71-v3` for MKV support. Old `v1` / `v2` static libraries cannot be reused.
Rename `.backend/ffmpeg-8.0.1/` to an unused backup directory, then run a full
platform build without `--reuse-backend` / `-ReuseBackend`. An executable-only
build is insufficient; the program rejects the old backend.

Both platforms share one build workflow and always link four FFmpeg `.a`
archives through cgo. They do not depend on system-installed FFmpeg shared
libraries or executables:

| Mode | macOS option | Windows option | Behavior |
|---|---|---|---|
| Full release (default) | None | None | Build CLI/UI together, sharing sources and notices; relink each and run CLI self-tests |
| Full release with reused libraries | `--reuse-backend` | `-ReuseBackend` | Reuse libraries; keep all other release steps |
| Full release with a fresh backend | `--clean-backend` | `-CleanBackend` | Remove the current FFmpeg source/build tree, rebuild libraries and CLI/UI; optionally clean the release too |
| Local executable-only build | `--binary-only` | `-BinaryOnly` | Require existing native static libraries; build both applications and check dynamic dependencies |
| Single target | `--target cli` / `--target ui` | `-Target cli` / `-Target ui` | Select one application in release or local mode; default is `both` |

Executable-only mode does not download/rebuild FFmpeg, run tests or `doctor`,
or package a release. It does not require the original FFmpeg source archive,
make, curl, tar, xz or patch, but still needs Go, a native C toolchain and
macOS `otool` or the matching Windows MSYS2 environment's `objdump` and `windres`.
It also uses `go build -a` and replaces
the existing applications. macOS also gets an app bundle with its icon.
Single-target local builds leave the other existing application untouched.
It is for local development and **does not replace a
complete release with LGPL materials**.

#### macOS

Install Xcode Command Line Tools and the other build prerequisites, then run:

```sh
export GOTOOLCHAIN=go1.26.10
bash scripts/build-macos.sh

# Reuse native static libraries and choose a new release directory
bash scripts/build-macos.sh --reuse-backend --output-dir dist/macos-new

# Clean and rebuild the default release without rebuilding FFmpeg
bash scripts/build-macos.sh --reuse-backend --clean

# Rebuild FFmpeg too (local changes in its source directory will be lost)
bash scripts/build-macos.sh --clean-backend --clean

# Local development: rebuild CLI, UI and macOS app bundle in bin/
bash scripts/build-macos.sh --binary-only

# Build only the CLI in a custom directory
bash scripts/build-macos.sh --binary-only --target cli --output-dir bin/dev

# Build only the UI (including its app bundle)
bash scripts/build-macos.sh --binary-only --target ui

# Build both applications in a custom directory
bash scripts/build-macos.sh --binary-only --output-dir bin/dev
```

#### Windows

Install Go and [MSYS2](https://www.msys2.org/). On the first build, the entry point
can install and check MSYS2 dependencies:

```powershell
.\scripts\build-windows.ps1 -InstallDependencies
```

Use native amd64 Go with UCRT64 GCC on x64 hosts, or native arm64 Go with
CLANGARM64 Clang on ARM64 hosts. The script chooses the environment from Go's
architecture and rejects mismatched host/compiler architectures.

`-InstallDependencies` runs `pacman -S --needed` in the selected MSYS2 UCRT64 or CLANGARM64
environment to install missing build packages. Pacman asks for confirmation;
installation failure stops the build. After installation, the script rechecks
tools and continues. It does not install Go/MSYS2, upgrade the system or change
the execution policy. The switch is not needed for subsequent builds.
Combined with `-BinaryOnly`, it installs only the matching compiler, linker and resource tools, not the additional
full-release tools.

Alternatively, install the same full prerequisites manually in a **UCRT64**
terminal:

```sh
pacman -S --needed make diffutils patch curl tar xz mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-binutils
```

For ARM64, install from a **CLANGARM64** terminal instead:

```sh
pacman -S --needed make diffutils patch curl tar xz mingw-w64-clang-aarch64-clang mingw-w64-clang-aarch64-llvm-tools mingw-w64-clang-aarch64-lld mingw-w64-clang-aarch64-compiler-rt
```

Then run from the project directory in **PowerShell**:

```powershell
.\scripts\build-windows.ps1

# MSYS2 installed in a non-default location
.\scripts\build-windows.ps1 -Msys2Root 'D:\msys64'

# Select MSYS2 and install missing build dependencies
.\scripts\build-windows.ps1 -Msys2Root 'D:\msys64' -InstallDependencies

# Reuse existing Windows static libraries
.\scripts\build-windows.ps1 -ReuseBackend -OutputDir 'dist\windows-new'

# Clean and rebuild the default release without rebuilding FFmpeg
.\scripts\build-windows.ps1 -ReuseBackend -Clean

# Rebuild FFmpeg too (local changes in its source directory will be lost)
.\scripts\build-windows.ps1 -CleanBackend -Clean

# Local development: rebuild both EXEs in bin\
.\scripts\build-windows.ps1 -BinaryOnly

# Build only the CLI in a custom directory
.\scripts\build-windows.ps1 -BinaryOnly -Target cli -OutputDir 'bin\dev'

# Build only the UI
.\scripts\build-windows.ps1 -BinaryOnly -Target ui

# Build both applications in a custom directory
.\scripts\build-windows.ps1 -BinaryOnly -OutputDir 'bin\dev'
```

The default MSYS2 path is `C:\msys64`; it can also be set with `MSYS2_ROOT`.
The PowerShell entry point starts the matching MSYS2 UCRT64 / CLANGARM64 login shell and inherits the
system PATH. By default it only checks dependencies; MSYS2 packages are installed
only when `-InstallDependencies` is explicitly supplied. It does not change the
execution policy or permanently modify environment variables.

Native Go for the selected architecture must be on the Windows PATH.
FFmpeg and Go/cgo must use matching target architectures and
toolchains. `CGO_ENABLED=0` or a pure-Go cross build with only `GOOS=windows`
cannot replace a native build. Windows linking uses static compiler runtimes;
actual system DLL dependencies and compatibility still need native Windows
CI/runtime validation.

Full releases default to `dist/hdma2truehd-<os>-<arch>/` on both platforms,
containing CLI, UI and the macOS `HDMA Converter.app`.
CLI relinking inputs live in `relink/`; UI inputs live in `relink-ui/`.
FFmpeg sources and licenses are packaged once. Single-target releases use
`relink/`; UI-only releases default to `dist/hdma2truehd-ui-<os>-<arch>/`.
Executable-only builds place both applications in `bin/`, including
`bin/HDMA Converter.app` on macOS.
macOS checks that dynamic dependencies are system libraries only; Windows checks
that there are no FFmpeg/GCC runtime DLL dependencies. Relative output directories
are resolved from the project root.

The lower-level `build-ffmpeg.sh` and `build-release.sh` remain shared steps to
avoid duplicating FFmpeg configuration and licensing logic. The Go linker helper
lives in `tools/link-wrapper/` and participates in regular `go test ./...`, rather
than being mixed with shell/PowerShell scripts. Relinking documentation is in
`docs/RELINKING.md`.

### Usage

**Place options before input paths.** Quote paths containing spaces:

```sh
hdma2truehd doctor
hdma2truehd probe "input.dts"
hdma2truehd convert -o "output.thd" "input.dts"
hdma2truehd convert "input.dtshd"
hdma2truehd convert --output-dir "converted" "film-5.1.dts" "film-7.1.dtshd"
hdma2truehd convert --jobs 4 --output-dir "converted" "film-5.1.dts" "film-7.1.dtshd"
hdma2truehd convert --overwrite -o "output.thd" "input.dts"
hdma2truehd convert --skip-verify -o "output.thd" "input.dts"
hdma2truehd convert --format eac3 -o "output.eac3" "input.dts"
hdma2truehd convert --format both "input.dts"
hdma2truehd convert --format both --eac3-bitrate 1536 "input-7.1.dts"
```

`--format` accepts `truehd` (default), `eac3` or `both`. Outputs default to the
input directory and basename with `.thd` / `.eac3` extensions.
Dual-format conversion cannot use `-o`; use `--output-dir` instead.
Outputs are verified and published independently, not atomically as a pair;
one failure does not delete the other successfully published output.

The `--ffmpeg` and `--ffprobe` options have been removed; no external executable
paths are needed. `doctor` checks the linked version and codec capabilities,
then performs actual 24-bit PCM round trips for all three layouts. It does not
replace real MA fixture testing or hardware certification.
It also encodes and fully decodes E-AC-3 5.1(side)/7.1 to check format and length.

Default TrueHD verification additionally decodes the complete source and output, so it
takes extra processing time but uses bounded memory. It compares speaker labels,
all effective integer PCM bits, per-channel SHA-256 and exact sample counts.
Approximate duration, floating-point tolerances, automatic alignment and trimming
are not used to conceal differences. Skipping the full comparison explicitly
marks the result `UNVERIFIED`; it is not presented as proven lossless.

#### E-AC-3 lossy encoding boundaries

- Only 48 kHz `5.1(side)` or `7.1` is accepted, without automatic resampling
  or layout changes. Higher sample rates and back-surround `5.1` can still be
  converted to TrueHD, but the E-AC-3 branch explicitly rejects them.
- Default total bitrate: **960 kb/s for 5.1**, **1536 kb/s for 7.1**.
  `--eac3-bitrate` is in kb/s: 384–1024 for 5.1, 768–1536 for 7.1, in steps
  of 32. `0` selects the layout-specific default.
- 7.1 uses an independent 5.1 core plus a four-channel dependent substream,
  splitting total bitrate **5:3**. The core contains a compatibility surround
  downmix; full decoding replaces those surrounds and adds the rears from the
  dependent stream. All eight original channels remain separately encoded in
  the final presentation; this is not a 5.1 upmix.
- Raw `.eac3` has no gapless trim metadata. It retains **256 samples of encoder
  delay (about 5.33 ms)** and final-frame zero padding. For `N` source samples,
  decoded length must be `ceil((N + 256) / 1536) * 1536`. Handle synchronization
  correctly when remuxing; do not assume sample-aligned boundaries with TrueHD.
- Default validation fully decodes the output and checks codec, 48 kHz, original
  layout and the exact length above. It reports `VALIDATED (LOSSY)`, not
  lossless `VERIFIED`. `--skip-verify` skips full decoding and reports
  `UNVERIFIED`, while retaining output format checks.
- Experimental 7.1 requires a player that supports dependent substreams.
  Core-only devices may play just 5.1. Software tests do not provide Dolby,
  Blu-ray or hardware certification, or guarantee subjective audio quality.

Each job uses a unique temporary directory on the destination filesystem.
Publication happens only after encoding, metadata checks and, by default, full
PCM verification succeed. Outputs are not overwritten by default. Non-overwrite
publication uses an atomic hard link, so the destination filesystem must support
hard links, such as APFS or NTFS. Unsupported filesystems produce an explicit
error rather than an unsafe fallback. `--overwrite` also replaces an existing
output only after the new result succeeds.

Batch jobs default to **2** concurrent conversions. Use `--jobs N` for **1–8**
or `--jobs 1` for serial operation. A job is one track in one output format;
tracks and formats from the same file may run concurrently. Higher concurrency
increases CPU, memory and disk load. A job failure does not stop other jobs;
results are summarized at the end. Output-name collisions are rejected before
processing. Progress includes job indices and output paths to identify each job.
Ctrl+C stops scheduling, cancels all active native operations and waits for
temporary-file cleanup, preserving published outputs.
Exit codes: `0` success, `1` codec/verification failure,
`2` argument error, `130` cancellation.

### Experimental support matrix

| Item | Scope |
|---|---|
| Input | Local single-audio-stream files or explicitly selected MKV audio tracks confirmed by the libraries as `dts` / `DTS-HD MA`, `DTS-HD MA + DTS:X` or `DTS-HD MA + DTS:X IMAX` |
| Layouts | `5.1(side)`, `5.1` (back surrounds), `7.1` (preserves both side and back surrounds) |
| Effective precision | 16 or 24 bits; unknown, 20-bit and 32-bit inputs are explicitly rejected |
| Sample rates | 44.1, 48, 88.2, 96, 176.4 and 192 kHz; no resampling |
| Output | Raw TrueHD `.thd`, without an AC-3 companion stream |
| Optional lossy output | E-AC-3 `.eac3`, 48 kHz / 5.1(side) or 7.1 only; no Atmos |
| Codec round trips | Three layouts × six sample rates × two bit depths × eight lengths: 288 cases |
| Real MA validation | Two local approximately 30-second samples passed on macOS: 48 kHz, 5.1(side)/16 bit and 7.1/24 bit; full PCM verification and external FFmpeg decoding matched |
| Real DTS:X validation | A local approximately 30-second 48 kHz / 24 bit / 7.1 clip passed on macOS; per-channel PCM verification and external FFmpeg decoding matched; real DTS:X 5.1 / IMAX fixtures remain unverified |

This matrix describes experimental software codec capabilities, not player,
AV receiver or Blu-ray support. In particular, 7.1 at 176.4/192 kHz must not be
treated as Blu-ray-compatible output. Dolby publishes Blu-ray limits of eight
channels at 96 kHz and six channels at 192 kHz.

7.1 output does not include an additional 5.1 downmix presentation. For a specific
device or disc-authoring workflow, separately validate bitrate, presentations
and playback compatibility.

Lossy DTS core or DTS-HD HRA cannot be accepted as MA, and unknown layouts are
not guessed. MKV output/remuxing, DTS:X object metadata retention, Atmos generation
and parallel batch processing are not provided.

### Testing

#### Batch-convert local samples with the main executable

Build a release as described above, place `.dts` / `.dtshd` files you are authorized
to test in `samples/`, then run:

```sh
# macOS: discover the release executable in dist
bash scripts/test-samples.sh

# Explicitly replace existing .thd files and repeat full verification
bash scripts/test-samples.sh --overwrite

# Test a specific release executable
bash scripts/test-samples.sh --binary ./dist/hdma2truehd-darwin-arm64/hdma2truehd

# Test both formats and explicitly replace earlier test outputs
bash scripts/test-samples.sh --format both --overwrite
```

On Windows, run from PowerShell; MSYS2 Bash is not required for testing:

```powershell
# Discover the release executable in dist
.\scripts\test-samples.ps1

# Explicitly replace existing .thd files and repeat full verification
.\scripts\test-samples.ps1 -Overwrite

# Test a specific release executable
.\scripts\test-samples.ps1 -Binary '.\dist\hdma2truehd-windows-amd64\hdma2truehd.exe'

# Test both formats
.\scripts\test-samples.ps1 -Format both -Overwrite
```

By default, the test scripts search the repository's `dist/` directory and its
immediate release subdirectories for the ordinary executable: `hdma2truehd`
in Bash (`.exe` on Windows), or `hdma2truehd.exe` in PowerShell.
One match is printed and used. No matches produce instructions to build a release;
multiple matches are listed and require an explicit `--binary` / `-Binary`
selection. The scripts do not automatically select the newest release.

They never select `hdma2truehd-relinked`, no longer default to `bin/`, and do not
build automatically. To test an executable-only build, explicitly pass
`--binary ./bin/hdma2truehd` or `-Binary '.\bin\hdma2truehd.exe'`.

The scripts scan only top-level inputs in `samples/`, matching extensions
case-insensitively. They write same-basename `.thd` / `.eac3` outputs there,
never treat existing outputs as inputs, and do not modify sources.
TrueHD-only remains the default; select formats with `--format` / `-Format`
and total bitrate with `--eac3-bitrate` / `-Eac3Bitrate`.
Full format-appropriate verification is kept; existing outputs are not overwritten
by default. Any conversion failure produces a nonzero exit code.

Scripts may be launched from any working directory; relative `--binary` /
`-Binary` paths are resolved from the caller's working directory. PowerShell
passes through the executable's exit code. Testing itself requires neither
MSYS2, Go nor an external FFmpeg. The Bash test script can also run in MSYS2
UCRT64 / CLANGARM64 Bash on Windows, matching the native architecture.

#### Automated tests

Build the static libraries first. Native integration tests no longer depend on
FFmpeg commands or backend-path environment variables:

```sh
go test -race ./...
go vet ./...

# Optional: decode with another FFmpeg executable as an additional check
HDMA_TEST_DECODER=/absolute/path/to/ffmpeg \
go test ./tests/integration -run TestTrueHDRoundTrip -count=1
```

The external decoder is an **optional test tool only**; the product has no
corresponding subprocess path. Another FFmpeg still belongs to the same decoder
family and does not constitute certification by an independent implementation.

Real-fixture tests do not automatically select or download inputs. Use environment
variables to specify local files you are authorized to test:

```sh
HDMA_TEST_DTS_51="/absolute/path/to/5.1.dts" \
HDMA_TEST_DTS_71="/absolute/path/to/7.1.dtshd" \
HDMA_TEST_DTSX_51="/absolute/path/to/dtsx-5.1.dts" \
HDMA_TEST_DTSX_71="/absolute/path/to/dtsx-7.1.dts" \
go test ./tests/integration -run TestDTSHDMAConversion -count=1 -v
```

Each of the four fixture variables is optional. DTS:X tests require one of the
two DTS:X profiles listed above and perform full compatible-channel PCM
verification. Ordinary MA is not substituted for missing DTS:X coverage.
Eligible 48 kHz 5.1(side)/7.1 fixtures also exercise E-AC-3 conversion.
Native E-AC-3 tests cover short inputs, frame boundaries, bitrate and delay/padding,
and use distinct per-channel tones to check routing and measurable SNR thresholds.
These thresholds are not a general audio-quality guarantee.

Missing real fixtures explicitly produce `SKIP`. CI is configured to build the
native libraries on macOS/Windows, run tests, package complete release materials
and actually relink the executable. The presence of CI configuration does not
prove that a remote run has succeeded.

### Single-executable releases and LGPL relinking

Runtime requires only one executable. macOS still depends on operating-system
frameworks and libraries; "single executable" does not mean statically linking
the entire operating system.

**Statically linking FFmpeg does not permit distributing only the executable
while omitting required LGPL materials.** Generate a release directory whose
contents must be packaged together:

```sh
bash scripts/build-release.sh
# Defaults to dist/hdma2truehd-<os>-<arch>/; refuses an existing directory

bash dist/hdma2truehd-darwin-arm64/relink/relink.sh
dist/hdma2truehd-darwin-arm64/hdma2truehd-relinked doctor
bash dist/hdma2truehd-darwin-arm64/relink-ui/relink.sh
```

The release contains:

- CLI and UI executables, plus the macOS app bundle; use `--target cli` or
  `--target ui` to package just one application.
- Relinkable Go/cgo objects, FFmpeg static libraries, linker scripts, Windows
  export definitions when present, and the captured external link command.
  Relinking does not depend on Go's temporary build directory.
- The original FFmpeg source archive, patches, complete configured source tree,
  build scripts and toolchain information.
- LGPL and Go licenses, plus [relinking instructions](docs/RELINKING.md).

On the same platform/architecture, users can replace compatible FFmpeg libraries
and run `relink.sh`. Different SDK paths may require adjustments; ABI/header
changes require recompiling the application. Distribute the complete release
directory and verify license obligations for the actual distribution method.

The encoder patches follow the upstream files' LGPL 2.1-or-later terms.
See the [backend notes](patches/ffmpeg/README.md) for versions, checksums and
caveats. Open-source licensing is separate from Dolby/DTS trademarks, patent
permissions and product certification.
