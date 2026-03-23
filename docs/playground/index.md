---
hide:
  - toc
  - navigation
---

<div class="playground-header">
  <h1>Shoutrrr Playground</h1>
  <p>Generate  and test Shoutrrr URLs directly in your browser.</p>
</div>

<div id="playground-app">
  <noscript>JavaScript is required for the Playground.</noscript>
  <div id="playground-loading">
    <p>Loading Shoutrrr WASM module...</p>
  </div>
  <div id="playground-content" style="display:none;">
    <!-- Service selector and URL parser -->
    <div class="playground-section playground-section--centered">
      <div class="playground-input-row">
        <div class="playground-input-group">
          <h3>Select a Service</h3>
          <select id="service-select">
            <option value=""></option>
          </select>
        </div>
        <div class="playground-input-or">or</div>
        <div class="playground-input-group">
          <h3>Parse a Shoutrrr URL</h3>
          <input
            type="text"
            id="url-input"
            placeholder="discord://token@webhookID?color=0x50D9ff"
            aria-label="Shoutrrr URL to parse"
            autocomplete="off"
          />
        </div>
      </div>
    </div>

    <!-- Config form -->
    <div id="config-section" class="playground-section" style="display:none;">
      <div class="playground-section-header">
        <h3>
          <button id="config-toggle" type="button" class="playground-collapse-btn" aria-expanded="true">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path d="M12.78 5.22a.749.749 0 0 1 0 1.06l-4.25 4.25a.749.749 0 0 1-1.06 0L3.22 6.28a.749.749 0 1 1 1.06-1.06L8 8.94l3.72-3.72a.749.749 0 0 1 1.06 0Z"></path></svg>
            Configuration
          </button>
        </h3>
        <button id="clear-config-btn" type="button" class="playground-clear-btn" aria-label="Clear all configuration values">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="14" height="14"><path d="M3.72 3.72a.75.75 0 0 1 1.06 0L8 6.94l3.22-3.22a.749.749 0 0 1 1.275.326.749.749 0 0 1-.215.734L9.06 8l3.22 3.22a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215L8 9.06l-3.22 3.22a.751.751 0 0 1-1.042-.018.751.751 0 0 1-.018-1.042L6.94 8 3.72 4.78a.75.75 0 0 1 0-1.06Z"></path></svg>
          Clear
        </button>
      </div>
      <div id="config-content" class="playground-config-table-wrapper">
        <table id="config-table" class="playground-config-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Description</th>
              <th>Value</th>
            </tr>
          </thead>
          <tbody id="config-tbody"></tbody>
        </table>
      </div>
    </div>

    <!-- Output: Shoutrrr URL -->
    <div id="output-section" class="playground-section" style="display:none;">
      <h3>Generated Shoutrrr URL</h3>
      <div class="playground-output-group">
        <div class="playground-output-item">
          <div class="playground-code-block">
            <code id="url-output" role="textbox" aria-readonly="true" aria-label="Generated Shoutrrr URL"></code>
            <button id="copy-btn" type="button" aria-label="Copy URL to clipboard" class="playground-copy-btn">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"></path><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"></path></svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Send test -->
    <div id="send-section" class="playground-section" style="display:none;">
      <h3>Test</h3>
      <div class="playground-output-item">
        <label>Message</label>
        <div class="playground-send">
          <input
            type="text"
            id="message-input"
            placeholder="Hello World"
            aria-label="Test message"
          />
          <button id="send-btn" type="button">Send</button>
        </div>
        <div id="send-result" aria-live="polite"></div>
      </div>
      <div class="playground-output-group">
        <div class="playground-output-item">
          <label>CLI Command</label>
          <div class="playground-code-block">
            <code id="cli-output" role="textbox" aria-readonly="true" aria-label="Generated CLI command"></code>
            <button id="copy-cli-btn" type="button" aria-label="Copy CLI command to clipboard" class="playground-copy-btn">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"></path><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"></path></svg>
            </button>
          </div>
        </div>
      </div>
      <p class="playground-note">Send may not work for all services due to browser CORS restrictions. Use the <a href="/usage/cli/" target="_blank">Shoutrrr CLI</a> for guaranteed compatibility.</p>
    </div>
  </div>
</div>
