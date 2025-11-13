/**
 * GraphQL Editor Manager
 * Manages CodeMirror 6 GraphQL editors for the GraphQL Explorer
 */

import { createGraphQLEditor, updateEditorSchema, getEditorValue, setEditorValue } from "./codemirror-graphql";
import type { EditorView } from "@codemirror/view";

export class GraphQLEditorManager {
  queryEditor: EditorView | null = null;
  variablesEditor: EditorView | null = null;

  /**
   * Initialize the query editor
   * @param container - Container element for the editor
   * @param onChange - Callback when content changes
   * @param initialValue - Initial editor value
   */
  initQueryEditor(container: HTMLElement, onChange: (value: string) => void, initialValue = "") {
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
   * @param container - Container element for the editor
   * @param onChange - Callback when content changes
   * @param initialValue - Initial editor value
   */
  initVariablesEditor(container: HTMLElement, onChange: (value: string) => void, initialValue = "{}") {
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
   * @param schema - GraphQL schema object
   */
  updateSchema(schema: any) {
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
  setQueryValue(value: string) {
    if (this.queryEditor) {
      setEditorValue(this.queryEditor, value);
    }
  }

  /**
   * Set the variables value
   */
  setVariablesValue(value: string) {
    if (this.variablesEditor) {
      setEditorValue(this.variablesEditor, value);
    }
  }

  /**
   * Insert text at the current cursor position
   * @param text - Text to insert
   */
  insertTextAtCursor(text: string) {
    if (!this.queryEditor) return;

    const view = this.queryEditor;
    const pos = view.state.selection.main.head;
    view.dispatch({
      changes: { from: pos, insert: text },
      selection: { anchor: pos + text.length },
    });
    view.focus();
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
