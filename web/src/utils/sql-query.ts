export function getSQLQuery(entry?: { message?: string; fields?: Record<string, unknown> }): string {
  const query = entry?.fields?.["sql.query"];
  if (typeof query === "string" && query) return query;
  const message = entry?.message || "";
  const index = message.indexOf("[sql]:");
  if (index >= 0) return message.slice(index + 6).trim();
  return entry?.fields?.type === "query" ? message : "";
}
