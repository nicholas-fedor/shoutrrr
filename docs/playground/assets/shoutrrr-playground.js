// Shoutrrr Playground - Frontend JavaScript
// Loads the WASM module and provides the interactive UI for configuring,
// generating, and testing Shoutrrr notification URLs.

(function () {
  "use strict";

  // --- State ---
  let wasmReady = false;
  let currentService = "";
  let currentSchema = null;

  // --- DOM Elements ---
  const $ = (id) => document.getElementById(id);
  const loading = $("playground-loading");
  const content = $("playground-content");
  const serviceSelect = $("service-select");
  const urlInput = $("url-input");
  const parseBtn = $("parse-btn");
  const configSection = $("config-section");
  const configTbody = $("config-tbody");
  const outputSection = $("output-section");
  const urlOutput = $("url-output");
  const copyBtn = $("copy-btn");
  const cliOutput = $("cli-output");
  const copyCliBtn = $("copy-cli-btn");
  const sendSection = $("send-section");
  const messageInput = $("message-input");
  const sendBtn = $("send-btn");
  const sendResult = $("send-result");

  // --- WASM Loading ---

  // Patch fetch to strip the User-Agent header from WASM requests.
  // Go's net/http sets User-Agent automatically, but Discord's CORS
  // preflight doesn't allow this header. Stripping it lets the
  // preflight succeed while Shoutrrr handles all other request details.
  const originalFetch = window.fetch.bind(window);
  window.fetch = function (url, options) {
    if (options && options.headers) {
      const headers = new Headers(options.headers);
      headers.delete("user-agent");
      headers.delete("User-Agent");
      options = Object.assign({}, options, { headers: headers });
    }
    return originalFetch(url, options);
  };

  async function loadWasm() {
    try {
      const go = new Go();
      const wasmPath = new URL("assets/shoutrrr.wasm", window.location.href).href;
      const result = await WebAssembly.instantiateStreaming(
        fetch(wasmPath),
        go.importObject
      );
      go.run(result.instance);
      wasmReady = true;
      onWasmReady();
    } catch (err) {
      loading.innerHTML =
        '<p class="playground-error">Failed to load WASM module: ' +
        escapeHtml(err.message) +
        "</p>";
    }
  }

  function onWasmReady() {
    loading.style.display = "none";
    content.style.display = "block";
    loadServices();
  }

  // --- Service Listing ---
  function loadServices() {
    const parsed = safeParseJSON(shoutrrrGetServices());
    if (parsed.error) {
      serviceSelect.innerHTML =
        '<option value="">Error loading services</option>';
      return;
    }

    parsed.result.sort();
    for (const svc of parsed.result) {
      const opt = document.createElement("option");
      opt.value = svc;
      opt.textContent = svc;
      serviceSelect.appendChild(opt);
    }
  }

  // --- Service Selection ---
  serviceSelect.addEventListener("change", function () {
    currentService = this.value;
    if (!currentService) {
      configSection.style.display = "none";
      outputSection.style.display = "none";
      sendSection.style.display = "none";
      currentSchema = null;
      return;
    }

    const parsed = safeParseJSON(shoutrrrGetConfigSchema(currentService));
    if (parsed.error) {
      configTbody.innerHTML =
        '<tr><td colspan="4" class="playground-error">Error: ' +
        escapeHtml(parsed.error) +
        "</td></tr>";
      configSection.style.display = "block";
      return;
    }

    currentSchema = parsed.result;
    renderForm(parsed.result);
    configSection.style.display = "block";
    outputSection.style.display = "block";
    sendSection.style.display = "block";
    updateUrl();
  });

  // --- Form Rendering ---
  function renderForm(schema) {
    configTbody.innerHTML = "";

    // Sort fields: required first, then optional.
    var fields = schema.fields.slice().sort(function (a, b) {
      if (a.required === b.required) return 0;
      return a.required ? -1 : 1;
    });

    for (const field of fields) {
      const row = document.createElement("tr");

      // Name column
      const nameCell = document.createElement("td");
      const nameSpan = document.createElement("span");
      nameSpan.className = "playground-field-name";
      nameSpan.textContent = formatFieldName(field);
      nameCell.appendChild(nameSpan);

      if (field.required) {
        const req = document.createElement("span");
        req.className = "playground-required";
        req.textContent = "*";
        nameCell.appendChild(req);
      }

      row.appendChild(nameCell);

      // Type column
      const typeCell = document.createElement("td");
      if (field.type && field.type !== "string") {
        const typeSpan = document.createElement("span");
        typeSpan.className = "playground-field-type";
        typeSpan.textContent = field.type;
        typeCell.appendChild(typeSpan);
      }
      row.appendChild(typeCell);

      // Description column
      const descCell = document.createElement("td");
      descCell.className = "playground-description";
      descCell.textContent = field.description || "";
      row.appendChild(descCell);

      // Value column
      const valueCell = document.createElement("td");
      let input;
      if (field.enumValues && field.enumValues.length > 0) {
        input = createEnumInput(field);
      } else if (field.type === "bool") {
        input = createBoolInput(field);
      } else if (field.type === "int" || field.type === "uint") {
        input = createNumberInput(field);
      } else if (field.type === "array") {
        input = createArrayInput(field);
      } else {
        input = createTextInput(field);
      }

      valueCell.appendChild(input);
      row.appendChild(valueCell);

      configTbody.appendChild(row);
    }
  }

  function formatFieldName(field) {
    let name = field.keys && field.keys.length > 0 ? field.keys[0] : field.name;
    if (field.urlPart) {
      name += " (" + field.urlPart + ")";
    }
    return name;
  }

  function createTextInput(field) {
    const input = document.createElement("input");
    input.type = field.urlPart === "password" ? "password" : "text";
    input.id = "field-" + field.name;
    input.name = field.name;
    input.dataset.fieldType = "string";
    input.autocomplete = "off";

    if (field.defaultValue) {
      input.placeholder = field.defaultValue;
    }

    input.addEventListener("input", debounce(updateUrl, 300));
    return input;
  }

  function createBoolInput(field) {
    const wrapper = document.createElement("div");
    wrapper.className = "playground-checkbox-wrapper";

    const input = document.createElement("input");
    input.type = "checkbox";
    input.id = "field-" + field.name;
    input.name = field.name;
    input.dataset.fieldType = "bool";
    input.autocomplete = "off";

    if (
      field.defaultValue &&
      (field.defaultValue.toLowerCase() === "yes" ||
        field.defaultValue.toLowerCase() === "true")
    ) {
      input.checked = true;
    }

    input.addEventListener("change", updateUrl);

    const checkLabel = document.createElement("span");
    checkLabel.className = "playground-checkbox-hint";
    checkLabel.textContent = field.defaultValue
      ? "default: " + field.defaultValue.toLowerCase()
      : "";

    wrapper.appendChild(input);
    wrapper.appendChild(checkLabel);
    return wrapper;
  }

  function createEnumInput(field) {
    const select = document.createElement("select");
    select.id = "field-" + field.name;
    select.name = field.name;
    select.dataset.fieldType = "enum";
    select.autocomplete = "off";

    for (const val of field.enumValues) {
      const opt = document.createElement("option");
      opt.value = val;
      opt.textContent = val;
      if (val === field.defaultValue) {
        opt.selected = true;
      }
      select.appendChild(opt);
    }

    select.addEventListener("change", updateUrl);
    return select;
  }

  function createNumberInput(field) {
    const input = document.createElement("input");
    input.type = "text";
    input.id = "field-" + field.name;
    input.name = field.name;
    input.dataset.fieldType = field.type;
    input.pattern = "[0-9]*";
    input.autocomplete = "off";

    if (field.defaultValue) {
      input.placeholder = field.defaultValue;
    }

    input.addEventListener("input", debounce(updateUrl, 300));
    return input;
  }

  function createArrayInput(field) {
    const input = document.createElement("input");
    input.type = "text";
    input.id = "field-" + field.name;
    input.name = field.name;
    input.dataset.fieldType = "array";
    input.autocomplete = "off";

    if (field.defaultValue) {
      input.placeholder = field.defaultValue;
    }

    input.addEventListener("input", debounce(updateUrl, 300));
    return input;
  }

  // --- URL Generation ---
  function updateUrl() {
    if (!currentService || !wasmReady) return;

    const config = collectFormValues();
    const parsed = safeParseJSON(
      shoutrrrGenerateURL(currentService, JSON.stringify(config))
    );

    if (parsed.error) {
      urlOutput.textContent = "Error: " + parsed.error;
      urlOutput.className = "playground-error";
      cliOutput.textContent = "";
      cliOutput.className = "";
    } else if (parsed.result.url) {
      var url = parsed.result.url;
      urlOutput.textContent = url;
      urlOutput.className = "";
      cliOutput.textContent = buildCliCommand(url);
      cliOutput.className = "";
    } else {
      urlOutput.textContent = "Error: unexpected response";
      urlOutput.className = "playground-error";
      cliOutput.textContent = "";
      cliOutput.className = "";
    }
  }

  function buildCliCommand(url) {
    return 'shoutrrr send --url "' + url + '" --message "Hello World"';
  }

  function collectFormValues() {
    const values = {};
    const fields = configTbody.querySelectorAll("[name]");

    for (const field of fields) {
      const name = field.name;
      const type = field.dataset.fieldType;

      if (type === "bool") {
        values[name] = field.checked ? "Yes" : "No";
      } else {
        values[name] = field.value;
      }
    }

    return values;
  }

  // --- URL Parsing ---
  parseBtn.addEventListener("click", function () {
    const rawUrl = urlInput.value.trim();
    if (!rawUrl || !wasmReady) return;

    const parsed = safeParseJSON(shoutrrrParseURL(rawUrl));

    if (parsed.error) {
      urlOutput.textContent = "Error: " + parsed.error;
      urlOutput.className = "playground-error";
      outputSection.style.display = "block";
      return;
    }

    currentService = parsed.result.service;
    serviceSelect.value = currentService;

    const schemaParsed = safeParseJSON(shoutrrrGetConfigSchema(currentService));
    if (schemaParsed.error) return;

    currentSchema = schemaParsed.result;
    renderForm(schemaParsed.result);
    populateForm(parsed.result.config);
    configSection.style.display = "block";
    outputSection.style.display = "block";
    sendSection.style.display = "block";
    updateUrl();
  });

  function populateForm(values) {
    for (const [name, value] of Object.entries(values)) {
      const field = configTbody.querySelector(
        '[name="' + CSS.escape(name) + '"]'
      );
      if (!field) continue;

      if (field.type === "checkbox") {
        field.checked =
          value.toLowerCase() === "yes" || value.toLowerCase() === "true";
      } else {
        field.value = value;
      }
    }
  }

  // --- Copy to Clipboard ---
  var copyBtnTimeout = null;

  copyBtn.addEventListener("click", function () {
    const text = urlOutput.textContent;
    if (!text || text.startsWith("Error")) return;

    navigator.clipboard.writeText(text).then(function () {
      clearTimeout(copyBtnTimeout);
      copyBtn.textContent = "Copied!";
      copyBtnTimeout = setTimeout(function () {
        copyBtn.textContent = "Copy";
      }, 2000);
    });
  });

  var copyCliBtnTimeout = null;

  copyCliBtn.addEventListener("click", function () {
    const text = cliOutput.textContent;
    if (!text) return;

    navigator.clipboard.writeText(text).then(function () {
      clearTimeout(copyCliBtnTimeout);
      copyCliBtn.textContent = "Copied!";
      copyCliBtnTimeout = setTimeout(function () {
        copyCliBtn.textContent = "Copy";
      }, 2000);
    });
  });

  // --- Send Message ---
  sendBtn.addEventListener("click", function () {
    if (!wasmReady || !currentService) return;

    const url = urlOutput.textContent;
    const message =
      messageInput.value.trim() || "Hello World";

    if (!url || url.startsWith("Error")) {
      sendResult.innerHTML =
        '<span class="playground-error">Generate a valid URL first.</span>';
      return;
    }

    sendBtn.disabled = true;
    sendResult.innerHTML = "Sending...";

    // shoutrrrSend returns a Promise to avoid blocking the JS event loop.
    // The fetch runs in a Go goroutine; resolve/reject are called when done.
    var promise = shoutrrrSend(url, message);
    promise.then(
      function (resultJSON) {
        var parsed = safeParseJSON(resultJSON);
        if (parsed.error) {
          sendResult.innerHTML =
            '<span class="playground-error">Error: ' +
            escapeHtml(parsed.error) +
            "</span>";
        } else {
          sendResult.innerHTML =
            '<span class="playground-success">Message sent successfully.</span>';
        }
        sendBtn.disabled = false;
      },
      function (errorJSON) {
        var parsed = safeParseJSON(errorJSON);
        var msg = parsed.error || String(errorJSON);
        var friendly = describeSendError(msg);
        sendResult.innerHTML =
          '<span class="playground-error">' + escapeHtml(friendly) + "</span>";
        sendBtn.disabled = false;
      }
    );
  });

  // --- Utilities ---
  function describeSendError(msg) {
    if (msg.indexOf("Failed to fetch") !== -1 ||
        msg.indexOf("NetworkError") !== -1) {
      return "Request blocked by browser. This is usually caused by a " +
        "privacy extension (e.g., Privacy Badger, uBlock Origin) or " +
        "content filter blocking the request. Try disabling extensions " +
        "for this site, or use the Shoutrrr CLI.";
    }
    if (msg.indexOf("ERR_BLOCKED_BY_CLIENT") !== -1) {
      return "Request blocked by browser extension or content filter. " +
        "Try disabling extensions for this site, or use the Shoutrrr CLI.";
    }
    if (msg.indexOf("CORS") !== -1 ||
        msg.indexOf("Access-Control-Allow-Origin") !== -1 ||
        msg.indexOf("ERR_FAILED") !== -1) {
      return "Request blocked by CORS policy. The target server does not " +
        "allow cross-origin requests from browsers. Browsers enforce CORS " +
        "as a security measure, but the Shoutrrr CLI is not subject to " +
        "this restriction and will work for this service.";
    }
    if (msg.indexOf("timed out") !== -1) {
      return "Request timed out. Check your network connection and try again.";
    }
    return "Error: " + msg;
  }

  function safeParseJSON(str) {
    if (typeof str !== "string") {
      return { error: "Invalid response: expected string" };
    }
    try {
      const parsed = JSON.parse(str);
      if (parsed && parsed.error) return { error: parsed.error };
      return { result: parsed };
    } catch (e) {
      return { error: "Invalid response: " + str.substring(0, 100) };
    }
  }

  function debounce(fn, delay) {
    let timer;
    return function () {
      clearTimeout(timer);
      timer = setTimeout(fn, delay);
    };
  }

  function escapeHtml(str) {
    const div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
  }

  // --- Init ---
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", loadWasm);
  } else {
    loadWasm();
  }
})();
