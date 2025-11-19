import js from "@eslint/js";

export default [
  {
    ignores: ["lib/**", "node_modules/**", "dist/**", "build/**", "bin/**", "*.min.js", "coverage/**"],
  },
  js.configs.recommended,
  {
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
      globals: {
        Vue: "readonly",
        console: "readonly",
        document: "readonly",
        window: "readonly",
        localStorage: "readonly",
        WebSocket: "readonly",
        fetch: "readonly",
        location: "readonly",
        navigator: "readonly",
        alert: "readonly",
        setTimeout: "readonly",
        clearTimeout: "readonly",
        setInterval: "readonly",
        clearInterval: "readonly",
        URLSearchParams: "readonly",
        prompt: "readonly",
        confirm: "readonly",
        hljs: "readonly",
        customElements: "readonly",
        matchMedia: "readonly",
        IntersectionObserver: "readonly",
        MutationObserver: "readonly",
        Element: "readonly",
      },
    },
    rules: {
      "no-unused-vars": [
        "warn",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
        },
      ],
      "no-console": "off",
      "no-control-regex": "off", // Allow ANSI color codes in regex
    },
  },
];
