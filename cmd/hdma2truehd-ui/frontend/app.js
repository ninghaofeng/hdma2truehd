"use strict";

const translations = {
  zh: {
    title: "音轨转换", language: "界面语言", drop: "拖入 DTS 文件或 MKV 电影",
    local: "支持批量添加 · 直接读取本地文件，不上传、不复制整部电影", add: "选择文件",
    tracks: "文件与音轨", clear: "清空列表", empty: "添加文件后，在这里选择需要转换的音轨。",
    settings: "输出设置", formats: "输出格式", lossless: "无损", lossy: "有损",
    bitrate51: "E-AC-3 5.1 码率", auto51: "自动：960 kb/s",
    bitrate71: "E-AC-3 7.1 总码率", auto71: "自动：1536 kb/s",
    limits: "E-AC-3：仅支持 48 kHz 的 5.1(side) / 7.1。5.1 码率范围 384–1024，7.1 为 768–1536 kb/s。混合队列按音轨布局分别使用对应设置。",
    output: "输出目录", browse: "选择目录", reset: "重置", sameDir: "与每个源文件相同的目录",
    overwrite: "允许覆盖已有输出（默认不覆盖）",
    notice: "DTS:X 仅转换兼容声道，不渲染对象。TrueHD 校验完整 PCM；E-AC-3 校验完整解码与长度，并保留 256 采样编码延迟和帧补零。两种输出独立发布，不封装回 MKV。",
    queue: "转换队列", start: "开始转换", cancel: "取消转换", cancelling: "正在取消…",
    busy: "正在读取音轨信息…", idle: "就绪", working: "正在编码与完整校验；进度条按已完成任务计数。",
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
    bitrate51: "E-AC-3 5.1 bitrate", auto51: "Automatic: 960 kb/s",
    bitrate71: "E-AC-3 7.1 total bitrate", auto71: "Automatic: 1536 kb/s",
    limits: "E-AC-3: 48 kHz 5.1(side) / 7.1 only. 5.1: 384–1024; 7.1: 768–1536 kb/s. Mixed queues use each track's layout-specific setting.",
    output: "Output directory", browse: "Browse", reset: "Reset", sameDir: "Same directory as each source file",
    overwrite: "Allow replacing existing outputs (off by default)",
    notice: "DTS:X compatible channels only, no object rendering. TrueHD verifies complete PCM; E-AC-3 validates full decoding and length, retaining 256 samples of delay and frame padding. Outputs are published independently, not remuxed into MKV.",
    queue: "Conversion queue", start: "Start conversion", cancel: "Cancel", cancelling: "Cancelling…",
    busy: "Reading audio tracks…", idle: "Ready", working: "Encoding and full validation; progress counts completed jobs.",
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
let busy = false;
let cancelling = false;
let backend = null;
const t = (key) => translations[language][key];
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
      checkbox.disabled = !!track.problem || busy || state.running;
      checkbox.addEventListener("change", () => {
        if (checkbox.checked) selected.add(id); else selected.delete(id);
        renderState();
      });
      const details = node("div", undefined, "track-details");
      const title = `${t("track")} ${stream.index} · ${stream.profile || stream.codec_name}${track.title ? " · " + track.title : ""}`;
      details.append(node("div", title, "track-title"));
      const duration = track.duration > 0 ? `${Math.floor(track.duration / 60)}:${String(Math.floor(track.duration % 60)).padStart(2, "0")}` : "";
      details.append(node("div", [track.language, stream.channel_layout || `${stream.channels} ch`,
        info.sample_rate ? `${info.sample_rate} Hz` : "", info.effective_bits ? `${info.effective_bits} bit` : "",
        duration, track.default ? t("default") : ""].filter(Boolean).join(" · "), "track-meta"));
      if (track.problem) details.append(node("p", `${t("unsupported")}: ${track.problem}`, "problem"));
      else if (track.eac3Problem) details.append(node("p", `${t("eac3Unavailable")}: ${track.eac3Problem}`, "problem"));
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
  for (const id of ["add", "clear", "truehd", "eac3", "browse", "reset-output", "overwrite"]) $(id).disabled = locked;
  $("drop").setAttribute("aria-disabled", String(locked));
  for (const id of ["bitrate51", "bitrate71"]) $(id).disabled = locked || !$("eac3").checked;
  $("start").disabled = locked || !backend || selected.size === 0 || (!$("truehd").checked && !$("eac3").checked);
  $("cancel").disabled = !state.running || cancelling;
  $("cancel").textContent = t(cancelling ? "cancelling" : "cancel");
  $("output").placeholder = t("sameDir");
  $("activity").textContent = t(busy ? "busy" : state.running ? "working" : "idle");
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
  state = next;
  if (!state.running) cancelling = false;
  renderInputs();
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
        for (const track of input.tracks) if (!track.problem) selected.add(keyFor(input.path, track.info.stream.index));
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
$("truehd").addEventListener("change", renderState);
$("eac3").addEventListener("change", renderState);
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
  busy = true;
  renderInputs();
  renderState();
  try {
    acceptState(await backend.Start({tracks, truehd: $("truehd").checked, eac3: $("eac3").checked,
      bitrate51: $("eac3").checked ? Number($("bitrate51").value) : 0,
      bitrate71: $("eac3").checked ? Number($("bitrate71").value) : 0,
      outputDir: $("output").value, overwrite: $("overwrite").checked}));
  } catch (error) { showError(error); }
  finally { busy = false; renderInputs(); renderState(); }
});
$("cancel").addEventListener("click", async () => {
  cancelling = true;
  renderState();
  try { await backend.Cancel(); }
  catch (error) { cancelling = false; showError(error); renderState(); }
});

applyLanguage();
if (window.go?.main?.App && window.runtime) {
  backend = window.go.main.App;
  window.runtime.EventsOn("queue", acceptState);
  window.runtime.OnFileDrop((_x, _y, paths) => addFiles(paths), false);
  backend.State().then(acceptState).catch(showError);
  renderState();
} else {
  showError(t("unavailable"));
}
