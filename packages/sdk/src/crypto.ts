import { bytesToBase64Url, concatBytes, utf8 } from "./encoding";

const PASSWORD_KDF_ITERATIONS = 310_000;

export async function passwordVerifier(email: string, password: string): Promise<string> {
  const material = await deriveBits(password, utf8(`auth:${email.trim().toLowerCase()}`), 256);
  const digest = await crypto.subtle.digest("SHA-256", bytesToArrayBuffer(concatBytes(utf8("auth-verifier:v1"), new Uint8Array(material))));
  return bytesToBase64Url(new Uint8Array(digest));
}

async function deriveBits(password: string, salt: Uint8Array, bits: number): Promise<ArrayBuffer> {
  const key = await crypto.subtle.importKey("raw", bytesToArrayBuffer(utf8(password)), "PBKDF2", false, ["deriveBits"]);
  return crypto.subtle.deriveBits(
    {
      name: "PBKDF2",
      salt: bytesToArrayBuffer(salt),
      iterations: PASSWORD_KDF_ITERATIONS,
      hash: "SHA-256",
    },
    key,
    bits,
  );
}

function bytesToArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(bytes.byteLength);
  copy.set(bytes);
  return copy.buffer;
}
