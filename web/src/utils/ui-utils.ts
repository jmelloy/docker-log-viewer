/**
 * UI Utility Functions
 * Shared utilities for UI interactions and DOM manipulation
 */

import hljs from "highlight.js/lib/core";
import json from "highlight.js/lib/languages/json";
import sql from "highlight.js/lib/languages/sql";
import graphql from "highlight.js/lib/languages/graphql";
import type { Plan, PlanNodeType } from "@/types";

// Register languages
hljs.registerLanguage("json", json);
hljs.registerLanguage("sql", sql);
hljs.registerLanguage("graphql", graphql);

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
        const text = (block.textContent || "").trim();
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
        const text = block.textContent || "";
        const highlighted = hljs.highlight(text, { language: "sql" });
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

/**
 * Escapes HTML special characters
 * @param text - The text to escape
 */
export function escapeHtml(text: string): string {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

/**
 * Converts ANSI color codes to HTML with CSS classes
 * @param text - The text with ANSI codes
 */
export function convertAnsiToHtml(text: string): string {
  const ansiMap: Record<number, string> = {
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

  const parts: string[] = [];
  const regex = /\x1b\[([0-9;]+)m/g;
  let lastIndex = 0;
  let currentClasses: string[] = [];
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
      const ansiClass = ansiMap[parseInt(code)];
      if (ansiClass) {
        currentClasses.push(ansiClass);
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
 * Formats SQL for better readability
 * @param sql - The SQL query to format
 */
export function formatSQL(sql: string): string {
  if (!sql?.trim()) return sql;

  let formatted = sql.replace(/\s+/g, " ").trim();

  const keywords = [
    `\\bSELECT\\b`,
    `(?<!\\()\\bWHERE\\b`,
    `\\bGROUP BY\\b`,
    `\\bORDER BY\\b`,
    `\\bHAVING\\b`,
    `\\bUNION ALL\\b`,
    `\\bUNION\\b`,
    `\\bINSERT INTO\\b`,
    `\\bUPDATE\\b`,
    `\\bSET\\b`,
    `\\bVALUES\\b`,
    `\\b(?:LEFT|RIGHT|INNER|FULL)?\\s*JOIN\\b`,
    `\\bDELETE FROM\\b`,
    `\\bFROM\\b`,
  ];

  keywords.forEach((kw) => {
    formatted = formatted.replace(new RegExp(kw, "gi"), (match) => `\n${match.toUpperCase()}`);
  });
  const lines = formatted
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean);
  const output: string[] = [];
  const indentStack: boolean[] = [];

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
      const chunkPositions = [];
      while (i < line.length) {
        const char = line[i];
        if (char === "(") {
          if (depth === -1) {
            indentStack.push(true);
          }
          depth++;
        } else if (char === ")") {
          depth--;
          if (depth < 0) {
            indentStack.pop();
            result += "\n" + "  ".repeat(indentStack.length);
          }
        }

        if (depth == 0) {
          const remaining = line.substring(i);
          if (/^\s+(AND|OR)\b/i.test(remaining)) {
            const match = remaining.match(/^(\s+)(AND|OR)\b/i);
            if (match) {
              result += "\n" + "  ".repeat(indentStack.length + 1) + match[2].toUpperCase();
              i += match[0].length;
              continue;
            }
          }

          if (char == ",") {
            chunkPositions.push(i);
          }
        }
        result += char;
        i++;
      }

      if (chunkPositions.length > 0 && result.length > 100) {
        chunkPositions.push(line.length);

        result = "  ".repeat(indentStack.length);
        let previousChunkPosition = 0;
        for (const position of chunkPositions) {
          var chunk = line.substring(previousChunkPosition, position + 1);
          if (result.length + chunk.length > 100) {
            if (result.trim().length > 0) {
              output.push(result);
            }
            result = "  ".repeat(indentStack.length + 1);
          }
          result += chunk;
          previousChunkPosition = position + 1;
        }
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

export function explainPlanLine(node: PlanNodeType): string {
  let line = "";
  if (node["Index Name"]) {
    line += ` using ${node["Index Name"]}`;
  }

  if (node["Relation Name"]) {
    line += ` on ${node["Relation Name"]}`;
    if (node["Alias"] && node["Alias"] !== node["Relation Name"]) {
      line += ` ${node["Alias"]}`;
    }
  }

  // Cost and rows
  const costStart = node["Startup Cost"] !== undefined ? node["Startup Cost"].toFixed(2) : "0.00";
  const costEnd = node["Total Cost"] !== undefined ? node["Total Cost"].toFixed(2) : "0.00";
  const rows = node["Plan Rows"] !== undefined ? node["Plan Rows"] : 0;
  const width = node["Plan Width"] !== undefined ? node["Plan Width"] : 0;

  line += `  (cost=${costStart}..${costEnd} rows=${rows} width=${width})`;

  // Actual time if available
  if (node["Actual Startup Time"] !== undefined && node["Actual Total Time"] !== undefined) {
    const actualStart = node["Actual Startup Time"].toFixed(3);
    const actualEnd = node["Actual Total Time"].toFixed(3);
    const actualRows = node["Actual Rows"] !== undefined ? node["Actual Rows"] : 0;
    const loops = node["Actual Loops"] !== undefined ? node["Actual Loops"] : 1;
    line += ` (actual time=${actualStart}..${actualEnd} rows=${actualRows} loops=${loops})`;
  }
  return line;
}
export function formatExplainPlanAsText(planJson: Plan | Plan[]): string {
  let output = [];

  const formatNode = (node: PlanNodeType, level = 0, isLast = true, prefix = "") => {
    let indent = level;

    let spaces = "  ".repeat(indent);
    let line = spaces;

    // Add tree structure characters
    if (indent > 0) {
      line = prefix + (isLast ? "└─ " : "├─ ");
    }

    if (node["Subplan Name"] !== undefined) {
      output.push(`${spaces}${node["Subplan Name"]}`);
    }

    // Node type and relation
    line += node["Node Type"];
    line += explainPlanLine(node);

    output.push(line);

    // Filter condition
    if (node["Filter"]) {
      const filterPrefix = prefix + (isLast ? "   " : "│  ");
      output.push(filterPrefix + `Filter: ${node["Filter"]}`);
      if (node["Rows Removed by Filter"] !== undefined) {
        output.push(filterPrefix + `Rows Removed by Filter: ${node["Rows Removed by Filter"]}`);
      }
    }

    // Index condition
    if (node["Index Cond"]) {
      const condPrefix = prefix + (isLast ? "   " : "│  ");
      output.push(condPrefix + `Index Cond: ${node["Index Cond"]}`);
    }

    // Hash condition
    if (node["Hash Cond"]) {
      const hashPrefix = prefix + (isLast ? "   " : "│  ");
      output.push(hashPrefix + `Hash Cond: ${node["Hash Cond"]}`);
    }

    // Join Filter
    if (node["Join Filter"]) {
      const joinPrefix = prefix + (isLast ? "   " : "│  ");
      output.push(joinPrefix + `Join Filter: ${node["Join Filter"]}`);
    }

    // Sort Key
    if (node["Sort Key"]) {
      const sortPrefix = prefix + (isLast ? "   " : "│  ");
      const sortKeys = Array.isArray(node["Sort Key"]) ? node["Sort Key"].join(", ") : node["Sort Key"];
      output.push(sortPrefix + `Sort Key: ${sortKeys}`);
    }

    // Process child plans
    if (node["Plans"] && node["Plans"].length > 0) {
      const childPrefix = prefix + (isLast ? "   " : "│  ");
      node["Plans"].forEach((child, idx) => {
        const childIsLast = idx === node["Plans"].length - 1;
        formatNode(child, indent + 1, childIsLast, childPrefix);
      });
    }
  };

  if (planJson && Array.isArray(planJson)) {
    planJson.forEach((plan) => {
      if (plan["Plan"]) {
        formatNode(plan["Plan"], 0);
      }
      if (plan["Planning Time"] !== undefined) {
        output.push(`Planning Time: ${plan["Planning Time"].toFixed(3)} ms`);
      }
      if (plan["Execution Time"] !== undefined) {
        output.push(`Execution Time: ${plan["Execution Time"].toFixed(3)} ms`);
      }
      output.push("");
    });
  }
  // Format the main plan
  if (planJson && planJson["Plan"]) {
    formatNode(planJson["Plan"], 0);

    // Add planning and execution time
    if (planJson["Planning Time"] !== undefined) {
      output.push(`Planning Time: ${planJson["Planning Time"].toFixed(3)} ms`);
    }
    if (planJson["Execution Time"] !== undefined) {
      output.push(`Execution Time: ${planJson["Execution Time"].toFixed(3)} ms`);
    }
  }
  return output.join("\n");
}
