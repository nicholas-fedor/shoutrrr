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
  const configSection = $("config-section");
  const configToggle = $("config-toggle");
  const configContent = $("config-content");
  const clearConfigBtn = $("clear-config-btn");
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

      // Wrap input with copy button for non-checkbox fields
      if (field.type !== "bool") {
        const wrapper = document.createElement("div");
        wrapper.className = "playground-input-copy-wrapper";
        wrapper.appendChild(input);
        wrapper.appendChild(createCopyButton(input));
        valueCell.appendChild(wrapper);
      } else {
        valueCell.appendChild(input);
      }
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

  // --- Config Section Toggle ---
  var collapseIcon =
    '<path d="M12.78 5.22a.749.749 0 0 1 0 1.06l-4.25 4.25a.749.749 0 0 1-1.06 0L3.22 6.28a.749.749 0 1 1 1.06-1.06L8 8.94l3.72-3.72a.749.749 0 0 1 1.06 0Z"></path>';
  var expandIcon =
    '<path d="M6.22 3.22a.749.749 0 0 1 1.06 0l4.25 4.25a.749.749 0 0 1 0 1.06l-4.25 4.25a.749.749 0 0 1-1.06-1.06L9.94 8 6.22 4.28a.749.749 0 0 1 0-1.06Z"></path>';

  configToggle.addEventListener("click", function () {
    var expanded = configToggle.getAttribute("aria-expanded") === "true";
    configToggle.setAttribute("aria-expanded", !expanded);
    configContent.style.display = expanded ? "none" : "";
    configToggle.querySelector("svg").innerHTML = expanded
      ? expandIcon
      : collapseIcon;
  });

  // --- Clear Config ---
  clearConfigBtn.addEventListener("click", function () {
    var inputs = configTbody.querySelectorAll("input, select");
    inputs.forEach(function (el) {
      if (el.type === "checkbox") {
        el.checked = false;
      } else {
        el.value = "";
      }
    });
    updateUrl();
  });

  var clipboardIcon =
    '<path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"></path><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"></path>';
  var checkIcon =
    '<path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z"></path>';

  function createCopyButton(sourceEl) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "playground-inline-copy-btn";
    btn.setAttribute("aria-label", "Copy to clipboard");
    btn.innerHTML =
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="14" height="14">' +
      clipboardIcon +
      "</svg>";

    btn.addEventListener("click", function () {
      var value = sourceEl.value || sourceEl.textContent;
      if (!value) return;

      navigator.clipboard.writeText(value).then(function () {
        btn.classList.add("copied");
        btn.querySelector("svg").innerHTML = checkIcon;
        setTimeout(function () {
          btn.classList.remove("copied");
          btn.querySelector("svg").innerHTML = clipboardIcon;
        }, 1500);
      });
    });

    return btn;
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

  function buildCliCommand(url, message) {
    var msg = message || messageInput.value.trim() || "Hello World";
    return 'shoutrrr send --url "' + url + '" --message "' + msg + '"';
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
  // --- URL Auto-Parse ---
  // Automatically parse Shoutrrr URLs on paste or after typing stops.
  function parseUrlInput() {
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
  }

  // Parse immediately on paste.
  urlInput.addEventListener("paste", function () {
    setTimeout(parseUrlInput, 0);
  });

  // Parse on input with debounce for typing.
  urlInput.addEventListener("input", debounce(parseUrlInput, 500));

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
  var copyCliBtnTimeout = null;

  function handleCopy(btn, sourceEl, timeoutRef) {
    var text = sourceEl.textContent;
    if (!text || text.startsWith("Error")) return;

    navigator.clipboard.writeText(text).then(function () {
      clearTimeout(timeoutRef);
      btn.classList.add("copied");
      btn.querySelector("svg").innerHTML = checkIcon;
      timeoutRef = setTimeout(function () {
        btn.classList.remove("copied");
        btn.querySelector("svg").innerHTML = clipboardIcon;
      }, 1500);
    });
  }

  copyBtn.addEventListener("click", function () {
    handleCopy(copyBtn, urlOutput, copyBtnTimeout);
  });

  copyCliBtn.addEventListener("click", function () {
    handleCopy(copyCliBtn, cliOutput, copyCliBtnTimeout);
  });

  // --- Message Input ---
  // Update CLI command when message changes.
  messageInput.addEventListener("input", debounce(function () {
    var url = urlOutput.textContent;
    if (url && !url.startsWith("Error")) {
      cliOutput.textContent = buildCliCommand(url);
    }
  }, 300));

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
