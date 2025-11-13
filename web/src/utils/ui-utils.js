/**
 * UI Utility Functions
 * Shared utilities for UI interactions and DOM manipulation
 */

/**
 * Applies syntax highlighting to code blocks using highlight.js
 * @param {Object} options - Highlighting options
 * @param {string} options.jsonSelector - Selector for JSON blocks (default: ".json-display")
 * @param {string} options.graphqlSelector - Selector for GraphQL blocks (default: ".graphql-query")
 * @param {string} options.sqlSelector - Selector for SQL blocks (default: ".sql-query-text, .query-text-compact")
 */
export function applySyntaxHighlighting(options = {}) {
  // Only apply if hljs is available
  if (typeof hljs === "undefined") return;

  const {
    jsonSelector = ".json-display",
    graphqlSelector = ".graphql-query",
    sqlSelector = ".sql-query-text, .query-text-compact",
  } = options;

  // Highlight JSON (only if selector is provided)
  if (jsonSelector) {
    document.querySelectorAll(`${jsonSelector}:not(.hljs)`).forEach((block) => {
      try {
        const text = block.textContent.trim();
        if (text.startsWith("{") || text.startsWith("[")) {
          const highlighted = hljs.highlight(text, { language: "json" });
          block.innerHTML = highlighted.value;
          block.classList.add("hljs");
        }
      } catch (e) {
        console.error("Error highlighting JSON:", e);
      }
    });
  }

  // Highlight GraphQL queries (only if selector is provided)
  if (graphqlSelector) {
    document.querySelectorAll(`${graphqlSelector}:not(.hljs)`).forEach((block) => {
      try {
        const text = block.textContent.trim();
        const highlighted = hljs.highlight(text, { language: "graphql" });
        block.innerHTML = highlighted.value;
        block.classList.add("hljs");
      } catch (e) {
        console.error("Error highlighting GraphQL query:", e);
      }
    });
  }

  // Highlight SQL queries (only if selector is provided)
  if (sqlSelector) {
    document.querySelectorAll(`${sqlSelector}:not(.hljs)`).forEach((block) => {
      try {
        const text = block.textContent;
        const highlighted = hljs.highlight(text, { language: "sql" });
        block.innerHTML = highlighted.value;
        block.classList.add("hljs");
      } catch (e) {
        console.error("Error highlighting SQL query:", e);
      }
    });
  }
}

/**
 * Copies text to clipboard and shows a notification
 * @param {string} text - The text to copy
 * @param {Object} options - Options for the notification
 * @param {boolean} options.showNotification - Whether to show a notification (default: true)
 * @param {string} options.notificationText - Custom notification text (default: "Copied to clipboard!")
 * @param {number} options.notificationDuration - How long to show notification in ms (default: 2000)
 */
export async function copyToClipboard(text, options = {}) {
  const { showNotification = true, notificationText = "Copied to clipboard!", notificationDuration = 2000 } = options;

  try {
    await navigator.clipboard.writeText(text);
    if (showNotification) {
      // Show a brief notification
      const notification = document.createElement("div");
      notification.textContent = notificationText;
      notification.style.cssText =
        "position: fixed; top: 20px; right: 20px; background: #238636; color: white; padding: 0.75rem 1rem; border-radius: 4px; z-index: 10000; font-size: 0.875rem;";
      document.body.appendChild(notification);
      setTimeout(() => notification.remove(), notificationDuration);
    }
  } catch (err) {
    console.error("Failed to copy:", err);
    alert("Failed to copy to clipboard");
  }
}
