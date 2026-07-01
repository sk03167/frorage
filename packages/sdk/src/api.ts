import type { AdminUser, AuthResponse, DownloadedFile, FileMetadata, FileRecord } from "./types";

export type ClientOptions = {
  baseUrl: string;
  token?: string;
  adminToken?: string;
};

export class PrivateCloudClient {
  private token?: string;
  private adminToken?: string;

  constructor(private readonly options: ClientOptions) {
    this.token = options.token;
    this.adminToken = options.adminToken;
  }

  setToken(token: string): void {
    this.token = token;
  }

  setAdminToken(token: string): void {
    this.adminToken = token;
  }

  async signup(email: string, passwordVerifier: string): Promise<AuthResponse> {
    const response = await this.request<AuthResponse>("/v1/auth/signup", {
      method: "POST",
      body: JSON.stringify({ email, passwordVerifier }),
    });
    this.token = response.token;
    return response;
  }

  async login(email: string, passwordVerifier: string): Promise<AuthResponse> {
    const response = await this.request<AuthResponse>("/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, passwordVerifier }),
    });
    this.token = response.token;
    return response;
  }

  async forgotPassword(email: string): Promise<{ resetToken?: string }> {
    return this.request<{ resetToken?: string }>("/v1/auth/forgot-password", {
      method: "POST",
      body: JSON.stringify({ email }),
    });
  }

  async resetPassword(token: string, passwordVerifier: string): Promise<AuthResponse> {
    const response = await this.request<AuthResponse>("/v1/auth/reset-password", {
      method: "POST",
      body: JSON.stringify({ token, passwordVerifier }),
    });
    this.token = response.token;
    return response;
  }

  async listFiles(): Promise<FileRecord[]> {
    const response = await this.request<{ files: FileRecord[] }>("/v1/files");
    return response.files;
  }

  async createFolder(parentId: string | null, metadata: FileMetadata): Promise<FileRecord> {
    return this.request<FileRecord>("/v1/files", {
      method: "POST",
      body: JSON.stringify({
        parentId,
        name: metadata.name,
      }),
    });
  }

  async uploadFile(parentId: string | null, file: File): Promise<FileRecord> {
    const form = new FormData();
    form.set("file", file);
    if (parentId) form.set("parentId", parentId);
    form.set("name", file.name);
    form.set("mimeType", file.type);
    form.set("lastModified", String(file.lastModified));
    return this.request<FileRecord>("/v1/files/upload", {
      method: "POST",
      body: form,
    });
  }

  async uploadBytes(parentId: string | null, metadata: FileMetadata, plaintext: Uint8Array): Promise<FileRecord> {
    const file = new File([bytesToArrayBuffer(plaintext)], metadata.name, {
      type: metadata.mimeType,
      lastModified: metadata.lastModified,
    });
    return this.uploadFile(parentId, file);
  }

  async moveFile(fileId: string, parentId: string | null): Promise<FileRecord> {
    return this.request<FileRecord>(`/v1/files/${fileId}`, {
      method: "PATCH",
      body: JSON.stringify({ parentId }),
    });
  }

  async copyFile(file: FileRecord, parentId: string | null): Promise<FileRecord> {
    const download = await this.downloadFile(file);
    return this.uploadBytes(parentId, download.metadata, download.bytes);
  }

  async deleteFile(fileId: string): Promise<void> {
    await this.request<void>(`/v1/files/${fileId}`, { method: "DELETE" });
  }

  async downloadFile(file: FileRecord): Promise<DownloadedFile> {
    return this.fetchFileBytes(`/v1/files/${file.id}/download`, file, "POST");
  }

  async previewFile(file: FileRecord): Promise<DownloadedFile> {
    return this.fetchFileBytes(`/v1/files/${file.id}/preview`, file, "GET");
  }

  async adminUsers(email: string): Promise<AdminUser[]> {
    const response = await this.request<{ users: AdminUser[] }>(`/v1/admin/users?email=${encodeURIComponent(email)}`, {}, true);
    return response.users;
  }

  async adminFiles(userId: string): Promise<FileRecord[]> {
    const response = await this.request<{ files: FileRecord[] }>(`/v1/admin/users/${userId}/files`, {}, true);
    return response.files;
  }

  async adminPreviewFile(file: FileRecord): Promise<DownloadedFile> {
    return this.fetchFileBytes(`/v1/admin/files/${file.id}/preview`, file, "GET", true);
  }

  async adminDownloadFile(file: FileRecord): Promise<DownloadedFile> {
    return this.fetchFileBytes(`/v1/admin/files/${file.id}/download`, file, "POST", true);
  }

  async usage(): Promise<unknown> {
    return this.request<unknown>("/v1/billing/usage");
  }

  private async fetchFileBytes(path: string, file: FileRecord, method: string, admin = false): Promise<DownloadedFile> {
    const headers = new Headers();
    this.applyAuth(headers, admin);
    const response = await fetch(`${this.options.baseUrl}${path}`, { method, headers });
    if (!response.ok) {
      const error = await response.text();
      throw new Error(error || response.statusText);
    }
    return {
      metadata: {
        name: file.name,
        mimeType: file.mimeType || response.headers.get("Content-Type") || undefined,
        lastModified: file.lastModified,
      },
      bytes: new Uint8Array(await response.arrayBuffer()),
    };
  }

  private async request<T>(path: string, init: RequestInit = {}, admin = false): Promise<T> {
    const headers = new Headers(init.headers);
    if (!(init.body instanceof FormData)) {
      headers.set("Content-Type", "application/json");
    }
    this.applyAuth(headers, admin);
    const response = await fetch(`${this.options.baseUrl}${path}`, { ...init, headers });
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: response.statusText }));
      throw new Error(error.error ?? response.statusText);
    }
    if (response.status === 204) {
      return undefined as T;
    }
    const text = await response.text();
    if (text === "") {
      return undefined as T;
    }
    return JSON.parse(text) as T;
  }

  private applyAuth(headers: Headers, admin: boolean): void {
    const token = admin ? this.adminToken : this.token;
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
}

function bytesToArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(bytes.byteLength);
  copy.set(bytes);
  return copy.buffer;
}
