export const API = {
  async get<T = any>(url: string): Promise<T> {
    try {
      const response: Response = await fetch(url);
      if (!response.ok) {
        const contentType = response.headers.get("content-type");
        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
        if (contentType && contentType.includes("application/json")) {
          try {
            const errorData = await response.json();
            errorMessage = errorData.message || errorData.error || errorMessage;
          } catch {
            // Fall back to status text if JSON parsing fails
          }
        } else {
          try {
            const text = await response.text();
            if (text) errorMessage = text;
          } catch {
            // Fall back to status text if text parsing fails
          }
        }
        throw new Error(errorMessage);
      }
      // Try to parse as JSON, but handle gracefully if it fails
      try {
        const text = await response.text();
        if (!text) {
          return null as T;
        }
        return JSON.parse(text) as T;
      } catch (parseError) {
        if (parseError instanceof SyntaxError) {
          throw new Error(`Invalid JSON response from ${url}: ${parseError.message}`);
        }
        throw parseError;
      }
    } catch (error) {
      console.error(`GET ${url} failed:`, error);
      throw error;
    }
  },

  async post<T = any>(url: string, data: unknown): Promise<T> {
    try {
      const response: Response = await fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
      });
      if (!response.ok) {
        const contentType = response.headers.get("content-type");
        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
        if (contentType && contentType.includes("application/json")) {
          try {
            const errorData = await response.json();
            errorMessage = errorData.message || errorData.error || errorMessage;
          } catch {
            // Fall back to status text if JSON parsing fails
          }
        } else {
          try {
            const text = await response.text();
            if (text) errorMessage = text;
          } catch {
            // Fall back to status text if text parsing fails
          }
        }
        throw new Error(errorMessage);
      }
      // Try to parse as JSON, but handle gracefully if it fails
      try {
        const text = await response.text();
        if (!text) {
          return null as T;
        }
        return JSON.parse(text) as T;
      } catch (parseError) {
        if (parseError instanceof SyntaxError) {
          throw new Error(`Invalid JSON response from ${url}: ${parseError.message}`);
        }
        throw parseError;
      }
    } catch (error) {
      console.error(`POST ${url} failed:`, error);
      throw error;
    }
  },

  async put<T = any>(url: string, data: unknown): Promise<T> {
    try {
      const response: Response = await fetch(url, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
      });
      if (!response.ok) {
        const contentType = response.headers.get("content-type");
        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
        if (contentType && contentType.includes("application/json")) {
          try {
            const errorData = await response.json();
            errorMessage = errorData.message || errorData.error || errorMessage;
          } catch {
            // Fall back to status text if JSON parsing fails
          }
        } else {
          try {
            const text = await response.text();
            if (text) errorMessage = text;
          } catch {
            // Fall back to status text if text parsing fails
          }
        }
        throw new Error(errorMessage);
      }
      // Try to parse as JSON, but handle gracefully if it fails
      try {
        const text = await response.text();
        if (!text) {
          return null as T;
        }
        return JSON.parse(text) as T;
      } catch (parseError) {
        if (parseError instanceof SyntaxError) {
          throw new Error(`Invalid JSON response from ${url}: ${parseError.message}`);
        }
        throw parseError;
      }
    } catch (error) {
      console.error(`PUT ${url} failed:`, error);
      throw error;
    }
  },

  async delete<T = any>(url: string): Promise<T | null> {
    try {
      const response: Response = await fetch(url, {
        method: "DELETE",
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      const text = await response.text();
      return text ? (JSON.parse(text) as T) : null;
    } catch (error) {
      console.error(`DELETE ${url} failed:`, error);
      throw error;
    }
  },
};

export const Format = {
  date(date: string | Date | null | undefined): string {
    if (!date) return "";
    const d = typeof date === "string" ? new Date(date) : date;
    return d.toLocaleString();
  },

  json(data: any, indent = 2): string {
    try {
      const obj = typeof data === "string" ? JSON.parse(data) : data;
      return JSON.stringify(obj, null, indent);
    } catch (e) {
      return String(data);
    }
  },

  duration(ms: number): string {
    if (ms < 1000) {
      return `${ms.toFixed(2)}ms`;
    }
    return `${(ms / 1000).toFixed(2)}s`;
  },
};

export const Storage = {
  get<T = any>(key: string, defaultValue: T | null = null): T | null {
    try {
      const item = localStorage.getItem(key);
      return item ? JSON.parse(item) : defaultValue;
    } catch (e) {
      console.warn(`Failed to get ${key} from localStorage:`, e);
      return defaultValue;
    }
  },

  set(key: string, value: any): void {
    try {
      localStorage.setItem(key, JSON.stringify(value));
    } catch (e) {
      console.warn(`Failed to set ${key} in localStorage:`, e);
    }
  },

  remove(key: string): void {
    try {
      localStorage.removeItem(key);
    } catch (e) {
      console.warn(`Failed to remove ${key} from localStorage:`, e);
    }
  },
};
