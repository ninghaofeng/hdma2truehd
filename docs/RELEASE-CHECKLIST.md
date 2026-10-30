# v1.0.0 发布验收 / Release acceptance

## 当前结论 / Current decision

**暂不可正式公开 / Not ready for public release.**

维护者已授权提交并推送本轮发布准备改动，以继续四平台 CI；修改标签和正式公开
仍需另行确认。本清单记录提交前的本机验收快照，后续 CI 以对应提交的 Actions 结果
为准，不把构建成功当作实机或硬件播放验收。

The maintainer authorized committing/pushing these preparation changes for CI.
Tag changes and publication still require separate confirmation. This is the
pre-commit local acceptance snapshot; consult Actions for subsequent commit
results. Build success is not device or hardware playback acceptance.

## 版本与 CI / Version and CI

- [x] 仓库公开 API 返回 200；`v1.0.0` Release API 返回 404，未发现公开的该版 Release。
  This does not distinguish an existing draft from no draft.
- [x] 保留源码版本 `v1.0.0`；本轮不移动现有标签。
- [x] 核对旧标签指向 `a3dafe1`，最新已提交候选为 `a2f2c12`，两者间有六项提交。
- [x] 查明上一轮 macOS ARM64 任务未取得 runner：`runner_id=0`，
  GitHub 注释为 hosted runner 无法分配、ARM64 容量不足；没有可下载的构建日志。
- [x] `a2f2c12` 的候选 CI 四个平台均已成功；分支构建的 Release 步骤按设计跳过。
  本轮许可证、打包和发布说明改动尚未包含在该提交中。
- [ ] 四个平台对**最终提交**全部通过 CI；不能使用旧提交或部分成功代替。
- [x] 维护者批准提交并推送本轮发布准备改动，继续核查四平台 CI。
- [ ] 维护者确认最终提交，再批准更新未公开的 `v1.0.0` 标签。
  不要仅重跑旧标签；它不会包含后续修复。
- [ ] 如期间发现 `v1.0.0` 已正式公开，改用新版本，不覆盖同名公开发行。

CI evidence:
[previous run](https://github.com/ninghaofeng/hdma2truehd/actions/runs/41095547361),
[candidate run](https://github.com/ninghaofeng/hdma2truehd/actions/runs/41097724297).
No four-platform success is claimed for uncommitted changes.

## 许可证与分发 / Licensing and distribution

- [x] 按维护者选择添加项目 [MIT 许可证](../LICENSE)。
- [x] 明确 MIT 不重新授权 FFmpeg、补丁或第三方依赖。
- [x] 完整发布脚本包含项目许可证及 [安装指南](INSTALLATION.md)；
  继续提供 FFmpeg 源码、四份补丁、依赖许可证和两套重新链接材料。
- [x] 本版决定不做发布者身份签名或公证；说明 ad-hoc 签名不证明发布者身份。
- [x] 安装指南包含 SHA-256 校验、WebView2 和系统安全提示处理，不要求关闭安全防护。
- [x] 准备 [中英文发布说明](RELEASE-NOTES.md)，接入草稿 Release 正文。
- [x] 修复构建自检日志进入归档的问题：只清理新发布目录中匹配的自检日志，
  对意外文件拒绝删除并使构建失败；覆盖正常日志、轮转日志及保留无关文件的回归测试。
- [ ] 维护者最终审查许可证、第三方声明和实际分发方式；专利/商标授权不由 MIT 或 LGPL 代替。

## 本机验收 / Local acceptance

测试主机：macOS 26.7.1，Apple Silicon arm64。这不是 macOS 12.0 实机。
以下结果记录于 2026-10-31（UTC+08:00）。未勾选项均不算通过。

- [x] 完整 macOS 发布构建：`go test -a -race ./...`、vet、CLI/UI 编译、部署目标检查。
  使用 Go 1.26.10，复用已验证的 v6 FFmpeg 静态库并强制重新链接。
- [x] 三份本地真实样本：DTS-HD MA 5.1、7.1 与 DTS:X 7.1；普通 DTS core 与 MKV 回归。
  配置外部 FFmpeg 仅用于测试夹具和额外解码检查；未提供的 DTS:X 5.1 仍明确跳过。
- [x] CLI/UI 重新链接验证及两份 CLI 的 `doctor` 自检。
- [x] 从新生成归档解压，校验 SHA-256、可执行权限、版本、许可证和源码材料。
  确认归档没有运行诊断日志、用户样本或 `.git` 目录。
- [x] 解压后的 CLI 在 `PATH=/nonexistent` 下通过 `version`、`doctor` 和十项实际输出：
  MA 5.1 三格式、MA 7.1 两格式、普通 DTS 两格式及 MKV 指定轨道三格式。
- [x] 验证普通 DTS E-AC-3 1024 kb/s、AC-3 640 kb/s；中文/空格路径、全局轨道 2、
  `[eng]_DELAY 125ms` 命名、不覆盖、只读目录错误、普通 DTS 拒绝 TrueHD，以及
  SIGINT 退出码 130 且不发布取消结果。临时转换目录无残留。
- [x] 解压后的原生 UI 完成启动，由 AppKit 确认 `isFinishedLaunching=true`，
  接受正常退出并返回 0；仅测试空队列，不代替拖放/转换中关闭等人工交互验收。
- [x] 日志清理改动后重新通过完整 `go test ./tests/build`；
  草稿门禁/发布说明 YAML 接线、桌面入口构建及文档链接检查通过。

本地候选（不是最终四平台 Release 附件）：

```text
dist/assets/hdma2truehd-v1.0.0-darwin-arm64-candidate.tar.gz
SHA-256: 4ecd0eff00521a7deea6a28385825966dfaff2a26cd1d8964d6b30b0d714d8d4
Size: 65961032 bytes
```

完整目录：`dist/release-candidate-v1.0.0-darwin-arm64-20261031/`。
归档带独立 `.sha256`；未生成冒充四平台产物的最终 `SHA256SUMS`。
候选包在提交发布准备改动之前生成，不能把该包标成旧标签的产物。

Local acceptance passed on the stated arm64 host: full race/vet/native build,
real fixtures, relinking, extracted CLI without external-tool PATH, ten output
conversions, fixed core bitrates, MKV naming, failure/cancellation checks, and
native empty-queue UI launch/normal quit. Packaging regression tests passed
after the diagnostic-log cleanup fix. The archive is a local candidate from
an uncommitted source tree, not the final tagged four-platform distribution.

## 其他平台和人工验收 / Other devices and manual checks

对每个目标从**最终发布附件**下载，在相应原生设备上测试。CI 只证明其实际运行的
构建/测试步骤，不证明最低系统版本、拖放、系统安全提示或功放播放兼容性。

For each target, test the final downloaded asset on the corresponding native
device. CI does not establish minimum-OS, desktop interaction, trust-prompt or
hardware playback compatibility.

| 目标 / Target | 最低系统 / Minimum | CI（最终提交） | 原生 CLI/UI 与真实样本 | 浏览器下载首次启动 |
|---|---|---|---|---|
| macOS arm64 | macOS 12.0 | 待完成 / Pending | 最低系统待测 / Pending | 待人工确认 / Pending |
| macOS Intel | macOS 12.0 | 待完成 / Pending | 待实机 / Pending | 待人工确认 / Pending |
| Windows x64 | Windows 10 22H2 / 19045 | 待完成 / Pending | 待实机 / Pending | 待人工确认 / Pending |
| Windows ARM64 | Windows 11 24H2 / 26100 | 待完成 / Pending | 待实机 / Pending | 待人工确认 / Pending |

每个平台记录设备/系统、附件 SHA-256、测试时间和实际结果，并确认：

- [ ] 版本、帮助、`doctor`；Windows WebView2 缺失/安装后的启动。
- [ ] 文件选择与拖放、中英文、中文/空格路径、紧凑音轨列表和日志等级颜色。
- [ ] 支持的 TrueHD/E-AC-3/AC-3 组合、固定码率和不支持组合的明确拒绝。
- [ ] MKV 全局选轨、语言/正负延迟命名、无关视频/字幕和附件不阻断音轨。
- [ ] 取消、转换中关闭、失败重试、输出冲突、不覆盖和只读目录错误。
- [ ] 下载后的 Gatekeeper/SmartScreen 行为；不绕过恶意软件警报或组织策略。
- [ ] DTS:X 5.1 / IMAX 真实样本，或在发布说明中保留覆盖缺口。
- [ ] 目标播放器/功放播放兼容性，或明确保留未验证声明。

## 公开前最后检查 / Final publication gate

- [ ] 最终提交和标签指向一致，版本号、UI/CLI/系统资源版本一致。
- [ ] 草稿内四份完整归档、各自 `.sha256` 和汇总 `SHA256SUMS` 全部存在并校验。
- [ ] 发布说明和安装指南与附件一致；日志/测试样本/本机隐私信息不意外进入附件。
- [ ] 维护者审阅仍未验证的项目，决定延期或明确披露，不将待测项标成通过。
- [ ] 提交、推送、修改标签和正式公开均取得维护者对应授权；不要将本机验收视为发布批准。
