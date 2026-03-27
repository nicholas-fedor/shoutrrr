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
  var wasmReady = false;

  /** @type {string} */
  var currentService = "";

  // --- DOM Elements (initialized lazily) ---
  var dom = {};

  // Cached DOM element for escapeHtml to avoid repeated allocation.
  var _escapeDiv = document.createElement("div");

  /**
   * Initializes DOM element references.
   * Called after DOM is ready to ensure elements exist.
   */
  function initDOMRefs() {
    dom.loading = document.getElementById("playground-loading");
    dom.content = document.getElementById("playground-content");
    dom.serviceSelect = document.getElementById("service-select");
    dom.urlInput = document.getElementById("url-input");
    dom.configSection = document.getElementById("config-section");
    dom.configToggle = document.getElementById("config-toggle");
    dom.configContent = document.getElementById("config-content");
    dom.clearConfigBtn = document.getElementById("clear-config-btn");
    dom.configTbody = document.getElementById("config-tbody");
    dom.outputSection = document.getElementById("output-section");
    dom.urlOutput = document.getElementById("url-output");
    dom.copyBtn = document.getElementById("copy-btn");
    dom.cliOutput = document.getElementById("cli-output");
    dom.copyCliBtn = document.getElementById("copy-cli-btn");
    dom.sendSection = document.getElementById("send-section");
    dom.messageInput = document.getElementById("message-input");
    dom.sendBtn = document.getElementById("send-btn");
    dom.sendResult = document.getElementById("send-result");
  }

  /**
   * Attaches all event listeners after DOM is ready.
   */
  function attachEventListeners() {
    // Service selection
    dom.serviceSelect.addEventListener("change", function () {
      currentService = this.value;
      if (!currentService) {
        dom.configSection.style.display = "none";
        dom.outputSection.style.display = "none";
        dom.sendSection.style.display = "none";
        return;
      }

      const parsed = safeParseJSON(shoutrrrGetConfigSchema(currentService));
      if (parsed.error) {
        dom.configTbody.innerHTML =
          '<tr><td colspan="4" class="playground-error">Error: ' +
          escapeHtml(parsed.error) +
          "</td></tr>";
        dom.configSection.style.display = "block";
        return;
      }

      renderForm(parsed.result);
      dom.configSection.style.display = "block";
      dom.outputSection.style.display = "block";
      dom.sendSection.style.display = "block";

      // Show just the scheme URL as default, update when user changes fields.
      dom.urlOutput.textContent = currentService + "://";
      dom.urlOutput.className = "";
      dom.cliOutput.textContent = buildCliCommand(currentService + "://");
      dom.cliOutput.className = "";
    });

    // Config section toggle - uses CSS class for visual state
    dom.configToggle.addEventListener("click", function () {
      var expanded = dom.configToggle.getAttribute("aria-expanded") === "true";
      dom.configToggle.setAttribute("aria-expanded", String(!expanded));
      dom.configContent.style.display = expanded ? "none" : "";
      dom.configToggle.classList.toggle("collapsed", expanded);
    });

    // Clear config - restores each field to its schema-defined default.
    dom.clearConfigBtn.addEventListener("click", function () {
      var inputs = dom.configTbody.querySelectorAll("input, select");
      inputs.forEach(function (el) {
        if (el.type === "checkbox") {
          // Prefer data-default attribute, fall back to intrinsic defaultChecked
          // or the presence of the HTML checked attribute.
          var def = el.getAttribute("data-default");
          if (def !== null) {
            el.checked =
              def.toLowerCase() === "yes" || def.toLowerCase() === "true";
          } else {
            el.checked = el.hasAttribute("checked") || el.defaultChecked;
          }
        } else {
          // Prefer data-default attribute, fall back to intrinsic defaultValue.
          el.value =
            el.getAttribute("data-default") || el.defaultValue || "";
        }
      });
      updateUrl();
    });

    // Copy URL button
    dom.copyBtn.addEventListener("click", function () {
      handleCopy(dom.copyBtn, dom.urlOutput, copyBtnTimeout);
    });

    // Copy CLI button
    dom.copyCliBtn.addEventListener("click", function () {
      handleCopy(dom.copyCliBtn, dom.cliOutput, copyCliBtnTimeout);
    });

    // URL input - auto-parse on paste
    dom.urlInput.addEventListener("paste", function () {
      setTimeout(parseUrlInput, 0);
    });

    // URL input - parse on input with debounce
    dom.urlInput.addEventListener("input", debounce(parseUrlInput, 500));

    // Message input - update CLI command
    dom.messageInput.addEventListener("input", debounce(function () {
      if (!dom.urlOutput.classList.contains("playground-error")) {
        var url = dom.urlOutput.textContent;
        if (url) {
          dom.cliOutput.textContent = buildCliCommand(url);
        }
      }
    }, 300));

    // Send button
    dom.sendBtn.addEventListener("click", function () {
      if (!wasmReady || !currentService) return;

      var url = dom.urlOutput.textContent;
      var config = (window.__shoutrrrPlayground || {}).config || {};
      var message = dom.messageInput.value.trim() || config.defaultMessage || "Hello World";

      if (!url || dom.urlOutput.classList.contains("playground-error")) {
        dom.sendResult.innerHTML =
          '<span class="playground-error">Generate a valid URL first.</span>';
        return;
      }

      dom.sendBtn.disabled = true;
      dom.sendResult.innerHTML = "Sending...";

      var promise = shoutrrrSend(url, message);
      promise.then(
        function (resultJSON) {
          var parsed = safeParseJSON(resultJSON);
          if (parsed.error) {
            dom.sendResult.innerHTML =
              '<span class="playground-error">Error: ' +
              escapeHtml(parsed.error) +
              "</span>";
          } else {
            dom.sendResult.innerHTML =
              '<span class="playground-success">Message sent successfully.</span>';
          }
          dom.sendBtn.disabled = false;
        },
        function (errorJSON) {
          var parsed = safeParseJSON(errorJSON);
          var msg = parsed.error || String(errorJSON);
          var friendly = describeSendError(msg);
          dom.sendResult.innerHTML =
            '<span class="playground-error">' + escapeHtml(friendly) + "</span>";
          dom.sendBtn.disabled = false;
        }
      );
    });
  }

  // --- WASM Loading ---

  // Store the original fetch for non-WASM page requests.
  const originalFetch = window.fetch.bind(window);

  /**
   * Fetch wrapper used exclusively for WASM-initiated HTTP requests.
   * Strips the User-Agent header to avoid browser security restrictions
   * on WASM net/http calls.
   * @param {RequestInfo} url - The request URL or Request object
   * @param {RequestInit} [options] - Fetch options
   * @returns {Promise<Response>}
   */
  function wasmFetch(url, options) {
    if (options && options.headers) {
      var headers = new Headers(options.headers);
      headers.delete("user-agent");
      headers.delete("User-Agent");
      options = Object.assign({}, options, { headers: headers });
    }
    return originalFetch(url, options);
  }

  /**
   * Loads the WASM module and initializes the playground.
   *
   * Temporarily overrides window.fetch with wasmFetch so that WASM-initiated
   * HTTP requests have User-Agent stripped. The override is restored once
   * go.run settles or the WASM module signals readiness.
   *
   * @async
   * @param {PlaygroundOptions} config - Configuration options
   * @returns {Promise<void>}
   */
  async function loadWasm(config) {
    try {
      var go = new Go();
      var result = await WebAssembly.instantiateStreaming(
        fetch(config.wasmPath),
        go.importObject
      );

      // Temporarily override window.fetch for WASM-initiated requests only.
      window.fetch = wasmFetch;

      // go.run returns a promise that may not resolve for long-running
      // interactive WASM modules. Attach handlers to catch initialization
      // errors and restore the original fetch when the WASM exits.
      go.run(result.instance).then(
        function () {
          window.fetch = originalFetch;
        },
        function (err) {
          window.fetch = originalFetch;
          dom.loading.innerHTML =
            '<p class="playground-error">WASM runtime error: ' +
            escapeHtml(err.message) +
            "</p>";
        }
      );

      // The WASM Go runtime is initialised and functions are callable
      // immediately after go.run starts the program.
      wasmReady = true;
      onWasmReady();
    } catch (err) {
      // Restore fetch on any error during loading.
      window.fetch = originalFetch;
      dom.loading.innerHTML =
        '<p class="playground-error">Failed to load WASM module: ' +
        escapeHtml(err.message) +
        "</p>";
    }
  }

  /** Called when WASM module is ready. */
  function onWasmReady() {
    dom.loading.style.display = "none";
    dom.content.style.display = "block";
    loadServices();
  }

  // --- Service Listing ---

  /** Loads available services into the dropdown. */
  function loadServices() {
    var parsed = safeParseJSON(shoutrrrGetServices());
    if (parsed.error) {
      dom.serviceSelect.innerHTML =
        '<option value="">Error loading services</option>';
      return;
    }

    parsed.result.sort();
    for (var i = 0; i < parsed.result.length; i++) {
      var opt = document.createElement("option");
      opt.value = parsed.result[i];
      opt.textContent = parsed.result[i];
      dom.serviceSelect.appendChild(opt);
    }
  }

  // --- Form Rendering ---

  /**
   * Renders the configuration form from a schema.
   * @param {ConfigSchema} schema - The service config schema
   */
  function renderForm(schema) {
    dom.configTbody.innerHTML = "";

    // Sort fields: required first, then optional.
    var fields = schema.fields.slice().sort(function (a, b) {
      if (a.required === b.required) return 0;
      return a.required ? -1 : 1;
    });

    for (var i = 0; i < fields.length; i++) {
      var field = fields[i];
      var row = document.createElement("tr");

      // Name column
      var nameCell = document.createElement("td");
      var nameSpan = document.createElement("span");
      nameSpan.className = "playground-field-name";
      nameSpan.textContent = formatFieldName(field);
      nameCell.appendChild(nameSpan);

      if (field.required) {
        var req = document.createElement("span");
        req.className = "playground-required";
        req.textContent = "*";
        nameCell.appendChild(req);
      }

      row.appendChild(nameCell);

      // Type column
      var typeCell = document.createElement("td");
      if (field.type && field.type !== "string") {
        var typeSpan = document.createElement("span");
        typeSpan.className = "playground-field-type";
        typeSpan.textContent = field.type;
        typeCell.appendChild(typeSpan);
      }
      row.appendChild(typeCell);

      // Description column
      var descCell = document.createElement("td");
      descCell.className = "playground-description";
      descCell.textContent = field.description || "";
      row.appendChild(descCell);

      // Value column
      var valueCell = document.createElement("td");
      var input;
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
        var wrapper = document.createElement("div");
        wrapper.className = "playground-input-copy-wrapper";
        wrapper.appendChild(input);
        wrapper.appendChild(createCopyButton(input));
        valueCell.appendChild(wrapper);
      } else {
        valueCell.appendChild(input);
      }
      row.appendChild(valueCell);

      dom.configTbody.appendChild(row);
    }
  }

  /**
   * Formats a field name for display.
   * @param {FieldSchema} field - The field schema
   * @returns {string} Formatted field name
   */
  function formatFieldName(field) {
    var name = field.keys && field.keys.length > 0 ? field.keys[0] : field.name;
    if (field.urlPart) {
      name += " (" + field.urlPart + ")";
    }
    return name;
  }

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
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "playground-inline-copy-btn";
    btn.setAttribute("aria-label", "Copy to clipboard");
    btn.innerHTML = '<svg width="14" height="14">' + clipboardIcon + "</svg>";

    // Use object wrapper for mutable timeout reference
    var timeoutRef = { timeout: null };

    btn.addEventListener("click", function () {
      var value = sourceEl.value || sourceEl.textContent;
      if (!value) return;

      navigator.clipboard
        .writeText(value)
        .then(function () {
          clearTimeout(timeoutRef.timeout);
          btn.classList.add("copied");
          btn.querySelector("svg").innerHTML = checkIcon;
          timeoutRef.timeout = setTimeout(function () {
            btn.classList.remove("copied");
            btn.querySelector("svg").innerHTML = clipboardIcon;
          }, 1500);
        })
        .catch(function (err) {
          clearTimeout(timeoutRef.timeout);
          btn.classList.remove("copied");
          btn.querySelector("svg").innerHTML = clipboardIcon;
          btn.setAttribute("aria-label", "Copy failed");
          console.error("Clipboard write failed:", err);
          timeoutRef.timeout = setTimeout(function () {
            btn.setAttribute("aria-label", "Copy to clipboard");
          }, 3000);
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
    var input = document.createElement("input");
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
    var wrapper = document.createElement("div");
    wrapper.className = "playground-checkbox-wrapper";

    var input = document.createElement("input");
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

    var checkLabel = document.createElement("span");
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
    var select = document.createElement("select");
    select.id = "field-" + field.name;
    select.name = field.name;
    select.dataset.fieldType = "enum";
    select.autocomplete = "off";

    // Insert an empty placeholder option for non-required enums with no default
    // so the browser does not auto-select the first value.
    if (!field.required && !field.defaultValue) {
      var placeholder = document.createElement("option");
      placeholder.value = "";
      placeholder.textContent = "";
      select.appendChild(placeholder);
    }

    for (var i = 0; i < field.enumValues.length; i++) {
      var val = field.enumValues[i];
      var opt = document.createElement("option");
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
    var input = document.createElement("input");
    input.type = "text";
    input.id = "field-" + field.name;
    input.name = field.name;
    input.dataset.fieldType = field.type;
    input.pattern = "[0-9]*";
    input.inputMode = "numeric";
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
    var input = document.createElement("input");
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

    var config = collectFormValues();
    var parsed = safeParseJSON(
      shoutrrrGenerateURL(currentService, JSON.stringify(config))
    );

    if (parsed.error) {
      dom.urlOutput.textContent = "Error: " + parsed.error;
      dom.urlOutput.className = "playground-error";
      dom.cliOutput.textContent = "";
      dom.cliOutput.className = "";
    } else if (parsed.result.url) {
      var url = parsed.result.url;
      dom.urlOutput.textContent = url;
      dom.urlOutput.className = "";
      dom.cliOutput.textContent = buildCliCommand(url);
      dom.cliOutput.className = "";
    } else {
      dom.urlOutput.textContent = "Error: unexpected response";
      dom.urlOutput.className = "playground-error";
      dom.cliOutput.textContent = "";
      dom.cliOutput.className = "";
    }
  }

  /**
   * Escapes a value for safe use in a POSIX shell command.
   * Wraps in single quotes and escapes embedded single quotes.
   * @param {string} value - Value to escape
   * @returns {string} Shell-safe escaped value
   */
  function shellEscape(value) {
    return "'" + value.replace(/'/g, "'\"'\"'") + "'";
  }

  /**
   * Builds a CLI command string with shell-safe escaping.
   * @param {string} url - The Shoutrrr URL
   * @param {string} [message] - Override message
   * @returns {string} The CLI command
   */
  function buildCliCommand(url, message) {
    var config = (window.__shoutrrrPlayground || {}).config || {};
    var defaultMsg = config.defaultMessage || "Hello World";
    var msg = message || dom.messageInput.value.trim() || defaultMsg;
    return "shoutrrr send --url " + shellEscape(url) + " --message " + shellEscape(msg);
  }

  /**
   * Collects all form values into a config object.
   * @returns {Object<string, string>} Config key-value pairs
   */
  function collectFormValues() {
    var values = {};
    var fields = dom.configTbody.querySelectorAll("[name]");

    for (var i = 0; i < fields.length; i++) {
      var field = fields[i];
      var name = field.name;
      var type = field.dataset.fieldType;

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
    var rawUrl = dom.urlInput.value.trim();
    if (!rawUrl || !wasmReady) return;

    var parsed = safeParseJSON(shoutrrrParseURL(rawUrl));

    if (parsed.error) {
      dom.urlOutput.textContent = "Error: " + parsed.error;
      dom.urlOutput.className = "playground-error";
      dom.outputSection.style.display = "block";
      return;
    }

    currentService = parsed.result.service;
    dom.serviceSelect.value = currentService;

    var schemaParsed = safeParseJSON(shoutrrrGetConfigSchema(currentService));
    if (schemaParsed.error) return;

    renderForm(schemaParsed.result);
    populateForm(parsed.result.config);
    dom.configSection.style.display = "block";
    dom.outputSection.style.display = "block";
    dom.sendSection.style.display = "block";
    updateUrl();
  }

  /**
   * Populates form fields from parsed values.
   * @param {Object<string, string>} values - Field name-value pairs
   */
  function populateForm(values) {
    var keys = Object.keys(values);
    for (var i = 0; i < keys.length; i++) {
      var name = keys[i];
      var value = values[name];
      var field = dom.configTbody.querySelector(
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

  /**
   * Handles copy button click with mutable timeout reference.
   * @param {HTMLButtonElement} btn - The copy button
   * @param {HTMLElement} sourceEl - Element to copy from
   * @param {Object} timeoutRef - Mutable object with .timeout property
   */
  function handleCopy(btn, sourceEl, timeoutRef) {
    if (sourceEl.classList.contains("playground-error")) return;
    var text = sourceEl.textContent;
    if (!text) return;

    navigator.clipboard
      .writeText(text)
      .then(function () {
        clearTimeout(timeoutRef.timeout);
        btn.classList.add("copied");
        btn.querySelector("svg").innerHTML = checkIcon;
        timeoutRef.timeout = setTimeout(function () {
          btn.classList.remove("copied");
          btn.querySelector("svg").innerHTML = clipboardIcon;
        }, 1500);
      })
      .catch(function (err) {
        clearTimeout(timeoutRef.timeout);
        btn.classList.remove("copied");
        btn.querySelector("svg").innerHTML = clipboardIcon;
        btn.setAttribute("aria-label", "Copy failed");
        console.error("Clipboard write failed:", err);
        timeoutRef.timeout = setTimeout(function () {
          btn.setAttribute("aria-label", "Copy to clipboard");
        }, 3000);
      });
  }

  // Mutable timeout references for copy buttons.
  var copyBtnTimeout = { timeout: null };
  var copyCliBtnTimeout = { timeout: null };

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
      var parsed = JSON.parse(str);
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
    var timer;
    return function () {
      var ctx = this;
      var args = arguments;
      clearTimeout(timer);
      timer = setTimeout(function () {
        fn.apply(ctx, args);
      }, delay);
    };
  }

  /**
   * Escapes HTML special characters.
   * @param {string} str - String to escape
   * @returns {string} Escaped string
   */
  function escapeHtml(str) {
    _escapeDiv.textContent = str;
    return _escapeDiv.innerHTML;
  }

  // --- Init ---

  /** @type {boolean} Guard to prevent duplicate initialisation. */
  var initialised = false;

  /**
   * @typedef {Object} PlaygroundOptions
   * @property {string} [wasmPath] - Path to the WASM binary
   * @property {string} [containerId] - Container element ID
   * @property {string} [defaultMessage] - Default test message
   * @property {boolean} [autoInit] - Set to false to skip auto-init
   */

  /**
   * Initializes the Shoutrrr Playground.
   * Safe to call multiple times; subsequent calls are no-ops.
   * @param {PlaygroundOptions} [options] - Configuration options
   */
  function init(options) {
    if (initialised) return;

    var config = Object.assign(
      {
        wasmPath: "assets/shoutrrr.wasm",
        containerId: "playground-app",
        defaultMessage: "Hello World",
        autoInit: true,
      },
      window.ShoutrrrPlaygroundConfig || {},
      options || {}
    );

    initialised = true;
    window.__shoutrrrPlayground = { config: config, initialised: true };

    var startWasm = function () {
      initDOMRefs();
      attachEventListeners();
      loadWasm(config);
    };

    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", startWasm);
    } else {
      startWasm();
    }
  }

  // Expose public API.
  window.ShoutrrrPlayground = {
    init: init,
  };

  // Auto-init if container exists, deferring to DOMContentLoaded if needed.
  function tryAutoInit() {
    var cfg = Object.assign(
      {},
      window.ShoutrrrPlaygroundConfig || {}
    );
    if (cfg.autoInit === false) return;

    if (document.getElementById("playground-app")) {
      init();
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", tryAutoInit);
  } else {
    tryAutoInit();
  }
})();
