export type KeyBundle = {
  passwordWrappedMasterKey: string;
  passwordKdfSalt: string;
  recoveryPhraseWrappedMasterKey: string;
  recoveryFileWrappedMasterKey: string;
};

export type RecoveryKit = {
  phrase: string;
  file: RecoveryFile;
};

export type RecoveryFile = {
  version: 1;
  kind: "frorage-recovery-file";
  secret: string;
  createdAt: string;
};

export type AccountCrypto = {
  masterKey: CryptoKey;
  keyBundle: KeyBundle;
  recoveryKit: RecoveryKit;
  passwordVerifier: string;
};

export type FileMetadata = {
  name: string;
  mimeType?: string;
  lastModified?: number;
};

export type FileRecord = {
  id: string;
  kind: "file" | "folder";
  parentId: string | null;
  encryptedMetadata: string;
  ciphertextSize: number;
  objectKey?: string | null;
  createdAt?: string;
  updatedAt?: string;
};

export type AuthResponse = {
  token: string;
  user: { id: string; email: string };
  keyBundle: KeyBundle;
};
