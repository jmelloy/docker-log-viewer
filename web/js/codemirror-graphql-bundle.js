// CodeMirror 6 GraphQL Editor Bundle
// This file bundles CodeMirror 6 with GraphQL language support

import { EditorView, keymap, placeholder } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import { autocompletion, completionKeymap } from "@codemirror/autocomplete";
import { lintKeymap } from "@codemirror/lint";
import { graphql, updateSchema } from "cm6-graphql";
import { buildSchema } from "graphql";

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
  const {
    value = "",
    onChange = () => {},
    placeholder: placeholderText = "",
    schema = null,
  } = options;

  // Create extensions array
  const extensions = [
    graphql(),
    history(),
    autocompletion(),
    keymap.of([
      ...defaultKeymap,
      ...historyKeymap,
      ...completionKeymap,
      ...lintKeymap,
      indentWithTab,
    ]),
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
 * @param {Object|string} schema - GraphQL schema object or SDL string
 */
export function updateEditorSchema(view, schema) {
  try {
    let graphqlSchema;

    if (typeof schema === "string") {
      // If schema is a string, build it
      graphqlSchema = buildSchema(schema);
    } else {
      // Otherwise assume it's already a GraphQL schema object
      graphqlSchema = schema;
    }

    // Update the schema in the editor
    updateSchema(view, graphqlSchema);
  } catch (error) {
    console.error("Failed to update GraphQL schema:", error);
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
