import hljs from "highlight.js/lib/core";
import sql from "highlight.js/lib/languages/sql";
import json from "highlight.js/lib/languages/json";
import graphql from "highlight.js/lib/languages/graphql";

hljs.registerLanguage("sql", sql);
hljs.registerLanguage("json", json);
hljs.registerLanguage("graphql", graphql);

// Export as global
globalThis.hljs = hljs;
