import assert from "node:assert/strict";
import { test } from "node:test";
import { getSQLQuery } from "./sql-query.ts";

test("SQL fields take precedence without changing the message", () => {
  const entry = { message: "sql query done", fields: { "sql.query": "SELECT 1\nFROM users" } };
  assert.equal(getSQLQuery(entry), "SELECT 1\nFROM users");
  assert.equal(entry.message, "sql query done");
  assert.equal(getSQLQuery({ ...entry, message: "[sql]: SELECT 0" }), "SELECT 1\nFROM users");
  assert.equal(getSQLQuery({ fields: entry.fields }), "SELECT 1\nFROM users");
});

test("legacy SQL and query-type messages still work", () => {
  assert.equal(getSQLQuery({ message: "[sql]: SELECT 1\nFROM users" }), "SELECT 1\nFROM users");
  assert.equal(getSQLQuery({ message: "SELECT 2", fields: { type: "query" } }), "SELECT 2");
  assert.equal(getSQLQuery({ message: "[sql]: SELECT 3", fields: { "sql.query": "" } }), "SELECT 3");
  assert.equal(getSQLQuery({ message: "ordinary log" }), "");
  assert.equal(getSQLQuery(), "");
});
