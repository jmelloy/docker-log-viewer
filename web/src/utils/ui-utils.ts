/**
 * UI Utility Functions
 * Shared utilities for UI interactions and DOM manipulation
 */

interface HighlightOptions {
  jsonSelector?: string;
  graphqlSelector?: string;
  sqlSelector?: string;
}

/**
 * Applies syntax highlighting to code blocks using highlight.js
 * @param options - Highlighting options
 */
export function applySyntaxHighlighting(options: HighlightOptions = {}) {
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
        const text = (block.textContent || "").trim();
        if (text.startsWith("{") || text.startsWith("[")) {
          const highlighted = (hljs as any).highlight(text, { language: "json" });
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
        const text = (block.textContent || "").trim();
        const highlighted = (hljs as any).highlight(text, { language: "graphql" });
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
        const text = block.textContent || "";
        const highlighted = (hljs as any).highlight(text, { language: "sql" });
        block.innerHTML = highlighted.value;
        block.classList.add("hljs");
      } catch (e) {
        console.error("Error highlighting SQL query:", e);
      }
    });
  }
}

interface ClipboardOptions {
  showNotification?: boolean;
  notificationText?: string;
  notificationDuration?: number;
}

/**
 * Copies text to clipboard and shows a notification
 * @param text - The text to copy
 * @param options - Options for the notification
 */
export async function copyToClipboard(text: string, options: ClipboardOptions = {}) {
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

/**
 * Normalizes a SQL query for comparison
 * @param query - The SQL query to normalize
 */
export function normalizeQuery(query: string): string {
  return query
    .replace(/\$\d+/g, "$N")
    .replace(/'[^']*'/g, "'?'")
    .replace(/\d+/g, "N")
    .replace(/\s+/g, " ")
    .trim();
}
