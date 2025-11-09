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

  formatted = formatted.replaceAll(
    new RegExp(`\\b(LEFT JOIN|RIGHT JOIN|INNER JOIN|FULL JOIN|JOIN)\\b`, "gi"),
    `\n$1`
  );

  formatted = formatted.replaceAll(
    new RegExp(`\\b(DELETE FROM|FROM)\\b`, "gi"),
    `\n$1`
  );

  keywords.forEach((kw) => {
    formatted = formatted.replace(
      new RegExp(`\\b${kw}\\b`, "gi"),
      `\n${kw.toUpperCase()}`
    );
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
            result +=
              "\n" +
              "  ".repeat(indentStack.length + 1) +
              match[2].toUpperCase();
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
