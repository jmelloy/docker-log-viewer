export const API = {
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

  async delete(url) {
    try {
      const response = await fetch(url, {
        method: 'DELETE',
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      const text = await response.text();
      return text ? JSON.parse(text) : null;
    } catch (error) {
      console.error(`DELETE ${url} failed:`, error);
      throw error;
    }
  }
};

export const Format = {
  date(date) {
    if (!date) return '';
    const d = typeof date === 'string' ? new Date(date) : date;
    return d.toLocaleString();
  },

  json(data, indent = 2) {
    try {
      const obj = typeof data === 'string' ? JSON.parse(data) : data;
      return JSON.stringify(obj, null, indent);
    } catch (e) {
      return String(data);
    }
  },

  duration(ms) {
    if (ms < 1000) {
      return `${ms.toFixed(2)}ms`;
    }
    return `${(ms / 1000).toFixed(2)}s`;
  }
};

export const Storage = {
  get(key, defaultValue = null) {
    try {
      const item = localStorage.getItem(key);
      return item ? JSON.parse(item) : defaultValue;
    } catch (e) {
      console.warn(`Failed to get ${key} from localStorage:`, e);
      return defaultValue;
    }
  },

  set(key, value) {
    try {
      localStorage.setItem(key, JSON.stringify(value));
    } catch (e) {
      console.warn(`Failed to set ${key} in localStorage:`, e);
    }
  },

  remove(key) {
    try {
      localStorage.removeItem(key);
    } catch (e) {
      console.warn(`Failed to remove ${key} from localStorage:`, e);
    }
  }
};
