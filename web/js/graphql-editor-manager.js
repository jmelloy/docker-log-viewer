/**
 * GraphQL Editor Manager
 * Manages CodeMirror 6 GraphQL editors for the GraphQL Explorer
 */

import {
  createGraphQLEditor,
  updateEditorSchema,
  getEditorValue,
  setEditorValue,
} from "../lib/codemirror-graphql.js";

export class GraphQLEditorManager {
  constructor() {
    this.queryEditor = null;
    this.variablesEditor = null;
  }

  /**
   * Initialize the query editor
   * @param {HTMLElement} container - Container element for the editor
   * @param {Function} onChange - Callback when content changes
   * @param {string} initialValue - Initial editor value
   */
  initQueryEditor(container, onChange, initialValue = "") {
    if (this.queryEditor) {
      this.queryEditor.destroy();
    }

    this.queryEditor = createGraphQLEditor(container, {
      value: initialValue,
      onChange,
      placeholder: "Enter your GraphQL query here...",
    });

    return this.queryEditor;
  }

  /**
   * Initialize the variables editor (JSON editor)
   * @param {HTMLElement} container - Container element for the editor
   * @param {Function} onChange - Callback when content changes
   * @param {string} initialValue - Initial editor value
   */
  initVariablesEditor(container, onChange, initialValue = "{}") {
    if (this.variablesEditor) {
      this.variablesEditor.destroy();
    }

    // For variables, we'll use a plain JSON editor without GraphQL syntax
    // We can create a simpler editor or reuse the GraphQL one
    this.variablesEditor = createGraphQLEditor(container, {
      value: initialValue,
      onChange,
      placeholder: '{"key": "value"}',
    });

    return this.variablesEditor;
  }

  /**
   * Update the GraphQL schema for autocomplete
   * @param {Object} schema - GraphQL schema object
   */
  updateSchema(schema) {
    console.log("GraphQLEditorManager.updateSchema", { 
      hasQueryEditor: !!this.queryEditor,
      queryEditorType: this.queryEditor?.constructor?.name,
      schema 
    });
    if (this.queryEditor && schema) {
      updateEditorSchema(this.queryEditor, schema);
    }
  }

  /**
   * Get the current query value
   */
  getQueryValue() {
    return this.queryEditor ? getEditorValue(this.queryEditor) : "";
  }

  /**
   * Get the current variables value
   */
  getVariablesValue() {
    return this.variablesEditor ? getEditorValue(this.variablesEditor) : "{}";
  }

  /**
   * Set the query value
   */
  setQueryValue(value) {
    if (this.queryEditor) {
      setEditorValue(this.queryEditor, value);
    }
  }

  /**
   * Set the variables value
   */
  setVariablesValue(value) {
    if (this.variablesEditor) {
      setEditorValue(this.variablesEditor, value);
    }
  }

  /**
   * Destroy all editors
   */
  destroy() {
    if (this.queryEditor) {
      this.queryEditor.destroy();
      this.queryEditor = null;
    }
    if (this.variablesEditor) {
      this.variablesEditor.destroy();
      this.variablesEditor = null;
    }
  }
}
