"use strict";

const translations = {
  zh: {
    title: "音轨转换", language: "界面语言", drop: "拖入 DTS 文件或 MKV 电影",
    local: "支持批量添加 · 直接读取本地文件，不上传、不复制整部电影", add: "选择文件",
    tracks: "文件与音轨", clear: "清空列表", empty: "添加文件后，在这里选择需要转换的音轨。",
    settings: "输出设置", formats: "输出格式", lossless: "无损", lossy: "有损",
    concurrency: "并行任务数",
    retryFailed: "重试失败任务", viewLogs: "查看日志", logs: "诊断日志", close: "关闭",
    technical: "技术详情", warning: "警告", logTruncated: "仅显示最新 128 KiB；更早记录请查看日志文件及同名 .1 文件。",
    logNotice: "日志仅保存在程序旁的 logs 目录，含文件路径和原始诊断信息，不上传。每次启动单独记录，单次最多保留两段各 5 MiB；旧会话不会自动删除。再次点击“查看日志”可刷新。",
    integrityNotice: "源文件完整性未确认：没有可信的原始长度或校验值时，无法保证识别所有截断。可疑时长会警告，确认的解码或校验错误仍阻止输出。",
    parallelNotice: "默认同时处理 2 项任务，可选 1–8；1 为串行。同一文件的音轨和输出格式也可并行，较高并行数会增加 CPU、内存与磁盘负载。",
    bitrate51: "E-AC-3 5.1 码率", auto51: "自动：960 kb/s",
    bitrate71: "E-AC-3 7.1 总码率", auto71: "自动：1536 kb/s",
    limits: "DTS-HD MA → E-AC-3：仅 48 kHz 5.1(side) / 7.1；码率分别为 384–1024 / 768–1536 kb/s。普通 DTS（profile 必须为 DTS）：仅 48 kHz 5.1(side)，E-AC-3 固定 1024 kb/s，忽略码率选择，不支持 TrueHD。AC-3：仅 48 kHz 5.1(side)，固定 640 kb/s。不自动重映射或重采样；不支持 HRA。",
    output: "输出目录", browse: "选择目录", reset: "重置", sameDir: "与每个源文件相同的目录",
    overwrite: "允许覆盖已有输出（默认不覆盖）",
    notice: "DTS:X 仅转换兼容声道，不渲染对象。TrueHD 校验完整 PCM；AC-3 / E-AC-3 校验完整解码与长度，保留 256 采样编码延迟及补齐至 1536 采样帧的补零。各格式独立发布，不封装回 MKV。",
    queue: "转换队列", start: "开始转换", cancel: "取消转换", cancelling: "正在取消…",
    busy: "正在读取音轨信息…", idle: "就绪", working: "正在处理；下方显示当前文件的编码与校验进度。",
    probe: "读取音轨信息", encode: "转码", check_output: "检查输出格式", verify_source: "校验：解码源音轨 PCM",
    verify_output: "校验：解码并检查输出", publish: "保存输出", processed: "已处理", elapsed: "已耗时", stageDone: "阶段完成",
    estimated: "估计", unknownDuration: "总时长未知", incompatible: "所选输出格式不兼容，请调整格式或取消选择标记的音轨。",
    verified: "无损验证通过", validated: "有损校验通过", pending: "等待中", running: "编码 / 校验中",
    failed: "失败", cancelled: "已取消", selected: "已选音轨", total: "任务", complete: "已结束",
    track: "音轨", default: "默认", unsupported: "不可转换", eac3Unavailable: "无法输出 E-AC-3", available: "可用", unavailableFormat: "不可用",
    samples: "源采样 / 声道", decoded: "解码采样（含延迟和补零）", choose: "请选择音轨和输出格式。",
    unavailable: "桌面运行时未就绪，请通过桌面应用打开此界面。", busyError: "转换或读取音轨时不能添加文件。",
    selectionDisabled: "转换过程中不能修改音轨选择。", formatDetails: "格式限制"
  },
  en: {
    title: "Audio conversion", language: "Language", drop: "Drop DTS files or MKV movies",
    local: "Batch input · Local files only, no upload or full-movie copy", add: "Add files",
    tracks: "Files & audio tracks", clear: "Clear list", empty: "Add files, then select the audio tracks to convert.",
    settings: "Output settings", formats: "Output formats", lossless: "lossless", lossy: "lossy",
    concurrency: "Parallel jobs",
    retryFailed: "Retry failed jobs", viewLogs: "View logs", logs: "Diagnostic logs", close: "Close",
    technical: "Technical details", warning: "Warning", logTruncated: "Showing the latest 128 KiB. Earlier records are in the log file and its .1 segment.",
    logNotice: "Logs remain in the logs directory beside the application, include file paths and raw diagnostics, and are never uploaded. Each launch keeps up to two 5 MiB segments; older sessions are not deleted automatically. Click View logs again to refresh.",
    integrityNotice: "Source completeness is unconfirmed: without a trusted original length/checksum, not every truncation can be detected. Suspicious duration produces warnings; confirmed decoding/verification errors still block publication.",
    parallelNotice: "Default: 2 concurrent jobs, adjustable from 1–8; 1 is serial. Tracks and formats from the same file may run concurrently. Higher concurrency increases CPU, memory and disk load.",
    bitrate51: "E-AC-3 5.1 bitrate", auto51: "Automatic: 960 kb/s",
    bitrate71: "E-AC-3 7.1 total bitrate", auto71: "Automatic: 1536 kb/s",
    limits: "DTS-HD MA → E-AC-3: 48 kHz 5.1(side) / 7.1 only, 384–1024 / 768–1536 kb/s respectively. Ordinary DTS (profile exactly DTS): 48 kHz 5.1(side) only, E-AC-3 fixed 1024 kb/s regardless of bitrate selection; no TrueHD. AC-3: 48 kHz 5.1(side) only, fixed 640 kb/s. No remapping/resampling or HRA support.",
    output: "Output directory", browse: "Browse", reset: "Reset", sameDir: "Same directory as each source file",
    overwrite: "Allow replacing existing outputs (off by default)",
    notice: "DTS:X compatible channels only, no object rendering. TrueHD verifies complete PCM; AC-3 / E-AC-3 validates full decoding and length, retaining 256 samples of delay and padding to 1536-sample frames. Outputs are published independently, not remuxed into MKV.",
    queue: "Conversion queue", start: "Start conversion", cancel: "Cancel", cancelling: "Cancelling…",
    busy: "Reading audio tracks…", idle: "Ready", working: "Processing; current-file encoding and verification progress appears below.",
    probe: "Reading audio track", encode: "Encoding", check_output: "Checking output format", verify_source: "Verification: decoding source PCM",
    verify_output: "Verification: decoding and checking output", publish: "Saving output", processed: "processed", elapsed: "elapsed", stageDone: "stage complete",
    estimated: "estimated", unknownDuration: "total duration unknown", incompatible: "Selected output formats are incompatible. Change formats or deselect marked tracks.",
    verified: "Lossless verified", validated: "Lossy validated", pending: "Pending", running: "Encoding / validating",
    failed: "Failed", cancelled: "Cancelled", selected: "selected tracks", total: "jobs", complete: "finished",
    track: "Track", default: "default", unsupported: "Cannot convert", eac3Unavailable: "E-AC-3 unavailable", available: "available", unavailableFormat: "unavailable",
    samples: "source samples/channel", decoded: "decoded samples (with delay/padding)", choose: "Select tracks and at least one output format.",
    unavailable: "Desktop runtime unavailable. Open this interface using the desktop application.", busyError: "Cannot add files while inspecting or converting.",
    selectionDisabled: "Track selection is locked during conversion.", formatDetails: "Format limits"
  }
};

const $ = (id) => document.getElementById(id);
let language = localStorage.getItem("hdma-language") === "en" ? "en" : "zh";
let inputs = [];
const selected = new Set();
let state = {revision: 0, running: false, jobs: []};
// busy covers an outstanding dialog/probe/Start RPC; running belongs to the
// backend queue and remains authoritative after those promises resolve.
let busy = false;
let cancelling = false;
let backend = null;
let lastError = null;
const t = (key) => translations[language][key];
// Tuple serialization avoids delimiter collisions with arbitrary file paths,
// including Windows drive letters. The index remains the container's index.
const keyFor = (path, index) => JSON.stringify([path, index]);
const filename = (path) => path.split(/[\\/]/).pop();

// All media metadata and errors are untrusted text, never HTML.
function node(tag, text, className) {
  const element = document.createElement(tag);
  if (text !== undefined) element.textContent = text;
  if (className) element.className = className;
  return element;
}

function showError(error) {
  lastError = error;
  $("error").replaceChildren();
  const key = typeof error === "string" ? Object.keys(translations.zh).find(key => translations.zh[key] === error || translations.en[key] === error) : "";
  if (key) {
    $("error").textContent = t(key);
  } else {
    appendProblem($("error"), error);
  }
  $("error").hidden = false;
}

function clearError() { lastError = null; $("error").hidden = true; $("error").textContent = ""; }

// Queue snapshots carry objects; Wails RPC rejections carry JSON strings.
// Preserve non-JSON runtime errors as technical detail rather than hiding them.
function normalizeProblem(error) {
  if (typeof error?.code === "string") return error;
  const detail = String(error?.message || error);
  try {
    const parsed = JSON.parse(detail.replace(/^Error: /, ""));
    if (typeof parsed?.code === "string") return parsed;
  } catch (error) {
    if (!(error instanceof SyntaxError)) throw error;
  }
  return {code: "unexpected", detail};
}

function problemText(problem) {
  const policy = problem.code?.startsWith("unsupported_") ? reasonMessages()[problem.code.slice(12)] : "";
  if (policy) return policy;
  const zh = {
    unexpected: "操作失败，请查看技术详情或诊断日志。", cancelled: "操作已取消。", deadline: "操作超时，已停止处理。",
    permission: "没有读写权限，或目标位置只读。", not_found: "文件或目录不存在。", output_exists: "输出已存在；请更换位置或明确允许覆盖。",
    no_space: "磁盘空间不足，请释放空间后重试。", memory: "内存不足，请降低并行任务数后重试。",
    temporary_io: "遇到临时读写错误。", io: "文件读写失败，请检查磁盘和文件。",
    corrupt_data: "音频数据损坏或流信息发生变化，已停止当前任务。", track_missing: "没有找到指定音轨，请重新读取文件。",
    codec_unavailable: "当前后端缺少所需编解码器或封装支持。", native_failed: "音频库处理失败，请查看技术详情。",
    probe_failed: "无法解析音轨信息，请检查文件或查看技术详情。", encode_failed: "转码失败，未发布当前输出。",
    verification_failed: "输出校验失败，未发布当前输出。", publish_failed: "保存输出失败，请检查目标目录、权限和文件系统。",
    preflight_failed: "转换预检失败，请检查音轨选择、输出设置或技术详情。",
    cleanup_failed: "临时文件清理失败；输出可能已经发布，请检查日志后再重试。",
    source_changed: "转换期间源文件发生变化，已阻止发布输出。", log_unavailable: "日志无法读取或保存。请检查程序旁 logs 目录的权限与磁盘空间；不会自动改存其他位置。",
    ffmpeg_warning: "音频库发出诊断警告，请查看技术详情；警告本身不代表转换失败。",
    duration_short: "实际解码时长明显短于元数据时长；源文件可能不完整，也可能是时长信息不准确。",
    retrying: "临时读写错误：正在重新尝试一次；格式、损坏及校验错误不会自动重试。",
    warnings_truncated: "此任务警告较多，界面仅保留前几项；其他诊断请查看日志。"
  };
  const en = {
    unexpected: "Operation failed. See technical details or logs.", cancelled: "Operation cancelled.", deadline: "Operation timed out.",
    permission: "Access denied or the destination is read-only.", not_found: "File or directory not found.", output_exists: "Output exists. Choose another location or explicitly allow overwrite.",
    no_space: "Not enough disk space. Free space before retrying.", memory: "Not enough memory. Reduce parallel jobs before retrying.",
    temporary_io: "Temporary I/O failure.", io: "File I/O failed. Check the disk and file.",
    corrupt_data: "Damaged audio or changed stream information; the current job was stopped.", track_missing: "The selected audio track was not found. Inspect the file again.",
    codec_unavailable: "Required codec or container support is unavailable.", native_failed: "Audio library operation failed. See technical details.",
    probe_failed: "Cannot read audio metadata. Check the file or technical details.", encode_failed: "Encoding failed; this output was not published.",
    verification_failed: "Verification failed; this output was not published.", publish_failed: "Cannot publish output. Check directory permissions and filesystem support.",
    preflight_failed: "Preflight failed. Check tracks, output settings or technical details.",
    cleanup_failed: "Temporary-file cleanup failed. Output may already be published; check logs before retrying.",
    source_changed: "Source changed during conversion; output publication was blocked.", log_unavailable: "Cannot read or save logs. Check permissions and disk space beside the application. No alternate location is used automatically.",
    ffmpeg_warning: "Audio library diagnostic warning. See technical details; a warning alone does not mean conversion failed.",
    duration_short: "Decoded duration is significantly shorter than metadata. The source may be incomplete or the metadata inaccurate.",
    retrying: "Temporary I/O failure: retrying once. Format, corruption and verification errors are not retried automatically.",
    warnings_truncated: "Many warnings were reported. The per-job display is limited; see logs for further diagnostics."
  };
  return (language === "zh" ? zh : en)[problem.code] || (language === "zh" ? zh : en).unexpected;
}

function appendProblem(parent, error, warning = false) {
  const problem = normalizeProblem(error);
  parent.append(node("p", `${warning ? t("warning") + ": " : ""}${problemText(problem)}`, "problem"));
  if (problem.detail) {
    const details = node("details", undefined, "technical");
    details.append(node("summary", t("technical")), node("pre", problem.detail));
    parent.append(details);
  }
}

const outputFormats = [["truehd", "TrueHD"], ["eac3", "E-AC-3"], ["ac3", "AC-3"]];

// General source failures apply to every output; otherwise consult the
// requested format's policy rather than treating TrueHD as the source gate.
function formatProblem(track, format) {
  const field = format === "truehd" ? "truehdProblem" : `${format}Problem`;
  return {reason: track.problem || track[field], code: track.problemCode || track[`${field}Code`]};
}

function describeTrackProblem(track, reason, kind) {
  if (!reason) return "";
  const info = track.info;
  const stream = info.stream;
  const actual = `${[stream.codec_name, stream.profile].filter(Boolean).join(" / ") || "?"} · ${stream.channel_layout || `${stream.channels} ch`} · ${info.sample_rate || "?"} Hz · ${info.effective_bits || "?"} bit`;
  return `${reasonMessages()[kind] || reason} (${actual})`;
}

// Never silently change format defaults when an ordinary DTS track is added.
function trackProblem(track) {
  return outputFormats.filter(([format]) => $(format).checked).map(([format, label]) => {
    const {reason, code} = formatProblem(track, format);
    return reason ? `${label}: ${describeTrackProblem(track, reason, code)}` : "";
  }).filter(Boolean).join(" · ") || describeTrackProblem(track, track.problem, track.problemCode);
}

function reasonMessages() {
  return language === "zh" ? {
    content_encoding: "该 MKV 音轨使用当前后端不支持的内容压缩、加密或编码链，无法安全转换。",
    codec: "仅支持 DTS-HD MA 或符合限制的普通 DTS；当前编码不支持转换。",
    profile: "仅接受已确认的 DTS-HD MA 或普通 DTS；不支持 HRA 或未知 profile。",
    layout: "仅支持 5.1(side)、5.1（后环绕）和标准 7.1；不自动重映射或降混。",
    channels: "声道数量与布局信息不一致，无法安全转换。",
    rate: "采样率不支持；支持 44.1/48/88.2/96/176.4/192 kHz，不自动重采样。",
    bits: "有效位深未知或不支持；仅接受确认的 16/24 位音轨。",
    eac3_format: "E-AC-3 需要 48 kHz 5.1(side)；仅 DTS-HD MA 另支持 7.1。",
    truehd_source: "TrueHD 仅支持 DTS-HD MA；普通 DTS 请选择 E-AC-3 或 AC-3。",
    ac3_format: "AC-3 仅支持 48 kHz 5.1(side)，固定 640 kb/s，不重映射或重采样。",
    dts_format: "普通 DTS 仅支持 48 kHz 5.1(side)，输出 E-AC-3 或 AC-3。"
  } : {
    content_encoding: "This MKV audio track uses unsupported content compression, encryption or an encoding chain; conversion is unsafe.",
    codec: "Only DTS-HD MA or eligible ordinary DTS sources are supported.",
    profile: "Only confirmed DTS-HD MA or ordinary DTS profiles are supported; no HRA or unknown profiles.",
    layout: "Only 5.1(side), rear 5.1 and standard 7.1 are supported; no automatic remapping/downmix.",
    channels: "Channel count and speaker layout disagree; conversion is unsafe.",
    rate: "Unsupported rate; use 44.1/48/88.2/96/176.4/192 kHz. No automatic resampling.",
    bits: "Unknown or unsupported effective precision; confirmed 16/24-bit sources only.",
    eac3_format: "E-AC-3 requires 48 kHz 5.1(side); only DTS-HD MA also supports 7.1.",
    truehd_source: "TrueHD requires DTS-HD MA. For ordinary DTS, select E-AC-3 or AC-3.",
    ac3_format: "AC-3 requires 48 kHz 5.1(side), fixed 640 kb/s; no remapping or resampling.",
    dts_format: "Ordinary DTS requires 48 kHz 5.1(side) and E-AC-3 or AC-3 output."
  };
}

function incompatibleSelection() {
  return inputs.some((input) => input.tracks.some((track) =>
    selected.has(keyFor(input.path, track.info.stream.index)) && trackProblem(track)));
}

function clock(seconds) {
  seconds = Math.max(0, Math.floor(seconds || 0));
  return `${Math.floor(seconds / 3600) ? Math.floor(seconds / 3600) + ":" : ""}${String(Math.floor(seconds / 60) % 60).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

function renderInputs() {
  const expanded = new Set([...$("inputs").querySelectorAll(".track-issues[open]")].map(details => details.dataset.track));
  const fragment = document.createDocumentFragment();
  for (const input of inputs) {
    const file = node("div", undefined, "input-file");
    file.append(node("div", filename(input.path), "file-name"), node("div", input.path, "file-path"));
    if (input.error) appendProblem(file, input.problem || input.error);
    for (const track of input.tracks) {
      const info = track.info;
      const stream = info.stream;
      const id = keyFor(input.path, stream.index);
      const row = node("div", undefined, "track");
      const label = node("label", undefined, "track-choice");
      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = selected.has(id);
      const problem = trackProblem(track);
      if (checkbox.checked) row.classList.add("is-selected");
      // Keep selected incompatible tracks removable rather than trapping the selection.
      checkbox.disabled = (!!problem && !checkbox.checked) || busy || state.running;
      checkbox.addEventListener("change", () => {
        if (checkbox.checked) selected.add(id); else selected.delete(id);
        renderInputs(); renderState();
      });
      const details = node("div", undefined, "track-details");
      const summary = node("div", undefined, "track-summary");
      const title = `${t("track")} ${stream.index} · ${stream.profile || stream.codec_name}${track.title ? " · " + track.title : ""}`;
      summary.append(node("div", title, "track-title"));
      const duration = track.duration > 0 ? `${Math.floor(track.duration / 60)}:${String(Math.floor(track.duration % 60)).padStart(2, "0")}` : "";
      summary.append(node("div", [track.language, stream.channel_layout || `${stream.channels} ch`,
        info.sample_rate ? `${info.sample_rate} Hz` : "", info.effective_bits ? `${info.effective_bits} bit` : "",
        duration, track.default ? t("default") : ""].filter(Boolean).join(" · "), "track-meta"));
      details.append(summary);
      label.append(checkbox, details);
      const formats = node("div", undefined, "track-formats");
      const reasons = new Map();
      for (const [format, name] of outputFormats) {
        const {reason, code} = formatProblem(track, format);
        const fixed = format === "ac3" ? " · 640 kb/s" : format === "eac3" && stream.profile === "DTS" ? " · 1024 kb/s" : "";
        const badge = node("span", `${name} · ${reason ? t("unavailableFormat") : t("available") + fixed}`, `format-badge${reason ? " unavailable" : ""}`);
        if (reason) {
          const text = reasonMessages()[code] || reason;
          badge.title = `${name}: ${text}`;
          if (!reasons.has(text)) reasons.set(text, []);
          reasons.get(text).push(name);
        }
        formats.append(badge);
      }
      if (reasons.size) {
        const issues = node("details", undefined, `track-issues${problem ? " incompatible" : ""}`);
        issues.dataset.track = id;
        issues.open = expanded.has(id);
        issues.append(node("summary", `${t(problem ? "unsupported" : "formatDetails")} (${reasons.size})`));
        for (const [reason, names] of reasons) issues.append(node("div", `${names.join(" / ")}: ${reason}`, "format-reason"));
        formats.append(issues);
      }
      row.append(label, formats);
      file.append(row);
    }
    fragment.append(file);
  }
  $("inputs").replaceChildren(fragment);
  $("empty").hidden = inputs.length > 0;
}

function renderState() {
  const locked = busy || state.running;
  for (const id of ["add", "clear", "truehd", "eac3", "ac3", "concurrency", "browse", "reset-output", "overwrite"]) $(id).disabled = locked;
  $("drop").setAttribute("aria-disabled", String(locked));
  for (const id of ["bitrate51", "bitrate71"]) $(id).disabled = locked || !$("eac3").checked;
  $("start").disabled = locked || !backend || selected.size === 0 || incompatibleSelection() || !outputFormats.some(([format]) => $(format).checked);
  $("cancel").disabled = !state.running || cancelling;
  $("cancel").textContent = t(cancelling ? "cancelling" : "cancel");
  $("retry-failed").disabled = locked || !backend || !state.jobs.some(job => job.status === "failed");
  $("view-logs").disabled = !backend;
  $("log-error").replaceChildren();
  $("log-error").hidden = !state.logProblem;
  if (state.logProblem) appendProblem($("log-error"), state.logProblem);
  $("output").placeholder = t("sameDir");
  $("activity").textContent = t(busy ? "busy" : state.running ? "working" : "idle");
  if (!locked && incompatibleSelection()) $("activity").textContent = t("incompatible");
  // Failed/cancelled jobs also finish. This is queue completion, not a measured
  // encoding percentage or an assertion that every completed output succeeded.
  const finished = state.jobs.filter((job) => !["pending", "running"].includes(job.status)).length;
  $("summary").textContent = `${selected.size} ${t("selected")} · ${finished}/${state.jobs.length} ${t("total")} ${t("complete")}`;
  $("progress").max = Math.max(1, state.jobs.length);
  $("progress").value = finished;
  const fragment = document.createDocumentFragment();
  for (const job of state.jobs) {
    const item = node("li");
    const heading = node("div", undefined, "job-heading");
    heading.append(node("span", filename(job.output)), node("span", t(job.status), `status ${job.status}`));
    item.append(heading, node("div", job.output, "file-path"));
    if (job.format === "ac3" || job.bitrate) item.append(node("div", `${job.format === "ac3" ? "AC-3" : "E-AC-3"} · ${job.format === "ac3" ? 640 : job.bitrate} kb/s`, "track-meta"));
    const p = job.progress;
    if (p?.stage) {
      const terminal = !["pending", "running"].includes(job.status);
      const text = `${t(p.stage) || p.stage} · ${p.done ? t("stageDone") : p.duration > 0 ? `${t("estimated")} ${(p.fraction * 100).toFixed(1)}%` : t("unknownDuration")}`;
      const bar = node("progress");
      bar.max = 1;
      if (p.duration > 0 || p.done) bar.value = p.fraction;
      bar.setAttribute("aria-label", t(p.stage) || p.stage);
      if (!terminal) item.append(bar);
      item.append(node("div", `${text} · ${t("processed")} ${clock(p.seconds)}${p.duration > 0 ? " / " + clock(p.duration) : ""} · ${t("elapsed")} ${clock(p.elapsed)}`, "track-meta"));
    }
    if (job.samples) item.append(node("div", `${job.samples} ${t("samples")}${job.decodedSamples ? ` · ${job.decodedSamples} ${t("decoded")}` : ""}`, "track-meta"));
    if (job.error) appendProblem(item, job.problem || job.error);
    for (const warning of job.warnings || []) appendProblem(item, warning, true);
    fragment.append(item);
  }
  $("jobs").replaceChildren(fragment);
}

function applyLanguage() {
  document.documentElement.lang = language === "zh" ? "zh-CN" : "en";
  $("language").value = language;
  $("language").setAttribute("aria-label", t("language"));
  $("progress").setAttribute("aria-label", t("queue"));
  document.querySelectorAll("[data-i18n]").forEach((element) => { element.textContent = t(element.dataset.i18n); });
  renderInputs();
  renderState();
  if (lastError !== null) showError(lastError);
}

function acceptState(next) {
  // Events may overtake the response to Start. Never redraw an older snapshot.
  if (next.revision < state.revision) return;
  const wasRunning = state.running;
  state = next;
  if (!state.running) cancelling = false;
  if (wasRunning !== state.running) renderInputs();
  renderState();
}

async function addFiles(paths) {
  if (busy || state.running) { showError(t("busyError")); return; }
  clearError();
  busy = true;
  renderInputs();
  renderState();
  try {
    const added = paths ? await backend.Inspect(paths) : await backend.ChooseFiles();
    for (const input of added) {
      if (inputs.some((existing) => existing.path === input.path)) continue;
      inputs.push(input);
      // Raw DTS has no track ambiguity; MKV requires an explicit user choice.
      if (!/\.mkv$/i.test(input.path)) {
        for (const track of input.tracks) if (!trackProblem(track)) selected.add(keyFor(input.path, track.info.stream.index));
      }
    }
  } catch (error) { showError(error); }
  finally { busy = false; renderInputs(); renderState(); }
}

$("add").addEventListener("click", (event) => { event.stopPropagation(); if (backend) addFiles(); });
$("drop").addEventListener("click", () => { if (backend && !busy && !state.running) addFiles(); });
$("drop").addEventListener("keydown", (event) => {
  if (event.target === $("drop") && ["Enter", " "].includes(event.key)) { event.preventDefault(); if (backend) addFiles(); }
});
$("language").addEventListener("change", () => { language = $("language").value; localStorage.setItem("hdma-language", language); applyLanguage(); });
$("clear").addEventListener("click", () => { inputs = []; selected.clear(); clearError(); renderInputs(); renderState(); });
for (const [id] of outputFormats) $(id).addEventListener("change", () => { renderInputs(); renderState(); });
$("reset-output").addEventListener("click", () => { $("output").value = ""; });
$("browse").addEventListener("click", async () => {
  if (!backend) return;
  try { const path = await backend.ChooseOutput(); if (path) $("output").value = path; }
  catch (error) { showError(error); }
});
$("start").addEventListener("click", async () => {
  clearError();
  const tracks = [...selected].map((id) => { const [path, index] = JSON.parse(id); return {path, index}; });
  if (tracks.length === 0 || !outputFormats.some(([format]) => $(format).checked)) { showError(t("choose")); return; }
  if (incompatibleSelection()) { showError(t("incompatible")); return; }
  busy = true;
  renderInputs();
  renderState();
  try {
    // Keep zero as "automatic" and send kb/s, not bits/s. The backend resolves
    // and validates each track's layout-specific rate during queue preflight.
    acceptState(await backend.Start({tracks, truehd: $("truehd").checked, eac3: $("eac3").checked, ac3: $("ac3").checked,
      bitrate51: $("eac3").checked ? Number($("bitrate51").value) : 0,
      bitrate71: $("eac3").checked ? Number($("bitrate71").value) : 0,
      concurrency: Number($("concurrency").value),
      outputDir: $("output").value, overwrite: $("overwrite").checked}));
  } catch (error) { showError(error); }
  finally { busy = false; renderInputs(); renderState(); }
});
// Cancel only acknowledges the request; keep controls locked until a terminal
// backend snapshot confirms that native conversion and cleanup have unwound.
$("cancel").addEventListener("click", async () => {
  cancelling = true;
  renderState();
  try { await backend.Cancel(); }
  catch (error) { cancelling = false; showError(error); renderState(); }
});

$("retry-failed").addEventListener("click", async () => {
  if (!backend || busy || state.running) return;
  clearError();
  busy = true;
  renderInputs(); renderState();
  // The backend owns history and fresh preflight; rebuilding Start from the
  // current selection could re-encode successes or change original settings.
  try { acceptState(await backend.RetryFailed()); }
  catch (error) { showError(error); }
  finally { busy = false; renderInputs(); renderState(); }
});
$("view-logs").addEventListener("click", async () => {
  if (!backend) return;
  try {
    const view = await backend.Logs();
    $("log-path").textContent = view.path;
    $("log-content").textContent = view.content;
    $("log-truncated").hidden = !view.truncated;
    $("logs").hidden = false;
    if (view.problem) showError(view.problem);
  } catch (error) { showError(error); }
});
$("close-logs").addEventListener("click", () => { $("logs").hidden = true; });

applyLanguage();
if (window.go?.main?.App && window.runtime) {
  backend = window.go.main.App;
  backend.BuildInfo().then((info) => {
    $("app-version").textContent = `HDMA Converter ${info.version} · ${info.architecture}`;
    $("app-version").title = info.minimumSystem;
    $("app-copyright").textContent = info.copyright;
    document.title = `HDMA Converter ${info.version}`;
  }).catch(showError);
  // Subscribe before fetching State so a queue change cannot fall into a
  // setup gap. Revision checks reconcile an event that overtakes the RPC.
  window.runtime.EventsOn("queue", acceptState);
  // Native paths let the backend read large MKVs in place, rather than upload
  // browser File blobs or copy the movie into frontend memory.
  window.runtime.OnFileDrop((_x, _y, paths) => addFiles(paths), false);
  backend.State().then(acceptState).catch(showError);
  renderState();
} else {
  showError(t("unavailable"));
}
