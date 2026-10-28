# hdma2truehd

[简体中文](#简体中文) | [English](#english)

## 简体中文

将 **DTS-HD MA 音频（含 DTS:X 兼容声道）转换为 Dolby TrueHD `.thd`** 的实验性 Go 命令行工具，
面向 macOS 和 Windows。通过 **cgo 静态链接 FFmpeg 库**，探测、解码、编码及
PCM 校验均在进程内完成；运行时不启动 `ffmpeg` / `ffprobe`，也不需要安装它们。

支持 5.1、7.1，默认完整比较逐声道 PCM SHA-256 和精确采样数；
不自动降混、重采样或降低有效精度。

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

### 架构

- Go：CLI、任务编排、路径安全、逐声道哈希和最终输出发布。
- cgo/C 桥接：使用 `libavformat` 探测与封装、`libavcodec` 解码与 TrueHD 编码。
- `libswresample`：只转换整数样本的平面/交错和容器表示；输入输出采样率、
  声道标签完全相同，关闭抖动，不执行重采样或降混，并检查所有有效位。
- 进程内处理检查 context 取消，FFmpeg I/O 配置中断回调；
  出错时释放 decoder、encoder、packet、frame、FIFO 和格式上下文。

普通 FFmpeg 的原生 TrueHD 编码器未提供 7.1 编码支持。本项目固定使用
FFmpeg 8.0.1 及 [TrueHD 补丁](patches/ffmpeg/README.md)，处理八声道
restart header、声道顺序和极短输入 flush。库版本在运行时检查。

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

参考项目 [HG3112/md71](https://github.com/HG3112/md71) 生成的是**有损 E-AC-3 7.1**，
不是 TrueHD。本项目不使用其有损算法或核心/依赖流拼接代码。

### 构建

需要 **Go 1.24+、cgo 和原生 C 工具链**，以及 make、curl、tar、xz、patch 和
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
替换静态库后应使用 `go build -a` / `go test -a` 强制重新链接，避免 Go 缓存
继续使用外部库的旧版本。

两平台共用构建流程，始终通过 cgo 链接四个 FFmpeg `.a` 静态库，
不依赖系统安装的 FFmpeg 动态库或命令行程序：

| 模式 | macOS 参数 | Windows 参数 | 行为 |
|---|---|---|---|
| 完整发布（默认） | 无 | 无 | 构建静态库、测试、打包 LGPL 材料、重新链接及自检 |
| 复用库完整发布 | `--reuse-backend` | `-ReuseBackend` | 复用现有库，其余发布步骤不变 |
| 日常仅编译主程序 | `--binary-only` | `-BinaryOnly` | 必须已有本机静态库；仅编译主程序并检查动态依赖 |

日常模式不下载、不重建 FFmpeg，不执行测试、`doctor` 或发布打包；
无需 FFmpeg 原始源码归档及 make、curl、tar、xz、patch，但仍需 Go、原生 C 工具链，
以及 macOS 的 `otool` 或 Windows UCRT64 的 `objdump`。
该模式也使用 `go build -a`，避免替换静态库后误用缓存；已有主程序会被重新编译覆盖。
它只用于本地开发，**不能代替附带 LGPL 材料的完整发布目录**。

#### macOS

安装 Xcode Command Line Tools 并准备其他构建工具后：

```sh
bash scripts/build-macos.sh

# 已有本机静态库时复用，并选择新的发布目录
bash scripts/build-macos.sh --reuse-backend --output-dir dist/macos-new

# 清理默认发布目录后重建，不重新编译 FFmpeg 静态库
bash scripts/build-macos.sh --reuse-backend --clean

# 日常开发：复用静态库，只编译 bin/hdma2truehd
bash scripts/build-macos.sh --binary-only

# 仅编译主程序到指定目录
bash scripts/build-macos.sh --binary-only --output-dir bin/dev
```

#### Windows

安装 Go 和 [MSYS2](https://www.msys2.org/)。首次可由构建入口安装并校验 MSYS2 依赖：

```powershell
.\scripts\build-windows.ps1 -InstallDependencies
```

`-InstallDependencies` 在所选 MSYS2 的 UCRT64 环境中执行
`pacman -S --needed`，安装缺失的构建包；由 pacman 提示确认，失败则立即停止，
成功后重新检查工具并继续构建。它不安装 Go/MSYS2、不执行系统升级，
也不会自动修改执行策略。后续构建无需再次指定该参数。
搭配 `-BinaryOnly` 时仅安装 GCC 和 binutils，不安装完整发布所需的其他工具。

也可以在 **UCRT64** 终端手动安装同样的完整依赖：

```sh
pacman -S --needed make diffutils patch curl tar xz mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-binutils
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

# 日常开发：复用静态库，只编译 bin\hdma2truehd.exe
.\scripts\build-windows.ps1 -BinaryOnly

# 仅编译主程序到指定目录
.\scripts\build-windows.ps1 -BinaryOnly -OutputDir 'bin\dev'
```

默认 MSYS2 路径为 `C:\msys64`，也可以通过 `MSYS2_ROOT` 环境变量指定。
PowerShell 入口会自动启动 MSYS2 UCRT64 登录 shell，并继承系统 PATH。
默认只检查依赖，仅显式指定 `-InstallDependencies` 时安装 MSYS2 构建包；
不会改变执行策略或永久修改环境变量。
Go 必须在 Windows PATH 中；当前 Windows 构建入口要求 amd64 Go 和 UCRT64 GCC。
FFmpeg 与 Go/cgo 必须使用匹配的目标架构和工具链；
不能用 `CGO_ENABLED=0` 或只设置 `GOOS=windows` 的纯 Go 交叉编译代替原生构建。
Windows 链接配置使用静态 GCC 运行库；仍需通过 Windows 原生 CI/运行验证
确认实际系统 DLL 依赖及兼容性。

两个平台完整发布默认输出 `dist/hdma2truehd-<os>-<arch>/`，
仅编译主程序默认输出 `bin/hdma2truehd` 或 `bin/hdma2truehd.exe`。
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
hdma2truehd convert --overwrite -o "output.thd" "input.dts"
hdma2truehd convert --skip-verify -o "output.thd" "input.dts"
```

`--ffmpeg` 和 `--ffprobe` 参数已移除；无需指定外部程序。
`doctor` 检查链接版本、编解码能力，并实际执行三种布局的 24 位 PCM 回环；
它不能代替真实 MA 文件验收或硬件认证。

默认校验会另外完整解码源文件和输出文件，因此需要额外处理时间，但内存有界。
比较声道标签、整数 PCM 的所有有效位、每个声道的 SHA-256 和精确采样数。
不使用时长近似、浮点容差、自动对齐或裁剪来掩盖差异。
跳过完整比较时明确标记 `UNVERIFIED`，不宣称已证明无损。

每个任务在目标文件系统创建唯一临时目录，编码、元数据检查及默认 PCM 校验
成功后才发布。默认不覆盖；无需覆盖时使用原子硬链接，因此目标文件系统必须
支持硬链接（如 APFS、NTFS）。不支持时明确报错，不采用不安全回退。
`--overwrite` 也只在新结果成功后替换原输出。

批量任务默认串行，单文件失败不阻止后续文件，最后统一汇总；同名输出冲突在
开始前拒绝。Ctrl+C 取消当前原生操作、清理临时文件，并保留已发布的结果。
退出码：`0` 成功，`1` 编解码/校验失败，`2` 参数错误，`130` 取消。

### 实验性支持矩阵

| 项目 | 范围 |
|---|---|
| 输入 | 本地单音轨音频文件，库探测确认为 `dts` / `DTS-HD MA`、`DTS-HD MA + DTS:X` 或 `DTS-HD MA + DTS:X IMAX` |
| 布局 | `5.1(side)`、`5.1`（后环绕）、`7.1`（保留侧环绕和后环绕） |
| 有效精度 | 16 或 24 位；未知、20 位、32 位输入明确拒绝 |
| 采样率 | 44.1、48、88.2、96、176.4、192 kHz；不重采样 |
| 输出 | 原始 TrueHD `.thd`，无 AC-3 伴随流 |
| 编解码回环 | 三种布局 × 六种采样率 × 两种位深 × 八种长度，共 288 组 |
| 真实 MA 验收 | macOS 本地两份约 30 秒样本通过：48 kHz，5.1(side)/16 bit、7.1/24 bit；完整 PCM 校验及外部 FFmpeg 解码比较一致 |
| 真实 DTS:X 验收 | macOS 本地约 30 秒的 48 kHz / 24 bit / 7.1 片段通过；逐声道 PCM 校验及外部 FFmpeg 解码比较一致；尚无 DTS:X 5.1 / IMAX 真实样本验收 |

矩阵描述的是实验性软件编解码能力，不是播放器、功放或 Blu-ray 支持声明。
特别是 7.1 的 176.4/192 kHz 不应被当作 Blu-ray 兼容输出；Dolby 公布的
Blu-ray 上限是八声道 96 kHz、六声道 192 kHz。
7.1 输出不生成额外的 5.1 降混 presentation；如用于特定设备或光盘制作，
需另行验证码率、各 presentation 及播放兼容性。

不支持将 DTS 有损核心、DTS-HD HRA 冒充 MA，不自动猜测未知布局。
首版不提供 GUI、MKV 提取/封装、DTS:X 对象元数据保留、Atmos 生成或批量并行。

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
```

Windows 在 PowerShell 中运行，无需通过 MSYS2 Bash 启动测试：

```powershell
# 自动查找 dist 中的发布程序
.\scripts\test-samples.ps1

# 显式覆盖已有 .thd，重新转换并完整校验
.\scripts\test-samples.ps1 -Overwrite

# 测试指定的发布程序
.\scripts\test-samples.ps1 -Binary '.\dist\hdma2truehd-windows-amd64\hdma2truehd.exe'
```

测试脚本默认在仓库 `dist/` 及其一级发布子目录中查找普通版主程序：
Bash 为 `hdma2truehd`（Windows 下为 `.exe`），PowerShell 为 `hdma2truehd.exe`。
只找到一个时打印所选路径并运行；找不到则提示先构建发布版本，
找到多个则列出候选并要求通过 `--binary` / `-Binary` 显式选择，不自动选择最新版本。
不会选择 `hdma2truehd-relinked`，不再默认查找 `bin/`，也不自动构建。
如需测试仅编译主程序的结果，可显式指定 `--binary ./bin/hdma2truehd`
或 `-Binary '.\bin\hdma2truehd.exe'`。
扫描仓库 `samples/` 顶层的输入文件，扩展名不区分大小写，
将同名 `.thd` 写回该目录，不处理已有 `.thd`，也不修改输入。
默认保留完整 PCM 校验，不覆盖已有输出；任何转换失败均返回非零退出码。
可从任意工作目录运行脚本；`--binary` / `-Binary` 的相对路径以当前工作目录为准。
PowerShell 脚本直接传递主程序退出码；测试本身不需要 MSYS2、Go 或外部 FFmpeg。
Windows 也可在 MSYS2 UCRT64 Bash 中运行 Bash 测试脚本。

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
```

发布目录包含：

- 单文件程序。
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

An experimental Go command-line tool that **converts DTS-HD MA audio, including
DTS:X compatible channels, to Dolby TrueHD `.thd`** on macOS and Windows.
It **statically links FFmpeg libraries through cgo**: probing, decoding,
encoding and PCM verification all run in-process. No `ffmpeg` / `ffprobe`
executable is launched or required at runtime.

Supports 5.1 and 7.1, with complete per-channel PCM SHA-256 and exact sample-count
comparison enabled by default. No automatic downmixing, resampling or reduction
in effective precision is performed.

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

### Architecture

- Go: CLI, job orchestration, path safety, per-channel hashing and final output
  publication.
- cgo/C bridge: `libavformat` for probing and muxing; `libavcodec` for decoding
  and TrueHD encoding.
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

The reference project [HG3112/md71](https://github.com/HG3112/md71) produces
**lossy E-AC-3 7.1**, not TrueHD. This project does not use its lossy algorithms
or core/dependent-stream concatenation code.

### Building

Requires **Go 1.24+, cgo and a native C toolchain**, plus make, curl, tar, xz,
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

After replacing static libraries, use `go build -a` / `go test -a` to force
relinking rather than reusing a cached result linked against old external
archives.

Both platforms share one build workflow and always link four FFmpeg `.a`
archives through cgo. They do not depend on system-installed FFmpeg shared
libraries or executables:

| Mode | macOS option | Windows option | Behavior |
|---|---|---|---|
| Full release (default) | None | None | Build libraries, test, package LGPL materials, relink and self-test |
| Full release with reused libraries | `--reuse-backend` | `-ReuseBackend` | Reuse libraries; keep all other release steps |
| Local executable-only build | `--binary-only` | `-BinaryOnly` | Require existing native static libraries; build the executable and check dynamic dependencies |

Executable-only mode does not download/rebuild FFmpeg, run tests or `doctor`,
or package a release. It does not require the original FFmpeg source archive,
make, curl, tar, xz or patch, but still needs Go, a native C toolchain and
macOS `otool` or Windows UCRT64 `objdump`. It also uses `go build -a` and replaces
the existing executable. It is for local development and **does not replace a
complete release with LGPL materials**.

#### macOS

Install Xcode Command Line Tools and the other build prerequisites, then run:

```sh
bash scripts/build-macos.sh

# Reuse native static libraries and choose a new release directory
bash scripts/build-macos.sh --reuse-backend --output-dir dist/macos-new

# Clean and rebuild the default release without rebuilding FFmpeg
bash scripts/build-macos.sh --reuse-backend --clean

# Local development: reuse libraries and rebuild bin/hdma2truehd only
bash scripts/build-macos.sh --binary-only

# Build only the executable in a custom directory
bash scripts/build-macos.sh --binary-only --output-dir bin/dev
```

#### Windows

Install Go and [MSYS2](https://www.msys2.org/). On the first build, the entry point
can install and check MSYS2 dependencies:

```powershell
.\scripts\build-windows.ps1 -InstallDependencies
```

`-InstallDependencies` runs `pacman -S --needed` in the selected MSYS2 UCRT64
environment to install missing build packages. Pacman asks for confirmation;
installation failure stops the build. After installation, the script rechecks
tools and continues. It does not install Go/MSYS2, upgrade the system or change
the execution policy. The switch is not needed for subsequent builds.
Combined with `-BinaryOnly`, it installs only GCC and binutils, not the additional
full-release tools.

Alternatively, install the same full prerequisites manually in a **UCRT64**
terminal:

```sh
pacman -S --needed make diffutils patch curl tar xz mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-binutils
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

# Local development: rebuild bin\hdma2truehd.exe only
.\scripts\build-windows.ps1 -BinaryOnly

# Build only the executable in a custom directory
.\scripts\build-windows.ps1 -BinaryOnly -OutputDir 'bin\dev'
```

The default MSYS2 path is `C:\msys64`; it can also be set with `MSYS2_ROOT`.
The PowerShell entry point starts an MSYS2 UCRT64 login shell and inherits the
system PATH. By default it only checks dependencies; MSYS2 packages are installed
only when `-InstallDependencies` is explicitly supplied. It does not change the
execution policy or permanently modify environment variables.

Go must be on the Windows PATH. The current Windows build requires amd64 Go and
UCRT64 GCC. FFmpeg and Go/cgo must use matching target architectures and
toolchains. `CGO_ENABLED=0` or a pure-Go cross build with only `GOOS=windows`
cannot replace a native build. Windows linking uses the static GCC runtime;
actual system DLL dependencies and compatibility still need native Windows
CI/runtime validation.

Full releases default to `dist/hdma2truehd-<os>-<arch>/` on both platforms;
executable-only builds default to `bin/hdma2truehd` or `bin/hdma2truehd.exe`.
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
hdma2truehd convert --overwrite -o "output.thd" "input.dts"
hdma2truehd convert --skip-verify -o "output.thd" "input.dts"
```

The `--ffmpeg` and `--ffprobe` options have been removed; no external executable
paths are needed. `doctor` checks the linked version and codec capabilities,
then performs actual 24-bit PCM round trips for all three layouts. It does not
replace real MA fixture testing or hardware certification.

Default verification additionally decodes the complete source and output, so it
takes extra processing time but uses bounded memory. It compares speaker labels,
all effective integer PCM bits, per-channel SHA-256 and exact sample counts.
Approximate duration, floating-point tolerances, automatic alignment and trimming
are not used to conceal differences. Skipping the full comparison explicitly
marks the result `UNVERIFIED`; it is not presented as proven lossless.

Each job uses a unique temporary directory on the destination filesystem.
Publication happens only after encoding, metadata checks and, by default, full
PCM verification succeed. Outputs are not overwritten by default. Non-overwrite
publication uses an atomic hard link, so the destination filesystem must support
hard links, such as APFS or NTFS. Unsupported filesystems produce an explicit
error rather than an unsafe fallback. `--overwrite` also replaces an existing
output only after the new result succeeds.

Batch jobs run sequentially. A file failure does not stop later files; results
are summarized at the end. Output-name collisions are rejected before processing.
Ctrl+C cancels the active native operation, cleans temporary files and preserves
already published outputs. Exit codes: `0` success, `1` codec/verification failure,
`2` argument error, `130` cancellation.

### Experimental support matrix

| Item | Scope |
|---|---|
| Input | Local single-audio-stream files confirmed by the libraries as `dts` / `DTS-HD MA`, `DTS-HD MA + DTS:X` or `DTS-HD MA + DTS:X IMAX` |
| Layouts | `5.1(side)`, `5.1` (back surrounds), `7.1` (preserves both side and back surrounds) |
| Effective precision | 16 or 24 bits; unknown, 20-bit and 32-bit inputs are explicitly rejected |
| Sample rates | 44.1, 48, 88.2, 96, 176.4 and 192 kHz; no resampling |
| Output | Raw TrueHD `.thd`, without an AC-3 companion stream |
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
not guessed. The initial version does not provide a GUI, MKV extraction/muxing,
DTS:X object metadata retention, Atmos generation or parallel batch processing.

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
```

On Windows, run from PowerShell; MSYS2 Bash is not required for testing:

```powershell
# Discover the release executable in dist
.\scripts\test-samples.ps1

# Explicitly replace existing .thd files and repeat full verification
.\scripts\test-samples.ps1 -Overwrite

# Test a specific release executable
.\scripts\test-samples.ps1 -Binary '.\dist\hdma2truehd-windows-amd64\hdma2truehd.exe'
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
case-insensitively. They write same-basename `.thd` outputs there, do not process
existing `.thd` files, and do not modify inputs. Full PCM verification is kept;
existing outputs are not overwritten by default. Any conversion failure produces
a nonzero exit code.

Scripts may be launched from any working directory; relative `--binary` /
`-Binary` paths are resolved from the caller's working directory. PowerShell
passes through the executable's exit code. Testing itself requires neither
MSYS2, Go nor an external FFmpeg. The Bash test script can also run in MSYS2
UCRT64 Bash on Windows.

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
```

The release contains:

- The single executable.
- Relinkable Go/cgo objects, FFmpeg static libraries, linker scripts, Windows
  export definitions when present, and the captured external link command.
  Relinking does not depend on Go's temporary build directory.
- The original FFmpeg source archive, patch, complete configured source tree,
  build scripts and toolchain information.
- LGPL and Go licenses, plus [relinking instructions](docs/RELINKING.md).

On the same platform/architecture, users can replace compatible FFmpeg libraries
and run `relink.sh`. Different SDK paths may require adjustments; ABI/header
changes require recompiling the application. Distribute the complete release
directory and verify license obligations for the actual distribution method.

The encoder patch follows the upstream file's LGPL 2.1-or-later terms.
See the [backend notes](patches/ffmpeg/README.md) for versions, checksums and
caveats. Open-source licensing is separate from Dolby/DTS trademarks, patent
permissions and product certification.
