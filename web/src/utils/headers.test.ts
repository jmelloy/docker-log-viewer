import assert from "node:assert/strict";
import { test } from "node:test";
import { parseHeaders, serverHeaders } from "./headers.ts";
import type { Server } from "../types/index.ts";

test("custom header JSON accepts only objects of strings", () => {
  assert.deepEqual(parseHeaders(""), {});
  assert.deepEqual(parseHeaders('{"X-User-ID":"user-1","X-Empty":""}'), {
    "X-User-ID": "user-1",
    "X-Empty": "",
  });
  for (const input of ["null", "[]", "42", '"header"', '{"X-Test":1}', "bad json"]) {
    assert.throws(() => parseHeaders(input), /JSON object with string values/);
  }
});

test("server editing converts legacy settings and preserves custom overrides", () => {
  const server: Server = {
    id: 1,
    name: "server",
    url: "http://localhost/graphql",
    createdAt: "",
    updatedAt: "",
    devId: "legacy-user",
    experimentalMode: "legacy-mode",
    headers: { "x-gluedev-userid": "custom-user", "X-Other": "kept" },
  };
  assert.deepEqual(serverHeaders(server), {
    "x-gluedev-userid": "custom-user",
    "X-Glue-Experimental-Mode": "legacy-mode",
    "X-Other": "kept",
  });
  assert.equal(server.devId, "legacy-user");
  assert.deepEqual(server.headers, { "x-gluedev-userid": "custom-user", "X-Other": "kept" });
});
