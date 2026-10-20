import type { Server } from "@/types";

export function parseHeaders(text: string): Record<string, string> {
  let headers: unknown;
  try {
    headers = JSON.parse(text.trim() || "{}");
  } catch {
    throw new Error("Custom headers must be a JSON object with string values.");
  }
  if (
    !headers ||
    Array.isArray(headers) ||
    typeof headers !== "object" ||
    Object.values(headers).some((value) => typeof value !== "string")
  ) {
    throw new Error("Custom headers must be a JSON object with string values.");
  }
  return headers as Record<string, string>;
}

export function serverHeaders(server: Server): Record<string, string> {
  const headers: Record<string, string> = {};
  if (server.devId) headers["X-Gluedev-Userid"] = server.devId;
  if (server.experimentalMode) headers["X-Glue-Experimental-Mode"] = server.experimentalMode;
  for (const [name, value] of Object.entries(server.headers || {})) {
    const oldName = Object.keys(headers).find((key) => key.toLowerCase() === name.toLowerCase());
    if (oldName) delete headers[oldName];
    headers[name] = value;
  }
  return headers;
}
