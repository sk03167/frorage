import { base64ToBytes, base64UrlToBytes, bytesToBase64, bytesToBase64Url, concatBytes, fromUtf8, utf8 } from "./encoding";
import type { AccountCrypto, FileMetadata, KeyBundle, RecoveryFile } from "./types";

const AES_GCM_IV_BYTES = 12;
const MASTER_KEY_BITS = 256;
const PASSWORD_KDF_ITERATIONS = 310_000;

export async function createAccountCrypto(email: string, password: string): Promise<AccountCrypto> {
  const masterKey = await crypto.subtle.generateKey({ name: "AES-GCM", length: MASTER_KEY_BITS }, true, ["encrypt", "decrypt"]);
  const passwordKdfSalt = randomBytes(16);
  const passwordKey = await derivePasswordKey(password, passwordKdfSalt);
  const recoveryPhraseSecret = randomBytes(32);
  const recoveryFileSecret = randomBytes(32);
  const recoveryFile: RecoveryFile = {
    version: 1,
    kind: "frorage-recovery-file",
    secret: bytesToBase64Url(recoveryFileSecret),
    createdAt: new Date().toISOString(),
  };
  const keyBundle: KeyBundle = {
    passwordWrappedMasterKey: await wrapKey(masterKey, passwordKey),
    passwordKdfSalt: bytesToBase64(passwordKdfSalt),
    recoveryPhraseWrappedMasterKey: await wrapKey(masterKey, await importRawAesKey(recoveryPhraseSecret)),
    recoveryFileWrappedMasterKey: await wrapKey(masterKey, await importRawAesKey(recoveryFileSecret)),
  };

  return {
    masterKey,
    keyBundle,
    recoveryKit: {
      phrase: recoveryPhraseFromSecret(recoveryPhraseSecret),
      file: recoveryFile,
    },
    passwordVerifier: await passwordVerifier(email, password),
  };
}

export async function unlockWithPassword(password: string, bundle: KeyBundle): Promise<CryptoKey> {
  const passwordKey = await derivePasswordKey(password, base64ToBytes(bundle.passwordKdfSalt));
  return unwrapKey(bundle.passwordWrappedMasterKey, passwordKey);
}

export async function unlockWithRecoveryPhrase(phrase: string, bundle: KeyBundle): Promise<CryptoKey> {
  return unwrapKey(bundle.recoveryPhraseWrappedMasterKey, await importRawAesKey(secretFromRecoveryPhrase(phrase)));
}

export async function unlockWithRecoveryFile(file: RecoveryFile, bundle: KeyBundle): Promise<CryptoKey> {
  if (file.version !== 1 || file.kind !== "frorage-recovery-file") {
    throw new Error("Invalid recovery file");
  }
  return unwrapKey(bundle.recoveryFileWrappedMasterKey, await importRawAesKey(base64UrlToBytes(file.secret)));
}

export async function rewrapAfterRecovery(email: string, masterKey: CryptoKey, newPassword: string, existingBundle: KeyBundle): Promise<{
  keyBundle: KeyBundle;
  passwordVerifier: string;
}> {
  const passwordKdfSalt = randomBytes(16);
  const passwordKey = await derivePasswordKey(newPassword, passwordKdfSalt);
  return {
    keyBundle: {
      ...existingBundle,
      passwordWrappedMasterKey: await wrapKey(masterKey, passwordKey),
      passwordKdfSalt: bytesToBase64(passwordKdfSalt),
    },
    passwordVerifier: await passwordVerifier(email, newPassword),
  };
}

export async function encryptMetadata(masterKey: CryptoKey, metadata: FileMetadata): Promise<string> {
  return encryptJSON(masterKey, metadata);
}

export async function decryptMetadata(masterKey: CryptoKey, encryptedMetadata: string): Promise<FileMetadata> {
  return decryptJSON<FileMetadata>(masterKey, encryptedMetadata);
}

export async function encryptBytes(masterKey: CryptoKey, plaintext: Uint8Array): Promise<Uint8Array> {
  const iv = randomBytes(AES_GCM_IV_BYTES);
  const ciphertext = new Uint8Array(
    await crypto.subtle.encrypt({ name: "AES-GCM", iv: bytesToArrayBuffer(iv) }, masterKey, bytesToArrayBuffer(plaintext)),
  );
  return concatBytes(iv, ciphertext);
}

export async function decryptBytes(masterKey: CryptoKey, payload: Uint8Array): Promise<Uint8Array> {
  const iv = payload.slice(0, AES_GCM_IV_BYTES);
  const ciphertext = payload.slice(AES_GCM_IV_BYTES);
  return new Uint8Array(
    await crypto.subtle.decrypt({ name: "AES-GCM", iv: bytesToArrayBuffer(iv) }, masterKey, bytesToArrayBuffer(ciphertext)),
  );
}

export async function passwordVerifier(email: string, password: string): Promise<string> {
  const material = await deriveBits(password, utf8(`auth:${email.trim().toLowerCase()}`), 256);
  const digest = await crypto.subtle.digest("SHA-256", bytesToArrayBuffer(concatBytes(utf8("auth-verifier:v1"), new Uint8Array(material))));
  return bytesToBase64Url(new Uint8Array(digest));
}

async function encryptJSON(key: CryptoKey, value: unknown): Promise<string> {
  return bytesToBase64(await encryptBytes(key, utf8(JSON.stringify(value))));
}

async function decryptJSON<T>(key: CryptoKey, payload: string): Promise<T> {
  return JSON.parse(fromUtf8(await decryptBytes(key, base64ToBytes(payload)))) as T;
}

async function wrapKey(masterKey: CryptoKey, wrappingKey: CryptoKey): Promise<string> {
  const raw = new Uint8Array(await crypto.subtle.exportKey("raw", masterKey));
  return bytesToBase64(await encryptBytes(wrappingKey, raw));
}

async function unwrapKey(wrapped: string, wrappingKey: CryptoKey): Promise<CryptoKey> {
  const raw = await decryptBytes(wrappingKey, base64ToBytes(wrapped));
  return importRawAesKey(raw);
}

async function derivePasswordKey(password: string, salt: Uint8Array): Promise<CryptoKey> {
  const bits = await deriveBits(password, salt, MASTER_KEY_BITS);
  return importRawAesKey(new Uint8Array(bits));
}

async function deriveBits(password: string, salt: Uint8Array, length: number): Promise<ArrayBuffer> {
  const keyMaterial = await crypto.subtle.importKey("raw", bytesToArrayBuffer(utf8(password)), "PBKDF2", false, ["deriveBits"]);
  return crypto.subtle.deriveBits(
    { name: "PBKDF2", salt: bytesToArrayBuffer(salt), iterations: PASSWORD_KDF_ITERATIONS, hash: "SHA-256" },
    keyMaterial,
    length,
  );
}

async function importRawAesKey(raw: Uint8Array): Promise<CryptoKey> {
  return crypto.subtle.importKey("raw", bytesToArrayBuffer(raw), { name: "AES-GCM", length: MASTER_KEY_BITS }, true, ["encrypt", "decrypt"]);
}

function bytesToArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(bytes.byteLength);
  copy.set(bytes);
  return copy.buffer;
}

function randomBytes(size: number): Uint8Array {
  const bytes = new Uint8Array(size);
  crypto.getRandomValues(bytes);
  return bytes;
}

function recoveryPhraseFromSecret(secret: Uint8Array): string {
  const token = bytesToBase64Url(secret);
  return token.match(/.{1,4}/g)?.join(" ") ?? token;
}

function secretFromRecoveryPhrase(phrase: string): Uint8Array {
  return base64UrlToBytes(phrase.replace(/\s/g, ""));
}
