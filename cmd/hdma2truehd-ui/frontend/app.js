"use strict";

const translations = {
  zh: {
    title: "音轨转换", language: "界面语言", drop: "拖入 DTS 文件或 MKV 电影",
    local: "支持批量添加 · 直接读取本地文件，不上传、不复制整部电影", add: "选择文件",
    tracks: "文件与音轨", clear: "清空列表", empty: "添加文件后，在这里选择需要转换的音轨。",
    settings: "输出设置", formats: "输出格式", lossless: "无损", lossy: "有损",
    concurrency: "并行任务数",
    parallelNotice: "默认同时处理 2 项任务，可选 1–8；1 为串行。同一文件的音轨和输出格式也可并行，较高并行数会增加 CPU、内存与磁盘负载。",
    bitrate51: "E-AC-3 5.1 码率", auto51: "自动：960 kb/s",
    bitrate71: "E-AC-3 7.1 总码率", auto71: "自动：1536 kb/s",
    limits: "E-AC-3：仅支持 48 kHz 的 5.1(side) / 7.1。5.1 码率范围 384–1024，7.1 为 768–1536 kb/s。混合队列按音轨布局分别使用对应设置。",
    output: "输出目录", browse: "选择目录", reset: "重置", sameDir: "与每个源文件相同的目录",
    overwrite: "允许覆盖已有输出（默认不覆盖）",
    notice: "DTS:X 仅转换兼容声道，不渲染对象。TrueHD 校验完整 PCM；E-AC-3 校验完整解码与长度，并保留 256 采样编码延迟和帧补零。两种输出独立发布，不封装回 MKV。",
    queue: "转换队列", start: "开始转换", cancel: "取消转换", cancelling: "正在取消…",
    busy: "正在读取音轨信息…", idle: "就绪", working: "正在处理；下方显示当前文件的编码与校验进度。",
    probe: "读取音轨信息", encode: "转码", check_output: "检查输出格式", verify_source: "校验：解码源音轨 PCM",
    verify_output: "校验：解码并检查输出", publish: "保存输出", processed: "已处理", elapsed: "已耗时", stageDone: "阶段完成",
    estimated: "估计", unknownDuration: "总时长未知", incompatible: "所选输出格式不兼容，请调整格式或取消选择标记的音轨。",
    verified: "无损验证通过", validated: "有损校验通过", pending: "等待中", running: "编码 / 校验中",
    failed: "失败", cancelled: "已取消", selected: "已选音轨", total: "任务", complete: "已结束",
    track: "音轨", default: "默认", unsupported: "不可转换", eac3Unavailable: "无法输出 E-AC-3",
    samples: "源采样 / 声道", decoded: "解码采样（含延迟和补零）", choose: "请选择音轨和输出格式。",
    unavailable: "桌面运行时未就绪，请通过桌面应用打开此界面。", busyError: "转换或读取音轨时不能添加文件。",
    selectionDisabled: "转换过程中不能修改音轨选择。"
  },
  en: {
    title: "Audio conversion", language: "Language", drop: "Drop DTS files or MKV movies",
    local: "Batch input · Local files only, no upload or full-movie copy", add: "Add files",
    tracks: "Files & audio tracks", clear: "Clear list", empty: "Add files, then select the audio tracks to convert.",
    settings: "Output settings", formats: "Output formats", lossless: "lossless", lossy: "lossy",
    concurrency: "Parallel jobs",
    parallelNotice: "Default: 2 concurrent jobs, adjustable from 1–8; 1 is serial. Tracks and formats from the same file may run concurrently. Higher concurrency increases CPU, memory and disk load.",
    bitrate51: "E-AC-3 5.1 bitrate", auto51: "Automatic: 960 kb/s",
    bitrate71: "E-AC-3 7.1 total bitrate", auto71: "Automatic: 1536 kb/s",
    limits: "E-AC-3: 48 kHz 5.1(side) / 7.1 only. 5.1: 384–1024; 7.1: 768–1536 kb/s. Mixed queues use each track's layout-specific setting.",
    output: "Output directory", browse: "Browse", reset: "Reset", sameDir: "Same directory as each source file",
    overwrite: "Allow replacing existing outputs (off by default)",
    notice: "DTS:X compatible channels only, no object rendering. TrueHD verifies complete PCM; E-AC-3 validates full decoding and length, retaining 256 samples of delay and frame padding. Outputs are published independently, not remuxed into MKV.",
    queue: "Conversion queue", start: "Start conversion", cancel: "Cancel", cancelling: "Cancelling…",
    busy: "Reading audio tracks…", idle: "Ready", working: "Processing; current-file encoding and verification progress appears below.",
    probe: "Reading audio track", encode: "Encoding", check_output: "Checking output format", verify_source: "Verification: decoding source PCM",
    verify_output: "Verification: decoding and checking output", publish: "Saving output", processed: "processed", elapsed: "elapsed", stageDone: "stage complete",
    estimated: "estimated", unknownDuration: "total duration unknown", incompatible: "Selected output formats are incompatible. Change formats or deselect marked tracks.",
    verified: "Lossless verified", validated: "Lossy validated", pending: "Pending", running: "Encoding / validating",
    failed: "Failed", cancelled: "Cancelled", selected: "selected tracks", total: "jobs", complete: "finished",
    track: "Track", default: "default", unsupported: "Cannot convert", eac3Unavailable: "E-AC-3 unavailable",
    samples: "source samples/channel", decoded: "decoded samples (with delay/padding)", choose: "Select tracks and at least one output format.",
    unavailable: "Desktop runtime unavailable. Open this interface using the desktop application.", busyError: "Cannot add files while inspecting or converting.",
    selectionDisabled: "Track selection is locked during conversion."
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
  $("error").textContent = String(error);
  $("error").hidden = false;
}

function clearError() { $("error").hidden = true; $("error").textContent = ""; }

function trackProblem(track) {
  const info = track.info;
  const stream = info.stream;
  const code = track.problemCode;
  const eac3 = $("eac3").checked && track.eac3Problem;
  const reason = track.problem || (eac3 ? track.eac3Problem : "");
  if (!reason) return "";
  const kind = code || (eac3 ? track.eac3ProblemCode : "");
  const actual = `${[stream.codec_name, stream.profile].filter(Boolean).join(" / ") || "?"} · ${stream.channel_layout || `${stream.channels} ch`} · ${info.sample_rate || "?"} Hz · ${info.effective_bits || "?"} bit`;
  const messages = language === "zh" ? {
    codec: "仅支持 DTS-HD MA 无损源音轨；当前编码不支持转换。",
    profile: "当前为 DTS 有损核心、HRA 或未确认的 profile，不是已确认的 DTS-HD MA。",
    layout: "仅支持 5.1(side)、5.1（后环绕）和标准 7.1；不自动重映射或降混。",
    channels: "声道数量与布局信息不一致，无法安全转换。",
    rate: "采样率不支持；支持 44.1/48/88.2/96/176.4/192 kHz，不自动重采样。",
    bits: "有效位深未知或不支持；仅接受确认的 16/24 位音轨。",
    eac3_format: "当前音轨可转 TrueHD，但 E-AC-3 仅支持 48 kHz 的 5.1(side)/7.1；请取消 E-AC-3 输出。"
  } : {
    codec: "Only lossless DTS-HD MA sources are supported; this codec cannot be converted.",
    profile: "DTS core, HRA or an unconfirmed profile is not confirmed lossless DTS-HD MA.",
    layout: "Only 5.1(side), rear 5.1 and standard 7.1 are supported; no automatic remapping/downmix.",
    channels: "Channel count and speaker layout disagree; conversion is unsafe.",
    rate: "Unsupported rate; use 44.1/48/88.2/96/176.4/192 kHz. No automatic resampling.",
    bits: "Unknown or unsupported effective precision; confirmed 16/24-bit sources only.",
    eac3_format: "TrueHD is available, but E-AC-3 requires 48 kHz 5.1(side)/7.1. Deselect E-AC-3."
  };
  return `${messages[kind] || reason} (${actual})`;
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
  const fragment = document.createDocumentFragment();
  for (const input of inputs) {
    const file = node("div", undefined, "input-file");
    file.append(node("div", filename(input.path), "file-name"), node("div", input.path, "file-path"));
    if (input.error) file.append(node("p", input.error, "problem"));
    for (const track of input.tracks) {
      const info = track.info;
      const stream = info.stream;
      const id = keyFor(input.path, stream.index);
      const label = node("label", undefined, "track");
      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = selected.has(id);
      const problem = trackProblem(track);
      // Keep selected incompatible tracks removable rather than trapping the selection.
      checkbox.disabled = (!!problem && !checkbox.checked) || busy || state.running;
      checkbox.addEventListener("change", () => {
        if (checkbox.checked) selected.add(id); else selected.delete(id);
        renderInputs(); renderState();
      });
      const details = node("div", undefined, "track-details");
      const title = `${t("track")} ${stream.index} · ${stream.profile || stream.codec_name}${track.title ? " · " + track.title : ""}`;
      details.append(node("div", title, "track-title"));
      const duration = track.duration > 0 ? `${Math.floor(track.duration / 60)}:${String(Math.floor(track.duration % 60)).padStart(2, "0")}` : "";
      details.append(node("div", [track.language, stream.channel_layout || `${stream.channels} ch`,
        info.sample_rate ? `${info.sample_rate} Hz` : "", info.effective_bits ? `${info.effective_bits} bit` : "",
        duration, track.default ? t("default") : ""].filter(Boolean).join(" · "), "track-meta"));
      if (problem) details.append(node("p", `${t("unsupported")}: ${problem}`, "problem"));
      label.append(checkbox, details);
      file.append(label);
    }
    fragment.append(file);
  }
  $("inputs").replaceChildren(fragment);
  $("empty").hidden = inputs.length > 0;
}

function renderState() {
  const locked = busy || state.running;
  for (const id of ["add", "clear", "truehd", "eac3", "concurrency", "browse", "reset-output", "overwrite"]) $(id).disabled = locked;
  $("drop").setAttribute("aria-disabled", String(locked));
  for (const id of ["bitrate51", "bitrate71"]) $(id).disabled = locked || !$("eac3").checked;
  $("start").disabled = locked || !backend || selected.size === 0 || incompatibleSelection() || (!$("truehd").checked && !$("eac3").checked);
  $("cancel").disabled = !state.running || cancelling;
  $("cancel").textContent = t(cancelling ? "cancelling" : "cancel");
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
    if (job.error) item.append(node("p", job.error, "problem"));
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
for (const id of ["truehd", "eac3"]) $(id).addEventListener("change", () => { renderInputs(); renderState(); });
$("reset-output").addEventListener("click", () => { $("output").value = ""; });
$("browse").addEventListener("click", async () => {
  if (!backend) return;
  try { const path = await backend.ChooseOutput(); if (path) $("output").value = path; }
  catch (error) { showError(error); }
});
$("start").addEventListener("click", async () => {
  clearError();
  const tracks = [...selected].map((id) => { const [path, index] = JSON.parse(id); return {path, index}; });
  if (tracks.length === 0 || (!$("truehd").checked && !$("eac3").checked)) { showError(t("choose")); return; }
  if (incompatibleSelection()) { showError(t("incompatible")); return; }
  busy = true;
  renderInputs();
  renderState();
  try {
    // Keep zero as "automatic" and send kb/s, not bits/s. The backend resolves
    // and validates each track's layout-specific rate during queue preflight.
    acceptState(await backend.Start({tracks, truehd: $("truehd").checked, eac3: $("eac3").checked,
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
