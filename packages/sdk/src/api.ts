import { decryptBytes, decryptMetadata, encryptBytes, encryptMetadata } from "./crypto";
import type { AuthResponse, FileMetadata, FileRecord, KeyBundle } from "./types";

export type ClientOptions = {
  baseUrl: string;
  token?: string;
};

export class PrivateCloudClient {
  private token?: string;

  constructor(private readonly options: ClientOptions) {
    this.token = options.token;
  }

  setToken(token: string): void {
    this.token = token;
  }

  async signup(email: string, passwordVerifier: string, keyBundle: KeyBundle): Promise<AuthResponse> {
    const response = await this.request<AuthResponse>("/v1/auth/signup", {
      method: "POST",
      body: JSON.stringify({ email, passwordVerifier, keyBundle }),
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

  async recover(email: string, passwordVerifier: string, keyBundle: KeyBundle): Promise<AuthResponse> {
    const response = await this.request<AuthResponse>("/v1/auth/recover", {
      method: "POST",
      body: JSON.stringify({ email, passwordVerifier, keyBundle }),
    });
    this.token = response.token;
    return response;
  }

  async listFiles(): Promise<FileRecord[]> {
    const response = await this.request<{ files: FileRecord[] }>("/v1/files");
    return response.files;
  }

  async createFolder(masterKey: CryptoKey, parentId: string | null, metadata: FileMetadata): Promise<FileRecord> {
    return this.request<FileRecord>("/v1/files", {
      method: "POST",
      body: JSON.stringify({
        parentId,
        encryptedMetadata: await encryptMetadata(masterKey, metadata),
      }),
    });
  }

  async uploadFile(masterKey: CryptoKey, parentId: string | null, file: File): Promise<FileRecord> {
    const plaintext = new Uint8Array(await file.arrayBuffer());
    const encrypted = await encryptBytes(masterKey, plaintext);
    const encryptedMetadata = await encryptMetadata(masterKey, {
      name: file.name,
      mimeType: file.type,
      lastModified: file.lastModified,
    });

    const session = await this.request<{ uploadId: string; uploadUrl: string }>("/v1/uploads/init", {
      method: "POST",
      body: JSON.stringify({
        parentId,
        encryptedMetadata,
        ciphertextSize: encrypted.byteLength,
      }),
    });

    const uploadResponse = await fetch(session.uploadUrl, {
      method: "PUT",
      body: encrypted,
    });
    if (!uploadResponse.ok) {
      throw new Error(`Object upload failed: ${uploadResponse.status}`);
    }

    return this.request<FileRecord>(`/v1/uploads/${session.uploadId}/commit`, { method: "POST" });
  }

  async downloadFile(masterKey: CryptoKey, file: FileRecord): Promise<{ metadata: FileMetadata; bytes: Uint8Array }> {
    const response = await this.request<{ downloadUrl: string }>(`/v1/files/${file.id}/download`, { method: "POST" });
    const objectResponse = await fetch(response.downloadUrl);
    if (!objectResponse.ok) {
      throw new Error(`Object download failed: ${objectResponse.status}`);
    }
    const encrypted = new Uint8Array(await objectResponse.arrayBuffer());
    return {
      metadata: await decryptMetadata(masterKey, file.encryptedMetadata),
      bytes: await decryptBytes(masterKey, encrypted),
    };
  }

  async usage(): Promise<unknown> {
    return this.request<unknown>("/v1/billing/usage");
  }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set("Content-Type", "application/json");
    if (this.token) headers.set("Authorization", `Bearer ${this.token}`);
    const response = await fetch(`${this.options.baseUrl}${path}`, { ...init, headers });
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: response.statusText }));
      throw new Error(error.error ?? response.statusText);
    }
    return response.json() as Promise<T>;
  }
}
