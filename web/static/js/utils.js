export function formatSQL(sql) {
  if (!sql?.trim()) return sql;

  let formatted = sql.replace(/\s+/g, " ").trim();

  const keywords = [
    "SELECT",
    "WHERE",
    "GROUP BY",
    "ORDER BY",
    "HAVING",
    "UNION",
    "INSERT INTO",
    "UPDATE",
    "SET",
    "VALUES",
  ];

  formatted = formatted.replaceAll(new RegExp(`\\b(LEFT JOIN|RIGHT JOIN|INNER JOIN|FULL JOIN|JOIN)\\b`, "gi"), `\n$1`);

  formatted = formatted.replaceAll(new RegExp(`\\b(DELETE FROM|FROM)\\b`, "gi"), `\n$1`);

  keywords.forEach((kw) => {
    formatted = formatted.replace(new RegExp(`\\b${kw}\\b`, "gi"), `\n${kw.toUpperCase()}`);
  });

  const lines = formatted
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean);
  const output = [];
  const indentStack = [];

  for (const line of lines) {
    if (/^(SELECT|FROM|WHERE|GROUP BY|ORDER BY|HAVING|UNION)$/i.test(line)) {
      indentStack.length = 0;
    }

    const opens = (line.match(/\(/g) || []).length;
    const closes = (line.match(/\)/g) || []).length;
    const netChange = opens - closes;

    if (netChange <= 0) {
      // Break on AND/OR outside parens
      let depth = 0;
      let result = "  ".repeat(indentStack.length);
      let i = 0;
      while (i < line.length) {
        const char = line[i];
        if (char === "(") depth++;
        else if (char === ")") depth--;

        if (depth == 0) {
          const remaining = line.substring(i);
          if (/^\s+(AND|OR)\b/i.test(remaining)) {
            const match = remaining.match(/^(\s+)(AND|OR)\b/i);
            result += "\n" + "  ".repeat(indentStack.length + 1) + match[2].toUpperCase();
            i += match[0].length;
            continue;
          }

          if (result.length > 100 && char == ",") {
            output.push(result + ",");
            result = "  ".repeat(indentStack.length + 1);
            i += 1;
            continue;
          }
        }
        result += char;
        i++;
      }
      output.push(result);

      if (netChange < 0) {
        for (let i = 0; i < Math.abs(netChange); i++) {
          indentStack.pop();
        }
      }
    } else {
      output.push("  ".repeat(indentStack.length) + line);
    }

    if (netChange > 0) {
      for (let i = 0; i < netChange; i++) {
        indentStack.push(true);
      }
    }
  }

  return output.join("\n");
}

/**
 * Escapes HTML special characters in a string
 * @param {string} text - The text to escape
 * @returns {string} The escaped HTML string
 */
export function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

/**
 * Converts ANSI escape codes to HTML spans with CSS classes
 * @param {string} text - The text containing ANSI escape codes
 * @returns {string} HTML string with ANSI codes converted to spans
 */
export function convertAnsiToHtml(text) {
  const ansiMap = {
    0: "",
    1: "ansi-bold",
    30: "ansi-gray",
    31: "ansi-red",
    32: "ansi-green",
    33: "ansi-yellow",
    34: "ansi-blue",
    35: "ansi-magenta",
    36: "ansi-cyan",
    37: "ansi-white",
    90: "ansi-gray",
    91: "ansi-bright-red",
    92: "ansi-bright-green",
    93: "ansi-bright-yellow",
    94: "ansi-bright-blue",
    95: "ansi-bright-magenta",
    96: "ansi-bright-cyan",
    97: "ansi-bright-white",
  };

  const parts = [];
  const regex = /\x1b\[([0-9;]+)m/g;
  let lastIndex = 0;
  let currentClasses = [];
  let match;

  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      const content = text.substring(lastIndex, match.index);
      if (currentClasses.length > 0) {
        parts.push(`<span class="${currentClasses.join(" ")}">${escapeHtml(content)}</span>`);
      } else {
        parts.push(escapeHtml(content));
      }
    }

    const codes = match[1].split(";");
    currentClasses = [];
    codes.forEach((code) => {
      if (ansiMap[code]) {
        currentClasses.push(ansiMap[code]);
      }
    });

    lastIndex = regex.lastIndex;
  }

  if (lastIndex < text.length) {
    const content = text.substring(lastIndex);
    if (currentClasses.length > 0) {
      parts.push(`<span class="${currentClasses.join(" ")}">${escapeHtml(content)}</span>`);
    } else {
      parts.push(escapeHtml(content));
    }
  }

  return parts.join("");
}

/**
 * Formats a field value for display, with special handling for certain field types
 * @param {string} key - The field key/name
 * @param {*} value - The field value
 * @param {Object} options - Formatting options
 * @param {number} options.maxLength - Maximum length before truncation (default: 50)
 * @param {boolean} options.handleStackTrace - Whether to handle stack_trace specially (default: true)
 * @returns {string} Formatted field value
 */
export function formatFieldValue(key, value, options = {}) {
  const { maxLength = 50, handleStackTrace = true } = options;
  const s = String(value);

  if (handleStackTrace && key === "stack_trace") {
    const ret = [];
    value.split("\\n").forEach((line, index) => {
      if (index < 5) {
        ret.push(line);
      }
    });
    return ret.join(" ").replaceAll("\\t", "    ");
  }

  if (key === "error" || key === "db.error") {
    return value;
  }

  return s.length > maxLength ? s.substring(0, 20) + "..." : s;
}

/**
 * Checks if a value looks like JSON (starts with { or [)
 * @param {*} value - The value to check
 * @returns {boolean} True if the value appears to be JSON
 */
export function isJsonField(value) {
  if (typeof value !== "string") return false;
  const trimmed = value.trim();
  return trimmed.startsWith("{") || trimmed.startsWith("[");
}

/**
 * Formats a JSON string with proper indentation
 * @param {string} value - The JSON string to format
 * @returns {string} Formatted JSON string, or original value if parsing fails
 */
export function formatJsonField(value) {
  try {
    const parsed = JSON.parse(value);
    return JSON.stringify(parsed, null, 2);
  } catch (e) {
    console.error("Error formatting JSON field:", e);
    return value;
  }
}

/**
 * Normalizes a SQL query by replacing parameters, literals, and numbers with placeholders
 * Used for grouping similar queries together
 * @param {string} query - The SQL query to normalize
 * @returns {string} Normalized query string
 */
export function normalizeQuery(query) {
  return query
    .replace(/\$\d+/g, "$N")
    .replace(/'[^']*'/g, "'?'")
    .replace(/\d+/g, "N")
    .replace(/\s+/g, " ")
    .trim();
}
