---
hide:
  - toc
  - navigation
---

<div class="playground-header">
  <h1>Shoutrrr Playground</h1>
  <p>Configure and test Shoutrrr notification URLs directly in your browser.</p>
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
          <label for="service-select">Service</label>
          <select id="service-select">
            <option value="">Select a service...</option>
          </select>
        </div>
        <div class="playground-input-or">or</div>
        <div class="playground-input-group playground-input-group--grow">
          <label for="url-input">Parse existing URL</label>
          <div class="playground-url-input">
            <input
              type="text"
              id="url-input"
              placeholder="discord://token@webhookID?color=0x50D9ff"
              aria-label="Shoutrrr URL to parse"
            />
            <button id="parse-btn" type="button">Parse</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Config form -->
    <div id="config-section" class="playground-section" style="display:none;">
      <h3>Configuration</h3>
      <div class="playground-config-table-wrapper">
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

    <!-- Output: Generated URL and CLI Command -->
    <div id="output-section" class="playground-section" style="display:none;">
      <h3>Output</h3>
      <div class="playground-output-group">
        <div class="playground-output-item">
          <label>Generated URL</label>
          <div class="playground-url-output">
            <code id="url-output" role="textbox" aria-readonly="true" aria-label="Generated Shoutrrr URL"></code>
            <button id="copy-btn" type="button" aria-label="Copy URL to clipboard">Copy</button>
          </div>
        </div>
        <div class="playground-output-item">
          <label>CLI Command</label>
          <div class="playground-url-output">
            <code id="cli-output" role="textbox" aria-readonly="true" aria-label="Generated CLI command"></code>
            <button id="copy-cli-btn" type="button" aria-label="Copy CLI command to clipboard">Copy</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Send test -->
    <div id="send-section" class="playground-section" style="display:none;">
      <h3>Test</h3>
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
  </div>
</div>
