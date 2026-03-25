// src/client.ts
var HttpClient = class {
  constructor(options) {
    this.accessToken = null;
    this.baseUrl = options.url.replace(/\/$/, "");
    this.headers = options.headers ?? {};
  }
  setAccessToken(token) {
    this.accessToken = token;
  }
  getAccessToken() {
    return this.accessToken;
  }
  buildHeaders(extra) {
    const headers = { ...this.headers, ...extra };
    if (this.accessToken) {
      headers["Authorization"] = `Bearer ${this.accessToken}`;
    }
    return headers;
  }
  async get(path, params) {
    const url = new URL(`${this.baseUrl}${path}`);
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        url.searchParams.set(k, v);
      }
    }
    return this.request(url.toString(), { method: "GET" });
  }
  async post(path, body) {
    return this.request(`${this.baseUrl}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : void 0
    });
  }
  async patch(path, body) {
    return this.request(`${this.baseUrl}${path}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
  }
  async delete(path) {
    return this.request(`${this.baseUrl}${path}`, { method: "DELETE" });
  }
  async uploadFile(path, file, fileName) {
    const formData = new FormData();
    formData.append("file", file, fileName);
    return this.request(`${this.baseUrl}${path}`, { method: "POST", body: formData });
  }
  async request(url, init) {
    const headers = this.buildHeaders(init.headers);
    try {
      const response = await fetch(url, { ...init, headers });
      if (response.status === 204) {
        return { data: void 0, error: null };
      }
      const body = await response.json();
      if (!response.ok) {
        return { data: null, error: body };
      }
      return { data: body, error: null };
    } catch (err) {
      return { data: null, error: { error: { code: "NETWORK_ERROR", message: String(err), status: 0 } } };
    }
  }
};

// src/auth.ts
var AuthClient = class {
  constructor(http) {
    this.http = http;
  }
  async signUp(params) {
    const result = await this.http.post("/auth/v1/signup", params);
    if (result.data?.token_pair) {
      this.http.setAccessToken(result.data.token_pair.access_token);
    }
    return result;
  }
  async signIn(params) {
    const result = await this.http.post("/auth/v1/signin", params);
    if (result.data?.token_pair) {
      this.http.setAccessToken(result.data.token_pair.access_token);
    }
    return result;
  }
  async signInWithMagicLink(params) {
    return this.http.post("/auth/v1/magic-link", params);
  }
  async signInWithOAuth(params) {
    return this.http.post("/auth/v1/oauth", params);
  }
  async refreshToken(refreshToken) {
    const result = await this.http.post("/auth/v1/token/refresh", { refresh_token: refreshToken });
    if (result.data?.token_pair) {
      this.http.setAccessToken(result.data.token_pair.access_token);
    }
    return result;
  }
  async signOut(refreshToken) {
    this.http.setAccessToken(null);
    return this.http.post("/auth/v1/signout", { refresh_token: refreshToken });
  }
  async getUser() {
    return this.http.get("/auth/v1/user");
  }
  async deleteUser() {
    return this.http.delete("/auth/v1/user");
  }
};

// src/data.ts
var QueryBuilder = class {
  constructor(http, table) {
    this._params = {};
    this._http = http;
    this._table = table;
  }
  select(columns) {
    this._selectCols = columns;
    return this;
  }
  eq(column, value) {
    this._params[column] = `eq.${value}`;
    return this;
  }
  neq(column, value) {
    this._params[column] = `neq.${value}`;
    return this;
  }
  gt(column, value) {
    this._params[column] = `gt.${value}`;
    return this;
  }
  gte(column, value) {
    this._params[column] = `gte.${value}`;
    return this;
  }
  lt(column, value) {
    this._params[column] = `lt.${value}`;
    return this;
  }
  lte(column, value) {
    this._params[column] = `lte.${value}`;
    return this;
  }
  order(column, direction = "asc") {
    this._params["order"] = `${column}.${direction}`;
    return this;
  }
  limit(n) {
    this._params["limit"] = String(n);
    return this;
  }
  offset(n) {
    this._params["offset"] = String(n);
    return this;
  }
  /** Execute the query — returns array of rows */
  async execute() {
    const params = { ...this._params };
    if (this._selectCols) params["select"] = this._selectCols;
    return this._http.get(`/api/v1/${this._table}`, params);
  }
  /** Get a single row by ID */
  async getById(id) {
    return this._http.get(`/api/v1/${this._table}/${id}`);
  }
  /** Insert a new row */
  async insert(data) {
    return this._http.post(`/api/v1/${this._table}`, data);
  }
  /** Update a row by ID */
  async update(id, data) {
    return this._http.patch(`/api/v1/${this._table}/${id}`, data);
  }
  /** Delete a row by ID */
  async deleteById(id) {
    return this._http.delete(`/api/v1/${this._table}/${id}`);
  }
};

// src/storage.ts
var StorageClient = class {
  constructor(http, baseUrl) {
    this.http = http;
    this.baseUrl = baseUrl;
  }
  from(bucket) {
    return new BucketClient(this.http, this.baseUrl, bucket);
  }
};
var BucketClient = class {
  constructor(http, baseUrl, bucket) {
    this.http = http;
    this.baseUrl = baseUrl;
    this.bucket = bucket;
  }
  async upload(path, file) {
    return this.http.uploadFile(`/storage/v1/${this.bucket}/upload`, file, path);
  }
  getPublicUrl(path, options) {
    let url = `${this.baseUrl}/storage/v1/${this.bucket}/${path}`;
    if (options?.transform) {
      const params = new URLSearchParams();
      if (options.transform.width) params.set("w", String(options.transform.width));
      if (options.transform.height) params.set("h", String(options.transform.height));
      if (options.transform.format) params.set("f", options.transform.format);
      url += `?${params.toString()}`;
    }
    return url;
  }
  async createSignedUrl(path, options) {
    return this.http.post(`/storage/v1/${this.bucket}/signed-url`, {
      file_name: path,
      expires_in: options.expiresIn
    });
  }
  async list(options) {
    const params = {};
    if (options?.prefix) params["prefix"] = options.prefix;
    if (options?.limit) params["limit"] = String(options.limit);
    if (options?.offset) params["offset"] = String(options.offset);
    return this.http.get(`/storage/v1/${this.bucket}`, params);
  }
  async remove(path) {
    return this.http.delete(`/storage/v1/${this.bucket}/${path}`);
  }
};

// src/index.ts
function createClient(options) {
  const http = new HttpClient(options);
  return {
    auth: new AuthClient(http),
    from: (table) => new QueryBuilder(http, table),
    storage: new StorageClient(http, options.url.replace(/\/$/, ""))
  };
}
export {
  AuthClient,
  HttpClient,
  QueryBuilder,
  StorageClient,
  createClient
};
//# sourceMappingURL=index.js.map