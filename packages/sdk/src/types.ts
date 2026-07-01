export type FileMetadata = {
  name: string;
  mimeType?: string;
  lastModified?: number;
};

export type FileRecord = FileMetadata & {
  id: string;
  kind: "file" | "folder";
  parentId: string | null;
  encryptedMetadata?: string;
  ciphertextSize: number;
  objectKey?: string | null;
  createdAt?: string;
  updatedAt?: string;
};

export type AuthResponse = {
  token: string;
  user: { id: string; email: string };
};

export type AdminUser = {
  id: string;
  email: string;
  storagePrefix: string;
  quotaBytes: number;
  usedBytes: number;
  createdAt?: string;
};

export type DownloadedFile = {
  metadata: FileMetadata;
  bytes: Uint8Array;
};
