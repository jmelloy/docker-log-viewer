// CodeMirror 6 GraphQL Editor Bundle
// This file bundles CodeMirror 6 with GraphQL language support

import { EditorView, keymap, placeholder } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { autocompletion, completionKeymap } from "@codemirror/autocomplete";
import { lintKeymap } from "@codemirror/lint";
import { syntaxHighlighting, HighlightStyle } from "@codemirror/language";
import { tags as t } from "@lezer/highlight";
import { graphql, updateSchema } from "cm6-graphql";
import { buildSchema, buildClientSchema } from "graphql";

// GitHub Dark theme highlighting (matching highlight.js)
const githubDarkHighlight = HighlightStyle.define([
  { tag: t.keyword, color: "#ff7b72" }, // keywords: query, mutation, fragment
  { tag: t.definitionKeyword, color: "#ff7b72" }, // type, interface, enum, etc.
  { tag: t.propertyName, color: "#7ee787" }, // field names
  { tag: t.variableName, color: "#ffa657" }, // $variables
  { tag: t.string, color: "#a5d6ff" }, // strings
  { tag: [t.number, t.integer, t.float], color: "#79c0ff" }, // numbers
  { tag: t.bool, color: "#79c0ff" }, // booleans
  { tag: t.null, color: "#79c0ff" }, // null
  { tag: t.typeName, color: "#d2a8ff" }, // type names
  { tag: t.special(t.variableName), color: "#d2a8ff" }, // enum values
  { tag: t.meta, color: "#d2a8ff" }, // directives
  { tag: t.attributeName, color: "#79c0ff" }, // argument names
  { tag: [t.comment, t.lineComment, t.blockComment], color: "#8b949e", fontStyle: "italic" }, // comments
  { tag: [t.punctuation, t.paren, t.brace, t.bracket], color: "#c9d1d9" }, // punctuation
  { tag: t.operator, color: "#ff7b72" }, // operators
]);

/**
 * Create a CodeMirror 6 editor with GraphQL support
 * @param {HTMLElement} parent - Parent element to attach the editor to
 * @param {Object} options - Configuration options
 * @param {string} options.value - Initial value
 * @param {Function} options.onChange - Callback when content changes
 * @param {string} options.placeholder - Placeholder text
 * @param {Object} options.schema - GraphQL schema object
 * @returns {EditorView} The CodeMirror editor view
 */
export function createGraphQLEditor(parent, options = {}) {
  const { value = "", onChange = () => {}, placeholder: placeholderText = "", schema = null } = options;

  // Create extensions array
  const extensions = [
    graphql(),
    syntaxHighlighting(githubDarkHighlight),
    history(),
    autocompletion(),
    keymap.of([...defaultKeymap, ...historyKeymap, ...completionKeymap, ...lintKeymap, indentWithTab]),
    EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        onChange(update.state.doc.toString());
      }
    }),
  ];

  if (placeholderText) {
    extensions.push(placeholder(placeholderText));
  }

  // Create editor state
  const state = EditorState.create({
    doc: value,
    extensions,
  });

  // Create editor view
  const view = new EditorView({
    state,
    parent,
  });

  // Update schema if provided
  if (schema) {
    updateEditorSchema(view, schema);
  }

  return view;
}

/**
 * Update the GraphQL schema in an existing editor
 * @param {EditorView} view - The CodeMirror editor view
 * @param {Object|string} schema - GraphQL schema object, introspection result, or SDL string
 */
export function updateEditorSchema(view, schema) {
  try {
    if (!view || !schema) {
      return;
    }

    let graphqlSchema;

    if (typeof schema === "string") {
      // If schema is a string, build it from SDL
      graphqlSchema = buildSchema(schema);
    } else if (schema.__schema || (schema.queryType && schema.types)) {
      // If it's an introspection result, build client schema
      const introspectionData = schema.__schema ? schema : { __schema: schema };
      graphqlSchema = buildClientSchema(introspectionData);
    } else {
      // Otherwise assume it's already a GraphQL schema object
      graphqlSchema = schema;
    }

    // Update the schema in the editor
    updateSchema(view, graphqlSchema);
  } catch (error) {
    console.error("Failed to update GraphQL schema:", error);
    throw error;
  }
}

/**
 * Get the current value from the editor
 * @param {EditorView} view - The CodeMirror editor view
 * @returns {string} The current editor content
 */
export function getEditorValue(view) {
  return view.state.doc.toString();
}

/**
 * Set the value in the editor
 * @param {EditorView} view - The CodeMirror editor view
 * @param {string} value - The new content
 */
export function setEditorValue(view, value) {
  view.dispatch({
    changes: {
      from: 0,
      to: view.state.doc.length,
      insert: value,
    },
  });
}

// Export individual components for advanced usage
export { EditorView, EditorState, graphql, updateSchema, buildSchema };
