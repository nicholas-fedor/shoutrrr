/**
 * Shoutrrr Playground - Frontend JavaScript
 *
 * Loads the WASM module and provides the interactive UI for configuring,
 * generating, and testing Shoutrrr notification URLs.
 *
 * @module ShoutrrrPlayground
 */

(function () {
  "use strict";

  /**
   * @typedef {Object} FieldSchema
   * @property {string} name - The field name
   * @property {string} type - The field type (string, bool, int, uint, enum, array)
   * @property {boolean} required - Whether the field is required
   * @property {string} description - Field description
   * @property {string} defaultValue - Default value
   * @property {string} urlPart - URL part mapping (host, user, password, path)
   * @property {string[]} keys - Query parameter keys
   * @property {string[]} [enumValues] - Enum options if type is enum
   */

  /**
   * @typedef {Object} ConfigSchema
   * @property {string} service - Service name
   * @property {string} scheme - URL scheme
   * @property {FieldSchema[]} fields - Configuration fields
   */

  /**
   * @typedef {Object} ParseResult
   * @property {string} service - Service name
   * @property {Object<string, string>} config - Field values
   */

  /**
   * @typedef {Object} WASMResult
   * @property {string} [error] - Error message if failed
   * @property {Object} [result] - Result data if succeeded
   */

  // --- State ---
  /** @type {boolean} */
  let wasmReady = false;

  /** @type {string} */
  let currentService = "";

  /** @type {ConfigSchema|null} */
  let currentSchema = null;

  // --- DOM Elements ---
  /** @param {string} id */
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
  // Go's net/http sets User-Agent automatically, but some CORS
  // preflight responses don't allow this header.
  const originalFetch = window.fetch.bind(window);

  /** @type {typeof fetch} */
  window.fetch = function (url, options) {
    if (options && options.headers) {
      const headers = new Headers(options.headers);
      headers.delete("user-agent");
      headers.delete("User-Agent");
      options = Object.assign({}, options, { headers: headers });
    }
    return originalFetch(url, options);
  };

  /**
   * Loads the WASM module and initializes the playground.
   * @async
   * @param {PlaygroundOptions} config - Configuration options
   * @returns {Promise<void>}
   */
  async function loadWasm(config) {
    try {
      const go = new Go();
      const result = await WebAssembly.instantiateStreaming(
        fetch(config.wasmPath),
        go.importObject
      );
      go.run(result.instance);
      wasmReady = true;
      onWasmReady(config);
    } catch (err) {
      loading.innerHTML =
        '<p class="playground-error">Failed to load WASM module: ' +
        escapeHtml(err.message) +
        "</p>";
    }
  }

  /**
   * Called when WASM module is ready.
   * @param {PlaygroundOptions} config - Configuration options
   */
  function onWasmReady(config) {
    loading.style.display = "none";
    content.style.display = "block";
    loadServices();
  }

  // --- Service Listing ---

  /** Loads available services into the dropdown. */
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

  /** @type {HTMLSelectElement} */
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

  /**
   * Renders the configuration form from a schema.
   * @param {ConfigSchema} schema - The service config schema
   */
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

  /**
   * Formats a field name for display.
   * @param {FieldSchema} field - The field schema
   * @returns {string} Formatted field name
   */
  function formatFieldName(field) {
    let name = field.keys && field.keys.length > 0 ? field.keys[0] : field.name;
    if (field.urlPart) {
      name += " (" + field.urlPart + ")";
    }
    return name;
  }

  // --- Config Section Toggle ---

  /** @type {string} */
  var collapseIcon = '<use href="#icon-chevron"/>';

  /** @type {string} */
  var expandIcon = '<use href="#icon-chevron"/>';

  configToggle.addEventListener("click", function () {
    var expanded = configToggle.getAttribute("aria-expanded") === "true";
    configToggle.setAttribute("aria-expanded", !expanded);
    configContent.style.display = expanded ? "none" : "";
    configToggle.querySelector("svg").innerHTML = expanded
      ? expandIcon
      : collapseIcon;
  });

  // --- Clear Config ---

  /** Clears all form inputs and regenerates the URL. */
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

  // --- Copy Button ---

  /** @type {string} */
  var clipboardIcon = '<use href="#icon-clipboard"/>';

  /** @type {string} */
  var checkIcon = '<use href="#icon-check"/>';

  /**
   * Creates a copy button for an input or code element.
   * @param {HTMLElement} sourceEl - Element to copy from
   * @returns {HTMLButtonElement} The copy button
   */
  function createCopyButton(sourceEl) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "playground-inline-copy-btn";
    btn.setAttribute("aria-label", "Copy to clipboard");
    btn.innerHTML =
      '<svg width="14" height="14">' + clipboardIcon + "</svg>";

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

  // --- Input Creators ---

  /**
   * Creates a text input for a field.
   * @param {FieldSchema} field - The field schema
   * @returns {HTMLInputElement} The input element
   */
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

  /**
   * Creates a checkbox input for a boolean field.
   * @param {FieldSchema} field - The field schema
   * @returns {HTMLDivElement} Wrapper with checkbox and hint
   */
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

  /**
   * Creates a select input for an enum field.
   * @param {FieldSchema} field - The field schema
   * @returns {HTMLSelectElement} The select element
   */
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

  /**
   * Creates a number input for int/uint fields.
   * @param {FieldSchema} field - The field schema
   * @returns {HTMLInputElement} The input element
   */
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

  /**
   * Creates a text input for array fields.
   * @param {FieldSchema} field - The field schema
   * @returns {HTMLInputElement} The input element
   */
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

  /** Updates the generated URL and CLI command from form values. */
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

  /**
   * Builds a CLI command string.
   * @param {string} url - The Shoutrrr URL
   * @param {string} [message] - Override message (uses input value if not provided)
   * @returns {string} The CLI command
   */
  function buildCliCommand(url, message) {
    var config = (window.__shoutrrrPlayground || {}).config || {};
    var defaultMsg = config.defaultMessage || "Hello World";
    var msg = message || messageInput.value.trim() || defaultMsg;
    return 'shoutrrr send --url "' + url + '" --message "' + msg + '"';
  }

  /**
   * Collects all form values into a config object.
   * @returns {Object<string, string>} Config key-value pairs
   */
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

  /**
   * Parses a Shoutrrr URL from the input field.
   * Called on paste and debounced on input.
   */
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

  /**
   * Populates form fields from parsed values.
   * @param {Object<string, string>} values - Field name-value pairs
   */
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

  /** @type {number|null} */
  var copyBtnTimeout = null;

  /** @type {number|null} */
  var copyCliBtnTimeout = null;

  /**
   * Handles copy button click.
   * @param {HTMLButtonElement} btn - The copy button
   * @param {HTMLElement} sourceEl - Element to copy from
   * @param {number|null} timeoutRef - Timeout reference for reset
   */
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

  /** Sends a test message using the configured URL. */
  sendBtn.addEventListener("click", function () {
    if (!wasmReady || !currentService) return;

    const url = urlOutput.textContent;
    var config = (window.__shoutrrrPlayground || {}).config || {};
    const message = messageInput.value.trim() || config.defaultMessage || "Hello World";

    if (!url || url.startsWith("Error")) {
      sendResult.innerHTML =
        '<span class="playground-error">Generate a valid URL first.</span>';
      return;
    }

    sendBtn.disabled = true;
    sendResult.innerHTML = "Sending...";

    // shoutrrrSend returns a Promise to avoid blocking the JS event loop.
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

  /**
   * Returns a user-friendly error message for common send failures.
   * @param {string} msg - The raw error message
   * @returns {string} User-friendly error message
   */
  function describeSendError(msg) {
    if (
      msg.indexOf("Failed to fetch") !== -1 ||
      msg.indexOf("NetworkError") !== -1
    ) {
      return (
        "Request blocked by browser. This is usually caused by a " +
        "privacy extension (e.g., Privacy Badger, uBlock Origin) or " +
        "content filter blocking the request. Try disabling extensions " +
        "for this site, or use the Shoutrrr CLI."
      );
    }
    if (msg.indexOf("ERR_BLOCKED_BY_CLIENT") !== -1) {
      return (
        "Request blocked by browser extension or content filter. " +
        "Try disabling extensions for this site, or use the Shoutrrr CLI."
      );
    }
    if (
      msg.indexOf("CORS") !== -1 ||
      msg.indexOf("Access-Control-Allow-Origin") !== -1 ||
      msg.indexOf("ERR_FAILED") !== -1
    ) {
      return (
        "Request blocked by CORS policy. The target server does not " +
        "allow cross-origin requests from browsers. Browsers enforce CORS " +
        "as a security measure, but the Shoutrrr CLI is not subject to " +
        "this restriction and will work for this service."
      );
    }
    if (msg.indexOf("timed out") !== -1) {
      return "Request timed out. Check your network connection and try again.";
    }
    return "Error: " + msg;
  }

  /**
   * Safely parses a JSON string.
   * @param {string} str - JSON string to parse
   * @returns {WASMResult} Parsed result or error
   */
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

  /**
   * Creates a debounced version of a function.
   * @param {Function} fn - Function to debounce
   * @param {number} delay - Delay in milliseconds
   * @returns {Function} Debounced function
   */
  function debounce(fn, delay) {
    let timer;
    return function () {
      clearTimeout(timer);
      timer = setTimeout(fn, delay);
    };
  }

  /**
   * Escapes HTML special characters.
   * @param {string} str - String to escape
   * @returns {string} Escaped string
   */
  function escapeHtml(str) {
    const div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
  }

  // --- Init ---

  /**
   * @typedef {Object} PlaygroundOptions
   * @property {string} [wasmPath] - Path to the WASM binary
   * @property {string} [containerId] - Container element ID
   * @property {string} [defaultMessage] - Default test message
   */

  /**
   * Initializes the Shoutrrr Playground.
   * @param {PlaygroundOptions} [options] - Configuration options
   */
  function init(options) {
    // Configuration can be overridden via options or global config.
    var config = Object.assign(
      {
        wasmPath: "assets/shoutrrr.wasm",
        containerId: "playground-app",
        defaultMessage: "Hello World",
      },
      options || {},
      window.ShoutrrrPlaygroundConfig || {}
    );

    // Store config for use by other functions.
    window.__shoutrrrPlayground = { config: config };

    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", function () {
        loadWasm(config);
      });
    } else {
      loadWasm(config);
    }
  }

  // Expose public API.
  window.ShoutrrrPlayground = {
    init: init,
  };

  // Auto-init if container exists (for standalone usage).
  if (document.getElementById("playground-app")) {
    init();
  }
})();
