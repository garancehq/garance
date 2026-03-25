interface GaranceClientOptions {
    url: string;
    headers?: Record<string, string>;
}
interface AuthTokens {
    access_token: string;
    refresh_token: string;
    expires_in: number;
    token_type: string;
}
interface User {
    id: string;
    email: string;
    email_verified: boolean;
    role: string;
    created_at: string;
    updated_at: string;
}
interface AuthResponse {
    user: User;
    token_pair: AuthTokens;
}
interface GaranceError {
    error: {
        code: string;
        message: string;
        status: number;
    };
}
type GaranceResult<T> = {
    data: T;
    error: null;
} | {
    data: null;
    error: GaranceError;
};

declare class HttpClient {
    private baseUrl;
    private headers;
    private accessToken;
    constructor(options: GaranceClientOptions);
    setAccessToken(token: string | null): void;
    getAccessToken(): string | null;
    private buildHeaders;
    get<T>(path: string, params?: Record<string, string>): Promise<GaranceResult<T>>;
    post<T>(path: string, body?: unknown): Promise<GaranceResult<T>>;
    patch<T>(path: string, body: unknown): Promise<GaranceResult<T>>;
    delete(path: string): Promise<GaranceResult<void>>;
    uploadFile(path: string, file: Blob | File, fileName?: string): Promise<GaranceResult<unknown>>;
    private request;
}

declare class AuthClient {
    private http;
    constructor(http: HttpClient);
    signUp(params: {
        email: string;
        password: string;
    }): Promise<GaranceResult<AuthResponse>>;
    signIn(params: {
        email: string;
        password: string;
    }): Promise<GaranceResult<AuthResponse>>;
    signInWithMagicLink(params: {
        email: string;
    }): Promise<GaranceResult<unknown>>;
    signInWithOAuth(params: {
        provider: string;
    }): Promise<GaranceResult<unknown>>;
    refreshToken(refreshToken: string): Promise<GaranceResult<AuthResponse>>;
    signOut(refreshToken: string): Promise<GaranceResult<unknown>>;
    getUser(): Promise<GaranceResult<User>>;
    deleteUser(): Promise<GaranceResult<void>>;
}

declare class QueryBuilder<T = unknown> {
    private _table;
    private _http;
    private _params;
    private _selectCols?;
    constructor(http: HttpClient, table: string);
    select(columns: string): this;
    eq(column: string, value: string | number | boolean): this;
    neq(column: string, value: string | number | boolean): this;
    gt(column: string, value: string | number): this;
    gte(column: string, value: string | number): this;
    lt(column: string, value: string | number): this;
    lte(column: string, value: string | number): this;
    order(column: string, direction?: 'asc' | 'desc'): this;
    limit(n: number): this;
    offset(n: number): this;
    /** Execute the query — returns array of rows */
    execute(): Promise<GaranceResult<T[]>>;
    /** Get a single row by ID */
    getById(id: string): Promise<GaranceResult<T>>;
    /** Insert a new row */
    insert(data: Partial<T>): Promise<GaranceResult<T>>;
    /** Update a row by ID */
    update(id: string, data: Partial<T>): Promise<GaranceResult<T>>;
    /** Delete a row by ID */
    deleteById(id: string): Promise<GaranceResult<void>>;
}

declare class StorageClient {
    private http;
    private baseUrl;
    constructor(http: HttpClient, baseUrl: string);
    from(bucket: string): BucketClient;
}
declare class BucketClient {
    private http;
    private baseUrl;
    private bucket;
    constructor(http: HttpClient, baseUrl: string, bucket: string);
    upload(path: string, file: Blob | File): Promise<GaranceResult<unknown>>;
    getPublicUrl(path: string, options?: {
        transform?: {
            width?: number;
            height?: number;
            format?: string;
        };
    }): string;
    createSignedUrl(path: string, options: {
        expiresIn: number;
    }): Promise<GaranceResult<{
        signed_url: string;
    }>>;
    list(options?: {
        prefix?: string;
        limit?: number;
        offset?: number;
    }): Promise<GaranceResult<unknown>>;
    remove(path: string): Promise<GaranceResult<void>>;
}

declare function createClient(options: GaranceClientOptions): {
    auth: AuthClient;
    from: <T = unknown>(table: string) => QueryBuilder<T>;
    storage: StorageClient;
};

export { AuthClient, type AuthResponse, type AuthTokens, type GaranceClientOptions, type GaranceError, type GaranceResult, HttpClient, QueryBuilder, StorageClient, type User, createClient };
