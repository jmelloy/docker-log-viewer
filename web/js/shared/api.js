// Shared API utility functions for making HTTP requests

/**
 * API client for making HTTP requests
 */
export const API = {
  /**
   * Make a GET request
   * @param {string} url - The URL to fetch
   * @returns {Promise<any>} The response data
   * @throws {Error} If the request fails
   */
  async get(url) {
    try {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      return await response.json();
    } catch (error) {
      console.error(`GET ${url} failed:`, error);
      throw error;
    }
  },

  /**
   * Make a POST request
   * @param {string} url - The URL to post to
   * @param {Object} data - The data to send
   * @returns {Promise<any>} The response data
   * @throws {Error} If the request fails
   */
  async post(url, data) {
    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      return await response.json();
    } catch (error) {
      console.error(`POST ${url} failed:`, error);
      throw error;
    }
  },

  /**
   * Make a PUT request
   * @param {string} url - The URL to put to
   * @param {Object} data - The data to send
   * @returns {Promise<any>} The response data
   * @throws {Error} If the request fails
   */
  async put(url, data) {
    try {
      const response = await fetch(url, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      return await response.json();
    } catch (error) {
      console.error(`PUT ${url} failed:`, error);
      throw error;
    }
  },

  /**
   * Make a DELETE request
   * @param {string} url - The URL to delete
   * @returns {Promise<any>} The response data
   * @throws {Error} If the request fails
   */
  async delete(url) {
    try {
      const response = await fetch(url, {
        method: 'DELETE',
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      // Some DELETE responses may not have a body
      const text = await response.text();
      return text ? JSON.parse(text) : null;
    } catch (error) {
      console.error(`DELETE ${url} failed:`, error);
      throw error;
    }
  }
};

/**
 * Format utilities
 */
export const Format = {
  /**
   * Format a date to a localized string
   * @param {Date|string} date - The date to format
   * @returns {string} Formatted date string
   */
  date(date) {
    if (!date) return '';
    const d = typeof date === 'string' ? new Date(date) : date;
    return d.toLocaleString();
  },

  /**
   * Format SQL query for display (already exists in utils.js)
   * This is a reference - the actual implementation is in utils.js
   */
  sql(query) {
    // This function exists in utils.js and should be imported from there
    return formatSQL(query);
  },

  /**
   * Format JSON for display
   * @param {Object|string} data - The data to format
   * @param {number} indent - Number of spaces for indentation
   * @returns {string} Formatted JSON string
   */
  json(data, indent = 2) {
    try {
      const obj = typeof data === 'string' ? JSON.parse(data) : data;
      return JSON.stringify(obj, null, indent);
    } catch (e) {
      return String(data);
    }
  },

  /**
   * Format duration in milliseconds
   * @param {number} ms - Duration in milliseconds
   * @returns {string} Formatted duration
   */
  duration(ms) {
    if (ms < 1000) {
      return `${ms.toFixed(2)}ms`;
    }
    return `${(ms / 1000).toFixed(2)}s`;
  }
};

/**
 * Storage utilities for localStorage
 */
export const Storage = {
  /**
   * Get an item from localStorage
   * @param {string} key - The key to retrieve
   * @param {any} defaultValue - Default value if key doesn't exist
   * @returns {any} The stored value or default
   */
  get(key, defaultValue = null) {
    try {
      const item = localStorage.getItem(key);
      return item ? JSON.parse(item) : defaultValue;
    } catch (e) {
      console.warn(`Failed to get ${key} from localStorage:`, e);
      return defaultValue;
    }
  },

  /**
   * Set an item in localStorage
   * @param {string} key - The key to store
   * @param {any} value - The value to store
   */
  set(key, value) {
    try {
      localStorage.setItem(key, JSON.stringify(value));
    } catch (e) {
      console.warn(`Failed to set ${key} in localStorage:`, e);
    }
  },

  /**
   * Remove an item from localStorage
   * @param {string} key - The key to remove
   */
  remove(key) {
    try {
      localStorage.removeItem(key);
    } catch (e) {
      console.warn(`Failed to remove ${key} from localStorage:`, e);
    }
  }
};
