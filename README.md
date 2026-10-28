# hdma2truehd

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

## 架构

- Go：CLI、任务编排、路径安全、逐声道哈希和最终输出发布。
- cgo/C 桥接：使用 `libavformat` 探测与封装、`libavcodec` 解码与 TrueHD 编码。
- `libswresample`：只转换整数样本的平面/交错和容器表示；输入输出采样率、
  声道标签完全相同，关闭抖动，不执行重采样或降混，并检查所有有效位。
- 进程内处理检查 context 取消，FFmpeg I/O 配置中断回调；
  出错时释放 decoder、encoder、packet、frame、FIFO 和格式上下文。

普通 FFmpeg 的原生 TrueHD 编码器未提供 7.1 编码支持。本项目固定使用
FFmpeg 8.0.1 及 [TrueHD 补丁](patches/ffmpeg/README.md)，处理八声道
restart header、声道顺序和极短输入 flush。库版本在运行时检查。

参考项目 [HG3112/md71](https://github.com/HG3112/md71) 生成的是**有损 E-AC-3 7.1**，
不是 TrueHD。本项目不使用其有损算法或核心/依赖流拼接代码。

## 构建

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

### macOS

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

### Windows

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

## 使用

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

## 实验性支持矩阵

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

## 测试

### 用主程序批量转换本地样本

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

### 自动化测试

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

真实样本不随仓库提供，也不自动下载。用自己有权测试的本地文件补充：

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

## 单文件发布与 LGPL 重新链接

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
