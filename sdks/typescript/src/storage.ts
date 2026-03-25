import type { HttpClient } from './client'

export class StorageClient {
  constructor(private http: HttpClient, private baseUrl: string) {}

  from(bucket: string) {
    return new BucketClient(this.http, this.baseUrl, bucket)
  }
}

class BucketClient {
  constructor(
    private http: HttpClient,
    private baseUrl: string,
    private bucket: string,
  ) {}

  async upload(path: string, file: Blob | File) {
    return this.http.uploadFile(`/storage/v1/${this.bucket}/upload`, file, path)
  }

  getPublicUrl(path: string, options?: { transform?: { width?: number; height?: number; format?: string } }) {
    let url = `${this.baseUrl}/storage/v1/${this.bucket}/${path}`
    if (options?.transform) {
      const params = new URLSearchParams()
      if (options.transform.width) params.set('w', String(options.transform.width))
      if (options.transform.height) params.set('h', String(options.transform.height))
      if (options.transform.format) params.set('f', options.transform.format)
      url += `?${params.toString()}`
    }
    return url
  }

  async createSignedUrl(path: string, options: { expiresIn: number }) {
    return this.http.post<{ signed_url: string }>(`/storage/v1/${this.bucket}/signed-url`, {
      file_name: path,
      expires_in: options.expiresIn,
    })
  }

  async list(options?: { prefix?: string; limit?: number; offset?: number }) {
    const params: Record<string, string> = {}
    if (options?.prefix) params['prefix'] = options.prefix
    if (options?.limit) params['limit'] = String(options.limit)
    if (options?.offset) params['offset'] = String(options.offset)
    return this.http.get(`/storage/v1/${this.bucket}`, params)
  }

  async remove(path: string) {
    return this.http.delete(`/storage/v1/${this.bucket}/${path}`)
  }
}
