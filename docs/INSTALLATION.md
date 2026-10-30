# 下载与启动 / Download and launch

## 中文

### 下载与校验

仅从项目 GitHub Releases 页面下载匹配系统和 CPU 架构的完整归档及对应
`.sha256` 文件。macOS 使用 `.tar.gz`；Windows 使用 `.zip`。
不要仅提取可执行文件后重新分发：源码、许可证和重新链接材料应一并保留。

将归档与对应 `.sha256` 放在同一目录，在解压前验证：

```sh
# macOS：把示例中的文件名替换为实际下载的归档名
shasum -a 256 -c hdma2truehd-v1.0.0-darwin-arm64.tar.gz.sha256
tar -xzf hdma2truehd-v1.0.0-darwin-arm64.tar.gz
```

```powershell
# Windows PowerShell：示例为 x64；ARM64 请替换文件名
$archive = 'hdma2truehd-v1.0.0-windows-amd64.zip'
$expected = ((Get-Content -LiteralPath "$archive.sha256" -Raw).Trim() -split '\s+')[0]
if ($expected -notmatch '^[0-9a-fA-F]{64}$') { throw 'Invalid checksum file' }
$actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash
if ($actual -ne $expected) { throw 'Checksum mismatch; do not launch' }
Expand-Archive -LiteralPath $archive -DestinationPath .
```

校验和用于发现内容变化，不证明发布者身份；应从可信发布页面同时取得文件和校验值。
校验不一致、架构错误或系统报告恶意软件时，不要继续启动。

### macOS

- Apple Silicon 下载 `darwin-arm64`；Intel 下载 `darwin-amd64`。
- 最低系统要求为 macOS 12.0；最低版本实机验证状态见发布验收清单。
- 解压后打开发布目录内的 `HDMA Converter.app`。CLI 可在终端运行
  `./hdma2truehd version` 和 `./hdma2truehd doctor`。
- 本版不提供 Developer ID 身份签名或公证；工具链可能生成临时签名
  （ad-hoc），它不证明发布者身份。浏览器下载后的首次启动可能被 Gatekeeper 拦截。
- 仅在核对来源与校验和、且提示是未识别开发者而非恶意软件警报时，按 macOS
  提示在“系统设置 → 隐私与安全性”中检查是否提供“仍要打开”，并再次确认。
  受管理设备可能不允许这样操作，应联系管理员。
- 不要关闭 Gatekeeper、删除隔离属性或绕过恶意软件警报。无法确认文件安全时停止操作。

### Windows

- x64 下载 `windows-amd64`，要求 Windows 10 22H2（build 19045）或更高版本。
- ARM64 下载 `windows-arm64`，要求 Windows 11 24H2（build 26100）或更高版本。
- 桌面 UI 需要匹配架构的 Microsoft Edge WebView2 Evergreen Runtime；
  缺失时从微软官方来源安装，不要从第三方站点下载运行库。
- 运行 `hdma2truehd-ui.exe`；CLI 使用 `.\hdma2truehd.exe version`、
  `.\hdma2truehd.exe doctor`。
- 本版没有 Authenticode 发布者签名，可能出现未知发布者或 SmartScreen 提示。
  仅在来源和校验和确认无误、没有恶意软件警报且设备政策允许时，才考虑提示中的
  “更多信息 → 仍要运行”。不要关闭 Defender/SmartScreen 或绕过组织策略。

### 数据、诊断和许可证

先将发布目录放到有写权限的位置；诊断日志保存在程序旁边的 `logs` 中。
日志可能包含本机文件路径和媒体元数据；分享前请自行检查和脱敏。
默认不覆盖已有输出，只有显式选择覆盖才会在验证后替换文件。

项目自身代码使用归档根目录 `LICENSE` 中的 MIT 许可证。
FFmpeg、其补丁和其他第三方组件保留各自许可；MIT 不将它们重新授权。
保留 `licenses`、`source`、`relink` 和 `relink-ui`（若有），参见
[重新链接说明](RELINKING.md)。本工具没有 Dolby/DTS 认证，也不保证硬件播放或 Blu-ray 合规。

## English

### Download and verify

Download the complete archive matching your OS/CPU and its `.sha256` file only
from the project's GitHub Releases page. macOS uses `.tar.gz`; Windows uses
`.zip`. Retain source, license and relinking materials when redistributing.

Place the archive and checksum file in the same directory. Before extraction,
run the macOS or PowerShell commands above with the actual archive name.
A checksum detects changed bytes, not publisher identity; obtain both files
from the trusted release page. Do not launch on a mismatch, wrong architecture
or malware alert.

### macOS

- Apple Silicon: `darwin-arm64`; Intel: `darwin-amd64`. Minimum: macOS 12.0.
  Minimum-system device testing is tracked separately in the release checklist.
- Open `HDMA Converter.app` after extraction. For CLI checks run
  `./hdma2truehd version` and `./hdma2truehd doctor`.
- This release is not Developer ID signed or notarized. A toolchain-generated
  ad-hoc signature does not establish publisher identity. Gatekeeper may block
  the first launch of a browser-downloaded application.
- Only after checking provenance/checksums, and only for an unidentified
  developer warning rather than a malware alert, inspect System Settings >
  Privacy & Security for the system-provided Open Anyway option and confirm.
  Managed devices may prohibit this; contact the administrator.
- Do not disable Gatekeeper, remove quarantine attributes or bypass malware
  alerts. Stop if you cannot establish that the download is trustworthy.

### Windows

- x64: `windows-amd64`, Windows 10 22H2 (build 19045) or newer.
- ARM64: `windows-arm64`, Windows 11 24H2 (build 26100) or newer.
- Desktop UI requires the matching Microsoft Edge WebView2 Evergreen Runtime.
  Install missing runtimes only from Microsoft.
- Run `hdma2truehd-ui.exe`; CLI checks are `.\hdma2truehd.exe version` and
  `.\hdma2truehd.exe doctor`.
- This release has no Authenticode publisher signature. Unknown-publisher or
  SmartScreen prompts may appear. Consider the system's More info > Run anyway
  option only after checking provenance/checksums, without malware alerts and
  where device policy permits. Do not disable Defender/SmartScreen or bypass
  organization policy.

### Data, diagnostics and licenses

Extract to a writable location; diagnostics are saved in `logs` beside the
application. Logs may contain local paths and media metadata; review and redact
before sharing. Existing outputs are preserved unless overwrite is explicitly
enabled, and replacement happens only after validation.

The project's own code uses the MIT license in the archive's root `LICENSE`.
FFmpeg, its patches and other third-party components retain their own licenses;
MIT does not relicense them. Retain `licenses`, `source`, `relink` and
`relink-ui` where present; see [relinking instructions](RELINKING.md).
There is no Dolby/DTS certification or guarantee of hardware playback or
Blu-ray compliance.
